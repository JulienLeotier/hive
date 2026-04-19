package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/project"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleAdminStats couvre le GET /admin/stats qui renvoie des
// compteurs de lignes par table. Vérifie que chaque table attendue
// est présente et que le nombre reflète ce qu'on a inséré.
func TestHandleAdminStats(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	// Seed 3 projects.
	store := project.NewStore(st.DB)
	for i := 0; i < 3; i++ {
		_, err := store.Create(context.Background(), "default", "idea", project.CreateOpts{})
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var env struct {
		Data map[string]int64 `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, int64(3), env.Data["projects"])
	// Tables attendues présentes (valeur peut être 0).
	for _, table := range []string{"projects", "stories", "epics", "bmad_phase_steps", "events", "audit_log", "reviews"} {
		_, ok := env.Data[table]
		assert.True(t, ok, "table %q absente de la réponse", table)
	}
}

// TestHandleAdminBulkDeleteFailed supprime les projets failed et
// garde les autres intacts.
func TestHandleAdminBulkDeleteFailed(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))
	store := project.NewStore(st.DB)

	ctx := context.Background()
	p1, err := store.Create(ctx, "default", "keep me", project.CreateOpts{})
	require.NoError(t, err)
	p2, err := store.Create(ctx, "default", "fail me", project.CreateOpts{})
	require.NoError(t, err)
	_, err = st.DB.Exec(`UPDATE projects SET status = 'failed' WHERE id = ?`, p2.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/delete-failed", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var env struct {
		Data struct {
			Deleted    int64    `json:"deleted"`
			ProjectIDs []string `json:"project_ids"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, int64(1), env.Data.Deleted)
	assert.Contains(t, env.Data.ProjectIDs, p2.ID)

	// p1 doit exister encore, p2 disparu.
	_, err = store.GetByID(ctx, p1.ID)
	assert.NoError(t, err)
	_, err = store.GetByID(ctx, p2.ID)
	assert.Error(t, err)
}

// TestHandleAdminUnwedgeStories rewind les stories dev/review quand
// aucun skill n'est running pour le projet.
func TestHandleAdminUnwedgeStories(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))
	store := project.NewStore(st.DB)

	ctx := context.Background()
	p, err := store.Create(ctx, "default", "idea", project.CreateOpts{})
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO epics (id, project_id, title, ordering, status) VALUES (?, ?, 'E', 0, 'pending')`,
		"epc_"+p.ID+"_0", p.ID)
	require.NoError(t, err)
	// Story coincée en dev.
	_, err = st.DB.Exec(
		`INSERT INTO stories (id, epic_id, title, ordering, status, iterations)
		 VALUES (?, ?, 'S', 0, 'dev', 1)`,
		"sty_test", "epc_"+p.ID+"_0")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/unwedge", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var status string
	require.NoError(t, st.DB.QueryRow(
		`SELECT status FROM stories WHERE id = 'sty_test'`).Scan(&status))
	assert.Equal(t, "pending", status, "dev avec 1 iter doit retomber en pending")
}

// TestHandleAdminSweep exécute un sweep (qui ne supprime rien sur une
// DB fraîche) et vérifie la shape de la réponse.
func TestHandleAdminSweep(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sweep", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Data struct {
			Status      string `json:"status"`
			RowsDeleted int64  `json:"rows_deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "swept", env.Data.Status)
	assert.GreaterOrEqual(t, env.Data.RowsDeleted, int64(0))
}
