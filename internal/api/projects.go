package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
		CloneRepo      string `json:"clone_repo"`
		CreateRepo     string `json:"create_repo"`
		RepoVisibility string `json:"repo_visibility"` // public|private|internal
		// Garde-fou budget Claude. 0 = pas de cap.
		CostCapUSD float64 `json:"cost_cap_usd"`
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
		CostCapUSD:     body.CostCapUSD,
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

// handleExportProject streams a .tar.gz containing everything about a
// project : le workdir complet (code + BMAD artefacts), le PRD, l'arbre
// epics/stories, l'historique phases en JSON. But : permettre à
// l'opérateur de sauvegarder / auditer / partager un projet hors Hive.
//
// Le tarball contient :
//   workdir/          → copie exhaustive du répertoire de travail
//   project.json      → métadonnées DB (name, idea, status, cost, etc.)
//   epics.json        → arbre epics/stories/ACs
//   phases.json       → historique complet bmad_phase_steps
//   intake.json       → conversation PM + messages
//
// Streamé en chunks pour ne pas charger 100MB en RAM.
func (s *Server) handleExportProject(w http.ResponseWriter, r *http.Request) {
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

	filename := "hive-project-" + p.ID + ".tar.gz"
	if p.Name != "" {
		// Sanitise name : alphanumeric + _ seulement, sinon on garde l'ID.
		clean := sanitizeFilename(p.Name)
		if clean != "" {
			filename = "hive-" + clean + ".tar.gz"
		}
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	gz := gzip.NewWriter(w)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	// --- 1. project.json : métadonnées + arbre epics/stories/ACs déjà
	// chargé par GetByID (p.Epics, chaque Epic a ses stories, chaque
	// story ses acceptance criteria).
	if err := writeTarJSON(tw, "project.json", p); err != nil {
		slog.Warn("export: project.json failed", "error", err)
		return
	}

	// --- 3. phases.json : historique phases
	phases := []map[string]any{}
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT id, phase, command, started_at, COALESCE(finished_at, ''),
		        status, input_tokens, output_tokens, cost_usd,
		        COALESCE(reply_preview, ''), COALESCE(error_text, '')
		 FROM bmad_phase_steps WHERE project_id = ?
		 ORDER BY id ASC`, p.ID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			st := make(map[string]any)
			var (
				id                              int64
				phase, cmd, startedAt, finished string
				status, preview, errText        string
				in_, out_                       int
				cost                            float64
			)
			if err := rows.Scan(&id, &phase, &cmd, &startedAt, &finished,
				&status, &in_, &out_, &cost, &preview, &errText); err == nil {
				st["id"] = id
				st["phase"] = phase
				st["command"] = cmd
				st["started_at"] = startedAt
				st["finished_at"] = finished
				st["status"] = status
				st["input_tokens"] = in_
				st["output_tokens"] = out_
				st["cost_usd"] = cost
				st["reply_preview"] = preview
				st["error"] = errText
				phases = append(phases, st)
			}
		}
		_ = rows.Err()
	}
	_ = writeTarJSON(tw, "phases.json", phases)

	// --- 4. intake.json : toutes les conversations + messages du projet.
	// Query directe plutôt que passer par le store car on veut les
	// conversations terminées aussi (pas juste l'active).
	type msg struct {
		Author    string `json:"author"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}
	type conv struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		Messages  []msg  `json:"messages"`
	}
	var convs []conv
	cRows, cerr := s.db().QueryContext(r.Context(),
		`SELECT id, role, status, created_at FROM project_conversations WHERE project_id = ? ORDER BY id ASC`,
		p.ID)
	if cerr == nil {
		defer func() { _ = cRows.Close() }()
		for cRows.Next() {
			var c conv
			if err := cRows.Scan(&c.ID, &c.Role, &c.Status, &c.CreatedAt); err != nil {
				continue
			}
			mRows, _ := s.db().QueryContext(r.Context(),
				`SELECT author, content, created_at FROM project_messages WHERE conversation_id = ? ORDER BY id ASC`,
				c.ID)
			if mRows != nil {
				func() {
					defer func() { _ = mRows.Close() }()
					for mRows.Next() {
						var m msg
						if err := mRows.Scan(&m.Author, &m.Content, &m.CreatedAt); err == nil {
							c.Messages = append(c.Messages, m)
						}
					}
					_ = mRows.Err()
				}()
			}
			convs = append(convs, c)
		}
		_ = cRows.Err()
	}
	_ = writeTarJSON(tw, "intake.json", convs)

	// --- 5. workdir/ : copie du répertoire de travail (best-effort).
	if p.Workdir != "" {
		_ = addDirToTar(tw, p.Workdir, "workdir")
	}
}

