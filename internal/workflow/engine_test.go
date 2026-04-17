package workflow

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/market"
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

func TestEngineMarketAllocationPersistsAuction(t *testing.T) {
	engine, st := setupEngine(t)

	// Two agents with different cost_per_run bid on the same task.
	srvCheap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			json.NewEncoder(w).Encode(adapter.HealthStatus{Status: "healthy"})
		case "/declare":
			json.NewEncoder(w).Encode(adapter.AgentCapabilities{Name: "cheap", TaskTypes: []string{"work"}, CostPerRun: 0.2})
		case "/invoke":
			var task adapter.Task
			json.NewDecoder(r.Body).Decode(&task)
			json.NewEncoder(w).Encode(adapter.TaskResult{TaskID: task.ID, Status: "completed", Output: map[string]string{"by": "cheap"}})
		}
	}))
	t.Cleanup(srvCheap.Close)
	srvExpensive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			json.NewEncoder(w).Encode(adapter.HealthStatus{Status: "healthy"})
		case "/declare":
			json.NewEncoder(w).Encode(adapter.AgentCapabilities{Name: "expensive", TaskTypes: []string{"work"}, CostPerRun: 2.0})
		case "/invoke":
			var task adapter.Task
			json.NewDecoder(r.Body).Decode(&task)
			json.NewEncoder(w).Encode(adapter.TaskResult{TaskID: task.ID, Status: "completed", Output: map[string]string{"by": "expensive"}})
		}
	}))
	t.Cleanup(srvExpensive.Close)

	capsCheap, _ := json.Marshal(adapter.AgentCapabilities{Name: "cheap", TaskTypes: []string{"work"}, CostPerRun: 0.2})
	cfgCheap, _ := json.Marshal(map[string]string{"base_url": srvCheap.URL})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, 'http', ?, ?, 'healthy')`,
		"cheap", "cheap", string(cfgCheap), string(capsCheap))
	capsExp, _ := json.Marshal(adapter.AgentCapabilities{Name: "expensive", TaskTypes: []string{"work"}, CostPerRun: 2.0})
	cfgExp, _ := json.Marshal(map[string]string{"base_url": srvExpensive.URL})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, 'http', ?, ?, 'healthy')`,
		"expensive", "expensive", string(cfgExp), string(capsExp))

	engine.RegisterAdapter("cheap", srvCheap.URL, adapter.NewHTTPAdapter(srvCheap.URL))
	engine.RegisterAdapter("expensive", srvExpensive.URL, adapter.NewHTTPAdapter(srvExpensive.URL))

	marketStore := market.NewStore(st.DB)
	engine.WithMarketStore(marketStore)

	cfg := &Config{
		Name:       "market-test",
		Allocation: "market",
		Tasks: []TaskDef{
			{Name: "auction-job", Type: "work"},
		},
	}

	result, err := engine.Run(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)

	// Auction row persisted as closed with the cheaper bidder as winner.
	var count int
	err = st.DB.QueryRow(`SELECT COUNT(*) FROM auctions WHERE status = 'closed'`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "one auction should be closed")

	var winnerAgent string
	err = st.DB.QueryRow(`
		SELECT b.agent_name FROM auctions a
		JOIN bids b ON a.winner_bid_id = b.id
		WHERE a.status = 'closed'`).Scan(&winnerAgent)
	require.NoError(t, err)
	assert.Equal(t, "cheap", winnerAgent, "lowest-cost bidder must win")

	var bidCount int
	err = st.DB.QueryRow(`SELECT COUNT(*) FROM bids`).Scan(&bidCount)
	require.NoError(t, err)
	assert.Equal(t, 2, bidCount, "every capable agent's bid must be persisted")
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
