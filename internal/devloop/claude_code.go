package devloop

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/git"
)

// ClaudeCodeDev et ClaudeCodeReviewer lancent les séquences BMAD
// officielles (workflow.go) telles que décrites dans les docs :
//
//   Dev  : /bmad-create-story puis /bmad-dev-story
//   Review : /bmad-code-review
//
// Les skills BMAD elles-mêmes gèrent les branches, commits, push,
// ouverture de PR, sélection de la story courante via
// sprint-status.yaml. Hive ne fait que lancer les commandes et lire
// les artefacts.

type ClaudeCodeDev struct {
	runner   *bmad.Runner
	fallback DevAgent
	timeout  time.Duration
	db       *sql.DB        // branch to track skill cost + sync sprint-status ; nil = no tracking
	publish  Publisher      // branch to emit WS events (bmad_step_output) ; nil = no live streaming UI
	registry CancelRegistry // branch to register per-step cancel ; nil = cancel par skill indispo
}

func NewClaudeCodeDev() DevAgent {
	fallback := NewScriptedDev()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop dev: claude CLI absent — fallback scripted")
		return fallback
	}
	// timeout 0 → hérite du parent ctx. Une story légitime peut
	// prendre >30min en dev-story sur du code non trivial ; ne pas
	// couper arbitrairement.
	return &ClaudeCodeDev{runner: r, fallback: fallback, timeout: 0}
}

// WithDB branche la DB pour que chaque invocation /bmad-dev-story
// track son coût en bmad_phase_steps + re-sync sprint-status.yaml
// vers les stories Hive. Sans DB, le devloop tourne comme avant
// (invisible aux dashboards mais fonctionnellement correct).
func (d *ClaudeCodeDev) WithDB(db *sql.DB) *ClaudeCodeDev {
	d.db = db
	return d
}

// WithPublisher branche l'event bus pour que chaque event stream-json
// de Claude soit diffusé en WS (project.bmad_step_output) et que la
// console UI se remplisse en live, pas juste à la fin du skill.
func (d *ClaudeCodeDev) WithPublisher(p Publisher) *ClaudeCodeDev {
	d.publish = p
	return d
}

// WithCancelRegistry branche le registre step-level pour que l'UI
// puisse annuler UN skill précis sans tuer toute la story.
func (d *ClaudeCodeDev) WithCancelRegistry(r CancelRegistry) *ClaudeCodeDev {
	d.registry = r
	return d
}

func (*ClaudeCodeDev) Name() string { return "bmad-dev" }

// Develop lance la séquence /bmad-create-story puis /bmad-dev-story.
// BMAD choisit lui-même la prochaine story ready-for-dev dans
// sprint-status.yaml. On collecte l'URL de PR + la branche soit
// depuis la story file BMAD (front-matter yaml), soit en fallback
// depuis le stdout des skills.
func (d *ClaudeCodeDev) Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, _ string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	callCtx := ctx
	cancel := func() {}
	if d.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, d.timeout)
	}
	defer cancel()

	// Snapshot sprint-status AVANT que BMAD tourne, pour que le
	// Reviewer puisse détecter la story que la skill a traitée.
	pre := snapshotSprint(workdir)

	obs := makeDevloopObserver(d.db, d.publish, d.registry, proj.ID, workdir, "story")
	history, err := d.runner.RunSequenceObserved(callCtx, workdir, bmad.StorySequence, obs)
	if err != nil {
		slog.Warn("devloop dev: séquence BMAD échouée — fallback scripted", "error", err)
		return d.fallback.Develop(ctx, proj, story, iteration, "")
	}

	combined := ""
	for _, step := range history {
		combined += step.Reply + "\n\n"
	}
	out := DevOutput{
		Summary:         firstLine(combined),
		Details:         strings.TrimSpace(combined),
		PreSprintStatus: pre,
	}
	// Branche + PR : on diffe la snapshot pour trouver la clé BMAD
	// que dev-story vient d'activer, puis on lit la story file
	// correspondante (elle contient branch + pr_url en front-matter
	// quand BMAD a poussé). Fallback regex sur le stdout.
	if key := activeBMADKey(pre, workdir); key != "" {
		if sf, _ := bmad.ReadStoryFile(workdir, key); sf != nil {
			out.Branch = sf.Branch
			out.PRURL = sf.PRURL
		}
	}
	if out.PRURL == "" {
		out.PRURL = bmad.ExtractPRURL(combined)
	}
	// Hive fallback : si BMAD n'a ni créé de PR ni commité, on le fait
	// nous-mêmes pour éviter que le projet wedge silencieusement avec
	// du code en local jamais poussé. Idempotent — si tout est déjà
	// fait, c'est un no-op.
	if out.PRURL == "" {
		if url, err := git.EnsureStoryPushed(ctx, workdir, out.Branch, firstLine(combined)); err != nil {
			slog.Warn("hive fallback git push failed",
				"workdir", workdir, "error", err)
		} else if url != "" {
			out.PRURL = url
			slog.Info("hive fallback pushed PR", "url", url)
		}
	}
	return out, nil
}