// writeTarJSON serialise obj en JSON indenté et l'écrit comme un seul
// fichier dans le tarball.
func writeTarJSON(tw *tar.Writer, name string, obj any) error {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = tw.Write(data)
	return err
}

// addDirToTar walk récursivement src et écrit chaque fichier sous
// prefix/ dans le tarball. Ignore .git/objects/ (lourd et re-derivable
// via fetch) et les .tmp/.swp communs.
func addDirToTar(tw *tar.Writer, src, prefix string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // best-effort : on skippe les erreurs de stat
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}
		// Skip .git/objects and common temp files to keep the tarball lean.
		if strings.Contains(rel, ".git/objects/") ||
			strings.HasSuffix(rel, ".tmp") ||
			strings.HasSuffix(rel, ".swp") {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		tarPath := prefix + "/" + rel
		hdr := &tar.Header{
			Name:    tarPath,
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		_, _ = io.Copy(tw, f)
		return nil
	})
}

// sanitizeFilename garde seulement alphanumériques + _ + - du nom du
// projet pour produire un nom de fichier safe cross-OS.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteRune('-')
		}
	}
	out := b.String()
	if len(out) > 60 {
		out = out[:60]
	}
	return strings.Trim(out, "-")
}

// handleCostSummary aggregates cumulative cost across every project,
// plus per-phase / per-command breakdowns. Feeds the /costs dashboard
// page so the operator can see where Claude tokens are going without
// drilling into each project individually.
func (s *Server) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	type projectLine struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Status     string  `json:"status"`
		TotalUSD   float64 `json:"total_usd"`
		CapUSD     float64 `json:"cap_usd,omitempty"`
		StepCount  int     `json:"step_count"`
		FailedStep string  `json:"failure_stage,omitempty"`
	}
	type phaseLine struct {
		Phase     string  `json:"phase"`
		TotalUSD  float64 `json:"total_usd"`
		StepCount int     `json:"step_count"`
		InTokens  int64   `json:"input_tokens"`
		OutTokens int64   `json:"output_tokens"`
	}
	type commandLine struct {
		Command   string  `json:"command"`
		TotalUSD  float64 `json:"total_usd"`
		StepCount int     `json:"step_count"`
	}
	type summary struct {
		GrandTotalUSD float64       `json:"grand_total_usd"`
		Projects      []projectLine `json:"projects"`
		Phases        []phaseLine   `json:"phases"`
		Commands      []commandLine `json:"commands"`
	}

	out := summary{Projects: []projectLine{}, Phases: []phaseLine{}, Commands: []commandLine{}}

	// Par projet
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT p.id, p.name, p.status,
		        COALESCE(p.total_cost_usd, 0),
		        COALESCE(p.cost_cap_usd, 0),
		        COALESCE(p.failure_stage, ''),
		        COALESCE((SELECT COUNT(*) FROM bmad_phase_steps s WHERE s.project_id = p.id), 0)
		 FROM projects p
		 ORDER BY p.total_cost_usd DESC, p.updated_at DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var p projectLine
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.TotalUSD,
			&p.CapUSD, &p.FailedStep, &p.StepCount); err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
		out.Projects = append(out.Projects, p)
		out.GrandTotalUSD += p.TotalUSD
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
		return
	}

	// Par phase (analysis, planning, solutioning, implementation-init, story, review)
	phaseRows, perr := s.db().QueryContext(r.Context(),
		`SELECT phase,
		        SUM(COALESCE(cost_usd, 0)),
		        COUNT(*),
		        SUM(COALESCE(input_tokens, 0)),
		        SUM(COALESCE(output_tokens, 0))
		 FROM bmad_phase_steps
		 WHERE status = 'done'
		 GROUP BY phase
		 ORDER BY 2 DESC`)
	if perr != nil && !strings.Contains(perr.Error(), "no such table") {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", perr.Error())
		return
	}
	if phaseRows != nil {
		defer func() { _ = phaseRows.Close() }()
		for phaseRows.Next() {
			var p phaseLine
			if err := phaseRows.Scan(&p.Phase, &p.TotalUSD, &p.StepCount,
				&p.InTokens, &p.OutTokens); err != nil {
				writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
				return
			}
			out.Phases = append(out.Phases, p)
		}
		if err := phaseRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
	}

	// Par commande (top 20)
	cmdRows, cerr := s.db().QueryContext(r.Context(),
		`SELECT command,
		        SUM(COALESCE(cost_usd, 0)),
		        COUNT(*)
		 FROM bmad_phase_steps
		 WHERE status = 'done'
		 GROUP BY command
		 ORDER BY 2 DESC
		 LIMIT 20`)
	if cerr != nil && !strings.Contains(cerr.Error(), "no such table") {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", cerr.Error())
		return
	}
	if cmdRows != nil {
		defer func() { _ = cmdRows.Close() }()
		for cmdRows.Next() {
			var c commandLine
			if err := cmdRows.Scan(&c.Command, &c.TotalUSD, &c.StepCount); err != nil {
				writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
				return
			}
			out.Commands = append(out.Commands, c)
		}
		if err := cmdRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
	}

	writeJSON(w, out)
}

