package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/git"
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
		// Options GitHub — exclusives. Au plus une des trois :
		//   - CloneRepo : URL ou owner/name à cloner dans workdir.
		//   - CreateRepo : nom du nouveau repo à créer via gh.
		//   - (les deux vides) : pas d'intégration GitHub.
		CloneRepo        string `json:"clone_repo"`
		CreateRepo       string `json:"create_repo"`
		RepoVisibility   string `json:"repo_visibility"` // public|private|internal
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
	if body.CloneRepo != "" && body.CreateRepo != "" {
		writeError(w, http.StatusBadRequest, "CONFLICTING_GIT_OPTIONS",
			"choisis entre cloner un repo existant OU en créer un nouveau, pas les deux")
		return
	}

	repoURL := ""
	workdir := body.Workdir

	switch {
	case body.CloneRepo != "":
		if workdir == "" {
			writeError(w, http.StatusBadRequest, "MISSING_WORKDIR",
				"workdir est requis quand on clone un repo")
			return
		}
		if err := git.CloneRepo(r.Context(), body.CloneRepo, workdir); err != nil {
			writeError(w, http.StatusBadRequest, "GIT_CLONE_FAILED", err.Error())
			return
		}
		if url, err := git.RemoteURL(r.Context(), workdir); err == nil {
			repoURL = url
		}

	case body.CreateRepo != "":
		if workdir == "" {
			writeError(w, http.StatusBadRequest, "MISSING_WORKDIR",
				"workdir est requis quand on crée un repo")
			return
		}
		url, err := git.CreateRepo(r.Context(), body.CreateRepo, workdir, body.RepoVisibility)
		if err != nil {
			writeError(w, http.StatusBadRequest, "GIT_CREATE_FAILED", err.Error())
			return
		}
		repoURL = url
	}

	tenant, _ := auth.TenantFromContext(r.Context())
	p, err := s.projectStore.Create(r.Context(), tenant, body.Idea, project.CreateOpts{
		Name:           body.Name,
		Workdir:        workdir,
		BMADOutputPath: body.BMADOutputPath,
		RepoPath:       body.RepoPath,
		RepoURL:        repoURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, p)
}

// handleGhStatus expose l'état de la CLI `gh` (installée,
// authentifiée, login). Utilisé par le formulaire de création pour
// montrer / masquer les options GitHub et guider le user vers
// `gh auth login` si besoin.
func (s *Server) handleGhStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, git.CheckGh(r.Context()))
}

// handleUpdatePRD lets the operator tweak the saved PRD text. Allowed
// in any lifecycle state except `shipped` (which is frozen by design).
// The PRD is the input to the Architect; editing it alone doesn't
// rebuild the plan — the operator has to call regenerate-plan for that.
func (s *Server) handleUpdatePRD(w http.ResponseWriter, r *http.Request) {
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
	if p.Status == project.StatusShipped {
		writeError(w, http.StatusConflict, "PROJECT_SHIPPED",
			"cannot edit the PRD of a shipped project")
		return
	}
	var body struct {
		PRD string `json:"prd"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(body.PRD) == "" {
		writeError(w, http.StatusBadRequest, "EMPTY_PRD",
			"prd cannot be empty — use regenerate-plan to restart from scratch")
		return
	}
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET prd = ?, updated_at = datetime('now') WHERE id = ?`,
		body.PRD, id,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{"project_id": id, "prd_length": len(body.PRD)})
}

// handleRegeneratePlan wipes the current epic/story/AC tree and re-runs
// the Architect on the saved PRD. Guarded so it can't clobber work in
// progress: rejects if any story has iterations > 0 (meaning the dev
// loop has already touched it). On success the project is left in
// `planning` while the Architect runs in the background; a new
// project.architect_* event cycle drives the UI.
func (s *Server) handleRegeneratePlan(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(p.PRD) == "" {
		writeError(w, http.StatusBadRequest, "NO_PRD",
			"project has no PRD yet — finalise the intake first")
		return
	}
	// Refuse if the dev loop has already started doing work anywhere in
	// the tree. Iteration 1 means dev+review ran, even if it failed —
	// that's committed effort we don't want to silently discard.
	var busy int
	if err := s.db().QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM stories s
		 JOIN epics e ON e.id = s.epic_id
		 WHERE e.project_id = ? AND s.iterations > 0`, id,
	).Scan(&busy); err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	if busy > 0 {
		writeError(w, http.StatusConflict, "BUILD_STARTED",
			"cannot regenerate the plan — at least one story has iterations; delete the project instead")
		return
	}
	// Cascade delete the tree. epics.ON DELETE CASCADE takes out stories
	// and ACs; reviews also cascade off stories.
	if _, err := s.db().ExecContext(r.Context(),
		`DELETE FROM epics WHERE project_id = ?`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	// Flip the project back to planning so the UI shows the spinner and
	// the supervisor won't pick it up mid-regeneration.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, id,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	go s.runArchitectAsync(p.ID, p.Idea, p.PRD) //nolint:gosec // G118: same pattern as finalize — request ctx would cancel the architect mid-run
	writeJSON(w, map[string]any{"project_id": id, "status": project.StatusPlanning})
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
