package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/intake"
	"github.com/JulienLeotier/hive/internal/project"
)

// runIterationAsync exécute le pipeline brownfield BMAD : on écrit
// le brief de la nouvelle feature dans un fichier dédié, puis on
// lance IterationPipeline (document-project → edit-prd → etc.).
// Les epics/stories existants sont conservés en DB ; les nouveaux
// seront ingérés en sortie via le même parseur json-hive.
func (s *Server) runIterationAsync(projectID, idea, seedDoc string) {
	// Pas de timeout : le brownfield IterationPipeline (14 skills) peut
	// légitimement tourner >60min sur un gros repo (document-project
	// seul peut prendre 15-20min). Les sorties sont le cancel UI ou
	// le cost cap — pas un hard timeout qui couperait au milieu.
	ctx, cancel := context.WithCancel(context.Background())
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
	res, err := s.trackedInvoke(ctx, runner, projectID, "iteration",
		"hive-ingest-iteration", workdir, ingestGoal)
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

// RecoverStuckPlanning re-kicks the BMAD pipeline pour les projets
// qui étaient légitimement mid-pipeline quand le serveur a crashé :
// status=planning SANS epics et SANS run déjà enregistré.
//
// Règles strictes pour éviter les doubles runs :
//   - status=planning seulement (PAS failed — un projet failed a été
//     arrêté pour une raison, soit cost cap soit user cancel, ne pas
//     relancer auto)
//   - aucun run actif dans runCancels (si le serveur vient juste de
//     hot-reload sous air, un run peut être encore registered)
//   - epics vides (si epics>0 on est dans le devloop, pas planning)
//
// Bug historique : sur un hot-reload air fréquent, cette fonction
// relançait le MÊME projet à chaque redémarrage → 3 pipelines en
// parallèle, 3× le coût, artefacts BMAD corrompus par concurrence
// sur le workdir.
func (s *Server) RecoverStuckPlanning(ctx context.Context) error {
	if s.projectStore == nil {
		return nil
	}
	rows, err := s.db().QueryContext(ctx,
		`SELECT p.id, p.idea
		 FROM projects p
		 WHERE p.status = ?
		   AND NOT EXISTS (SELECT 1 FROM epics e WHERE e.project_id = p.id)`,
		project.StatusPlanning,
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
		// Check si un run est déjà enregistré pour ce projet (cas d'un
		// double appel à RecoverStuckPlanning dans un même process).
		s.runMu.Lock()
		_, alreadyRunning := s.runCancels[st.id]
		s.runMu.Unlock()
		if alreadyRunning {
			slog.Info("skip recovery: run already active", "project", st.id)
			continue
		}
		// Check si le projet a DEJA des phase_steps finis — signifie que
		// BMAD a commencé. Sur un hot-reload (air, crash-restart), on
		// ne veut PAS relancer de zéro et brûler 30$ de tokens à
		// nouveau. L'user cliquera "Relancer" ou "Reprendre au step"
		// manuellement via l'UI s'il le souhaite.
		var existingSteps int
		_ = s.db().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM bmad_phase_steps WHERE project_id = ? AND status = 'done'`,
			st.id).Scan(&existingSteps)
		if existingSteps > 0 {
			slog.Info("skip recovery: project has prior phase steps, leaving for manual retry",
				"project", st.id, "done_steps", existingSteps)
			continue
		}
		slog.Info("recovering stuck project", "project", st.id)
		var seed string
		if s.intakeStore != nil {
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
// runArchitectAsyncFromStep is the resumable sibling of
// runArchitectAsync. `fromStep` is 0-based : 0 runs the full pipeline,
// N skips the first N skills (useful when a retry should pick up where
// the previous run died instead of re-running the expensive early
// skills like /bmad-create-prd). Ingestion (readFirst, parseBMADTree…)
// is the same because it only re-reads what BMAD wrote on disk — safe
// to re-run regardless of fromStep.
func (s *Server) runArchitectAsyncFromStep(projectID, idea, seedDoc string, fromStep int) {
	s.runArchitectAsyncInternal(projectID, idea, seedDoc, fromStep)
}

func (s *Server) runArchitectAsync(projectID, idea, seedDoc string) {
	s.runArchitectAsyncInternal(projectID, idea, seedDoc, 0)
}

func (s *Server) runArchitectAsyncInternal(projectID, idea, seedDoc string, fromStep int) {
	// Pas de timeout. FullPlanningPipeline = 13 skills qui peuvent
	// chacune prendre 3-15 min avec le vrai Claude sur un projet
	// moyen. Couper à 90min forçait des failures sur les pipelines
	// légitimes qui prenaient juste 2h. Le cancel UI et le cost cap
	// sont les sorties — pas un cap wall-clock arbitraire.
	ctx, cancel := context.WithCancel(context.Background())
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

	// Pré-écrit le brief dans planning-artifacts. seedDoc est le
	// Product Brief SCOPE LOCKED produit par le PM agent de l'intake.
	// On l'écrit sous deux chemins pour que /bmad-create-prd le trouve
	// sans qu'on ait à patcher la skill BMAD :
	//   - _intake.md : conservé pour audit + fallback si create-prd lit ce nom
	//   - product-brief-<slug>.md : nom standard que BMAD utilisait
	//     quand /bmad-product-brief tournait, à présent c'est le PM qui
	//     l'écrit directement.
	if err := os.MkdirAll(filepath.Join(workdir, bmad.PlanningDir), 0o755); err != nil {
		fail("prepare", err)
		return
	}
	slug := projectSlug(proj.Name)
	intakePath := filepath.Join(workdir, bmad.PlanningDir, "_intake.md")
	if err := os.WriteFile(intakePath, []byte(buildIntakeDoc(idea, seedDoc)), 0o644); err != nil {
		fail("prepare", err)
		return
	}
	briefPath := filepath.Join(workdir, bmad.PlanningDir, "product-brief-"+slug+".md")
	if err := os.WriteFile(briefPath, []byte(seedDoc), 0o644); err != nil {
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
	// Retry-from-step : on peut sauter les N premiers skills quand on
	// relance après un échec tardif. Ingestion et writes BMAD sont
	// idempotents (la skill bmad-create-prd détecte un PRD existant,
	// etc.), mais skipper évite de brûler ~2$ de tokens par skill
	// déjà-réussi. fromStep=0 = comportement historique.
	resumed := pipeline
	if fromStep > 0 && fromStep < len(pipeline) {
		resumed = pipeline[fromStep:]
		slog.Info("bmad resume", "project", projectID,
			"from_step", fromStep, "remaining", len(resumed))
	}
	if _, err := runner.RunSequenceObserved(ctx, workdir, resumed, obs); err != nil {
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
	res, err := s.trackedInvoke(ctx, runner, projectID, phaseLabel,
		"hive-ingest-epics", workdir, ingestGoal)
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

// projectSlug produit un nom de fichier friendly à partir du nom du
// projet, utilisé pour écrire product-brief-<slug>.md au format que
// BMAD attendait quand /bmad-product-brief tournait. Préserve uniquement
// [a-z0-9-]. Fallback "project" si la normalisation ne laisse rien.
func projectSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
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

