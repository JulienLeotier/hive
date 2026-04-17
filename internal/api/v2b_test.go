package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserInsertsRecord(t *testing.T) {
	srv := setupServer(t)
	users := auth.NewUserStore(srv.db())
	srv.WithUsers(users)
	// Seed an admin so the writer isn't blocked by the setup gate.
	require.NoError(t, users.Upsert(httptest.NewRequest("POST", "/", nil).Context(),
		auth.UserRecord{Subject: "admin", Role: auth.RoleAdmin, TenantID: "default"}))

	body := strings.NewReader(`{"subject":"bob","role":"operator","tenant_id":"team-b"}`)
	req := httptest.NewRequest("POST", "/api/v1/users", body)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	rec, err := users.Get(req.Context(), "bob")
	require.NoError(t, err)
	assert.Equal(t, auth.Role("operator"), rec.Role)
	assert.Equal(t, "team-b", rec.TenantID)
}

func TestCreateUserRejectsUnknownRole(t *testing.T) {
	srv := setupServer(t)
	srv.WithUsers(auth.NewUserStore(srv.db()))

	body := strings.NewReader(`{"subject":"x","role":"superuser"}`)
	req := httptest.NewRequest("POST", "/api/v1/users", body)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteUserRemovesRecord(t *testing.T) {
	srv := setupServer(t)
	users := auth.NewUserStore(srv.db())
	srv.WithUsers(users)
	require.NoError(t, users.Upsert(httptest.NewRequest("POST", "/", nil).Context(),
		auth.UserRecord{Subject: "alice", Role: auth.RoleOperator, TenantID: "default"}))

	req := httptest.NewRequest("DELETE", "/api/v1/users/alice", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	_, err := users.Get(req.Context(), "alice")
	assert.Error(t, err)
}

func TestCreateWorkflowAcceptsYAML(t *testing.T) {
	srv := setupServer(t)

	yaml := "name: my-wf\ntasks:\n  - name: review\n    type: code-review\n"
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(yaml))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "my-wf", resp.Data["name"])
}

func TestCreateWorkflowRejectsBadYAML(t *testing.T) {
	srv := setupServer(t)
	// Duplicate task names — parser should reject.
	yaml := "name: bad\ntasks:\n  - name: x\n    type: t\n  - name: x\n    type: t\n"
	req := httptest.NewRequest("POST", "/api/v1/workflows", strings.NewReader(yaml))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteWorkflowByName(t *testing.T) {
	srv := setupServer(t)

	// Seed a workflow row.
	_, err := srv.db().Exec(
		`INSERT INTO workflows (id, name, config, status) VALUES (?, ?, ?, 'idle')`,
		"wf-1", "to-delete", "{}",
	)
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/workflows/to-delete", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var remaining int
	require.NoError(t, srv.db().QueryRow(
		`SELECT COUNT(*) FROM workflows WHERE name = ?`, "to-delete",
	).Scan(&remaining))
	assert.Equal(t, 0, remaining)
}
