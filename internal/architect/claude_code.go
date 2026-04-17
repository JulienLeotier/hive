package architect

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

// ClaudeCodeAgent asks the local `claude` CLI to decompose a PRD. Falls
// back to the scripted agent on any failure so the autonomous flow never
// blocks on the model being unreachable. The fallback is also what ships
// in CI, where we don't want a network call per test.
type ClaudeCodeAgent struct {
	cliPath  string
	fallback Agent
	timeout  time.Duration
}

// NewClaudeCodeAgent builds an architect that prefers Claude but degrades
// gracefully to ScriptedAgent when the CLI is missing.
func NewClaudeCodeAgent() Agent {
	fallback := NewScripted()
	cli, err := exec.LookPath("claude")
	if err != nil {
		slog.Info("architect: claude CLI not found — using scripted agent", "error", err)
		return fallback
	}
	return &ClaudeCodeAgent{cliPath: cli, fallback: fallback, timeout: 60 * time.Second}
}

// Name identifies this agent in log/event tags.
func (*ClaudeCodeAgent) Name() string { return "claude-architect" }

// Decompose prompts Claude for a JSON-structured breakdown matching
// []EpicDraft. Validates shape + falls back to scripted on any issue.
func (a *ClaudeCodeAgent) Decompose(ctx context.Context, projectIdea, prd string) ([]EpicDraft, error) {
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	prompt := buildDecomposePrompt(projectIdea, prd)
	cmd := exec.CommandContext(callCtx, a.cliPath, "--print", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		slog.Warn("architect claude failed — falling back to scripted",
			"error", err, "stderr", truncate(stderr.String(), 200))
		return a.fallback.Decompose(ctx, projectIdea, prd)
	}

	raw := extractJSONPayload(stdout.String())
	var response struct {
		Epics []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Stories     []struct {
				Title              string   `json:"title"`
				Description        string   `json:"description"`
				AcceptanceCriteria []string `json:"acceptance_criteria"`
			} `json:"stories"`
		} `json:"epics"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		slog.Warn("architect claude returned non-JSON — falling back to scripted",
			"error", err, "raw", truncate(raw, 200))
		return a.fallback.Decompose(ctx, projectIdea, prd)
	}
	if len(response.Epics) == 0 {
		slog.Warn("architect claude returned zero epics — falling back to scripted")
		return a.fallback.Decompose(ctx, projectIdea, prd)
	}

	var out []EpicDraft
	for _, e := range response.Epics {
		if e.Title == "" {
			continue
		}
		draft := EpicDraft{Title: e.Title, Description: e.Description}
		for _, s := range e.Stories {
			if s.Title == "" {
				continue
			}
			draft.Stories = append(draft.Stories, StoryDraft{
				Title:              s.Title,
				Description:        s.Description,
				AcceptanceCriteria: s.AcceptanceCriteria,
			})
		}
		out = append(out, draft)
	}
	if len(out) == 0 {
		return a.fallback.Decompose(ctx, projectIdea, prd)
	}
	return out, nil
}

// extractJSONPayload strips any prose the CLI emits around the JSON so
// the unmarshaller gets a clean object. Looks for the first `{` and the
// matching last `}`.
func extractJSONPayload(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
}

// buildDecomposePrompt instructs the model to produce the JSON shape
// Decompose unmarshalls. Structured like this because a free-form
// decomposition would require a second pass to shape.
func buildDecomposePrompt(projectIdea, prd string) string {
	var b strings.Builder
	b.WriteString("You are the Architect agent in a BMAD autonomous product build.\n")
	b.WriteString("Break the PRD below into epics (3–8), each with 1–5 stories, each story with 2–5 concrete acceptance criteria.\n")
	b.WriteString("Order epics by build sequence: foundations first, then each user flow, then hardening, then ship prep.\n")
	b.WriteString("Acceptance criteria must be verifiable — no 'works well' fluff.\n\n")
	b.WriteString("Respond with ONLY a single JSON object of this exact shape. No prose, no code fence.\n")
	b.WriteString(`{"epics":[{"title":"...","description":"...","stories":[{"title":"...","description":"...","acceptance_criteria":["...","..."]}]}]}`)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Project idea: %s\n\nPRD:\n%s\n", projectIdea, prd)
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
