package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JulienLeotier/hive/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProjectRequiresIdea(t *testing.T) {
	srv := setupServer(t)
	srv.WithProjectStore(project.NewStore(srv.db()))

	req := httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateProjectReturnsDraft(t *testing.T) {
	srv := setupServer(t)
	srv.WithProjectStore(project.NewStore(srv.db()))

	body := strings.NewReader(`{"name":"writer-app","idea":"app for writers with AI assistance"}`)
	req := httptest.NewRequest("POST", "/api/v1/projects", body)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, "writer-app", resp.Data["name"])
	assert.Equal(t, "draft", resp.Data["status"])
	assert.Equal(t, "app for writers with AI assistance", resp.Data["idea"])
	id, _ := resp.Data["id"].(string)
	assert.True(t, strings.HasPrefix(id, "prj_"), "id should be prefixed so users recognise it in logs")
}

func TestGetProjectReturnsTree(t *testing.T) {
	srv := setupServer(t)
	store := project.NewStore(srv.db())
	srv.WithProjectStore(store)

	p, err := store.Create(httptest.NewRequest("POST", "/", nil).Context(),
		"default", "a small todo app", project.CreateOpts{Name: "demo"})
	require.NoError(t, err)

	// Seed one epic + one story + 2 ACs so we exercise the tree walk.
	_, err = srv.db().Exec(
		`INSERT INTO epics (id, project_id, title, ordering, status) VALUES (?, ?, ?, ?, ?)`,
		"epc_1", p.ID, "Auth flow", 0, "pending")
	require.NoError(t, err)
	_, err = srv.db().Exec(
		`INSERT INTO stories (id, epic_id, title, ordering, status) VALUES (?, ?, ?, ?, ?)`,
		"sty_1", "epc_1", "User can sign in", 0, "pending")
	require.NoError(t, err)
	_, err = srv.db().Exec(
		`INSERT INTO acceptance_criteria (story_id, ordering, text, passed) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		"sty_1", 0, "sign in form exists", 0,
		"sty_1", 1, "bad password shows an error", 0)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+p.ID, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	epics, _ := resp.Data["epics"].([]any)
	require.Len(t, epics, 1)
	stories, _ := epics[0].(map[string]any)["stories"].([]any)
	require.Len(t, stories, 1)
	acs, _ := stories[0].(map[string]any)["acceptance_criteria"].([]any)
	assert.Len(t, acs, 2, "both ACs must come back attached to the right story")
}

func TestDeleteProjectCascades(t *testing.T) {
	srv := setupServer(t)
	store := project.NewStore(srv.db())
	srv.WithProjectStore(store)

	p, err := store.Create(httptest.NewRequest("POST", "/", nil).Context(),
		"default", "an idea", project.CreateOpts{Name: "x"})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/"+p.ID, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var count int
	require.NoError(t, srv.db().QueryRow(`SELECT COUNT(*) FROM projects WHERE id = ?`, p.ID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestGetProjectNotFoundReturns404(t *testing.T) {
	srv := setupServer(t)
	srv.WithProjectStore(project.NewStore(srv.db()))

	req := httptest.NewRequest("GET", "/api/v1/projects/prj_ghost", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
