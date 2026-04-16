package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/trust"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockAgent returns an httptest.Server that implements the Agent Adapter Protocol.
// It accepts code-review and summarize task types. The failAfter parameter controls
// how many invocations succeed before the agent starts returning 500 errors.
// Pass 0 for failAfter to never fail.
func mockAgent(t *testing.T, name string, taskTypes []string, failAfter int) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	invocations := 0

	mux := http.NewServeMux()

	mux.HandleFunc("GET /declare", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(adapter.AgentCapabilities{
			Name:      name,
			TaskTypes: taskTypes,
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(adapter.HealthStatus{Status: "healthy"})
	})

	mux.HandleFunc("POST /invoke", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		invocations++
		n := invocations
		mu.Unlock()

		if failAfter > 0 && n > failAfter {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("simulated failure"))
			return
		}

		var t adapter.Task
		json.NewDecoder(r.Body).Decode(&t)
		json.NewEncoder(w).Encode(adapter.TaskResult{
			TaskID: t.ID,
			Status: "completed",
			Output: map[string]string{"result": "done by " + name},
		})
	})

	mux.HandleFunc("GET /checkpoint", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(adapter.Checkpoint{Data: map[string]string{"agent": name}})
	})

	mux.HandleFunc("POST /resume", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// openAll opens storage and creates all core components for integration testing.
func openAll(t *testing.T) (*storage.Store, *agent.Manager, *event.Bus, *task.Store, *task.Router) {
	t.Helper()

	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Create trust_history table (needed for trust engine tests)
	store.DB.Exec(`CREATE TABLE IF NOT EXISTS trust_history (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		old_level TEXT NOT NULL,
		new_level TEXT NOT NULL,
		reason TEXT NOT NULL,
		criteria TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)`)

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	taskStore := task.NewStore(store.DB, bus)
	router := task.NewRouter(store.DB)

	return store, mgr, bus, taskStore, router
}

// ---------------------------------------------------------------------------
// Test 1: Full orchestration flow
// ---------------------------------------------------------------------------

func TestFullOrchestrationFlow(t *testing.T) {
	store, mgr, bus, taskStore, router := openAll(t)
	ctx := context.Background()

	// Collect all events
	var collectedEvents []event.Event
	var mu sync.Mutex
	bus.Subscribe("*", func(e event.Event) {
		mu.Lock()
		collectedEvents = append(collectedEvents, e)
		mu.Unlock()
	})

	// Step 1: Register a mock HTTP agent
	srv := mockAgent(t, "reviewer", []string{"code-review", "summarize"}, 0)
	registeredAgent, err := mgr.Register(ctx, "reviewer", "http", srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "reviewer", registeredAgent.Name)
	assert.Equal(t, "healthy", registeredAgent.HealthStatus)

	// Step 2: Create a task
	createdTask, err := taskStore.Create(ctx, "wf-integration-1", "code-review", `{"file":"main.go"}`, nil)
	require.NoError(t, err)
	assert.Equal(t, task.StatusPending, createdTask.Status)

	// Step 3: Route the task to a capable agent
	agentID, agentName, err := router.FindCapableAgent(ctx, "code-review")
	require.NoError(t, err)
	assert.Equal(t, registeredAgent.ID, agentID)
	assert.Equal(t, "reviewer", agentName)

	// Step 4: Walk through task state machine: pending -> assigned -> running -> completed
	err = taskStore.Assign(ctx, createdTask.ID, agentID)
	require.NoError(t, err)

	assigned, err := taskStore.GetByID(ctx, createdTask.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusAssigned, assigned.Status)

	err = taskStore.Start(ctx, createdTask.ID)
	require.NoError(t, err)

	running, err := taskStore.GetByID(ctx, createdTask.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusRunning, running.Status)

	// Invoke the agent via adapter
	a := adapter.NewHTTPAdapter(srv.URL)
	result, err := a.Invoke(ctx, adapter.Task{
		ID:    createdTask.ID,
		Type:  "code-review",
		Input: map[string]string{"file": "main.go"},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)

	err = taskStore.Complete(ctx, createdTask.ID, `{"result":"done by reviewer"}`)
	require.NoError(t, err)

	completed, err := taskStore.GetByID(ctx, createdTask.ID)
	require.NoError(t, err)
	assert.Equal(t, task.StatusCompleted, completed.Status)
	assert.Contains(t, completed.Output, "reviewer")

	// Step 5: Verify events were emitted for each transition
	mu.Lock()
	eventTypes := make([]string, len(collectedEvents))
	for i, e := range collectedEvents {
		eventTypes[i] = e.Type
	}
	mu.Unlock()

	assert.Contains(t, eventTypes, event.TaskCreated)
	assert.Contains(t, eventTypes, event.TaskAssigned)
	assert.Contains(t, eventTypes, event.TaskStarted)
	assert.Contains(t, eventTypes, event.TaskCompleted)

	// Step 6: Verify events are persisted and queryable
	persistedEvents, err := bus.Query(ctx, event.QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(persistedEvents), 4)

	// Verify ordering
	for i := 1; i < len(persistedEvents); i++ {
		assert.Greater(t, persistedEvents[i].ID, persistedEvents[i-1].ID)
	}

	// Step 7: Verify agent health check works via adapter
	health, err := a.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)

	// Step 8: Verify checkpoint/resume works via adapter
	cp, err := a.Checkpoint(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cp.Data)

	err = a.Resume(ctx, cp)
	require.NoError(t, err)

	// Step 9: Cleanup -- verify agent removal works
	err = mgr.Remove(ctx, "reviewer")
	require.NoError(t, err)

	agents, err := mgr.List(ctx)
	require.NoError(t, err)
	assert.Nil(t, agents)

	_ = store // keep reference to prevent premature GC
}

// ---------------------------------------------------------------------------
// Test 2: Task state machine transitions
// ---------------------------------------------------------------------------

func TestTaskStateMachineTransitions(t *testing.T) {
	_, _, _, taskStore, _ := openAll(t)
	ctx := context.Background()

	t.Run("pending_to_assigned_to_running_to_completed", func(t *testing.T) {
		tk, err := taskStore.Create(ctx, "wf-sm", "test", `{}`, nil)
		require.NoError(t, err)
		assert.Equal(t, task.StatusPending, tk.Status)

		require.NoError(t, taskStore.Assign(ctx, tk.ID, "a1"))
		require.NoError(t, taskStore.Start(ctx, tk.ID))
		require.NoError(t, taskStore.Complete(ctx, tk.ID, `{"ok":true}`))

		final, err := taskStore.GetByID(ctx, tk.ID)
		require.NoError(t, err)
		assert.Equal(t, task.StatusCompleted, final.Status)
	})

	t.Run("pending_to_assigned_to_running_to_failed", func(t *testing.T) {
		tk, err := taskStore.Create(ctx, "wf-sm", "test", `{}`, nil)
		require.NoError(t, err)

		require.NoError(t, taskStore.Assign(ctx, tk.ID, "a1"))
		require.NoError(t, taskStore.Start(ctx, tk.ID))
		require.NoError(t, taskStore.Fail(ctx, tk.ID, "timeout"))

		final, err := taskStore.GetByID(ctx, tk.ID)
		require.NoError(t, err)
		assert.Equal(t, task.StatusFailed, final.Status)
	})

	t.Run("invalid_complete_from_pending_rejected", func(t *testing.T) {
		tk, err := taskStore.Create(ctx, "wf-sm", "test", `{}`, nil)
		require.NoError(t, err)

		// Cannot complete a pending task (must be running)
		err = taskStore.Complete(ctx, tk.ID, `{}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in running state")
	})

	t.Run("invalid_start_from_pending_rejected", func(t *testing.T) {
		tk, err := taskStore.Create(ctx, "wf-sm", "test", `{}`, nil)
		require.NoError(t, err)

		// Cannot start a pending task (must be assigned first)
		err = taskStore.Start(ctx, tk.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in assigned state")
	})

	t.Run("double_assign_rejected", func(t *testing.T) {
		tk, err := taskStore.Create(ctx, "wf-sm", "test", `{}`, nil)
		require.NoError(t, err)

		require.NoError(t, taskStore.Assign(ctx, tk.ID, "a1"))

		// Second assign should fail (task is now assigned, not pending)
		err = taskStore.Assign(ctx, tk.ID, "a2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in pending state")
	})
}

// ---------------------------------------------------------------------------
// Test 3: Events emitted for each state transition
// ---------------------------------------------------------------------------

func TestEventsEmittedForAllTransitions(t *testing.T) {
	_, _, bus, taskStore, _ := openAll(t)
	ctx := context.Background()

	var events []event.Event
	var mu sync.Mutex
	bus.Subscribe("task.", func(e event.Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})

	tk, err := taskStore.Create(ctx, "wf-events", "test", `{}`, nil)
	require.NoError(t, err)
	require.NoError(t, taskStore.Assign(ctx, tk.ID, "agent-1"))
	require.NoError(t, taskStore.Start(ctx, tk.ID))
	require.NoError(t, taskStore.Complete(ctx, tk.ID, `{"ok":true}`))

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, events, 4, "expected 4 events: created, assigned, started, completed")
	assert.Equal(t, event.TaskCreated, events[0].Type)
	assert.Equal(t, event.TaskAssigned, events[1].Type)
	assert.Equal(t, event.TaskStarted, events[2].Type)
	assert.Equal(t, event.TaskCompleted, events[3].Type)

	// Verify events are persisted
	persisted, err := bus.Query(ctx, event.QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.Len(t, persisted, 4)
}

// ---------------------------------------------------------------------------
// Test 4: Agent health check via adapter
// ---------------------------------------------------------------------------

func TestAgentHealthCheck(t *testing.T) {
	_, mgr, _, _, _ := openAll(t)
	ctx := context.Background()

	srv := mockAgent(t, "healthy-agent", []string{"test"}, 0)

	registeredAgent, err := mgr.Register(ctx, "healthy-agent", "http", srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "healthy", registeredAgent.HealthStatus)

	// Check health again via adapter
	a := adapter.NewHTTPAdapter(srv.URL)
	health, err := a.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)

	// Check health of unreachable agent
	a2 := adapter.NewHTTPAdapter("http://localhost:1")
	health2, err := a2.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "unavailable", health2.Status)
}

// ---------------------------------------------------------------------------
// Test 5: Circuit breaker triggers after failures
// ---------------------------------------------------------------------------

func TestCircuitBreakerTriggersAfterFailures(t *testing.T) {
	_, mgr, _, _, _ := openAll(t)
	ctx := context.Background()

	// Agent that fails after 2 successful invocations
	srv := mockAgent(t, "flaky-agent", []string{"code-review"}, 2)

	_, err := mgr.Register(ctx, "flaky-agent", "http", srv.URL)
	require.NoError(t, err)

	a := adapter.NewHTTPAdapter(srv.URL)
	breakers := resilience.NewBreakerRegistry(resilience.BreakerConfig{
		Threshold:    3,
		ResetTimeout: 100 * time.Millisecond,
	})
	cb := breakers.Get("flaky-agent")

	// First 2 invocations succeed
	for i := 0; i < 2; i++ {
		err := cb.Allow()
		require.NoError(t, err)

		result, err := a.Invoke(ctx, adapter.Task{
			ID:   fmt.Sprintf("task-%d", i),
			Type: "code-review",
		})
		require.NoError(t, err)
		assert.Equal(t, "completed", result.Status)
		cb.RecordSuccess()
	}

	assert.Equal(t, resilience.StateClosed, cb.State())

	// Next 3 invocations fail -- circuit should open
	for i := 2; i < 5; i++ {
		err := cb.Allow()
		if err != nil {
			// Circuit is open
			break
		}

		_, invokeErr := a.Invoke(ctx, adapter.Task{
			ID:   fmt.Sprintf("task-%d", i),
			Type: "code-review",
		})
		if invokeErr != nil {
			cb.RecordFailure()
		}
	}

	// Circuit should now be open
	assert.Equal(t, resilience.StateOpen, cb.State())
	assert.Error(t, cb.Allow())

	// Wait for reset timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, resilience.StateHalfOpen, cb.State())
	assert.NoError(t, cb.Allow())
}

// ---------------------------------------------------------------------------
// Test 6: Trust engine promotion
// ---------------------------------------------------------------------------

func TestTrustEnginePromotion(t *testing.T) {
	store, _, _, _, _ := openAll(t)
	ctx := context.Background()

	// Insert a test agent at supervised level
	store.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		VALUES ('trust-agent-1', 'trust-test', 'http', '{}', '{}', 'healthy', 'supervised')`)

	engine := trust.NewEngine(store.DB, trust.Thresholds{
		GuidedAfterTasks:     10,
		GuidedMaxErrorRate:   0.10,
		AutonomousAfterTasks: 25,
		AutonomousMaxError:   0.05,
		TrustedAfterTasks:    50,
		TrustedMaxError:      0.02,
	})

	// Insert 10 successful tasks -> should promote to guided
	for i := 0; i < 10; i++ {
		store.DB.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
			VALUES (?, 'wf', 'test', 'completed', 'trust-agent-1', '{}')`,
			fmt.Sprintf("trust-t-%d", i))
	}

	promoted, level, err := engine.Evaluate(ctx, "trust-agent-1")
	require.NoError(t, err)
	assert.True(t, promoted, "agent should be promoted after 10 successful tasks")
	assert.Equal(t, trust.LevelGuided, level)

	// Insert 15 more successful tasks (total 25) -> should promote to autonomous
	for i := 10; i < 25; i++ {
		store.DB.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
			VALUES (?, 'wf', 'test', 'completed', 'trust-agent-1', '{}')`,
			fmt.Sprintf("trust-t-%d", i))
	}

	promoted, level, err = engine.Evaluate(ctx, "trust-agent-1")
	require.NoError(t, err)
	assert.True(t, promoted, "agent should be promoted after 25 successful tasks")
	assert.Equal(t, trust.LevelAutonomous, level)

	// Insert 25 more successful tasks (total 50) -> should promote to trusted
	for i := 25; i < 50; i++ {
		store.DB.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
			VALUES (?, 'wf', 'test', 'completed', 'trust-agent-1', '{}')`,
			fmt.Sprintf("trust-t-%d", i))
	}

	promoted, level, err = engine.Evaluate(ctx, "trust-agent-1")
	require.NoError(t, err)
	assert.True(t, promoted, "agent should be promoted after 50 successful tasks")
	assert.Equal(t, trust.LevelTrusted, level)

	// Verify trust history was recorded
	var historyCount int
	store.DB.QueryRow(`SELECT COUNT(*) FROM trust_history WHERE agent_id = 'trust-agent-1'`).Scan(&historyCount)
	assert.Equal(t, 3, historyCount, "should have 3 promotion records")
}

// ---------------------------------------------------------------------------
// Test 7: Trust engine does not auto-demote
// ---------------------------------------------------------------------------

func TestTrustEngineNeverAutoDemotes(t *testing.T) {
	store, _, _, _, _ := openAll(t)
	ctx := context.Background()

	store.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		VALUES ('demote-agent', 'demote-test', 'http', '{}', '{}', 'healthy', 'trusted')`)

	engine := trust.NewEngine(store.DB, trust.DefaultThresholds())

	// Insert tasks with a 50% failure rate
	for i := 0; i < 5; i++ {
		store.DB.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
			VALUES (?, 'wf', 'test', 'completed', 'demote-agent', '{}')`,
			fmt.Sprintf("demote-c-%d", i))
	}
	for i := 0; i < 5; i++ {
		store.DB.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
			VALUES (?, 'wf', 'test', 'failed', 'demote-agent', '{}')`,
			fmt.Sprintf("demote-f-%d", i))
	}

	promoted, level, err := engine.Evaluate(ctx, "demote-agent")
	require.NoError(t, err)
	assert.False(t, promoted, "should not demote")
	assert.Equal(t, "trusted", level, "should remain trusted")
}

// ---------------------------------------------------------------------------
// Test 8: Manual trust override
// ---------------------------------------------------------------------------

func TestTrustEngineManualOverride(t *testing.T) {
	store, _, _, _, _ := openAll(t)
	ctx := context.Background()

	store.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		VALUES ('manual-agent', 'manual-test', 'http', '{}', '{}', 'healthy', 'supervised')`)

	engine := trust.NewEngine(store.DB, trust.DefaultThresholds())

	err := engine.SetManual(ctx, "manual-agent", trust.LevelTrusted)
	require.NoError(t, err)

	// Verify it was set
	var level string
	store.DB.QueryRow(`SELECT trust_level FROM agents WHERE id = 'manual-agent'`).Scan(&level)
	assert.Equal(t, trust.LevelTrusted, level)

	// Verify history recorded
	var reason string
	store.DB.QueryRow(`SELECT reason FROM trust_history WHERE agent_id = 'manual-agent'`).Scan(&reason)
	assert.Equal(t, "manual_override", reason)
}

// ---------------------------------------------------------------------------
// Test 9: Task routing to capable agent
// ---------------------------------------------------------------------------

func TestTaskRoutingIntegration(t *testing.T) {
	_, mgr, _, taskStore, router := openAll(t)
	ctx := context.Background()

	// Register two agents with different capabilities
	srvReview := mockAgent(t, "reviewer", []string{"code-review"}, 0)
	srvWrite := mockAgent(t, "writer", []string{"summarize", "write"}, 0)

	_, err := mgr.Register(ctx, "reviewer", "http", srvReview.URL)
	require.NoError(t, err)
	_, err = mgr.Register(ctx, "writer", "http", srvWrite.URL)
	require.NoError(t, err)

	// Create a code-review task -> should route to reviewer
	tk1, err := taskStore.Create(ctx, "wf-route", "code-review", `{}`, nil)
	require.NoError(t, err)

	agentID1, agentName1, err := router.FindCapableAgent(ctx, "code-review")
	require.NoError(t, err)
	assert.Equal(t, "reviewer", agentName1)

	err = taskStore.Assign(ctx, tk1.ID, agentID1)
	require.NoError(t, err)

	// Create a summarize task -> should route to writer
	tk2, err := taskStore.Create(ctx, "wf-route", "summarize", `{}`, nil)
	require.NoError(t, err)

	agentID2, agentName2, err := router.FindCapableAgent(ctx, "summarize")
	require.NoError(t, err)
	assert.Equal(t, "writer", agentName2)

	err = taskStore.Assign(ctx, tk2.ID, agentID2)
	require.NoError(t, err)

	// No agent for "deploy" -> should return empty
	agentID3, _, err := router.FindCapableAgent(ctx, "deploy")
	require.NoError(t, err)
	assert.Empty(t, agentID3)
}

// ---------------------------------------------------------------------------
// Test 10: Checkpoint and resume
// ---------------------------------------------------------------------------

func TestCheckpointAndResume(t *testing.T) {
	_, _, _, taskStore, _ := openAll(t)
	ctx := context.Background()

	tk, err := taskStore.Create(ctx, "wf-cp", "test", `{}`, nil)
	require.NoError(t, err)

	// Save a checkpoint
	err = taskStore.SaveCheckpoint(ctx, tk.ID, `{"step":3,"data":"partial"}`)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := taskStore.GetByID(ctx, tk.ID)
	require.NoError(t, err)
	assert.Contains(t, retrieved.Checkpoint, "step")
	assert.Contains(t, retrieved.Checkpoint, "partial")

	// Update checkpoint
	err = taskStore.SaveCheckpoint(ctx, tk.ID, `{"step":5,"data":"more-progress"}`)
	require.NoError(t, err)

	retrieved2, err := taskStore.GetByID(ctx, tk.ID)
	require.NoError(t, err)
	assert.Contains(t, retrieved2.Checkpoint, "more-progress")
}

// ---------------------------------------------------------------------------
// Test 11: Multiple tasks in a workflow
// ---------------------------------------------------------------------------

func TestMultipleTasksInWorkflow(t *testing.T) {
	_, _, _, taskStore, _ := openAll(t)
	ctx := context.Background()

	wfID := "wf-multi"
	_, err := taskStore.Create(ctx, wfID, "code-review", `{"file":"a.go"}`, nil)
	require.NoError(t, err)
	_, err = taskStore.Create(ctx, wfID, "summarize", `{"target":"results"}`, nil)
	require.NoError(t, err)
	_, err = taskStore.Create(ctx, wfID, "lint", `{"file":"a.go"}`, nil)
	require.NoError(t, err)

	tasks, err := taskStore.ListByWorkflow(ctx, wfID)
	require.NoError(t, err)
	assert.Len(t, tasks, 3)

	// All should be pending initially
	for _, tk := range tasks {
		assert.Equal(t, task.StatusPending, tk.Status)
	}

	// Pending filter should work
	pending, err := taskStore.ListPending(ctx, "code-review")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "code-review", pending[0].Type)
}

// ---------------------------------------------------------------------------
// Test 12: Event query filtering
// ---------------------------------------------------------------------------

func TestEventQueryFiltering(t *testing.T) {
	_, _, bus, _, _ := openAll(t)
	ctx := context.Background()

	bus.Publish(ctx, event.TaskCreated, "agent-a", map[string]string{"task_id": "t1"})
	bus.Publish(ctx, event.TaskCompleted, "agent-b", map[string]string{"task_id": "t2"})
	bus.Publish(ctx, event.AgentRegistered, "system", map[string]string{"agent": "new"})

	// Query by type prefix
	taskEvents, err := bus.Query(ctx, event.QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.Len(t, taskEvents, 2)

	// Query by source
	agentAEvents, err := bus.Query(ctx, event.QueryOpts{Source: "agent-a"})
	require.NoError(t, err)
	assert.Len(t, agentAEvents, 1)

	// Query with limit
	limited, err := bus.Query(ctx, event.QueryOpts{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, limited, 1)

	// Query with future since returns empty
	futureEvents, err := bus.Query(ctx, event.QueryOpts{Since: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Len(t, futureEvents, 0)
}

// ---------------------------------------------------------------------------
// Test 13: Circuit breaker registry integration
// ---------------------------------------------------------------------------

func TestBreakerRegistryIntegration(t *testing.T) {
	registry := resilience.NewBreakerRegistry(resilience.BreakerConfig{
		Threshold:    2,
		ResetTimeout: 50 * time.Millisecond,
	})

	// Two agents
	cbA := registry.Get("agent-a")
	cbB := registry.Get("agent-b")

	// agent-a fails
	cbA.RecordFailure()
	cbA.RecordFailure()
	assert.Equal(t, resilience.StateOpen, cbA.State())

	// agent-b is fine
	cbB.RecordSuccess()
	assert.Equal(t, resilience.StateClosed, cbB.State())

	// Registry reflects both states
	states := registry.AllStates()
	assert.Equal(t, resilience.StateOpen, states["agent-a"])
	assert.Equal(t, resilience.StateClosed, states["agent-b"])

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, resilience.StateHalfOpen, cbA.State())

	// Success in half-open closes circuit
	cbA.RecordSuccess()
	assert.Equal(t, resilience.StateClosed, cbA.State())
}
