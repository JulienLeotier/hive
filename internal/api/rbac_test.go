package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACBlocksViewerOnWrite(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
	keyMgr := NewKeyManager(store.DB)
	users := auth.NewUserStore(store.DB)

	// Two keys: one for a viewer, one for an operator.
	viewerKey, err := keyMgr.Generate(context.Background(), "viewer-key")
	require.NoError(t, err)
	operatorKey, err := keyMgr.Generate(context.Background(), "operator-key")
	require.NoError(t, err)

	require.NoError(t, users.Upsert(context.Background(), auth.UserRecord{Subject: "viewer-key", Role: auth.RoleViewer}))
	require.NoError(t, users.Upsert(context.Background(), auth.UserRecord{Subject: "operator-key", Role: auth.RoleOperator}))

	srv := NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

	// Viewer POST /agents → 403
	req := httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "viewer must not be allowed to write")

	// Viewer GET /agents → 200
	req = httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+viewerKey)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Operator POST /agents → passes RBAC. Empty body returns 400
	// MISSING_FIELDS (handler validates name/type/url); the key invariant
	// here is that we don't get 403 — role resolution let the write through.
	req = httptest.NewRequest("POST", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+operatorKey)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code, "operator must not be 403'd")
}
