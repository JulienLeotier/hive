package workflow

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

func setupEngine(t *testing.T) (*Engine, *storage.Store) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	bus := event.NewBus(st.DB)
	taskStore := task.NewStore(st.DB, bus)
	taskRouter := task.NewRouter(st.DB)
	wfStore := NewStore(st.DB, bus)

	return NewEngine(wfStore, taskStore, taskRouter, bus), st
}

func registerMockAgent(t *testing.T, st *storage.Store, name, agentID string, taskTypes []string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			json.NewEncoder(w).Encode(adapter.HealthStatus{Status: "healthy"})
		case "/declare":
			json.NewEncoder(w).Encode(adapter.AgentCapabilities{Name: name, TaskTypes: taskTypes})
		case "/invoke":
			var task adapter.Task
			json.NewDecoder(r.Body).Decode(&task)
			json.NewEncoder(w).Encode(adapter.TaskResult{
				TaskID: task.ID,
				Status: "completed",
				Output: map[string]string{"result": "done by " + name},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	caps, _ := json.Marshal(adapter.AgentCapabilities{Name: name, TaskTypes: taskTypes})
	cfg, _ := json.Marshal(map[string]string{"base_url": srv.URL})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, 'http', ?, ?, 'healthy')`,
		agentID, name, string(cfg), string(caps))

	return srv
}

func TestEngineRunSimpleWorkflow(t *testing.T) {
	engine, st := setupEngine(t)

	srv := registerMockAgent(t, st, "reviewer", "a1", []string{"code-review"})
	engine.RegisterAdapter("a1", srv.URL, adapter.NewHTTPAdapter(srv.URL))

	cfg := &Config{
		Name: "test-workflow",
		Tasks: []TaskDef{
			{Name: "review", Type: "code-review"},
		},
	}

	result, err := engine.Run(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.TaskResults, 1)
	assert.NotNil(t, result.TaskResults["review"])
}

func TestEngineRunMultiStepWorkflow(t *testing.T) {
	engine, st := setupEngine(t)

	srv1 := registerMockAgent(t, st, "reviewer", "a1", []string{"code-review"})
	srv2 := registerMockAgent(t, st, "summarizer", "a2", []string{"summarize"})
	engine.RegisterAdapter("a1", srv1.URL, adapter.NewHTTPAdapter(srv1.URL))
	engine.RegisterAdapter("a2", srv2.URL, adapter.NewHTTPAdapter(srv2.URL))

	cfg := &Config{
		Name: "multi-step",
		Tasks: []TaskDef{
			{Name: "review", Type: "code-review"},
			{Name: "summarize", Type: "summarize", DependsOn: []string{"review"}},
		},
	}

	result, err := engine.Run(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.TaskResults, 2)
}

func TestEngineRunParallelTasks(t *testing.T) {
	engine, st := setupEngine(t)

	srv := registerMockAgent(t, st, "worker", "a1", []string{"search", "summarize"})
	engine.RegisterAdapter("a1", srv.URL, adapter.NewHTTPAdapter(srv.URL))

	cfg := &Config{
		Name: "parallel",
		Tasks: []TaskDef{
			{Name: "search-a", Type: "search"},
			{Name: "search-b", Type: "search"},
			{Name: "aggregate", Type: "summarize", DependsOn: []string{"search-a", "search-b"}},
		},
	}

	result, err := engine.Run(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.TaskResults, 3)
}

func TestEngineRunNoAgentAvailable(t *testing.T) {
	engine, _ := setupEngine(t)
	// No agents registered

	cfg := &Config{
		Name: "no-agent",
		Tasks: []TaskDef{
			{Name: "review", Type: "code-review"},
		},
	}

	result, err := engine.Run(context.Background(), cfg)
	require.Error(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, err.Error(), "no agent available")
}

func TestEngineEmitsWorkflowEvents(t *testing.T) {
	engine, st := setupEngine(t)

	srv := registerMockAgent(t, st, "worker", "a1", []string{"test"})
	engine.RegisterAdapter("a1", srv.URL, adapter.NewHTTPAdapter(srv.URL))

	bus := event.NewBus(st.DB)
	var events []event.Event
	bus.Subscribe("workflow.", func(e event.Event) {
		events = append(events, e)
	})

	cfg := &Config{
		Name: "event-test",
		Tasks: []TaskDef{
			{Name: "task1", Type: "test"},
		},
	}

	_, err := engine.Run(context.Background(), cfg)
	require.NoError(t, err)

	// Query events from DB
	allEvents, _ := bus.Query(context.Background(), event.QueryOpts{Type: "workflow"})
	assert.GreaterOrEqual(t, len(allEvents), 2) // started + completed
}
