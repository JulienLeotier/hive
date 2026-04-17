package devloop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
)

// ClaudeCodeDev drives a story through the real BMAD `bmad-dev-story`
// skill. The Hive supervisor picks the next pending story per
// project per tick; here we hand that story to Claude + BMAD and
// collect the implementation output.
//
// No more hand-rolled dev prompt — BMAD's skill defines the dev
// process (implement, run tests, update the story file's dev record,
// etc.). We just feed it the story spec and the iteration context.
type ClaudeCodeDev struct {
	runner   *bmad.Runner
	fallback DevAgent
	timeout  time.Duration
}

// NewClaudeCodeDev returns the BMAD-backed Dev agent. Falls back to
// ScriptedDev when the claude CLI isn't on PATH so a fresh CI check
// doesn't dead-end.
func NewClaudeCodeDev() DevAgent {
	fallback := NewScriptedDev()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop dev: claude CLI missing — using scripted dev")
		return fallback
	}
	return &ClaudeCodeDev{runner: r, fallback: fallback, timeout: 20 * time.Minute}
}

// Name tags the agent in reviews + events.
func (*ClaudeCodeDev) Name() string { return "bmad-dev" }

// Develop invokes `bmad-dev-story` against the project's workdir. The
// skill expects BMAD to already be installed (the architect phase did
// that) and a story spec it can read. Since our stories come out of
// our own DB, we pass the spec inline in the prompt rather than
// writing a story file to disk — BMAD's skill accepts a
// context-filled story description in the activation text.
func (d *ClaudeCodeDev) Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, feedback string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	goal := buildDevGoal(proj, story, iteration, feedback)
	res, err := d.runner.Invoke(callCtx, workdir, goal, nil)
	if err != nil {
		slog.Warn("devloop dev: bmad-dev-story failed — falling back to scripted",
			"error", err)
		return d.fallback.Develop(ctx, proj, story, iteration, feedback)
	}
	if strings.TrimSpace(res.Text) == "" {
		slog.Warn("devloop dev: bmad-dev-story returned empty — falling back to scripted")
		return d.fallback.Develop(ctx, proj, story, iteration, feedback)
	}
	return DevOutput{
		Summary: firstLine(res.Text),
		Details: strings.TrimSpace(res.Text),
	}, nil
}

// ClaudeCodeReviewer uses `bmad-code-review` for per-AC verdicts. The
// skill inspects the implementation and returns a verdict we parse
// out of a JSON block Claude emits at the end of its reply.
type ClaudeCodeReviewer struct {
	runner   *bmad.Runner
	fallback ReviewerAgent
	timeout  time.Duration
}

// NewClaudeCodeReviewer returns the BMAD-backed Reviewer.
func NewClaudeCodeReviewer() ReviewerAgent {
	fallback := NewScriptedReviewer()
	r := bmad.NewRunner()
	if r == nil {
		slog.Info("devloop reviewer: claude CLI missing — using scripted reviewer")
		return fallback
	}
	return &ClaudeCodeReviewer{runner: r, fallback: fallback, timeout: 8 * time.Minute}
}

// Name tags the agent in the reviews table.
func (*ClaudeCodeReviewer) Name() string { return "bmad-reviewer" }