// handleNotifySettings reports which notification sinks are wired up.
// We don't leak the webhook URL itself — just whether one is set, so
// the UI can render "Slack: ON" vs a help card pointing at the env var.
func (s *Server) handleNotifySettings(w http.ResponseWriter, _ *http.Request) {
	slack := os.Getenv("HIVE_SLACK_WEBHOOK")
	writeJSON(w, map[string]any{
		"slack_enabled": slack != "",
		"slack_host": func() string {
			if slack == "" {
				return ""
			}
			// Strip the token path but keep the host for a "connected to hooks.slack.com" hint.
			if i := strings.Index(slack[8:], "/"); i > 0 {
				return slack[:8+i]
			}
			return slack
		}(),
		"events": []string{
			"project.shipped",
			"project.architect_failed",
			"project.iteration_failed",
			"project.cost_cap_warning",
			"project.cost_cap_reached",
		},
	})
}

// handleNotifyTest pings the configured Slack webhook with a synthetic
// "hello world" event so the operator can verify the endpoint works
// without having to trigger a real project.shipped. Returns 200 on
// delivery, 400 if the webhook is unset, 502 if Slack rejects.
func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	webhook := os.Getenv("HIVE_SLACK_WEBHOOK")
	if webhook == "" {
		writeError(w, http.StatusBadRequest, "SLACK_NOT_CONFIGURED",
			"Aucun webhook Slack configuré (HIVE_SLACK_WEBHOOK).")
		return
	}
	body, _ := json.Marshal(map[string]string{
		"text": ":wave: Hive test — webhook opérationnel.",
	})
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "REQUEST_FAILED", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "WEBHOOK_UNREACHABLE", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "WEBHOOK_REJECTED",
			fmt.Sprintf("slack a répondu %d", resp.StatusCode))
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// handleCostSummaryCSV streams the cost summary as CSV for download.
// Three sections concaténées dans un seul fichier :
//   1. par projet (nom, statut, cumul, cap, nb steps, stage d'échec)
//   2. par phase BMAD (coût total + tokens consommés)
//   3. par commande BMAD (coût total + nb invocations)
//
// Les sections sont séparées par une ligne vide + un nouveau header
// pour que l'opérateur puisse pivoter dans Excel / Numbers.
func (s *Server) handleCostSummaryCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="hive-costs.csv"`)
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Section 1 : projets
	_ = cw.Write([]string{"section", "project_id", "name", "status", "total_usd", "cap_usd", "steps", "failure_stage"})
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT p.id, p.name, p.status,
		        COALESCE(p.total_cost_usd, 0),
		        COALESCE(p.cost_cap_usd, 0),
		        COALESCE((SELECT COUNT(*) FROM bmad_phase_steps s WHERE s.project_id = p.id), 0),
		        COALESCE(p.failure_stage, '')
		 FROM projects p
		 ORDER BY p.total_cost_usd DESC, p.updated_at DESC`)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, name, status, failure string
			var total, cap_ float64
			var steps int
			if err := rows.Scan(&id, &name, &status, &total, &cap_, &steps, &failure); err != nil {
				continue
			}
			_ = cw.Write([]string{
				"project", id, name, status,
				fmt.Sprintf("%.4f", total),
				fmt.Sprintf("%.4f", cap_),
				fmt.Sprintf("%d", steps),
				failure,
			})
		}
		_ = rows.Err()
	}

	// Section 2 : par phase
	_ = cw.Write(nil)
	_ = cw.Write([]string{"section", "phase", "total_usd", "steps", "input_tokens", "output_tokens"})
	phaseRows, perr := s.db().QueryContext(r.Context(),
		`SELECT phase,
		        SUM(COALESCE(cost_usd, 0)),
		        COUNT(*),
		        SUM(COALESCE(input_tokens, 0)),
		        SUM(COALESCE(output_tokens, 0))
		 FROM bmad_phase_steps
		 WHERE status = 'done'
		 GROUP BY phase
		 ORDER BY 2 DESC`)
	if perr == nil {
		defer func() { _ = phaseRows.Close() }()
		for phaseRows.Next() {
			var phase string
			var total float64
			var count int
			var in_, out_ int64
			if err := phaseRows.Scan(&phase, &total, &count, &in_, &out_); err != nil {
				continue
			}
			_ = cw.Write([]string{
				"phase", phase,
				fmt.Sprintf("%.4f", total),
				fmt.Sprintf("%d", count),
				fmt.Sprintf("%d", in_),
				fmt.Sprintf("%d", out_),
			})
		}
		_ = phaseRows.Err()
	}

	// Section 3 : par commande
	_ = cw.Write(nil)
	_ = cw.Write([]string{"section", "command", "total_usd", "invocations"})
	cmdRows, cerr := s.db().QueryContext(r.Context(),
		`SELECT command,
		        SUM(COALESCE(cost_usd, 0)),
		        COUNT(*)
		 FROM bmad_phase_steps
		 WHERE status = 'done'
		 GROUP BY command
		 ORDER BY 2 DESC`)
	if cerr == nil {
		defer func() { _ = cmdRows.Close() }()
		for cmdRows.Next() {
			var cmd string
			var total float64
			var count int
			if err := cmdRows.Scan(&cmd, &total, &count); err != nil {
				continue
			}
			_ = cw.Write([]string{
				"command", cmd,
				fmt.Sprintf("%.4f", total),
				fmt.Sprintf("%d", count),
			})
		}
		_ = cmdRows.Err()
	}
}

