package devloop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCodeDev drives a story via the local `claude` CLI. The CLI is
// invoked in the project's workdir (or repo_path when set) so Claude
// Code's native file-editing tools write to the right place. Falls back
// to ScriptedDev on any failure — a missing CLI or timeout must not
// dead-end a build.
type ClaudeCodeDev struct {
	cliPath  string
	fallback DevAgent
	timeout  time.Duration
}

// NewClaudeCodeDev returns a Dev agent that prefers Claude Code.
func NewClaudeCodeDev() DevAgent {
	fallback := NewScriptedDev()
	cli, err := exec.LookPath("claude")
	if err != nil {
		slog.Info("devloop dev: claude CLI not found — using scripted dev", "error", err)
		return fallback
	}
	return &ClaudeCodeDev{cliPath: cli, fallback: fallback, timeout: 10 * time.Minute}
}

// Name tags the agent in reviews + events.
func (*ClaudeCodeDev) Name() string { return "claude-dev" }

// Develop invokes Claude with a prompt that includes the story, ACs,
// and any previous review feedback. Claude writes files directly via
// its own tools; we just capture its summary. Because we can't reliably
// observe a diff from a non-interactive CLI call, FilesTouched stays
// empty and the reviewer compares against what Claude *says* it did.
func (d *ClaudeCodeDev) Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, feedback string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	prompt := buildDevPrompt(proj, story, iteration, feedback)
	cmd := exec.CommandContext(callCtx, d.cliPath, "--print", "--output-format", "text")
	cmd.Dir = workdir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		slog.Warn("devloop dev: claude failed — falling back to scripted",
			"error", err, "stderr", truncateString(stderr.String(), 200))
		return d.fallback.Develop(ctx, proj, story, iteration, feedback)
	}

	out := stdout.String()
	if strings.TrimSpace(out) == "" {
		slog.Warn("devloop dev: claude returned empty output — falling back to scripted")
		return d.fallback.Develop(ctx, proj, story, iteration, feedback)
	}
	return DevOutput{
		Summary: firstLine(out),
		Details: strings.TrimSpace(out),
	}, nil
}

// ClaudeCodeReviewer asks Claude to evaluate the dev's output against
// each AC. Falls back to ScriptedReviewer on any CLI / parse failure.
type ClaudeCodeReviewer struct {
	cliPath  string
	fallback ReviewerAgent
	timeout  time.Duration
}

// NewClaudeCodeReviewer returns a Reviewer that prefers Claude Code.
func NewClaudeCodeReviewer() ReviewerAgent {
	fallback := NewScriptedReviewer()
	cli, err := exec.LookPath("claude")
	if err != nil {
		slog.Info("devloop reviewer: claude CLI not found — using scripted reviewer", "error", err)
		return fallback
	}
	return &ClaudeCodeReviewer{cliPath: cli, fallback: fallback, timeout: 3 * time.Minute}
}

// Name tags the agent in the reviews table.
func (*ClaudeCodeReviewer) Name() string { return "claude-reviewer" }

// Review asks the model for a per-AC pass/fail call plus a one-sentence
// reason. Response is expected as JSON so we can act on it without
// parsing prose.
func (r *ClaudeCodeReviewer) Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	prompt := buildReviewPrompt(proj, story, output)
	cmd := exec.CommandContext(callCtx, r.cliPath, "--print", "--output-format", "text")
	cmd.Dir = pickWorkdir(proj)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		slog.Warn("devloop reviewer: claude failed — falling back to scripted",
			"error", err, "stderr", truncateString(stderr.String(), 200))
		return r.fallback.Review(ctx, proj, story, output)
	}

	raw := extractJSON(stdout.String())
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
		slog.Warn("devloop reviewer: claude returned non-JSON — falling back",
			"error", err, "raw", truncateString(raw, 200))
		return r.fallback.Review(ctx, proj, story, output)
	}

	// Build a verdict. If Claude missed some ACs in its response, mark
	// those failed + overall fail so the dev gets a clear retry signal.
	seen := map[int64]bool{}
	var acs []ReviewedCriterion
	for _, rc := range parsed.ACs {
		seen[rc.ID] = true
		acs = append(acs, ReviewedCriterion{ID: rc.ID, Passed: rc.Passed, Reason: rc.Reason})
	}
	verdict := ReviewVerdict{Pass: parsed.Pass, Feedback: parsed.Feedback, ACs: acs}
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

// buildDevPrompt asks Claude to implement the story inside the workdir.
// We're explicit about the ACs so Claude addresses them one by one.
func buildDevPrompt(proj ProjectContext, story Story, iteration int, feedback string) string {
	var b strings.Builder
	b.WriteString("You are the Dev agent in a BMAD autonomous product build. ")
	b.WriteString("Work inside the current directory — that's the project's workdir. ")
	b.WriteString("Implement the story below. Make the code changes directly using your file-editing tools. ")
	b.WriteString("Run tests after changes when a test command is obvious from the repo layout.\n\n")
	fmt.Fprintf(&b, "Project idea: %s\n", proj.Idea)
	if proj.PRD != "" {
		fmt.Fprintf(&b, "\nPRD excerpt:\n%s\n", truncateString(proj.PRD, 1500))
	}
	fmt.Fprintf(&b, "\nStory: %s (iteration %d of %d)\n", story.Title, iteration, MaxIterations)
	if story.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", story.Description)
	}
	b.WriteString("\nAcceptance criteria:\n")
	for _, ac := range story.ACs {
		fmt.Fprintf(&b, "- %s\n", ac.Text)
	}
	if feedback != "" {
		fmt.Fprintf(&b, "\nPrevious review feedback:\n%s\n\nAddress each point above, then proceed.\n", feedback)
	}
	b.WriteString("\nWhen you're done, write a short summary of what you changed so the Reviewer can cross-check it against the ACs. ")
	b.WriteString("Do not include the raw diff — just the summary.\n")
	return b.String()
}

// buildReviewPrompt asks Claude for a structured per-AC verdict.
func buildReviewPrompt(proj ProjectContext, story Story, output DevOutput) string {
	var b strings.Builder
	b.WriteString("You are the Reviewer agent in a BMAD autonomous product build. ")
	b.WriteString("Evaluate the Dev's summary below against every acceptance criterion. ")
	b.WriteString("You may read files in the current directory to cross-check claims, but do NOT modify them.\n\n")
	b.WriteString("Respond with ONLY a single JSON object of this shape. No prose, no code fence.\n")
	b.WriteString(`{"pass": <bool>, "feedback": "<why it failed, empty on pass>", "acs": [{"id": <int>, "passed": <bool>, "reason": "<one sentence>"}, ...]}`)
	b.WriteString("\n\n")
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

// extractJSON pulls the first {…} block from the CLI output so leading
// prose the model might emit doesn't break Unmarshal.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
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
