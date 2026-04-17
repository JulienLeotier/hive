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

func TestSetupStatusBeforeAndAfter(t *testing.T) {
	srv := setupServer(t)
	users := auth.NewUserStore(srv.db())
	srv.WithUsers(users)

	// Fresh — needs setup.
	req := httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data map[string]bool `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Data["needs_setup"])

	// Add a user → no longer needs setup.
	require.NoError(t, users.Upsert(req.Context(), auth.UserRecord{
		Subject: "alice", Role: auth.RoleAdmin, TenantID: "default",
	}))
	req = httptest.NewRequest("GET", "/api/v1/setup/status", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.Data["needs_setup"])
}

func TestSetupBootstrapCreatesAdminAndKey(t *testing.T) {
	srv := setupServer(t)
	srv.WithUsers(auth.NewUserStore(srv.db()))

	body := strings.NewReader(`{"subject":"admin@example.com"}`)
	req := httptest.NewRequest("POST", "/api/v1/setup/bootstrap", body)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "admin@example.com", resp.Data["subject"])
	assert.Equal(t, string(auth.RoleAdmin), resp.Data["role"])
	assert.NotEmpty(t, resp.Data["api_key"], "raw key must be returned once")
	assert.True(t, strings.HasPrefix(resp.Data["api_key"], "hive_"))
}

func TestSetupBootstrapRejectedAfterInitialConfig(t *testing.T) {
	srv := setupServer(t)
	users := auth.NewUserStore(srv.db())
	srv.WithUsers(users)

	// Seed a user so setup is already done.
	require.NoError(t, users.Upsert(
		httptest.NewRequest("POST", "/", nil).Context(),
		auth.UserRecord{Subject: "already", Role: auth.RoleAdmin, TenantID: "default"},
	))

	req := httptest.NewRequest("POST", "/api/v1/setup/bootstrap", strings.NewReader(`{"subject":"new"}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestSetupBootstrapRejectsMissingSubject(t *testing.T) {
	srv := setupServer(t)
	srv.WithUsers(auth.NewUserStore(srv.db()))
	req := httptest.NewRequest("POST", "/api/v1/setup/bootstrap", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
