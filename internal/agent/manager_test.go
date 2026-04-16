package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*Manager, *httptest.Server) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			json.NewEncoder(w).Encode(adapter.HealthStatus{Status: "healthy"})
		case "/declare":
			json.NewEncoder(w).Encode(adapter.AgentCapabilities{
				Name:      "test-agent",
				TaskTypes: []string{"code-review"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return NewManager(store.DB), srv
}

func TestRegisterAgent(t *testing.T) {
	mgr, srv := setupTestManager(t)

	agent, err := mgr.Register(context.Background(), "reviewer", "http", srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "reviewer", agent.Name)
	assert.Equal(t, "http", agent.Type)
	assert.Equal(t, "healthy", agent.HealthStatus)
	assert.NotEmpty(t, agent.ID)
}

func TestRegisterDuplicateNameFails(t *testing.T) {
	mgr, srv := setupTestManager(t)

	_, err := mgr.Register(context.Background(), "reviewer", "http", srv.URL)
	require.NoError(t, err)

	_, err = mgr.Register(context.Background(), "reviewer", "http", srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inserting agent")
}

func TestListAgents(t *testing.T) {
	mgr, srv := setupTestManager(t)

	_, err := mgr.Register(context.Background(), "agent-a", "http", srv.URL)
	require.NoError(t, err)
	_, err = mgr.Register(context.Background(), "agent-b", "http", srv.URL)
	require.NoError(t, err)

	agents, err := mgr.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, agents, 2)
	assert.Equal(t, "agent-a", agents[0].Name) // sorted by name
	assert.Equal(t, "agent-b", agents[1].Name)
}

func TestListEmptyReturnsNil(t *testing.T) {
	mgr, _ := setupTestManager(t)

	agents, err := mgr.List(context.Background())
	require.NoError(t, err)
	assert.Nil(t, agents)
}

func TestRemoveAgent(t *testing.T) {
	mgr, srv := setupTestManager(t)

	_, err := mgr.Register(context.Background(), "to-remove", "http", srv.URL)
	require.NoError(t, err)

	err = mgr.Remove(context.Background(), "to-remove")
	require.NoError(t, err)

	agents, err := mgr.List(context.Background())
	require.NoError(t, err)
	assert.Nil(t, agents)
}

func TestRemoveNonExistentFails(t *testing.T) {
	mgr, _ := setupTestManager(t)

	err := mgr.Remove(context.Background(), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetByName(t *testing.T) {
	mgr, srv := setupTestManager(t)

	_, err := mgr.Register(context.Background(), "findme", "http", srv.URL)
	require.NoError(t, err)

	agent, err := mgr.GetByName(context.Background(), "findme")
	require.NoError(t, err)
	assert.Equal(t, "findme", agent.Name)
}

func TestGetByNameNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.GetByName(context.Background(), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
