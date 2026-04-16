package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupServer(t *testing.T) *Server {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
	keyMgr := NewKeyManager(store.DB)

	return NewServer(mgr, bus, breakers, keyMgr)
}

func TestListAgentsEndpoint(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Nil(t, resp.Error)
}

func TestMetricsEndpoint(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Nil(t, resp.Error)

	data := resp.Data.(map[string]any)
	agents := data["agents"].(map[string]any)
	assert.Equal(t, float64(0), agents["total"])
}

func TestEventsEndpoint(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("GET", "/api/v1/events?type=task", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
