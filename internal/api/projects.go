package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/bmad"
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

	// Un projet est « brownfield » dès qu'on part d'un code existant :
	// clone d'un repo GitHub, ou repo_path local fourni. Hive
	// basculera alors le pipeline BMAD sur IterationPipeline
	// (bmad-document-project + bmad-edit-prd + …) au lieu de
	// FullPlanningPipeline qui part d'une page blanche.
	isExisting := body.CloneRepo != "" || body.RepoPath != ""

	tenant, _ := auth.TenantFromContext(r.Context())
	p, err := s.projectStore.Create(r.Context(), tenant, body.Idea, project.CreateOpts{
		Name:           body.Name,
		Workdir:        workdir,
		BMADOutputPath: body.BMADOutputPath,
		RepoPath:       body.RepoPath,
		RepoURL:        repoURL,
		IsExisting:     isExisting,
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

// handleGhLogin authentifie `gh` avec un personal access token
// fourni par l'UI. Le token transite UNE SEULE FOIS — il est
// immédiatement piped dans `gh auth login --with-token` qui le
// stocke dans ~/.config/gh. Hive ne persiste pas le PAT.
//
// Scopes minimaux attendus sur le token : `repo` (read/write des
// repos), `workflow` (pour CI), `read:org` (pour cloner des repos
// d'organisation).
func (s *Server) handleGhLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TOKEN",
			"token GitHub requis — génère-en un sur https://github.com/settings/tokens/new")
		return
	}
	if err := git.LoginWithToken(r.Context(), body.Token); err != nil {
		writeError(w, http.StatusBadRequest, "GH_LOGIN_FAILED", err.Error())
		return
	}
	writeJSON(w, git.CheckGh(r.Context()))
}

// handleGhRepos retourne la liste des repos accessibles à l'opérateur
// pour alimenter l'autocomplete du champ clone.
func (s *Server) handleGhRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := git.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, "GH_LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, repos)
}

// handleGhLogout supprime l'auth gh locale.
func (s *Server) handleGhLogout(w http.ResponseWriter, r *http.Request) {
	if err := git.Logout(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "GH_LOGOUT_FAILED", err.Error())
		return
	}
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

// handleRetrospective déclenche manuellement la rétrospective BMAD
// (bmad-agent-dev + bmad-retrospective) pour un projet. Utile quand
// le trigger auto (fin d'epic détectée par epicComplete) ne s'est
// pas fait feu ou pour forcer un lessons-learned après une itération.
func (s *Server) handleRetrospective(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if p.Workdir == "" {
		writeError(w, http.StatusBadRequest, "NO_WORKDIR", "projet sans workdir")
		return
	}
	runner := bmad.NewRunner()
	if runner == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_CLAUDE",
			"CLI claude absente — rétrospective nécessite claude")
		return
	}
	//nolint:gosec // G118: retrospective tourne détachée pour ne pas bloquer la requête
	go func(wd, pid string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		obs := s.stepObserver(ctx, pid, "retrospective")
		if _, err := runner.RunSequenceObserved(ctx, wd, bmad.RetrospectiveSequence, obs); err != nil {
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.retrospective_failed", "api",
					map[string]any{"project_id": pid, "error": err.Error()})
			}
			return
		}
		if s.eventBus != nil {
			_, _ = s.eventBus.Publish(ctx, "project.retrospective_done", "api",
				map[string]string{"project_id": pid})
		}
	}(p.Workdir, p.ID)
	writeJSON(w, map[string]string{"project_id": p.ID, "status": "retrospective-scheduled"})
}

// handleCancelRun annule le pipeline BMAD en cours sur un projet.
// Ferme le ctx de la goroutine runArchitectAsync / runIterationAsync ;
// les `claude --print` en cours reçoivent SIGKILL via exec.CommandContext.
// Le projet passe en status `failed` avec failure_stage=cancelled.
func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "project id required")
		return
	}
	if !s.cancelRun(id) {
		writeError(w, http.StatusConflict, "NO_RUN",
			"aucun build BMAD en cours pour ce projet")
		return
	}
	_, _ = s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, failure_stage = 'cancelled',
		 failure_error = 'Build annulé par l''opérateur',
		 updated_at = datetime('now') WHERE id = ?`,
		project.StatusFailed, id)
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(r.Context(), "project.cancelled", "api",
			map[string]string{"project_id": id})
	}
	writeJSON(w, map[string]string{"project_id": id, "status": "cancelled"})
}

// handleRetryArchitect relance le pipeline BMAD depuis le début pour
// un projet qui s'est planté (status=failed OU coincé en planning).
// Efface failure_stage/error et re-fire runArchitectAsync ou
// runIterationAsync selon is_existing.
func (s *Server) handleRetryArchitect(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	// Sécurité : n'accepte que les projets échoués ou bloqués en
	// planning. Un projet building qui a des stories done ne doit
	// pas se faire écraser le PRD par un retry.
	if p.Status != project.StatusFailed && p.Status != project.StatusPlanning {
		writeError(w, http.StatusConflict, "BAD_STATE",
			"retry autorisé uniquement sur les projets failed ou planning")
		return
	}
	// Cancel un run éventuellement zombie avant de relancer.
	s.cancelRun(p.ID)
	_, _ = s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, failure_stage = NULL,
		 failure_error = NULL, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, p.ID)

	// On récupère la conversation d'intake pour reseed le brief.
	var seed string
	if s.intakeStore != nil {
		if conv, _ := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgentFor(p)); conv != nil {
			seed = flattenConversation(conv)
		}
	}
	go s.runArchitectAsync(p.ID, p.Idea, seed) //nolint:gosec // G118: request ctx dies; the goroutine uses its own 90-min ctx

	writeJSON(w, map[string]string{"project_id": p.ID, "status": "retry-scheduled"})
}

// handleProjectPhases liste les 50 dernières invocations de skill
// BMAD pour un projet : commande, statut (running/done/failed),
// durée, tokens, coût. Le dashboard s'en sert pour afficher un feed
// temps réel « skill 4/13 en cours : /bmad-create-architecture ».
func (s *Server) handleProjectPhases(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "project id required")
		return
	}
	type step struct {
		ID           int64   `json:"id"`
		Phase        string  `json:"phase"`
		Command      string  `json:"command"`
		StartedAt    string  `json:"started_at"`
		FinishedAt   string  `json:"finished_at,omitempty"`
		Status       string  `json:"status"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"`
		Preview      string  `json:"reply_preview,omitempty"`
		Error        string  `json:"error,omitempty"`
	}
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT id, phase, command, started_at, COALESCE(finished_at, ''),
		        status, input_tokens, output_tokens, cost_usd,
		        COALESCE(reply_preview, ''), COALESCE(error_text, '')
		 FROM bmad_phase_steps WHERE project_id = ?
		 ORDER BY id DESC LIMIT 50`, id)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			writeJSON(w, []step{})
			return
		}
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	var out []step
	for rows.Next() {
		var s step
		if err := rows.Scan(&s.ID, &s.Phase, &s.Command, &s.StartedAt,
			&s.FinishedAt, &s.Status, &s.InputTokens, &s.OutputTokens,
			&s.CostUSD, &s.Preview, &s.Error); err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
		return
	}
	writeJSON(w, out)
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