// handleGhDeviceStart kicks off a GitHub OAuth device flow. Returns
// the verification URL + user code to display, and an opaque device
// code the client passes back to /gh/device/poll.
func (s *Server) handleGhDeviceStart(w http.ResponseWriter, r *http.Request) {
	start, err := git.StartDeviceFlow(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "DEVICE_START_FAILED", err.Error())
		return
	}
	writeJSON(w, start)
}

// handleGhDevicePoll polls GitHub for the device code. When the user
// has authorized, the token is persisted via `gh auth login --with-
// token`. While pending, returns 202 with the GitHub error code
// (`authorization_pending`, `slow_down`) so the client can back off.
func (s *Server) handleGhDevicePoll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(body.DeviceCode) == "" {
		writeError(w, http.StatusBadRequest, "MISSING_DEVICE_CODE", "device_code requis")
		return
	}
	_, err := git.PollDeviceFlow(r.Context(), body.DeviceCode)
	if err != nil {
		switch err.Error() {
		case "authorization_pending", "slow_down":
			w.WriteHeader(http.StatusAccepted)
			writeJSON(w, map[string]string{"status": err.Error()})
			return
		case "expired_token", "access_denied":
			writeError(w, http.StatusBadRequest, strings.ToUpper(err.Error()), err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "DEVICE_POLL_FAILED", err.Error())
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
		// Pas de timeout : retrospective peut prendre son temps.
		ctx, cancel := context.WithCancel(context.Background())
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
	// On ne bail pas sur "aucun build en cours" : même sans run actif
	// enregistré, des rows bmad_phase_steps peuvent être coincées en
	// `running` (process tué par un crash/restart précédent dont l'
	// OnFinish n'a jamais pu tourner). Le sweep ci-dessous les passe à
	// `failed` pour que l'UI n'affiche plus "/bmad-create-story en cours"
	// éternellement. Si cancelRun a effectivement tué un ctx, tant mieux.
	cancelled := s.cancelRun(id)
	if res, err := s.db().ExecContext(r.Context(),
		`UPDATE bmad_phase_steps
		 SET status = 'failed',
		     finished_at = datetime('now'),
		     error_text = COALESCE(NULLIF(error_text, ''), 'annulé par l''opérateur')
		 WHERE project_id = ? AND status = 'running'`, id); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			slog.Info("cancel: swept running phase_steps", "project", id, "count", n)
		}
	}
	if !cancelled {
		// Aucun run enregistré ET aucun zombie — rien à annuler.
		writeError(w, http.StatusConflict, "NO_RUN",
			"aucun build BMAD en cours pour ce projet")
		return
	}
	// Règle : un cancel ne doit PAS forcément killer le projet.
	//   - 1ère planification (planning + aucun epic) → failed, le
	//     projet n'a rien livré, c'est un build avorté.
	//   - Itération brownfield (planning AVEC des epics déjà livrés)
	//     → on revient à 'shipped' pour préserver la release existante.
	//     L'opérateur peut relancer /iterate plus tard sans tout perdre.
	//   - Devloop en train de coder (building) → on stoppe la skill
	//     en vol mais le projet reste 'building' ; le prochain tick du
	//     supervisor reprendra ou l'opérateur peut retry une story.
	var currentStatus string
	var epicsCount int
	_ = s.db().QueryRowContext(r.Context(),
		`SELECT status, (SELECT COUNT(*) FROM epics WHERE project_id = ?) FROM projects WHERE id = ?`,
		id, id).Scan(&currentStatus, &epicsCount)

	nextStatus := currentStatus
	reason := ""
	switch {
	case currentStatus == string(project.StatusPlanning) && epicsCount == 0:
		nextStatus = string(project.StatusFailed)
		reason = "Build annulé par l'opérateur"
	case currentStatus == string(project.StatusPlanning) && epicsCount > 0:
		// Itération brownfield annulée : retour à shipped.
		nextStatus = string(project.StatusShipped)
	default:
		// building / shipped / autres : on laisse tel quel. La skill en
		// vol est déjà tuée ; le projet conserve son état utile.
	}
	if nextStatus != currentStatus {
		if reason != "" {
			_, _ = s.db().ExecContext(r.Context(),
				`UPDATE projects SET status = ?, failure_stage = 'cancelled',
				 failure_error = ?, updated_at = datetime('now') WHERE id = ?`,
				nextStatus, reason, id)
		} else {
			_, _ = s.db().ExecContext(r.Context(),
				`UPDATE projects SET status = ?, failure_stage = NULL,
				 failure_error = NULL, updated_at = datetime('now') WHERE id = ?`,
				nextStatus, id)
		}
	}
	// Pause systématique : un cancel = l'opérateur a décidé de STOP,
	// le devloop ne doit pas réessayer au prochain tick. "Reprendre"
	// clear paused=0.
	_, _ = s.db().ExecContext(r.Context(),
		`UPDATE projects SET paused = 1 WHERE id = ?`, id)
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(r.Context(), "project.cancelled", "api",
			map[string]any{
				"project_id":  id,
				"prev_status": currentStatus,
				"new_status":  nextStatus,
			})
	}
	writeJSON(w, map[string]any{
		"project_id":  id,
		"status":      "cancelled",
		"new_status":  nextStatus,
		"prev_status": currentStatus,
	})
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
	// Resume-from-step : si l'UI demande ?from_step=N, on skip les N
	// premières skills. Utile quand un pipeline a grillé $3 sur
	// create-prd et a failed à create-architecture — inutile de tout
	// re-dépenser, on reprend à l'architect. 0 ou absent = retry from
	// scratch (comportement par défaut).
	fromStep := 0
	if v := r.URL.Query().Get("from_step"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			fromStep = n
		}
	}
	go s.runArchitectAsyncFromStep(p.ID, p.Idea, seed, fromStep) //nolint:gosec // G118: request ctx dies; the goroutine uses its own 90-min ctx

	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     "retry-scheduled",
		"from_step":  fromStep,
	})
}

