package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/intake"
	"github.com/JulienLeotier/hive/internal/project"
)

// handleIntakeGet returns the PM conversation for a project, creating it
// on first access so the dashboard can render the chat without a separate
// "start" call. The response is the conversation with its full message
// list attached.
func (s *Server) handleIntakeGet(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
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
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgentFor(p))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	writeJSON(w, conv)
}

// handleIntakeMessage posts a user reply in the PM conversation and
// returns the updated conversation including the agent's follow-up.
// Body: {content}.
func (s *Server) handleIntakeMessage(w http.ResponseWriter, r *http.Request) {
	s.handleConversationMessage(w, r, s.intakeAgentFor)
}

// handleConversationMessage is the shared body between the initial
// intake chat and the brownfield iteration chat. agentFn est appelé
// après chargement du projet pour laisser l'appelant picker un agent
// dépendant du contexte (greenfield vs brownfield).
func (s *Server) handleConversationMessage(w http.ResponseWriter, r *http.Request, agentFn func(*project.Project) intake.Agent) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	agent := agentFn(p)
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	updated, done, err := s.intakeStore.AppendUserMessage(r.Context(), conv.ID, p.Idea, body.Content, agent)
	if err != nil {
		writeError(w, http.StatusBadRequest, "APPEND_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{"conversation": updated, "done": done})
}

// handleIntakeFinalize asks the PM agent for the final PRD, stores it on
// the project, and flips the project status from `draft` to `planning`
// so Phase 3's Architect pipeline picks it up.
func (s *Server) handleIntakeFinalize(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgentFor(p))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}

	// Flip to planning right away so the dashboard shows the spinner
	// while BMAD runs. The PRD itself is produced by bmad-create-prd
	// inside runBMADAsync — the old scripted PM agent that used to
	// Finalize() into a fake PRD is gone.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	// BMAD is the slow path — `npx bmad-method install` + multiple
	// `claude --print` invocations each take minutes. Detach so the
	// HTTP request returns immediately; progress is broadcast via WS
	// (project.architect_started / _done / _failed events).
	go s.runArchitectAsync(p.ID, p.Idea, flattenConversation(conv)) //nolint:gosec // G118: request ctx dies with the handler

	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     project.StatusPlanning,
	})
}

// handleIterateGet ouvre (ou retourne) la conversation d'itération
// pour un projet déjà livré. Conversation séparée de l'intake
// d'origine — role="pm-iterate" dans project_conversations.
func (s *Server) handleIterateGet(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.iterationAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ITERATION_START_FAILED", err.Error())
		return
	}
	writeJSON(w, conv)
}

// handleIterateMessage poste une réponse utilisateur dans la
// conversation d'itération (brownfield post-shipping). Délègue au
// shared body avec un factory qui ignore le projet (l'itération
// utilise le même IterationAgent quel que soit le state).
func (s *Server) handleIterateMessage(w http.ResponseWriter, r *http.Request) {
	s.handleConversationMessage(w, r, func(*project.Project) intake.Agent { return s.iterationAgent() })
}

// handleIterateFinalize clôture la conversation d'itération et
// déclenche le pipeline brownfield BMAD en background :
// document-project → edit-prd → sprint-planning, etc. Le supervisor
// reprendra automatiquement sur les nouvelles stories pending.
func (s *Server) handleIterateFinalize(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	agent := s.iterationAgent()
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ITERATION_START_FAILED", err.Error())
		return
	}

	// Flip direct en planning pour signaler à l'UI que l'itération
	// démarre. Un shipped project qui itère redevient building à la
	// fin du pipeline brownfield.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	go s.runIterationAsync(p.ID, p.Idea, flattenConversation(conv)) //nolint:gosec // G118: request ctx dies with the handler

	writeJSON(w, map[string]any{"project_id": p.ID, "status": project.StatusPlanning})
}

// iterationAgent renvoie le PM agent wrappé pour une conversation
// d'itération (role = pm-iterate).
func (s *Server) iterationAgent() intake.Agent {
	return &intake.IterationAgent{Base: s.intakeAgent()}
}

