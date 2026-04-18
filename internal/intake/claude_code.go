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
	// timeout 0 → hérite du parent ctx. Même principe que les autres
	// agents claude-code : pas de cap arbitraire sur l'invocation.
	return &ClaudeCodeAgent{cliPath: cli, fallback: fallback, timeout: 0}
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
	callCtx := ctx
	cancel := func() {}
	if a.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, a.timeout)
	}
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
	b.WriteString("until you have enough to write a SCOPE-LOCKED product brief. Cover: audience, core flows, non-goals, ")
	b.WriteString("tech constraints, and definition-of-done. Do NOT ask more than one question per turn. ")
	b.WriteString("Do NOT ask a question you already asked.\n\n")
	b.WriteString("MATCH THE USER'S AMBITION. If they say 'simple', 'basic', 'minimal', 'just X', do NOT propose ")
	b.WriteString("additional features, integrations, or roles. Ask about the 1-2 core flows and the stack, then be done.\n")
	b.WriteString("If the user tells you to 'just figure it out' or 'use defaults', accept it — state the minimal ")
	b.WriteString("defaults you picked (stack, scope, persistence) in one message, then set done=true.\n\n")
	b.WriteString("When you have enough to write the brief, set done=true and reply with a one-sentence handoff.\n\n")
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

// buildPRDPrompt asks Claude for a scope-locked Product Brief synthesised
// from the intake conversation. The document is consumed by BMAD's
// `/bmad-create-prd` skill downstream; keeping it strict and lean
// prevents the Analyst agent from re-inventing an ambitious product.
//
// Contract for downstream skills (enforced by a SCOPE LOCK header) :
//   - Ship EXACTLY what's in In-scope
//   - Anything in Non-goals must NOT be added, not even as "future"
//   - Stack is a constraint, not a suggestion
//   - If the brief says "simple/basic/minimal", do NOT expand
func buildPRDPrompt(projectIdea string, history []Message) string {
	var b strings.Builder
	b.WriteString("You are the PM agent. The intake Q&A below has concluded. ")
	b.WriteString("Write a SCOPE-LOCKED Product Brief that downstream BMAD skills ")
	b.WriteString("(create-prd, create-architecture, create-epics) must strictly respect.\n\n")
	b.WriteString("CRITICAL RULES :\n")
	b.WriteString("- Match the user's AMBITION LEVEL. If they said 'simple', 'basic', 'minimal' or 'just X', ")
	b.WriteString("the brief must be SMALL (1-2 epics, < 10 stories total). Do NOT brainstorm features they didn't ask for.\n")
	b.WriteString("- Every feature in In-scope must be traceable to something the user explicitly asked for or confirmed.\n")
	b.WriteString("- Non-goals must be EXPLICIT and LONG — list everything a typical product in this space has that we are NOT building.\n")
	b.WriteString("- Stack must be the leanest viable. No framework if vanilla works. No DB if localStorage works. No backend if static works.\n\n")
	b.WriteString("OUTPUT FORMAT — use EXACTLY these sections :\n")
	b.WriteString("```\n")
	b.WriteString("# Product Brief — <name>\n\n")
	b.WriteString("## SCOPE LOCK\n")
	b.WriteString("This brief is a HARD contract. Downstream agents (architect, PM, UX, dev) must NOT add features, screens, ")
	b.WriteString("integrations, or workflows beyond what is listed in In-scope. Expanding the scope is a review failure.\n\n")
	b.WriteString("## Summary\n")
	b.WriteString("<2-3 sentences, concrete, no marketing copy>\n\n")
	b.WriteString("## Users\n")
	b.WriteString("<1 short paragraph — who, what context>\n\n")
	b.WriteString("## In-scope (ship these — nothing else)\n")
	b.WriteString("- <feature 1, concrete>\n- <feature 2, concrete>\n... \n\n")
	b.WriteString("## Non-goals (DO NOT build these, not even as stubs)\n")
	b.WriteString("- <feature NOT shipped 1>\n- <feature NOT shipped 2>\n... (be exhaustive — list anything downstream agents might be tempted to add)\n\n")
	b.WriteString("## Stack (constraint — not a suggestion)\n")
	b.WriteString("<language / framework / DB / deploy — lean defaults>\n\n")
	b.WriteString("## Definition of Done\n")
	b.WriteString("- <observable behaviour 1>\n- <observable behaviour 2>\n...\n")
	b.WriteString("```\n\n")
	b.WriteString("Wrap the final document between the literal markers `<<<PRD` and `PRD>>>` so the outer ")
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