// handlePhaseStep retourne UNE row bmad_phase_steps avec sa console
// complète (reply_full). Utilisé par le drawer "Console" du dashboard
// pour afficher ce que Claude a réellement répondu — le preview de
// 600 caractères dans la liste ne suffit pas pour diagnostiquer
// pourquoi une skill a dérivé ou échoué.
func (s *Server) handlePhaseStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "step id required")
		return
	}
	type step struct {
		ID           int64   `json:"id"`
		ProjectID    string  `json:"project_id"`
		Phase        string  `json:"phase"`
		Command      string  `json:"command"`
		StartedAt    string  `json:"started_at"`
		FinishedAt   string  `json:"finished_at,omitempty"`
		Status       string  `json:"status"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"`
		ReplyFull    string  `json:"reply_full,omitempty"`
		Error        string  `json:"error,omitempty"`
	}
	var out step
	err := s.db().QueryRowContext(r.Context(),
		`SELECT id, project_id, phase, command, started_at, COALESCE(finished_at, ''),
		        status, input_tokens, output_tokens, cost_usd,
		        COALESCE(reply_full, COALESCE(reply_preview, '')), COALESCE(error_text, '')
		 FROM bmad_phase_steps WHERE id = ?`, id,
	).Scan(&out.ID, &out.ProjectID, &out.Phase, &out.Command, &out.StartedAt,
		&out.FinishedAt, &out.Status, &out.InputTokens, &out.OutputTokens,
		&out.CostUSD, &out.ReplyFull, &out.Error)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, out)
}