// runIterationAsync exécute le pipeline brownfield BMAD : on écrit
// le brief de la nouvelle feature dans un fichier dédié, puis on
// lance IterationPipeline (document-project → edit-prd → etc.).
// Les epics/stories existants sont conservés en DB ; les nouveaux
// seront ingérés en sortie via le même parseur json-hive.
func (s *Server) runIterationAsync(projectID, idea, seedDoc string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	s.registerRun(projectID, cancel)
	defer s.clearRun(projectID)

	fail := func(stage string, err error) {
		slog.Warn("iteration pipeline failed", "project", projectID, "stage", stage, "error", err)
		_, _ = s.db().ExecContext(ctx,
			`UPDATE projects SET failure_stage = ?, failure_error = ?,
			 status = ?, updated_at = datetime('now')
			 WHERE id = ?`,
			stage, err.Error(), project.StatusFailed, projectID)
		if s.eventBus != nil {
			_, _ = s.eventBus.Publish(ctx, "project.iteration_failed", "api", map[string]any{
				"project_id": projectID, "stage": stage, "error": err.Error(),
			})
		}
	}

	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.iteration_started", "api",
			map[string]string{"project_id": projectID})
	}

	proj, err := s.projectStore.GetByID(ctx, projectID)
	if err != nil {
		fail("lookup", err)
		return
	}
	if proj.Workdir == "" {
		fail("prepare", fmt.Errorf("project has no workdir"))
		return
	}
	workdir := proj.Workdir

	runner := bmad.NewRunner()
	if runner == nil {
		fail("prepare", fmt.Errorf("claude CLI missing"))
		return
	}

	// Écrire le brief de l'itération à côté du premier intake. La
	// skill edit-prd saura le lire comme input additionnel.
	iterPath := filepath.Join(workdir, bmad.PlanningDir, "_iteration.md")
	if err := os.MkdirAll(filepath.Dir(iterPath), 0o755); err != nil {
		fail("prepare", err)
		return
	}
	if err := os.WriteFile(iterPath, []byte(buildIterationDoc(idea, seedDoc)), 0o644); err != nil {
		fail("prepare", err)
		return
	}

	// BMAD doit déjà être installé (projet déjà livré une fois),
	// mais on rappelle Install() qui no-op si c'est le cas — c'est
	// aussi une porte de rattrapage si l'opérateur a wipé _bmad/.
	if err := runner.Install(ctx, workdir); err != nil {
		fail("install", err)
		return
	}

	obs := s.stepObserver(ctx, projectID, "iteration-feature")
	if _, err := runner.RunSequenceObserved(ctx, workdir, bmad.IterationPipeline, obs); err != nil {
		fail("iteration-pipeline", err)
		return
	}

	// Re-lire le PRD étendu et ingérer les nouveaux epics/stories.
	if prdText, err := readFirst(workdir, bmad.PRDFile, bmad.PRDFileLower); err == nil {
		_, _ = s.db().ExecContext(ctx,
			`UPDATE projects SET prd = ?, updated_at = datetime('now') WHERE id = ?`,
			prdText, projectID)
	}

	ingestGoal := "Lis les artefacts BMAD (epics.md + stories/) et émets UN bloc fencé " +
		"`json-hive` à la fin contenant TOUS les epics et stories (anciens + nouveaux) " +
		"dans ce schéma : [{\"title\":\"\",\"description\":\"\",\"stories\":" +
		"[{\"title\":\"\",\"description\":\"\",\"acceptance_criteria\":[]}]}]"
	res, err := runner.Invoke(ctx, workdir, ingestGoal, nil)
	if err != nil {
		fail("ingest-json", err)
		return
	}
	tree, err := parseBMADTree(res.Text)
	if err != nil {
		fail("parse-epics", err)
		return
	}
	if err := s.appendIterationTree(ctx, projectID, tree); err != nil {
		fail("ingest", err)
		return
	}

	// Retour en building — le supervisor reprendra le dev loop sur
	// les nouvelles stories pending.
	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusBuilding, projectID,
	); err != nil {
		slog.Warn("iteration done mais update échoué", "project", projectID, "error", err)
	}
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.iteration_done", "api", map[string]any{
			"project_id": projectID,
		})
	}
}

