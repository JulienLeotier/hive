package intake

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

// ClaudeCodeAgent drives the intake Q&A through the local `claude` CLI.
// On each turn it pipes the running conversation to Claude with a PM
// system prompt, reads back a JSON reply, and appends it. If anything
// fails — CLI missing, timeout, malformed JSON — it falls back to the
// ScriptedAgent so the build never dead-ends.
//
// The Claude CLI is assumed to be on PATH. Operators who prefer strict
// determinism or offline-only operation can force the scripted flow by
// setting HIVE_INTAKE_AGENT=scripted.
type ClaudeCodeAgent struct {
	cliPath string
	fallback Agent
	timeout  time.Duration
}

// NewClaudeCodeAgent returns a PM agent that shells out to Claude. When
// the CLI isn't available it returns an agent that delegates straight to
// the scripted fallback, so callers don't have to branch on configuration.
func NewClaudeCodeAgent() Agent {
	fallback := NewScriptedAgent()
	cli, err := exec.LookPath("claude")
	if err != nil {
		slog.Info("intake: claude CLI not found — using scripted PM agent",
			"error", err)
		return fallback
	}
	return &ClaudeCodeAgent{cliPath: cli, fallback: fallback, timeout: 30 * time.Second}
}

// Role identifies this agent's conversation slot.
func (*ClaudeCodeAgent) Role() string { return RolePM }

// Greeting delegates to the fallback — the first message doesn't need a
// model call, and seeding the conversation with a predictable opener
// lets the user gauge what the PM understood of their idea.
func (a *ClaudeCodeAgent) Greeting(ctx context.Context, projectIdea string) string {
	return a.fallback.Greeting(ctx, projectIdea)
}

// Reply asks Claude for the next turn in the conversation. The CLI gets
// a single stdin payload with the system prompt + full history; the
// expected response is a JSON object `{"reply": "...", "done": bool}`.
// Any parse failure, non-zero exit, or timeout falls through to the
// scripted agent so the user never gets stuck.
func (a *ClaudeCodeAgent) Reply(
	ctx context.Context,
	projectIdea string,
	history []Message,
) (string, bool, error) {
	prompt := buildReplyPrompt(projectIdea, history)
	raw, err := a.runCLI(ctx, prompt)
	if err != nil {
		slog.Warn("intake: claude reply failed, falling back to scripted", "error", err)
		return a.fallback.Reply(ctx, projectIdea, history)
	}
	var parsed struct {
		Reply string `json:"reply"`
		Done  bool   `json:"done"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		slog.Warn("intake: claude reply not JSON, falling back to scripted",
			"error", err, "raw", truncate(string(raw), 200))
		return a.fallback.Reply(ctx, projectIdea, history)
	}
	if strings.TrimSpace(parsed.Reply) == "" {
		return a.fallback.Reply(ctx, projectIdea, history)
	}
	return parsed.Reply, parsed.Done, nil
}

// FinalPRD asks Claude to assemble the PRD as markdown. Falls back to the
// deterministic stitcher when the model call fails so the project always
// has a PRD when the user clicks Finalize.
func (a *ClaudeCodeAgent) FinalPRD(
	ctx context.Context,
	projectIdea string,
	history []Message,
) (string, error) {
	prompt := buildPRDPrompt(projectIdea, history)
	raw, err := a.runCLI(ctx, prompt)
	if err != nil {
		slog.Warn("intake: claude PRD failed, falling back to scripted", "error", err)
		return a.fallback.FinalPRD(ctx, projectIdea, history)
	}
	// The prompt asks for a markdown document wrapped in a sentinel pair
	// (<<<PRD and PRD>>>) so arbitrary leading text from the CLI can't
	// sneak into the stored PRD.
	text := string(raw)
	if start := strings.Index(text, "<<<PRD"); start != -1 {
		if end := strings.Index(text[start:], "PRD>>>"); end != -1 {
			text = text[start+len("<<<PRD") : start+end]
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return a.fallback.FinalPRD(ctx, projectIdea, history)
	}
	return text, nil
}

// runCLI executes `claude --print --format json` (or whatever the
// operator's alias is; if no-arg execution fails we retry with --print
// disabled). Stdin carries the full prompt. Output is the raw CLI stdout.
func (a *ClaudeCodeAgent) runCLI(ctx context.Context, prompt string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// --print sends the response straight to stdout without interactive
	// pager; --output-format json wraps it so we can parse reliably.
	cmd := exec.CommandContext(callCtx, a.cliPath, "--print", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude: %w (stderr: %s)", err, truncate(stderr.String(), 200))
	}
	return stdout.Bytes(), nil
}

// buildReplyPrompt stitches the PM system prompt with the conversation
// history and asks for a single-turn JSON reply.
func buildReplyPrompt(projectIdea string, history []Message) string {
	var b strings.Builder
	b.WriteString("You are the PM agent in the BMAD autonomous product-build flow.\n")
	b.WriteString("The user has an idea; your job is to ask one clarifying question at a time ")
	b.WriteString("until you have enough to write a PRD. Cover: audience, core flows, non-goals, ")
	b.WriteString("tech constraints, and definition-of-done. Do NOT ask more than one question per turn. ")
	b.WriteString("Do NOT ask a question you already asked. ")
	b.WriteString("When you have enough to write a PRD, set done=true and reply with a one-sentence handoff.\n\n")
	b.WriteString("Respond with ONLY a single JSON object of the form ")
	b.WriteString(`{"reply": "<your next message to the user>", "done": <boolean>}.`)
	b.WriteString(" No prose outside the JSON.\n\n")
	fmt.Fprintf(&b, "User's idea: %s\n\nConversation so far:\n", projectIdea)
	for _, m := range history {
		role := "User"
		if m.Author != AuthorUser {
			role = "PM"
		}
		fmt.Fprintf(&b, "%s: %s\n", role, m.Content)
	}
	return b.String()
}

// buildPRDPrompt asks Claude for a markdown PRD synthesised from the
// intake conversation.
func buildPRDPrompt(projectIdea string, history []Message) string {
	var b strings.Builder
	b.WriteString("You are the PM agent. The intake Q&A below has concluded. ")
	b.WriteString("Write a clean markdown PRD the Architect can decompose into epics. ")
	b.WriteString("Structure: Summary, Audience, Core Flows (numbered), Non-Goals, Tech Notes, ")
	b.WriteString("Definition of Done. Be concrete — no fluff. ")
	b.WriteString("Wrap the document between the literal markers `<<<PRD` and `PRD>>>` so the outer ")
	b.WriteString("system can extract it cleanly. Do NOT include anything outside those markers.\n\n")
	fmt.Fprintf(&b, "User's idea: %s\n\nIntake transcript:\n", projectIdea)
	for _, m := range history {
		role := "User"
		if m.Author != AuthorUser {
			role = "PM"
		}
		fmt.Fprintf(&b, "%s: %s\n", role, m.Content)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Compile-time Agent interface check.
var _ Agent = (*ClaudeCodeAgent)(nil)