// snapshotSprint renvoie une copie de development_status, ou une
// map vide si le fichier n'existe pas encore.
func snapshotSprint(workdir string) map[string]string {
	st, err := bmad.ReadSprintStatus(workdir)
	if err != nil || st == nil {
		return map[string]string{}
	}
	cp := make(map[string]string, len(st.DevelopmentStatus))
	for k, v := range st.DevelopmentStatus {
		cp[k] = v
	}
	return cp
}

// activeBMADKey cherche la clé de story qui a bougé entre la
// snapshot pre-dev et l'état actuel. On privilégie celle qui a
// transité hors de "ready-for-dev" (ce que dev-story fait en premier).
func activeBMADKey(pre map[string]string, workdir string) string {
	post, err := bmad.ReadSprintStatus(workdir)
	if err != nil || post == nil {
		return ""
	}
	for k, newStatus := range post.DevelopmentStatus {
		old := pre[k]
		if old == "ready-for-dev" && newStatus != "ready-for-dev" {
			return k
		}
		if old == "" && newStatus == "in-progress" {
			return k
		}
		if old == "" && newStatus == "review" {
			return k
		}
	}
	return ""
}

type ClaudeCodeReviewer struct {
	runner   *bmad.Runner
	fallback ReviewerAgent
	timeout  time.Duration
	db       *sql.DB
	publish  Publisher
	registry CancelRegistry
}

func NewClaudeCodeReviewer() ReviewerAgent {
	fallback := NewScriptedReviewer()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop reviewer: claude CLI absent — fallback scripted")
		return fallback
	}
	// timeout 0 → hérite du parent ctx. Même logique que le Dev.
	return &ClaudeCodeReviewer{runner: r, fallback: fallback, timeout: 0}
}

// WithDB — même rôle que ClaudeCodeDev.WithDB : active le tracking
// cost + sync sprint-status après chaque /bmad-code-review.
func (r *ClaudeCodeReviewer) WithDB(db *sql.DB) *ClaudeCodeReviewer {
	r.db = db
	return r
}

// WithPublisher — même rôle que ClaudeCodeDev.WithPublisher : broadcast
// les chunks stream-json pour que la console UI défile en live.
func (r *ClaudeCodeReviewer) WithPublisher(p Publisher) *ClaudeCodeReviewer {
	r.publish = p
	return r
}

// WithCancelRegistry — même rôle que ClaudeCodeDev.WithCancelRegistry.
func (r *ClaudeCodeReviewer) WithCancelRegistry(reg CancelRegistry) *ClaudeCodeReviewer {
	r.registry = reg
	return r
}

func (*ClaudeCodeReviewer) Name() string { return "bmad-reviewer" }