// handleResumeProject clear paused=0 pour que le devloop reprenne le
// travail au prochain tick. Complémentaire du paused=1 posé par les
// handlers de cancel. Idempotent : appeler sur un projet non pausé
// est un no-op.
func (s *Server) handleResumeProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "project id required")
		return
	}
	res, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET paused = 0, updated_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "projet introuvable")
		return
	}
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(r.Context(), "project.resumed", "api",
			map[string]any{"project_id": id})
	}
	writeJSON(w, map[string]any{"project_id": id, "paused": false})
}

// handleCancelPhaseStep tue UN skill précis en s'appuyant sur
// stepCancels (registre keyed par phase_step.id). Contrairement à
// /projects/{id}/cancel, ce endpoint ne touche PAS à project.status
// et ne sweep PAS les autres rows — seul le skill visé est tué.
//
// Marque AUSSI le projet comme paused=1 pour que le devloop ne
// relance pas une nouvelle skill au prochain tick. Sans ce flag,
// l'opérateur voyait son cancel immédiatement remplacé par un
// /bmad-create-story frais 10s plus tard. L'UI doit afficher un
// bouton "Reprendre" pour clear paused=0.
func (s *Server) handleCancelPhaseStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "step id required")
		return
	}
	var stepID int64
	if _, err := fmt.Sscan(id, &stepID); err != nil || stepID <= 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "step id invalide")
		return
	}
	// Lookup project_id AVANT cancelStep (qui delete le registry entry).
	var projectID string
	_ = s.db().QueryRowContext(r.Context(),
		`SELECT project_id FROM bmad_phase_steps WHERE id = ?`, stepID,
	).Scan(&projectID)

	if !s.cancelStep(stepID) {
		writeError(w, http.StatusConflict, "NO_RUN",
			"ce skill n'est plus en cours (déjà terminé ou jamais démarré)")
		return
	}
	_, _ = s.db().ExecContext(r.Context(),
		`UPDATE bmad_phase_steps
		 SET status = 'failed', finished_at = datetime('now'),
		     error_text = COALESCE(NULLIF(error_text, ''), 'annulé par l''opérateur (per-step)')
		 WHERE id = ? AND status = 'running'`, stepID)
	if projectID != "" {
		_, _ = s.db().ExecContext(r.Context(),
			`UPDATE projects SET paused = 1, updated_at = datetime('now') WHERE id = ?`,
			projectID)
	}
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(r.Context(), "project.bmad_step_cancelled", "api",
			map[string]any{"step_id": stepID, "project_id": projectID})
	}
	writeJSON(w, map[string]any{"step_id": stepID, "status": "cancelled", "project_paused": projectID != ""})
}

