package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/project"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedProject crée un projet minimal avec epic+story+ACs pour tester
// les endpoints qui ont besoin d'un shape réaliste.
func seedSimpleProject(t *testing.T, db *storage.Store) string {
	t.Helper()
	store := project.NewStore(db.DB)
	p, err := store.Create(context.Background(), "default", "test idée", project.CreateOpts{
		Workdir: "/tmp/test",
	})
	require.NoError(t, err)
	return p.ID
}

// TestHandleResumeProject : /resume clear paused=0 et renvoie 200.
func TestHandleResumeProject(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	id := seedSimpleProject(t, st)
	// Simule un cancel qui a posé paused=1.
	_, err = st.DB.Exec(`UPDATE projects SET paused = 1 WHERE id = ?`, id)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/resume", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Vérifie en DB.
	var paused int
	require.NoError(t, st.DB.QueryRow(`SELECT paused FROM projects WHERE id = ?`, id).Scan(&paused))
	assert.Equal(t, 0, paused)
}

func TestHandleResumeProjectNotFound(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/prj_nope/resume", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleProjectReport : renvoie un Markdown bien formé avec les
// sections attendues (méta, PRD, plan, coût par phase).
func TestHandleProjectReport(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	id := seedSimpleProject(t, st)
	_, err = st.DB.Exec(`UPDATE projects SET prd = 'Ceci est le PRD' WHERE id = ?`, id)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+id+"/report.md", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/markdown")
	body := w.Body.String()

	assert.Contains(t, body, "# ") // titre
	assert.Contains(t, body, "## Idée")
	assert.Contains(t, body, "test idée")
	assert.Contains(t, body, "## Méta")
	assert.Contains(t, body, "## PRD")
	assert.Contains(t, body, "Ceci est le PRD")
}

func TestHandleProjectReportNotFound(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/prj_nope/report.md", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleCancelPhaseStepInvalid couvre les paths d'erreur sans
// toucher au registre stepCancels (pas de claude à invoquer).
func TestHandleCancelPhaseStepInvalid(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	cases := []struct {
		path   string
		status int
	}{
		{"/api/v1/phases/0/cancel", http.StatusBadRequest},
		{"/api/v1/phases/abc/cancel", http.StatusBadRequest},
		{"/api/v1/phases/99999/cancel", http.StatusConflict}, // existe pas → pas de cancel possible
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, c.path, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		assert.Equal(t, c.status, w.Code, "path=%s", c.path)
	}
}

// TestHandleRerunPhaseStepNotFound couvre le path où l'id n'existe pas.
func TestHandleRerunPhaseStepNotFound(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/phases/99999/rerun", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleRerunPhaseStepBadCommand : un phase_step avec une commande
// qui ne commence pas par /bmad- doit être refusé.
func TestHandleRerunPhaseStepBadCommand(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	id := seedSimpleProject(t, st)
	res, err := st.DB.Exec(
		`INSERT INTO bmad_phase_steps (project_id, phase, command, status)
		 VALUES (?, 'story', 'rm -rf /', 'done')`, id)
	require.NoError(t, err)
	stepID, _ := res.LastInsertId()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/phases/"+strOf(stepID)+"/rerun", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var envelope struct {
		Error *struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &envelope)
	if envelope.Error != nil {
		assert.Equal(t, "BAD_COMMAND", envelope.Error.Code)
	}
}

// strOf local pour éviter la dépendance à devloop.strOf.
func strOf(n int64) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if negative {
		out = append([]byte{'-'}, out...)
	}
	return strings.TrimSpace(string(out))
}