// Review lance /bmad-code-review. Après coup, on parse
// sprint-status.yaml avec un vrai parser yaml pour déterminer si la
// story est passée à "ready-for-done" (pass) ou renvoyée en
// "ready-for-dev" (fail à ré-itérer).
func (r *ClaudeCodeReviewer) Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	workdir := pickWorkdir(proj)
	callCtx := ctx
	cancel := func() {}
	if r.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, r.timeout)
	}
	defer cancel()

	obs := makeDevloopObserver(r.db, r.publish, r.registry, proj.ID, workdir, "review")
	history, err := r.runner.RunSequenceObserved(callCtx, workdir, bmad.ReviewSequence, obs)
	if err != nil {
		slog.Warn("devloop reviewer: séquence BMAD échouée — fallback scripted", "error", err)
		return r.fallback.Review(ctx, proj, story, output)
	}

	// Identifier la clé de story active en diffant pre vs post. C'est
	// la même clé que celle détectée dans Develop() — BMAD ne change
	// pas de story entre dev-story et code-review.
	key := activeBMADKey(output.PreSprintStatus, workdir)
	pass := false
	reason := "verdict indéterminé"
	// bmadDoneStatuses : statuts BMAD (wire strings de sprint-status.yaml)
	// qui comptent comme "story validée". Distincts de Hive storyStatusDone
	// — dédup sémantique seulement.
	bmadDoneStatuses := map[string]bool{
		"ready-for-done": true,
		"done":           true,
		"approved":       true,
	}
	if key != "" {
		if st, _ := bmad.ReadSprintStatus(workdir); st != nil {
			status := st.StoryStatus(key)
			switch {
			case bmadDoneStatuses[status]:
				pass = true
				reason = "BMAD code-review : " + status
			case status == "ready-for-dev":
				reason = "BMAD review : renvoyée en ready-for-dev"
			case status == "":
				reason = "story " + key + " absente de sprint-status.yaml"
			default:
				reason = "BMAD review : " + status
			}
		}
	}

	feedback := ""
	combined := ""
	if len(history) > 0 {
		feedback = firstLine(history[len(history)-1].Reply)
		for _, step := range history {
			combined += step.Reply + "\n\n"
		}
	}
	if feedback == "" {
		feedback = reason
	}
	decisions := countDecisionNeeded(combined)
	verdict := ReviewVerdict{
		Pass:           pass,
		Feedback:       feedback,
		NeedsArchitect: !pass && decisions > 0,
		DecisionCount:  decisions,
	}
	for _, ac := range story.ACs {
		verdict.ACs = append(verdict.ACs, ReviewedCriterion{
			ID: ac.ID, Passed: pass, Reason: reason,
		})
	}
	return verdict, nil
}

// countDecisionNeeded scanne le feedback d'un /bmad-code-review pour
// compter les findings tagged "decision-needed". BMAD utilise cette
// catégorie quand un finding ne peut pas être réglé par le dev seul :
// il faut un arbitrage d'architecte (choix d'API, trade-off perf vs
// lisibilité, rupture de compat). On matche large — BMAD formate
// parfois en "Category: decision-needed" ou "[decision-needed]" ou
// juste "decision needed" dans les PRs.
func countDecisionNeeded(reply string) int {
	if reply == "" {
		return 0
	}
	lowered := strings.ToLower(reply)
	count := strings.Count(lowered, "decision-needed")
	count += strings.Count(lowered, "decision_needed")
	// "decision needed" sans tiret : on le compte aussi, mais on
	// soustrait les occurrences déjà couvertes par "decision-needed"
	// pour éviter le double-compte (le tiret compte comme match du
	// pattern sans tiret si on lowercase). En pratique les deux formes
	// co-existent rarement ; on garde simple.
	loose := strings.Count(lowered, "decision needed")
	if loose > count {
		count = loose
	}
	return count
}

// ClaudeCodeArchitect pilote l'escalation autonome : quand le
// Reviewer a tagged des findings "decision-needed", Hive invoque
// cet agent pour réveiller /bmad-agent-architect et lancer
// /bmad-correct-course qui committe la décision dans la story.md.
// Après ça, le Dev reprendra avec la nouvelle spec au prochain tick.
type ClaudeCodeArchitect struct {
	runner   *bmad.Runner
	db       *sql.DB
	publish  Publisher
	registry CancelRegistry
}

// NewClaudeCodeArchitect construit l'agent. Retourne nil si le Claude
// CLI n'est pas disponible — l'appelant vérifie et skip l'escalation.
func NewClaudeCodeArchitect() *ClaudeCodeArchitect {
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop architect: claude CLI absent — escalation désactivée")
		return nil
	}
	return &ClaudeCodeArchitect{runner: r}
}

// WithDB active le tracking cost + reply_preview pour les skills
// architect via le même makeDevloopObserver que dev/review.
func (a *ClaudeCodeArchitect) WithDB(db *sql.DB) *ClaudeCodeArchitect {
	a.db = db
	return a
}

// WithPublisher broadcast les chunks stream-json des skills architect
// pour que la console UI défile en live pendant un correct-course.
func (a *ClaudeCodeArchitect) WithPublisher(p Publisher) *ClaudeCodeArchitect {
	a.publish = p
	return a
}

// WithCancelRegistry — même rôle que ClaudeCodeDev.WithCancelRegistry.
func (a *ClaudeCodeArchitect) WithCancelRegistry(r CancelRegistry) *ClaudeCodeArchitect {
	a.registry = r
	return a
}

func (*ClaudeCodeArchitect) Name() string { return "bmad-architect" }

