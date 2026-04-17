package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/task"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// pickAgent selects the agent for a task type. After the cleanup pass the
// only strategy left is capability-match: the first healthy agent declaring
// the required task type wins. Market/auction-based routing was removed with
// the enterprise cleanup; BMAD's role-driven dispatch replaces it.
func (e *Engine) pickAgent(ctx context.Context, _, taskType, _ string) (string, string, error) {
	return e.taskRouter.FindCapableAgent(ctx, taskType)
}

// depsKey stringifies a task's dependency set so sibling tasks can be grouped.
func depsKey(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	sorted := append([]string{}, deps...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// AgentLookup resolves an agent ID to its stored spec (type + config).
// The workflow engine calls this on every task dispatch so it can build the
// right adapter variant (HTTP, Claude Code, CrewAI, etc.) instead of
// assuming HTTP everywhere. The lookup is intentionally a callback rather
// than a direct *agent.Manager reference to keep the engine free of a
// storage import cycle.
type AgentLookup func(ctx context.Context, agentID string) (adapter.AgentSpec, error)

// Engine orchestrates workflow execution: creates tasks, routes to agents, executes in DAG order.
type Engine struct {
	workflowStore *Store
	taskStore     *task.Store
	taskRouter    *task.Router
	eventBus      *event.Bus
	adapters      map[string]adapter.Adapter // agentID -> adapter
	agentConfigs  map[string]string          // agentID -> baseURL (legacy HTTP shortcut)
	lookupAgent   AgentLookup                // optional — if set, used to build non-HTTP adapters
	concurrency   int                        // per-workflow level concurrency cap
	allocation    string                     // per-workflow allocation strategy
	retry         *adapter.RetryPolicy       // default retry for auto-built HTTP adapters
	mu            sync.Mutex
}

// WithAgentLookup wires the agent resolver so the engine can dispatch to
// non-HTTP adapter types (claude-code, crewai, autogen, langchain, mcp,
// openai). Without it, the engine degrades to HTTP-only — fine for
// single-adapter deployments but wrong for mixed fleets.
func (e *Engine) WithAgentLookup(l AgentLookup) *Engine {
	e.lookupAgent = l
	return e
}

// WithRetry installs a default retry policy applied to auto-built HTTP adapters.
// Story 5.5.
func (e *Engine) WithRetry(p *adapter.RetryPolicy) *Engine {
	e.retry = p
	return e
}

// NewEngine creates a workflow execution engine.
func NewEngine(ws *Store, ts *task.Store, tr *task.Router, eb *event.Bus) *Engine {
	return &Engine{
		workflowStore: ws,
		taskStore:     ts,
		taskRouter:    tr,
		eventBus:      eb,
		adapters:      make(map[string]adapter.Adapter),
		agentConfigs:  make(map[string]string),
	}
}

// RegisterAdapter makes an adapter available for task invocation.
func (e *Engine) RegisterAdapter(agentID, baseURL string, a adapter.Adapter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.adapters[agentID] = a
	e.agentConfigs[agentID] = baseURL
}

// RunResult holds the outcome of a workflow execution.
type RunResult struct {
	WorkflowID  string
	TaskResults map[string]*task.Task // taskName -> completed task
	Status      string                // "completed" or "failed"
	Error       string
	mu          sync.Mutex
}

// Run executes a workflow end-to-end following DAG order with parallel level execution.
func (e *Engine) Run(ctx context.Context, cfg *Config) (*RunResult, error) {
	ctx, span := otel.Tracer("hive/workflow").Start(ctx, "workflow.run",
	)
	span.SetAttributes(
		attribute.String("workflow.name", cfg.Name),
		attribute.Int("workflow.tasks", len(cfg.Tasks)),
		attribute.String("workflow.allocation", cfg.Allocation),
	)
	defer span.End()

	// Respect per-workflow concurrency cap (Story 2.5) and allocation strategy (Story 18.2).
	e.concurrency = cfg.Concurrency
	e.allocation = cfg.Allocation

	// 1. Create workflow record
	wf, err := e.workflowStore.Create(ctx, cfg.Name, cfg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "create workflow")
		return nil, fmt.Errorf("creating workflow: %w", err)
	}
	span.SetAttributes(attribute.String("workflow.id", wf.ID))

	// 2. Mark as running
	_ = e.workflowStore.UpdateStatus(ctx, wf.ID, StatusRunning)

	result := &RunResult{
		WorkflowID:  wf.ID,
		TaskResults: make(map[string]*task.Task),
	}

	// 3. Topological sort for execution levels
	levels, err := TopologicalSort(cfg.Tasks)
	if err != nil {
		_ = e.workflowStore.UpdateStatus(ctx, wf.ID, StatusFailed)
		return nil, fmt.Errorf("sorting tasks: %w", err)
	}

	slog.Info("workflow execution started", "workflow", cfg.Name, "id", wf.ID, "levels", len(levels), "tasks", len(cfg.Tasks))

	// 4. Execute level by level
	for levelIdx, level := range levels {
		slog.Info("executing level", "workflow", wf.ID, "level", levelIdx+1, "tasks", len(level))

		if err := e.executeLevel(ctx, wf.ID, level, result); err != nil {
			_ = e.workflowStore.UpdateStatus(ctx, wf.ID, StatusFailed)
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}
	}

	// 5. Mark completed
	_ = e.workflowStore.UpdateStatus(ctx, wf.ID, StatusCompleted)
	result.Status = "completed"

	slog.Info("workflow execution completed", "workflow", cfg.Name, "id", wf.ID, "tasks_completed", len(result.TaskResults))
	return result, nil
}

// executeLevel runs all tasks at a DAG level.
// Task creation and state transitions are serialized (SQLite), but agent invocations run in parallel.
func (e *Engine) executeLevel(ctx context.Context, workflowID string, level []TaskDef, result *RunResult) error {
	// Phase 1: Create and prepare all tasks sequentially (SQLite safe)
	type preparedTask struct {
		taskDef TaskDef
		taskID  string
		agentID string
		agentName string
		adapter adapter.Adapter
	}

	// Story 3.5 branch routing: first evaluate every task's condition so we can
	// decide which `default: true` siblings should run as the "else" branch.
	conditionPassed := map[string]bool{} // depsKey → any sibling passed
	hasCondition := map[string]bool{}    // depsKey → any sibling has a non-empty condition
	hasDefault := map[string]bool{}      // depsKey → any sibling is default
	passed := map[string]bool{}          // taskName → included in run
	for _, td := range level {
		key := depsKey(td.DependsOn)
		if td.Default {
			hasDefault[key] = true
			continue
		}
		if td.Condition == "" {
			passed[td.Name] = true
			conditionPassed[key] = true
			continue
		}
		hasCondition[key] = true
		evalCtx := buildEvalContext(result, td.DependsOn)
		ok, err := EvaluateCondition(td.Condition, evalCtx)
		if err != nil {
			return fmt.Errorf("evaluating condition for task %s: %w", td.Name, err)
		}
		passed[td.Name] = ok
		if ok {
			conditionPassed[key] = true
		}
	}
	// Defaults run iff no sibling at the same deps-key passed its condition.
	for _, td := range level {
		if !td.Default {
			continue
		}
		key := depsKey(td.DependsOn)
		passed[td.Name] = !conditionPassed[key]
	}
	// Story 3.5 AC: "missing default branch with unmatched condition produces
	// clear error". If every sibling had a condition and none passed and no
	// default is declared, the caller's workflow can't make progress.
	for key, hasCond := range hasCondition {
		if !hasCond {
			continue
		}
		if conditionPassed[key] || hasDefault[key] {
			continue
		}
		return fmt.Errorf("workflow branch (deps=%q) has conditions but none matched and no task is marked `default: true`", key)
	}

	var prepared []preparedTask
	for _, td := range level {
		if !passed[td.Name] {
			slog.Info("task skipped by condition", "task", td.Name, "condition", td.Condition, "default", td.Default)
			if e.eventBus != nil {
				_, _ = e.eventBus.Publish(ctx, "task.skipped", "workflow_engine", map[string]string{
					"workflow_id": workflowID,
					"task":        td.Name,
					"condition":   td.Condition,
				})
			}
			continue
		}

		inputJSON := e.buildInput(td, result)

		t, err := e.taskStore.Create(ctx, workflowID, td.Type, inputJSON, td.DependsOn)
		if err != nil {
			return fmt.Errorf("creating task %s: %w", td.Name, err)
		}

		agentID, agentName, err := e.pickAgent(ctx, t.ID, td.Type, e.allocation)
		if err != nil || agentID == "" {
			// Story 2.3 AC: task remains `pending` with a task.unroutable event.
			// task.Router.FindCapableAgent already emits task.unroutable; we
			// deliberately do NOT Fail() the task so a late-arriving capable
			// agent can still claim it via the self-assignment path.
			return fmt.Errorf("no agent available for task type %s (task %s left pending)", td.Type, t.ID)
		}

		_ = e.taskStore.Assign(ctx, t.ID, agentID)
		_ = e.taskStore.Start(ctx, t.ID)

		e.mu.Lock()
		a, ok := e.adapters[agentID]
		e.mu.Unlock()
		if !ok {
			// Build the adapter from whatever the agents table has on this
			// ID. Falls back to HTTP when there's no lookup installed, which
			// preserves the v0 behaviour for single-type deployments.
			var built adapter.Adapter
			if e.lookupAgent != nil {
				spec, lookupErr := e.lookupAgent(ctx, agentID)
				if lookupErr != nil {
					return fmt.Errorf("agent lookup for %s (%s): %w", td.Name, agentID, lookupErr)
				}
				built, err = adapter.BuildAdapter(spec)
				if err != nil {
					return fmt.Errorf("build adapter for %s (%s): %w", td.Name, agentID, err)
				}
			} else {
				built = adapter.NewHTTPAdapter(e.agentConfigs[agentID])
			}
			// HTTP adapters get the per-engine retry policy wrapped so
			// task.retry events fire. Other adapter types don't expose
			// retry yet — a future story can lift this into the Adapter
			// interface.
			if httpA, isHTTP := built.(*adapter.HTTPAdapter); isHTTP && e.retry != nil {
				policy := *e.retry
				tid := t.ID
				policy.OnAttempt = func(attempt int, wait time.Duration, lastErr error) {
					if e.eventBus != nil {
						_, _ = e.eventBus.Publish(ctx, "task.retry", "workflow_engine", map[string]any{
							"task_id": tid,
							"attempt": attempt,
							"wait_ms": wait.Milliseconds(),
							"error":   lastErr.Error(),
						})
					}
				}
				httpA.WithRetry(&policy)
			}
			a = built
		}

		prepared = append(prepared, preparedTask{
			taskDef: td, taskID: t.ID, agentID: agentID, agentName: agentName, adapter: a,
		})

		slog.Info("task dispatched", "task", td.Name, "agent", agentName)
	}

	// Phase 2: Invoke agents in parallel, bounded by workflow concurrency (Story 2.5).
	var wg sync.WaitGroup
	errCh := make(chan error, len(prepared))

	// A nil semaphore = unlimited parallelism; otherwise acquire before each invoke.
	var sem chan struct{}
	if e.concurrency > 0 {
		sem = make(chan struct{}, e.concurrency)
	}

	for _, pt := range prepared {
		wg.Add(1)
		go func(p preparedTask) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			taskResult, err := adapter.SafeInvoke(ctx, p.adapter, adapter.Task{
				ID: p.taskID, Type: p.taskDef.Type, Input: e.buildInput(p.taskDef, result),
			})
			if err != nil {
				errCh <- fmt.Errorf("task %s invoke failed: %w", p.taskDef.Name, err)
				return
			}

			// Phase 3: Record results sequentially via channel
			result.mu.Lock()
			defer result.mu.Unlock()

			if taskResult.Status == task.StatusCompleted {
				outputJSON, _ := json.Marshal(taskResult.Output)
				_ = e.taskStore.Complete(ctx, p.taskID, string(outputJSON))
				completed, _ := e.taskStore.GetByID(ctx, p.taskID)
				result.TaskResults[p.taskDef.Name] = completed
				slog.Info("task completed", "task", p.taskDef.Name, "agent", p.agentName)
			} else {
				_ = e.taskStore.Fail(ctx, p.taskID, taskResult.Error)
				errCh <- fmt.Errorf("task %s failed: %s", p.taskDef.Name, taskResult.Error)
			}
		}(pt)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}
	return nil
}

func (e *Engine) buildInput(taskDef TaskDef, result *RunResult) string {
	input := make(map[string]any)

	// Add task-level input
	if taskDef.Input != nil {
		input["config"] = taskDef.Input
	}

	// Add upstream results
	if len(taskDef.DependsOn) > 0 {
		upstream := make(map[string]string)
		result.mu.Lock()
		for _, dep := range taskDef.DependsOn {
			if t, ok := result.TaskResults[dep]; ok {
				upstream[dep] = t.Output
			}
		}
		result.mu.Unlock()
		input["upstream"] = upstream
	}

	data, _ := json.Marshal(input)
	return string(data)
}

