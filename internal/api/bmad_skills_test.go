package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/project"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBmadSkillsList vérifie que GET /api/v1/bmad/skills renvoie le
// registre et que le filtre ?scope=story ne garde que les skills
// story-scoped. Sert de canari quand on ajoute / retire un skill.
func TestBmadSkillsList(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bmad/skills", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Data []bmad.Skill `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.GreaterOrEqual(t, len(env.Data), 5, "le registre doit avoir au moins 5 skills")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bmad/skills?scope=story", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var env2 struct {
		Data []bmad.Skill `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env2))
	for _, s := range env2.Data {
		assert.Equal(t, bmad.ScopeStory, s.Scope, "filtre story ne doit retourner que des story-scoped")
	}
	assert.NotEmpty(t, env2.Data, "au moins un skill story-scoped doit exister")
}

// TestBmadRunValidation vérifie que POST /api/v1/bmad/run refuse les
// payloads invalides : skill absent, scope mismatch, project inconnu.
// On NE teste PAS l'invocation Claude (pas de CLI dispo en CI) — la
// partie exécution est couverte par les tests existants sur trackedInvoke.
func TestBmadRunValidation(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))

	// Seed un projet sans workdir.
	_, err = st.DB.Exec(
		`INSERT INTO projects (id, name, idea, status, tenant_id)
		 VALUES ('prj_test', 'demo', 'idea', 'building', 'default')`)
	require.NoError(t, err)

	cases := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{
			name:   "skill manquant",
			body:   map[string]any{"project_id": "prj_test"},
			status: http.StatusBadRequest, code: "BAD_REQUEST",
		},
		{
			name: "skill inconnu",
			body: map[string]any{
				"skill": "/bmad-not-real", "project_id": "prj_test",
			},
			status: http.StatusBadRequest, code: "UNKNOWN_SKILL",
		},
		{
			name: "scope mismatch — story-scoped sans story_id",
			body: map[string]any{
				"skill": "/bmad-code-review", "project_id": "prj_test",
			},
			status: http.StatusBadRequest, code: "SCOPE_MISMATCH",
		},
		{
			name: "scope mismatch — project-scoped avec story_id",
			body: map[string]any{
				"skill": "/bmad-validate-prd", "project_id": "prj_test",
				"story_id": "sty_x",
			},
			status: http.StatusBadRequest, code: "SCOPE_MISMATCH",
		},
		{
			name: "projet sans workdir",
			body: map[string]any{
				"skill": "/bmad-validate-prd", "project_id": "prj_test",
			},
			status: http.StatusConflict, code: "NO_WORKDIR",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, _ := json.Marshal(c.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/bmad/run", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			assert.Equal(t, c.status, w.Code)
			var resp struct {
				Error *struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Error != nil {
				assert.Equal(t, c.code, resp.Error.Code)
			}
		})
	}
}
