package devloop

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
)

// ClaudeCodeDev et ClaudeCodeReviewer ne font RIEN d'autre que lancer
// automatiquement les slash-commands BMAD (`/bmad-dev-story`,
// `/bmad-code-review`). Branches, commits, push, PR, tests, relecture
// par AC — tout est géré par le framework BMAD lui-même. Hive lit
// ensuite `_bmad-output/implementation-artifacts/sprint-status.yaml`
// pour connaître l'issue de l'itération.

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
	return &ClaudeCodeDev{runner: r, fallback: fallback, timeout: 25 * time.Minute}
}

func (*ClaudeCodeDev) Name() string { return "bmad-dev" }

// Develop invoque /bmad-dev-story. BMAD choisit la story
// ready-for-dev suivante dans sprint-status.yaml, crée une branche,
// code, commit, push et ouvre la PR lui-même. Aucune orchestration
// Go par-dessus.
func (d *ClaudeCodeDev) Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, _ string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	res, err := d.runner.Invoke(callCtx, workdir, "/bmad-dev-story", nil)
	if err != nil {
		slog.Warn("devloop dev: /bmad-dev-story a échoué — fallback scripted", "error", err)
		return d.fallback.Develop(ctx, proj, story, iteration, "")
	}
	out := DevOutput{Summary: firstLine(res.Text), Details: strings.TrimSpace(res.Text)}
	if branch := extractBranch(res.Text); branch != "" {
		out.Branch = branch
	}
	if pr := extractPRURL(res.Text); pr != "" {
		out.PRURL = pr
	}
	return out, nil
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
	return &ClaudeCodeReviewer{runner: r, fallback: fallback, timeout: 10 * time.Minute}
}

func (*ClaudeCodeReviewer) Name() string { return "bmad-reviewer" }

// Review invoque /bmad-code-review. BMAD lit le diff de la PR, poste
// ses commentaires et met à jour sprint-status.yaml. Hive relit le
// fichier pour déterminer pass/fail.
func (r *ClaudeCodeReviewer) Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	res, err := r.runner.Invoke(callCtx, workdir, "/bmad-code-review", nil)
	if err != nil {
		slog.Warn("devloop reviewer: /bmad-code-review a échoué — fallback scripted", "error", err)
		return r.fallback.Review(ctx, proj, story, output)
	}

	pass := readBMADVerdict(workdir, story)
	verdict := ReviewVerdict{
		Pass:     pass,
		Feedback: firstLine(res.Text),
	}
	// Hive a toujours besoin d'un verdict par AC pour son tableau de
	// bord ; on duplique simplement le verdict global.
	for _, ac := range story.ACs {
		verdict.ACs = append(verdict.ACs, ReviewedCriterion{
			ID:     ac.ID,
			Passed: pass,
			Reason: statusReason(pass),
		})
	}
	return verdict, nil
}

// readBMADVerdict lit sprint-status.yaml et retourne true quand
// development_status[<story>] vaut "ready-for-done" ou "done" — les
// deux états "pass" de BMAD après code-review. Retourne false si on
// ne trouve rien de concluant : le superviseur ré-itérera.
func readBMADVerdict(workdir string, story Story) bool {
	path := filepath.Join(workdir, "_bmad-output", "implementation-artifacts", "sprint-status.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return matchStatus(string(data), story.Title, "ready-for-done", "done", "approved")
}

// matchStatus cherche la clé qui ressemble le plus au titre de la
// story dans la section development_status du yaml, et teste si sa
// valeur est un des statuts "pass". Parser yaml minimal : on évite
// une dépendance supplémentaire pour deux regex triviaux.
func matchStatus(yaml, storyTitle string, passValues ...string) bool {
	slug := strings.ToLower(strings.ReplaceAll(storyTitle, " ", "-"))
	slug = nonAlnum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(slug) + `[^\n]*:\s*["']?([a-z0-9\-]+)["']?`)
	m := re.FindStringSubmatch(yaml)
	if len(m) < 2 {
		return false
	}
	status := strings.ToLower(m[1])
	for _, v := range passValues {
		if status == strings.ToLower(v) {
			return true
		}
	}
	return false
}

var (
	nonAlnum      = regexp.MustCompile(`[^a-z0-9]+`)
	branchHintRe  = regexp.MustCompile(`(?i)branch[^\n]*?[` + "`" + `"']([a-zA-Z0-9/_\-.]+)[` + "`" + `"']`)
	prURLRe       = regexp.MustCompile(`https://github\.com/[^\s)]+/pull/\d+`)
)

func extractBranch(s string) string {
	if m := branchHintRe.FindStringSubmatch(s); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func extractPRURL(s string) string {
	return prURLRe.FindString(s)
}

func statusReason(pass bool) string {
	if pass {
		return "BMAD code-review : ready-for-done"
	}
	return "BMAD code-review : story non validée, voir la PR"
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
