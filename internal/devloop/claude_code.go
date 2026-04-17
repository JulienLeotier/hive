package devloop

import (
	"context"
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
}

func NewClaudeCodeDev() DevAgent {
	fallback := NewScriptedDev()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop dev: claude CLI absent — fallback scripted")
		return fallback
	}
	return &ClaudeCodeDev{runner: r, fallback: fallback, timeout: 30 * time.Minute}
}

func (*ClaudeCodeDev) Name() string { return "bmad-dev" }

// Develop lance la séquence /bmad-create-story puis /bmad-dev-story.
// BMAD choisit lui-même la prochaine story ready-for-dev dans
// sprint-status.yaml. On collecte l'URL de PR + la branche soit
// depuis la story file BMAD (front-matter yaml), soit en fallback
// depuis le stdout des skills.
func (d *ClaudeCodeDev) Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, _ string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Snapshot sprint-status AVANT que BMAD tourne, pour que le
	// Reviewer puisse détecter la story que la skill a traitée.
	pre := snapshotSprint(workdir)

	history, err := d.runner.RunSequence(callCtx, workdir, bmad.StorySequence)
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
}

func NewClaudeCodeReviewer() ReviewerAgent {
	fallback := NewScriptedReviewer()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop reviewer: claude CLI absent — fallback scripted")
		return fallback
	}
	return &ClaudeCodeReviewer{runner: r, fallback: fallback, timeout: 12 * time.Minute}
}

func (*ClaudeCodeReviewer) Name() string { return "bmad-reviewer" }

// Review lance /bmad-code-review. Après coup, on parse
// sprint-status.yaml avec un vrai parser yaml pour déterminer si la
// story est passée à "ready-for-done" (pass) ou renvoyée en
// "ready-for-dev" (fail à ré-itérer).
func (r *ClaudeCodeReviewer) Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	history, err := r.runner.RunSequence(callCtx, workdir, bmad.ReviewSequence)
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
	if key != "" {
		if st, _ := bmad.ReadSprintStatus(workdir); st != nil {
			status := st.StoryStatus(key)
			switch status {
			case "ready-for-done", "done", "approved":
				pass = true
				reason = "BMAD code-review : " + status
			case "ready-for-dev":
				reason = "BMAD review : renvoyée en ready-for-dev"
			case "":
				reason = "story " + key + " absente de sprint-status.yaml"
			default:
				reason = "BMAD review : " + status
			}
		}
	}

	feedback := ""
	if len(history) > 0 {
		feedback = firstLine(history[len(history)-1].Reply)
	}
	if feedback == "" {
		feedback = reason
	}
	verdict := ReviewVerdict{Pass: pass, Feedback: feedback}
	for _, ac := range story.ACs {
		verdict.ACs = append(verdict.ACs, ReviewedCriterion{
			ID: ac.ID, Passed: pass, Reason: reason,
		})
	}
	return verdict, nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