// handleRerunPhaseStep relance un skill BMAD déjà exécuté. Crée une
// NOUVELLE row bmad_phase_steps (pas d'overwrite du vieux step) pour
// que l'historique reste lisible. Délègue au même trackedInvoke que
// /api/v1/bmad/run, donc streaming stream-json + WS broadcast + DB
// tracking sont gratuits.
//
// Ne valide PAS le skill contre le registre UI : l'opérateur peut
// vouloir relancer des skills d'activation (/bmad-agent-pm, etc.) qui
// ne sont pas exposées dans le menu. On ne laisse passer que les
// commandes /bmad-* pour bloquer une injection arbitraire.
func (s *Server) handleRerunPhaseStep(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE", "")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "step id required")
		return
	}
	var projectID, phase, command string
	err := s.db().QueryRowContext(r.Context(),
		`SELECT project_id, phase, command FROM bmad_phase_steps WHERE id = ?`,
		id).Scan(&projectID, &phase, &command)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if !strings.HasPrefix(command, "/bmad-") {
		writeError(w, http.StatusBadRequest, "BAD_COMMAND",
			"seuls les skills /bmad-* peuvent être relancés")
		return
	}
	proj, err := s.projectStore.GetByID(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if proj.Workdir == "" {
		writeError(w, http.StatusConflict, "NO_WORKDIR",
			"ce projet n'a pas de workdir configuré")
		return
	}

	go func() { //nolint:gosec // G118: request ctx dies with the handler
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.registerRun(projectID, cancel)
		defer s.clearRun(projectID)

		runner := bmad.NewRunner()
		if runner == nil {
			slog.Warn("rerun: runner indisponible", "skill", command)
			return
		}
		if _, err := s.trackedInvoke(ctx, runner, projectID, phase,
			command+" (rerun)", proj.Workdir, command); err != nil {
			slog.Warn("rerun failed",
				"skill", command, "project", projectID, "error", err)
		}
	}()

	writeJSON(w, map[string]any{
		"project_id": projectID,
		"skill":      command,
		"phase":      phase,
		"status":     "rerun-started",
	})
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
	defer func() { _ = rows.Close() }()
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
	purgeWorkdir := r.URL.Query().Get("purge_workdir") == "true"

	// Récupère le workdir AVANT la suppression pour pouvoir le rm -rf
	// après. Si la lookup échoue on continue : pas de raison de
	// bloquer le delete juste parce qu'on n'a pas pu trouver la ligne.
	var workdir string
	if purgeWorkdir {
		if p, err := s.projectStore.GetByID(r.Context(), id); err == nil {
			workdir = p.Workdir
		}
	}

	// Annule un run éventuellement en cours avant le delete, sinon la
	// goroutine continuera à écrire dans une ligne de DB effacée.
	s.cancelRun(id)

	if err := s.projectStore.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	purged := false
	if purgeWorkdir && workdir != "" && workdirIsSafeToPurge(workdir) {
		if err := os.RemoveAll(workdir); err == nil {
			purged = true
		}
	}
	writeJSON(w, map[string]any{
		"status":          "removed",
		"id":              id,
		"workdir_purged":  purged,
		"workdir_skipped": purgeWorkdir && !purged,
	})
}

// workdirIsSafeToPurge gate-keeps the rm -rf : we only accept absolute
// paths under known prefixes (the user's home, /tmp, /var/folders on
// macOS) and refuse pathologically-short paths like "/" or "/home".
// A malicious workdir field on an existing project could otherwise
// nuke the user's whole home directory via `?purge_workdir=true`.
func workdirIsSafeToPurge(p string) bool {
	if p == "" || !filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	if len(clean) < 8 {
		return false // "/", "/tmp", "/home" etc.
	}
	home, _ := os.UserHomeDir()
	allowed := []string{"/tmp/", "/var/folders/", "/private/var/folders/"}
	if home != "" {
		allowed = append(allowed, home+"/")
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(clean, prefix) && clean != strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}