// appendIterationTree ajoute SEULEMENT les epics qui n'existent pas
// déjà en DB (dédupliqués par titre). Les stories déjà done ne sont
// pas réinsérées ; les nouvelles stories d'un epic existant sont
// ajoutées à la suite.
func (s *Server) appendIterationTree(ctx context.Context, projectID string, epics []bmadEpic) error {
	tx, err := s.db().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Charger les epics existants pour dédupliquer.
	existing, err := loadEpicIDsByTitle(ctx, tx, projectID)
	if err != nil {
		return err
	}
	storySeen, err := loadStoriesByEpic(ctx, tx, projectID)
	if err != nil {
		return err
	}

	for ei, e := range epics {
		key := strings.ToLower(strings.TrimSpace(e.Title))
		epicID, found := existing[key]
		if !found {
			epicID = fmt.Sprintf("epc_%s_iter_%d_%d", projectID, time.Now().Unix(), ei)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO epics (id, project_id, title, description, ordering, status)
				 VALUES (?, ?, ?, ?, ?, 'pending')`,
				epicID, projectID, e.Title, e.Description, 1000+ei,
			); err != nil {
				return fmt.Errorf("insert iteration epic %d: %w", ei, err)
			}
			storySeen[epicID] = map[string]bool{}
		}
		for si, st := range e.Stories {
			skey := strings.ToLower(strings.TrimSpace(st.Title))
			if storySeen[epicID][skey] {
				continue
			}
			storyID := fmt.Sprintf("%s_iter_%d_s%d", epicID, time.Now().Unix(), si)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stories (id, epic_id, title, description, ordering, status)
				 VALUES (?, ?, ?, ?, ?, 'pending')`,
				storyID, epicID, st.Title, st.Description, 1000+si,
			); err != nil {
				return fmt.Errorf("insert iteration story %d/%d: %w", ei, si, err)
			}
			for ai, ac := range st.AcceptanceCriteria {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO acceptance_criteria (story_id, ordering, text, passed)
					 VALUES (?, ?, ?, 0)`,
					storyID, ai, ac,
				); err != nil {
					return fmt.Errorf("insert iteration ac %d/%d/%d: %w", ei, si, ai, err)
				}
			}
		}
	}
	return tx.Commit()
}

// loadEpicIDsByTitle retourne un index des epics d'un projet (titre
// lowercased → id). Close + Err check corrects pour le linter.
func loadEpicIDsByTitle(ctx context.Context, tx *sql.Tx, projectID string) (map[string]string, error) {
	out := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT id, title FROM epics WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		out[strings.ToLower(strings.TrimSpace(title))] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// loadStoriesByEpic retourne un index des stories existantes par
// epicID → {lowercased title → true}. Sert à dédupliquer à
// l'ingestion d'une itération.
func loadStoriesByEpic(ctx context.Context, tx *sql.Tx, projectID string) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	rows, err := tx.QueryContext(ctx,
		`SELECT s.epic_id, s.title FROM stories s JOIN epics e ON e.id = s.epic_id
		 WHERE e.project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var eid, title string
		if err := rows.Scan(&eid, &title); err != nil {
			return nil, err
		}
		if out[eid] == nil {
			out[eid] = map[string]bool{}
		}
		out[eid][strings.ToLower(strings.TrimSpace(title))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func buildIterationDoc(idea, conversation string) string {
	var b strings.Builder
	b.WriteString("# Nouvelle itération\n\n")
	b.WriteString("## Projet existant\n\n")
	b.WriteString(strings.TrimSpace(idea))
	b.WriteString("\n\n")
	b.WriteString("## Feature à ajouter\n\n")
	b.WriteString(strings.TrimSpace(conversation))
	b.WriteString("\n")
	return b.String()
}

// flattenConversation turns the intake conversation into a plain
// text transcript BMAD's create-prd skill can ingest as a product
// brief.
func flattenConversation(conv *intake.Conversation) string {
	if conv == nil {
		return ""
	}
	var b strings.Builder
	for _, m := range conv.Messages {
		role := "PM"
		if m.Author == "user" {
			role = "User"
		}
		fmt.Fprintf(&b, "%s: %s\n\n", role, strings.TrimSpace(m.Content))
	}
	return strings.TrimSpace(b.String())
}

// RecoverStuckPlanning re-kicks the BMAD pipeline for every project
// left in limbo : status=planning (crashed mid-pipeline) ou failed
// (dernier run abort). Idempotent — BMAD et le store sont safe
// contre la ré-exécution. Safe à appeler plusieurs fois depuis
// serve.go au démarrage.
func (s *Server) RecoverStuckPlanning(ctx context.Context) error {
	if s.projectStore == nil {
		return nil
	}
	// On récupère aussi les projets failed : un user peut relancer
	// explicitement via le bouton Retry, mais si on vient de crasher
	// en plein pipeline la reprise auto est pratique.
	rows, err := s.db().QueryContext(ctx,
		`SELECT p.id, p.idea
		 FROM projects p
		 WHERE p.status IN (?, ?)
		   AND NOT EXISTS (SELECT 1 FROM epics e WHERE e.project_id = p.id)`,
		project.StatusPlanning, project.StatusFailed,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	type stuck struct{ id, idea string }
	var list []stuck
	for rows.Next() {
		var s stuck
		if err := rows.Scan(&s.id, &s.idea); err != nil {
			return err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, st := range list {
		slog.Info("recovering stuck project", "project", st.id)
		var seed string
		if s.intakeStore != nil {
			// Reseed depuis la conversation d'intake pour que BMAD
			// ait le brief quand il relance create-prd/edit-prd.
			if p, _ := s.projectStore.GetByID(ctx, st.id); p != nil {
				if conv, _ := s.intakeStore.GetOrStart(ctx, p.ID, p.Idea, s.intakeAgentFor(p)); conv != nil {
					seed = flattenConversation(conv)
				}
			}
		}
		go s.runArchitectAsync(st.id, st.idea, seed) //nolint:gosec // G118: boot-time recovery; no request ctx exists here
	}
	return nil
}

// runArchitectAsync drives the real BMAD-METHOD planning pipeline
// against the project's workdir: install BMAD, run bmad-create-prd,
// run bmad-create-epics-and-stories, ingest the resulting artefacts
// back into our DB, flip the project to `building`.
//
// Our hand-rolled architect is gone — BMAD does the same work with a
// real framework (14-step PRD, story sharding, checklist validation,
// etc.). The only glue that remains is ingesting BMAD's output back
// into the epics/stories/ACs tables so the dashboard stays
// story-centric.
//
// `seedDoc` is a text blob we pass to BMAD as the product brief. On
// first finalize it's the flattened PM chat; on recovery/regenerate
// it's whatever we have on file (previous PRD, raw idea).
func (s *Server) runArchitectAsync(projectID, idea, seedDoc string) {
	// Ctx cancellable côté UI (via POST /cancel). Timeout généreux
	// parce que FullPlanningPipeline = 13 skills × 2-5 min chacune.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	s.registerRun(projectID, cancel)
	defer s.clearRun(projectID)

	fail := func(stage string, err error) {
		slog.Warn("bmad pipeline failed", "project", projectID, "stage", stage, "error", err)
		// Persiste le stage d'échec + l'erreur sur le projet : le
		// dashboard peut afficher une bannière "build failed à l'étape
		// create-prd" avec un bouton Retry.
		_, _ = s.db().ExecContext(ctx,
			`UPDATE projects SET failure_stage = ?, failure_error = ?,
			 status = ?, updated_at = datetime('now')
			 WHERE id = ?`,
			stage, err.Error(), project.StatusFailed, projectID)
		if s.eventBus != nil {
			_, _ = s.eventBus.Publish(ctx, "project.architect_failed", "api", map[string]any{
				"project_id": projectID,
				"stage":      stage,
				"error":      err.Error(),
			})
		}
	}
	// Au démarrage on efface une éventuelle erreur précédente (cas d'un
	// retry manuel ou d'une reprise crash-recovery).
	_, _ = s.db().ExecContext(ctx,
		`UPDATE projects SET failure_stage = NULL, failure_error = NULL WHERE id = ?`,
		projectID)

	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_started", "api",
			map[string]string{"project_id": projectID})
	}

	proj, err := s.projectStore.GetByID(ctx, projectID)
	if err != nil {
		fail("lookup", err)
		return
	}
	if proj.Workdir == "" {
		fail("prepare", fmt.Errorf("project has no workdir"))
		return
	}
	workdir := proj.Workdir
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		fail("prepare", err)
		return
	}

	runner := bmad.NewRunner()
	if runner == nil {
		fail("prepare", fmt.Errorf("claude CLI missing; BMAD cannot run"))
		return
	}

	// Seed BMAD with the intake transcript so its create-prd workflow
	// has a product brief to work from. BMAD scans the planning dir
	// for input docs at step-02-discovery — we drop this one there.
	briefPath := filepath.Join(workdir, bmad.PlanningDir, "_intake.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		fail("prepare", err)
		return
	}
	if err := os.WriteFile(briefPath, []byte(buildIntakeDoc(idea, seedDoc)), 0o644); err != nil {
		fail("prepare", err)
		return
	}

	if err := runner.Install(ctx, workdir); err != nil {
		fail("install", err)
		return
	}

	// Pipeline choisi selon le type de projet :
	//  - greenfield (is_existing=false) → FullPlanningPipeline (from
	//    scratch : analyst + product-brief + create-prd + ...).
	//  - brownfield (is_existing=true, repo cloné ou repo_path) →
	//    IterationPipeline (bmad-document-project +
	//    bmad-generate-project-context + bmad-edit-prd + ...). BMAD
	//    lit le code existant et étend le PRD au lieu d'en créer un
	//    de zéro.
	pipeline := bmad.FullPlanningPipeline
	phaseLabel := "planning"
	stageLabel := "planning-sequence"
	if proj.IsExisting {
		pipeline = bmad.IterationPipeline
		phaseLabel = "iteration"
		stageLabel = "brownfield-sequence"
	}
	obs := s.stepObserver(ctx, projectID, phaseLabel)
	if _, err := runner.RunSequenceObserved(ctx, workdir, pipeline, obs); err != nil {
		fail(stageLabel, err)
		return
	}

	// Une fois sprint-planning exécuté, BMAD a écrit :
	// - _bmad-output/planning-artifacts/prd.md (PRD)
	// - _bmad-output/planning-artifacts/epics.md (arbre epics + stories)
	// - _bmad-output/implementation-artifacts/sprint-status.yaml
	prdText, err := readFirst(workdir, bmad.PRDFile, bmad.PRDFileLower)
	if err != nil {
		fail("read-prd", err)
		return
	}
	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET prd = ?, updated_at = datetime('now') WHERE id = ?`,
		prdText, projectID,
	); err != nil {
		fail("save-prd", err)
		return
	}

	// Courte passe additionnelle : on demande à Claude de formatter
	// les artefacts en json-hive pour qu'on les ingère en DB. Pas
	// d'opinion : juste un adaptateur entre les fichiers markdown
	// BMAD et notre schéma epics/stories/ACs.
	ingestGoal := "Lis les artefacts BMAD sous `_bmad-output/planning-artifacts/` " +
		"(epics.md, stories/, etc.) et les story files dans `_bmad-output/implementation-artifacts/` " +
		"puis émets UN bloc fencé `json-hive` à la fin contenant TOUS les epics et stories " +
		"dans cet exact schéma :\n" +
		"```json-hive\n" +
		"[{\"title\":\"Epic\",\"description\":\"\",\"key\":\"epic-1\",\"stories\":" +
		"[{\"title\":\"Story\",\"description\":\"\",\"key\":\"1.1\"," +
		"\"acceptance_criteria\":[\"AC\"]}]}]\n" +
		"```\nLe `key` de chaque story DOIT correspondre exactement à celui utilisé par BMAD " +
		"dans sprint-status.yaml. Aucune prose après le bloc."
	res, err := runner.Invoke(ctx, workdir, ingestGoal, nil)
	if err != nil {
		fail("ingest-json", err)
		return
	}
	tree, err := parseBMADTree(res.Text)
	if err != nil {
		fail("parse-epics", err)
		return
	}
	if err := s.ingestBMADTree(ctx, projectID, tree); err != nil {
		fail("ingest", err)
		return
	}

	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusBuilding, projectID,
	); err != nil {
		slog.Warn("bmad done but project update failed", "project", projectID, "error", err)
		return
	}
	slog.Info("bmad planning done", "project", projectID,
		"epics", len(tree), "stories", countBMADStories(tree))
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_done", "api", map[string]any{
			"project_id": projectID,
			"epics":      len(tree),
			"stories":    countBMADStories(tree),
		})
	}
}

