package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/project"
)

// handleListProjects returns every BMAD project visible to the caller's
// tenant. The list endpoint intentionally omits the epic tree — the
// dashboard fetches that per-project via GET /projects/{id}.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	tenant, _ := auth.TenantFromContext(r.Context())
	projects, err := s.projectStore.List(r.Context(), tenant, parseLimit(r, 100, 500))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, projects)
}

// handleGetProject returns a single project with its full epic + story +
// AC tree. The tree is the BMAD project status board — everything the
// dashboard's detail page needs to render progress.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, p)
}

// handleCreateProject creates a BMAD project in `draft` state. Body is
// {name?, idea, workdir?}. The project stays in draft until the PM agent
// finishes its Q&A and produces a PRD (Phase 2 wiring) — until then the
// dashboard routes the user through the interactive intake.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	var body struct {
		Name           string `json:"name"`
		Idea           string `json:"idea"`
		Workdir        string `json:"workdir"`
		BMADOutputPath string `json:"bmad_output_path"`
		RepoPath       string `json:"repo_path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if body.Idea == "" {
		writeError(w, http.StatusBadRequest, "MISSING_IDEA",
			"idea is required — describe what you want built in plain language")
		return
	}
	tenant, _ := auth.TenantFromContext(r.Context())
	p, err := s.projectStore.Create(r.Context(), tenant, body.Idea, project.CreateOpts{
		Name:           body.Name,
		Workdir:        body.Workdir,
		BMADOutputPath: body.BMADOutputPath,
		RepoPath:       body.RepoPath,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, p)
}

// handleRetryStory clears a blocked story's iteration counter so the
// devloop picks it back up on the next tick. Without this endpoint, a
// story that exhausts MaxIterations leaves the whole project wedged —
// the only recovery path is manual SQL, which we don't want operators
// reaching for. Only applies to stories in status `blocked`; any other
// status is left alone so we don't rewind in-flight work.
func (s *Server) handleRetryStory(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	projectID := r.PathValue("id")
	storyID := r.PathValue("story_id")
	if projectID == "" || storyID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "project and story id required")
		return
	}
	if _, err := s.projectStore.GetByID(r.Context(), projectID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	// Only reset when the story is genuinely blocked and belongs to this project.
	res, err := s.db().ExecContext(r.Context(),
		`UPDATE stories
		 SET status = 'pending', iterations = 0, updated_at = datetime('now')
		 WHERE id = ?
		   AND status = 'blocked'
		   AND epic_id IN (SELECT id FROM epics WHERE project_id = ?)`,
		storyID, projectID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusConflict, "NOT_BLOCKED",
			"story is not in blocked state (or doesn't belong to this project)")
		return
	}
	// If the project had been flipped to `failed`/`review` because of this
	// blockage, nudge it back to `building` so the supervisor will tick it.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = 'building', updated_at = datetime('now')
		 WHERE id = ? AND status IN ('review','failed')`,
		projectID,
	); err != nil {
		// best-effort; the story reset is what matters
		_ = err
	}
	writeJSON(w, map[string]any{
		"status":   "retrying",
		"story_id": storyID,
	})
}

// handleDeleteProject removes a project and cascades its epic/story tree.
// Currently-building projects aren't stopped automatically — that's a
// Phase 3 concern when the BMADEngine orchestrator exists.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	if err := s.projectStore.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "removed", "id": id})
}