// Review runs `bmad-code-review` and asks Claude to append a
// json-hive fenced block with the per-AC verdicts so our supervisor
// can act on the result without a second parse pass.
func (r *ClaudeCodeReviewer) Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	goal := buildReviewGoal(proj, story, output)
	res, err := r.runner.Invoke(callCtx, workdir, goal, nil)
	if err != nil {
		slog.Warn("devloop reviewer: bmad-code-review failed — falling back to scripted",
			"error", err)
		return r.fallback.Review(ctx, proj, story, output)
	}

	raw := extractJSONHive(res.Text)
	if raw == "" {
		slog.Warn("devloop reviewer: no json-hive block in reply — falling back to scripted",
			"raw", truncateString(res.Text, 200))
		return r.fallback.Review(ctx, proj, story, output)
	}
	var parsed struct {
		Pass     bool   `json:"pass"`
		Feedback string `json:"feedback"`
		ACs      []struct {
			ID     int64  `json:"id"`
			Passed bool   `json:"passed"`
			Reason string `json:"reason"`
		} `json:"acs"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		slog.Warn("devloop reviewer: parse json-hive failed — falling back",
			"error", err, "raw", truncateString(raw, 200))
		return r.fallback.Review(ctx, proj, story, output)
	}

	seen := map[int64]bool{}
	verdict := ReviewVerdict{Pass: parsed.Pass, Feedback: parsed.Feedback}
	for _, rc := range parsed.ACs {
		seen[rc.ID] = true
		verdict.ACs = append(verdict.ACs, ReviewedCriterion{
			ID: rc.ID, Passed: rc.Passed, Reason: rc.Reason,
		})
	}
	for _, ac := range story.ACs {
		if !seen[ac.ID] {
			verdict.Pass = false
			verdict.ACs = append(verdict.ACs, ReviewedCriterion{
				ID: ac.ID, Passed: false,
				Reason: "reviewer did not return a verdict for this AC",
			})
		}
	}
	return verdict, nil
}

// buildDevGoal describes one story-iteration task to BMAD's dev skill.
// Passed inline because our stories live in the Hive DB, not in the
// BMAD-native story file format — the skill still applies its dev
// process to whatever context it's handed.
func buildDevGoal(proj ProjectContext, story Story, iteration int, feedback string) string {
	var b strings.Builder
	b.WriteString("Invoke the bmad-dev-story skill to implement the story below. ")
	b.WriteString("Treat this prompt as the context-filled story spec — the story text ")
	b.WriteString("stands in for a BMAD story file. Apply BMAD's dev process: ")
	b.WriteString("implement all tasks, verify every acceptance criterion, run tests where a ")
	b.WriteString("test command is obvious, commit is handled by the outer Hive supervisor.\n\n")
	fmt.Fprintf(&b, "Project idea: %s\n", proj.Idea)
	if proj.PRD != "" {
		fmt.Fprintf(&b, "\nPRD excerpt (first 1500 chars):\n%s\n", truncateString(proj.PRD, 1500))
	}
	fmt.Fprintf(&b, "\nStory: %s (iteration %d of %d)\n", story.Title, iteration, MaxIterations)
	if story.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", story.Description)
	}
	b.WriteString("\nAcceptance criteria (must ALL be satisfied on disk):\n")
	for _, ac := range story.ACs {
		fmt.Fprintf(&b, "- %s\n", ac.Text)
	}
	if feedback != "" {
		fmt.Fprintf(&b, "\nPrevious review feedback (address every point):\n%s\n", feedback)
	}
	b.WriteString("\nAt the end of your reply, write a short human summary of what files you changed or created. ")
	b.WriteString("The reviewer will cross-check your summary against the ACs.\n")
	return b.String()
}

// buildReviewGoal describes one review task to BMAD's code-review
// skill with a structured JSON return contract.
func buildReviewGoal(proj ProjectContext, story Story, output DevOutput) string {
	var b strings.Builder
	b.WriteString("Invoke the bmad-code-review skill. Evaluate the implementation that the ")
	b.WriteString("Dev agent just landed in this workdir against every acceptance criterion below. ")
	b.WriteString("You MAY read files in the workdir to cross-check claims; you MUST NOT modify them.\n\n")
	b.WriteString("When the skill finishes, emit ONE fenced code block with language `json-hive` ")
	b.WriteString("containing valid JSON of exactly this shape (no prose after):\n")
	b.WriteString("```json-hive\n")
	b.WriteString(`{"pass": <bool>, "feedback": "<why it failed, empty on pass>", "acs": [{"id": <int>, "passed": <bool>, "reason": "<one sentence>"}]}`)
	b.WriteString("\n```\n\n")
	fmt.Fprintf(&b, "Project idea: %s\n", proj.Idea)
	fmt.Fprintf(&b, "Story: %s\n\n", story.Title)
	b.WriteString("Acceptance criteria (use these exact ids in your response):\n")
	for _, ac := range story.ACs {
		fmt.Fprintf(&b, "- id=%d: %s\n", ac.ID, ac.Text)
	}
	fmt.Fprintf(&b, "\nDev summary:\n%s\n", output.Summary)
	if output.Details != "" {
		fmt.Fprintf(&b, "\nDev details:\n%s\n", truncateString(output.Details, 2000))
	}
	return b.String()
}

// extractJSONHive pulls the last ```json-hive ... ``` block out of a
// reply. Returns empty string when absent.
func extractJSONHive(s string) string {
	marker := "```json-hive"
	start := strings.LastIndex(s, marker)
	if start < 0 {
		return ""
	}
	body := s[start+len(marker):]
	end := strings.Index(body, "```")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(body[:end])
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func truncateString(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
