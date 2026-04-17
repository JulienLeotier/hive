package api

import (
	"context"
	"encoding/json"
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

// TestTenantIsolation_Agents proves that a caller authenticated under tenant A
// cannot see agents that belong to tenant B. This is a regression guard for a
// previously silent data leak: TenantFromContext was defined but never used
// by handlers, so every list endpoint returned cross-tenant rows.
func TestTenantIsolation_Agents(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Seed two agents in two tenants.
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO agents (id, name, type, config, capabilities, tenant_id)
		 VALUES ('id-a', 'agent-tenant-a', 'test', '{}', '{}', 'tenant-a')`)
	require.NoError(t, err)
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO agents (id, name, type, config, capabilities, tenant_id)
		 VALUES ('id-b', 'agent-tenant-b', 'test', '{}', '{}', 'tenant-b')`)
	require.NoError(t, err)

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
	keyMgr := NewKeyManager(store.DB)
	users := auth.NewUserStore(store.DB)

	// Two keys, two tenants, both viewers.
	keyA, err := keyMgr.Generate(ctx, "key-a")
	require.NoError(t, err)
	keyB, err := keyMgr.Generate(ctx, "key-b")
	require.NoError(t, err)
	require.NoError(t, users.Upsert(ctx, auth.UserRecord{Subject: "key-a", Role: auth.RoleViewer, TenantID: "tenant-a"}))
	require.NoError(t, users.Upsert(ctx, auth.UserRecord{Subject: "key-b", Role: auth.RoleViewer, TenantID: "tenant-b"}))

	srv := NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

	// Caller in tenant A must see only agent-tenant-a.
	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+keyA)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var respA struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &respA))
	names := make([]string, 0, len(respA.Data))
	for _, a := range respA.Data {
		names = append(names, a.Name)
	}
	assert.ElementsMatch(t, []string{"agent-tenant-a"}, names,
		"tenant A caller must not see agent-tenant-b")

	// Symmetric check for tenant B.
	req = httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+keyB)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var respB struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &respB))
	names = names[:0]
	for _, a := range respB.Data {
		names = append(names, a.Name)
	}
	assert.ElementsMatch(t, []string{"agent-tenant-b"}, names,
		"tenant B caller must not see agent-tenant-a")
}

// TestTenantIsolation_ReadEndpoints exercises every list endpoint that the
// tenantFilter helper now protects. Each seeds two rows with different
// tenant_id values and asserts that a viewer scoped to tenant-a never sees
// tenant-b data. Failing this is a data leak.
func TestTenantIsolation_ReadEndpoints(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	seed := []struct{ q string; args []any }{
		{`INSERT INTO events (type, source, payload, tenant_id) VALUES ('e','s','{}','tenant-a'),('e','s','{}','tenant-b')`, nil},
		{`INSERT INTO workflows (id, name, status, tenant_id) VALUES ('wf-a','a','idle','tenant-a'),('wf-b','b','idle','tenant-b')`, nil},
		{`INSERT INTO knowledge (task_type, approach, outcome, tenant_id) VALUES ('t','a','o','tenant-a'),('t','a','o','tenant-b')`, nil},
		{`INSERT INTO audit_log (action, actor, resource, tenant_id) VALUES ('x','u','r','tenant-a'),('x','u','r','tenant-b')`, nil},
		{`INSERT INTO cluster_members (node_id, hostname, address, status, last_heartbeat, tenant_id) VALUES ('n-a','h','a','active',datetime('now'),'tenant-a'),('n-b','h','b','active',datetime('now'),'tenant-b')`, nil},
	}
	for _, s := range seed {
		_, err := store.DB.ExecContext(ctx, s.q, s.args...)
		require.NoError(t, err, s.q)
	}

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
	keyMgr := NewKeyManager(store.DB)
	users := auth.NewUserStore(store.DB)

	keyA, err := keyMgr.Generate(ctx, "key-a")
	require.NoError(t, err)
	// Admin role so the caller can read the "system:*" endpoints (knowledge,
	// audit, cluster — only admin has these permissions in the RBAC policy).
	// Tenant isolation is enforced independently of RBAC: even an admin
	// scoped to tenant-a must not see tenant-b data. This guards both.
	require.NoError(t, users.Upsert(ctx, auth.UserRecord{Subject: "key-a", Role: auth.RoleAdmin, TenantID: "tenant-a"}))

	srv := NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

	// Each endpoint returns an array of objects. We don't need to fully
	// parse each shape — just verify the JSON has one row and that the row
	// does NOT mention tenant-b.
	endpoints := []string{
		"/api/v1/events",
		"/api/v1/workflows",
		"/api/v1/knowledge",
		"/api/v1/audit",
		"/api/v1/cluster",
	}
	for _, url := range endpoints {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer "+keyA)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			var env struct {
				Data []map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), url)
			assert.Len(t, env.Data, 1,
				"tenant A caller should see exactly its own row in %s (got %d)", url, len(env.Data))
			assert.NotContains(t, w.Body.String(), "tenant-b",
				"tenant A caller must not see tenant-b rows in %s", url)
		})
	}
}

// TestTenantIsolation_Tasks does the same check for the tasks endpoint,
// which uses a direct SQL query in the handler rather than a manager method.
func TestTenantIsolation_Tasks(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, tenant_id)
		 VALUES ('t-a', 'wf-1', 'test', 'pending', 'tenant-a'),
		        ('t-b', 'wf-2', 'test', 'pending', 'tenant-b')`)
	require.NoError(t, err)

	mgr := agent.NewManager(store.DB)
	bus := event.NewBus(store.DB)
	breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
	keyMgr := NewKeyManager(store.DB)
	users := auth.NewUserStore(store.DB)

	keyA, err := keyMgr.Generate(ctx, "key-a")
	require.NoError(t, err)
	require.NoError(t, users.Upsert(ctx, auth.UserRecord{Subject: "key-a", Role: auth.RoleViewer, TenantID: "tenant-a"}))

	srv := NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

	req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+keyA)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	ids := make([]string, 0, len(resp.Data))
	for _, t := range resp.Data {
		ids = append(ids, t.ID)
	}
	assert.ElementsMatch(t, []string{"t-a"}, ids,
		"tenant A caller must not see t-b")
}