// bmadEpic / bmadStory mirror the JSON shape we ask Claude to emit at
// the end of bmad-create-epics-and-stories. Kept flat so Claude
// doesn't have to match a deep nested format.
type bmadEpic struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Stories     []bmadStory `json:"stories"`
}
type bmadStory struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

func parseBMADTree(reply string) ([]bmadEpic, error) {
	marker := "```json-hive"
	start := strings.LastIndex(reply, marker)
	if start < 0 {
		return nil, fmt.Errorf("no json-hive block in reply")
	}
	body := reply[start+len(marker):]
	end := strings.Index(body, "```")
	if end < 0 {
		return nil, fmt.Errorf("json-hive block never closes")
	}
	var epics []bmadEpic
	if err := json.Unmarshal([]byte(strings.TrimSpace(body[:end])), &epics); err != nil {
		return nil, fmt.Errorf("parse json-hive: %w", err)
	}
	if len(epics) == 0 {
		return nil, fmt.Errorf("bmad emitted an empty epic tree")
	}
	return epics, nil
}

func (s *Server) ingestBMADTree(ctx context.Context, projectID string, epics []bmadEpic) error {
	tx, err := s.db().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for ei, e := range epics {
		epicID := fmt.Sprintf("epc_%s_%d", projectID, ei)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO epics (id, project_id, title, description, ordering, status)
			 VALUES (?, ?, ?, ?, ?, 'pending')`,
			epicID, projectID, e.Title, e.Description, ei,
		); err != nil {
			return fmt.Errorf("insert epic %d: %w", ei, err)
		}
		for si, st := range e.Stories {
			storyID := fmt.Sprintf("%s_s%d", epicID, si)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stories (id, epic_id, title, description, ordering, status)
				 VALUES (?, ?, ?, ?, ?, 'pending')`,
				storyID, epicID, st.Title, st.Description, si,
			); err != nil {
				return fmt.Errorf("insert story %d/%d: %w", ei, si, err)
			}
			for ai, ac := range st.AcceptanceCriteria {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO acceptance_criteria (story_id, ordering, text, passed)
					 VALUES (?, ?, ?, 0)`,
					storyID, ai, ac,
				); err != nil {
					return fmt.Errorf("insert ac %d/%d/%d: %w", ei, si, ai, err)
				}
			}
		}
	}
	return tx.Commit()
}

func countBMADStories(epics []bmadEpic) int {
	n := 0
	for _, e := range epics {
		n += len(e.Stories)
	}
	return n
}

// buildIntakeDoc formats the user's web chat as a product brief BMAD
// can ingest at step-02-discovery.
func buildIntakeDoc(idea, conversation string) string {
	var b strings.Builder
	b.WriteString("# Product Brief\n\n")
	b.WriteString("## Idea\n\n")
	b.WriteString(strings.TrimSpace(idea))
	b.WriteString("\n\n")
	if strings.TrimSpace(conversation) != "" {
		b.WriteString("## PM Q&A Transcript\n\n")
		b.WriteString(strings.TrimSpace(conversation))
		b.WriteString("\n")
	}
	return b.String()
}

func readFirst(workdir string, rels ...string) (string, error) {
	for _, rel := range rels {
		abs := filepath.Join(workdir, rel)
		if data, err := os.ReadFile(abs); err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("no BMAD output at any of %v", rels)
}

// stepObserver construit un bmad.StepObserver qui :
//   - insère une row `bmad_phase_steps` en `running` au start ;
//   - la finalise avec status=done/failed + tokens + cost au finish ;
//   - met à jour projects.total_cost_usd pour que le dashboard
//     puisse afficher le cumul ;
//   - émet un événement project.bmad_step (start + finish) pour que
//     le WS push la progression en temps réel.
//
// Les insertions échouées ne plantent pas le pipeline — au pire le
// dashboard n'a pas l'entrée, mais BMAD continue d'avancer.
func (s *Server) stepObserver(ctx context.Context, projectID, phase string) bmad.StepObserver {
	return bmad.StepObserver{
		OnStart: func(index, total int, command string) {
			res, err := s.db().ExecContext(ctx,
				`INSERT INTO bmad_phase_steps (project_id, phase, command, status)
				 VALUES (?, ?, ?, 'running')`,
				projectID, phase, command)
			if err != nil {
				slog.Warn("bmad step log insert failed", "project", projectID, "cmd", command, "error", err)
				return
			}
			id, _ := res.LastInsertId()
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_started", "api", map[string]any{
					"project_id": projectID,
					"phase":      phase,
					"step_id":    id,
					"index":      index,
					"total":      total,
					"command":    command,
				})
			}
		},
		OnFinish: func(index, total int, command string, res bmad.Result, err error) {
			status := "done"
			errText := ""
			if err != nil {
				status = "failed"
				errText = err.Error()
			}
			preview := res.Text
			if len(preview) > 600 {
				preview = preview[:600] + "…"
			}
			// Mise à jour : on cible la dernière step `running` de la
			// commande + projet + phase. Marche dans 99% des cas ; en
			// concurrence extrême (deux goroutines concurrentes) on
			// pourrait écrire sur la mauvaise ligne mais on bloque
			// déjà les pipelines en parallèle via la map
			// cancellations, donc ce cas n'arrive pas.
			if _, dbErr := s.db().ExecContext(ctx,
				`UPDATE bmad_phase_steps
				 SET finished_at = datetime('now'),
				     status = ?, input_tokens = ?, output_tokens = ?,
				     cost_usd = ?, reply_preview = ?, error_text = ?
				 WHERE id = (
				   SELECT id FROM bmad_phase_steps
				   WHERE project_id = ? AND phase = ? AND command = ? AND status = 'running'
				   ORDER BY started_at DESC LIMIT 1
				 )`,
				status, res.InputTokens, res.OutputTokens, res.CostUSD, preview, errText,
				projectID, phase, command,
			); dbErr != nil {
				slog.Warn("bmad step log finish failed", "project", projectID, "cmd", command, "error", dbErr)
			}
			if res.CostUSD > 0 {
				_, _ = s.db().ExecContext(ctx,
					`UPDATE projects SET total_cost_usd = total_cost_usd + ?,
					 updated_at = datetime('now') WHERE id = ?`,
					res.CostUSD, projectID)
			}
			if s.eventBus != nil {
				_, _ = s.eventBus.Publish(ctx, "project.bmad_step_finished", "api", map[string]any{
					"project_id":    projectID,
					"phase":         phase,
					"index":         index,
					"total":         total,
					"command":       command,
					"status":        status,
					"cost_usd":      res.CostUSD,
					"input_tokens":  res.InputTokens,
					"output_tokens": res.OutputTokens,
					"error":         errText,
				})
			}
		},
	}
}

// intakeAgent returns the PM agent to drive a conversation. Honours
// HIVE_INTAKE_AGENT=scripted so ops can force determinism when Claude
// CLI presence is unpredictable. Defaults to Claude with scripted
// fallback.
func (s *Server) intakeAgent() intake.Agent {
	if s.intakeAgentOverride != nil {
		return s.intakeAgentOverride
	}
	if getenv := s.envLookup; getenv != nil {
		if getenv("HIVE_INTAKE_AGENT") == "scripted" {
			return intake.NewScriptedAgent()
		}
	}
	return intake.NewClaudeCodeAgent()
}

// intakeAgentFor choisit l'agent adapté au projet : brownfield →
// IterationAgent (greeting "projet existant, qu'est-ce que tu veux
// ajouter ?") tout en conservant le role "pm" pour la conversation
// initiale ; greenfield → agent de base.
func (s *Server) intakeAgentFor(p *project.Project) intake.Agent {
	base := s.intakeAgent()
	if p != nil && p.IsExisting {
		return &intake.IterationAgent{Base: base, RoleOverride: intake.RolePM}
	}
	return base
}