// Resolve lance /bmad-agent-architect puis /bmad-correct-course avec
// le feedback de review en contexte, pour que l'architect puisse
// trancher sur les findings decision-needed et mettre à jour la
// story.md. BMAD fait le commit de la modification de spec ; Hive ne
// touche pas au code.
func (a *ClaudeCodeArchitect) Resolve(ctx context.Context, proj ProjectContext, story Story, reviewFeedback string) error {
	if a == nil || a.runner == nil {
		return fmt.Errorf("architect: runner indisponible")
	}
	workdir := pickWorkdir(proj)
	obs := makeDevloopObserver(a.db, a.publish, a.registry, proj.ID, workdir, "architect")
	_, err := a.runner.RunSequenceObserved(ctx, workdir, bmad.ArchitectEscalationSequence, obs)
	if err != nil {
		return fmt.Errorf("architect escalation: %w", err)
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// makeDevloopObserver construit un StepObserver qui :
//  1. Insère une ligne running dans bmad_phase_steps à chaque skill
//  2. L'update en done/failed avec cost + tokens à la fin
//  3. Incrémente projects.total_cost_usd pour le dashboard /costs
//  4. Re-syncise sprint-status.yaml → stories Hive après chaque finish
//     (corrige le déphasage quand BMAD a touché plusieurs stories en
//     parallèle pendant la même invocation)
//
// Si db est nil, renvoie un observer neutre (pas de tracking). Permet
// au fallback scripted de tourner sans polluer la DB.
func makeDevloopObserver(db *sql.DB, publish Publisher, registry CancelRegistry, projectID, workdir, phase string) bmad.StepObserver {
	if db == nil || projectID == "" {
		return bmad.StepObserver{}
	}
	// stepID capture par closure entre OnStart et OnFinish. Les skills
	// d'une sequence tournent sequentiellement, donc une seule variable
	// suffit. buffer accumule les events stream-json pour flush DB +
	// broadcast WS en live — permet à la console UI de défiler.
	var (
		stepID int64
		buffer strings.Builder
	)
	return bmad.StepObserver{
		OnStart: func(_, _ int, cmd string, stepCancel context.CancelFunc) {
			buffer.Reset()
			res, err := db.Exec(
				`INSERT INTO bmad_phase_steps (project_id, phase, command, status)
				 VALUES (?, ?, ?, 'running')`,
				projectID, phase, cmd)
			if err != nil {
				slog.Warn("devloop obs: insert running failed",
					"project", projectID, "cmd", cmd, "error", err)
				stepID = 0
				return
			}
			stepID, _ = res.LastInsertId()
			if registry != nil {
				registry.RegisterStepCancel(stepID, stepCancel)
			}
		},
		OnChunk: func(_, _ int, cmd string, evt bmad.StreamEvent) {
			if evt.Text == "" {
				return
			}
			line := "[" + evt.Type + "] " + evt.Text + "\n"
			buffer.WriteString(line)
			if stepID > 0 {
				_, _ = db.Exec(
					`UPDATE bmad_phase_steps SET reply_full = ? WHERE id = ?`,
					buffer.String(), stepID)
			}
			if publish != nil {
				_ = publish(context.Background(), "project.bmad_step_output", "devloop",
					map[string]any{
						"project_id": projectID,
						"step_id":    stepID,
						"command":    cmd,
						"chunk":      line,
						"event_type": evt.Type,
					})
			}
		},
		OnFinish: func(_, _ int, cmd string, r bmad.Result, err error) {
			status := "done"
			errText := ""
			if err != nil {
				status = "failed"
				errText = err.Error()
			}
			preview := r.Text
			if len(preview) > 600 {
				preview = preview[:600] + "…"
			}
			if stepID > 0 {
				_, _ = db.Exec(
					`UPDATE bmad_phase_steps
					 SET finished_at = datetime('now'), status = ?,
					     input_tokens = ?, output_tokens = ?, cost_usd = ?,
					     reply_preview = ?, reply_full = ?, error_text = ?
					 WHERE id = ?`,
					status, r.InputTokens, r.OutputTokens, r.CostUSD,
					preview, r.Text, errText, stepID)
				if registry != nil {
					registry.ClearStepCancel(stepID)
				}
			}
			if r.CostUSD > 0 {
				_, _ = db.Exec(
					`UPDATE projects SET total_cost_usd = total_cost_usd + ?,
					 updated_at = datetime('now') WHERE id = ?`,
					r.CostUSD, projectID)
			}
			// Re-sync sprint-status.yaml → DB stories. Fait à CHAQUE
			// fin de skill, donc si BMAD a touché 5 stories dans une
			// seule invocation, on les rattrape toutes d'un coup.
			syncSprintStatus(context.Background(), db, projectID, workdir)
		},
	}
}
