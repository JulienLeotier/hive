package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/task"
)

// Engine orchestrates workflow execution: creates tasks, routes to agents, executes in DAG order.
type Engine struct {
	workflowStore *Store
	taskStore     *task.Store
	taskRouter    *task.Router
	eventBus      *event.Bus
	adapters      map[string]adapter.Adapter // agentID -> adapter
	agentConfigs  map[string]string          // agentID -> baseURL
	mu            sync.Mutex
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
	// 1. Create workflow record
	wf, err := e.workflowStore.Create(ctx, cfg.Name, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating workflow: %w", err)
	}

	// 2. Mark as running
	e.workflowStore.UpdateStatus(ctx, wf.ID, StatusRunning)

	result := &RunResult{
		WorkflowID:  wf.ID,
		TaskResults: make(map[string]*task.Task),
	}

	// 3. Topological sort for execution levels
	levels, err := TopologicalSort(cfg.Tasks)
	if err != nil {
		e.workflowStore.UpdateStatus(ctx, wf.ID, StatusFailed)
		return nil, fmt.Errorf("sorting tasks: %w", err)
	}

	slog.Info("workflow execution started", "workflow", cfg.Name, "id", wf.ID, "levels", len(levels), "tasks", len(cfg.Tasks))

	// 4. Execute level by level
	for levelIdx, level := range levels {
		slog.Info("executing level", "workflow", wf.ID, "level", levelIdx+1, "tasks", len(level))

		if err := e.executeLevel(ctx, wf.ID, level, result); err != nil {
			e.workflowStore.UpdateStatus(ctx, wf.ID, StatusFailed)
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}
	}

	// 5. Mark completed
	e.workflowStore.UpdateStatus(ctx, wf.ID, StatusCompleted)
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

	var prepared []preparedTask
	for _, td := range level {
		// Story 3.5: skip tasks whose condition evaluates to false.
		if td.Condition != "" {
			evalCtx := buildEvalContext(result, td.DependsOn)
			ok, err := EvaluateCondition(td.Condition, evalCtx)
			if err != nil {
				return fmt.Errorf("evaluating condition for task %s: %w", td.Name, err)
			}
			if !ok {
				slog.Info("task skipped by condition", "task", td.Name, "condition", td.Condition)
				if e.eventBus != nil {
					_, _ = e.eventBus.Publish(ctx, "task.skipped", "workflow_engine", map[string]string{
						"workflow_id": workflowID,
						"task":        td.Name,
						"condition":   td.Condition,
					})
				}
				continue
			}
		}

		inputJSON := e.buildInput(td, result)

		t, err := e.taskStore.Create(ctx, workflowID, td.Type, inputJSON, td.DependsOn)
		if err != nil {
			return fmt.Errorf("creating task %s: %w", td.Name, err)
		}

		agentID, agentName, err := e.taskRouter.FindCapableAgent(ctx, td.Type)
		if err != nil || agentID == "" {
			e.taskStore.Fail(ctx, t.ID, "no capable agent for type: "+td.Type)
			return fmt.Errorf("no agent available for task type %s", td.Type)
		}

		e.taskStore.Assign(ctx, t.ID, agentID)
		e.taskStore.Start(ctx, t.ID)

		e.mu.Lock()
		a, ok := e.adapters[agentID]
		if !ok {
			a = adapter.NewHTTPAdapter(e.agentConfigs[agentID])
		}
		e.mu.Unlock()

		prepared = append(prepared, preparedTask{
			taskDef: td, taskID: t.ID, agentID: agentID, agentName: agentName, adapter: a,
		})

		slog.Info("task dispatched", "task", td.Name, "agent", agentName)
	}

	// Phase 2: Invoke agents in parallel
	var wg sync.WaitGroup
	errCh := make(chan error, len(prepared))

	for _, pt := range prepared {
		wg.Add(1)
		go func(p preparedTask) {
			defer wg.Done()

			taskResult, err := p.adapter.Invoke(ctx, adapter.Task{
				ID: p.taskID, Type: p.taskDef.Type, Input: e.buildInput(p.taskDef, result),
			})
			if err != nil {
				errCh <- fmt.Errorf("task %s invoke failed: %w", p.taskDef.Name, err)
				return
			}

			// Phase 3: Record results sequentially via channel
			result.mu.Lock()
			defer result.mu.Unlock()

			if taskResult.Status == "completed" {
				outputJSON, _ := json.Marshal(taskResult.Output)
				e.taskStore.Complete(ctx, p.taskID, string(outputJSON))
				completed, _ := e.taskStore.GetByID(ctx, p.taskID)
				result.TaskResults[p.taskDef.Name] = completed
				slog.Info("task completed", "task", p.taskDef.Name, "agent", p.agentName)
			} else {
				e.taskStore.Fail(ctx, p.taskID, taskResult.Error)
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

func (e *Engine) getAgentURL(ctx context.Context, agentID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if url, ok := e.agentConfigs[agentID]; ok {
		return url
	}
	return ""
}

