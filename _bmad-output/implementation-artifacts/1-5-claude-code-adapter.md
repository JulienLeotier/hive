# Story 1.5: Claude Code Adapter

Status: done

## Story

As a user,
I want to register Claude Code agents (skills/workflows) with Hive,
so that I can orchestrate my existing Claude Code agents alongside other frameworks.

## Acceptance Criteria

1. **Given** a Claude Code skill at a local path
   **When** the user creates a `ClaudeCodeAdapter` with the skill path
   **Then** the adapter wraps the Claude Code invocation in the Hive adapter protocol

2. **Given** the `ClaudeCodeAdapter`
   **When** `Declare()` is called
   **Then** it returns `AgentCapabilities` with the agent name and `"claude-code-skill"` task type

3. **Given** the `ClaudeCodeAdapter`
   **When** `Invoke()` is called with a task
   **Then** it executes the `claude` CLI with `--skill` flag and the skill path
   **And** passes task input as JSON via stdin (prevents command injection)
   **And** returns the command output as the task result

4. **Given** the `ClaudeCodeAdapter`
   **When** `Health()` is called
   **Then** it checks if the `claude` CLI is available in PATH via `exec.LookPath`
   **And** returns `"healthy"` if found, `"unavailable"` with message if not

5. **Given** the `ClaudeCodeAdapter`
   **When** `Checkpoint()` is called
   **Then** it returns an empty checkpoint (Claude Code skills are stateless per invocation)

6. **Given** the `ClaudeCodeAdapter`
   **When** compile-time interface compliance is checked
   **Then** `var _ Adapter = (*ClaudeCodeAdapter)(nil)` compiles successfully

## Tasks / Subtasks

- [x] Task 1: Claude Code adapter implementation (AC: #1, #2, #3, #4, #5)
  - [x] Create `internal/adapter/claude_code.go`
  - [x] `ClaudeCodeAdapter` struct with `SkillPath` and `Name` fields
  - [x] `NewClaudeCodeAdapter(skillPath, name)` constructor
  - [x] `Declare()` — returns capabilities with agent name and `claude-code-skill` task type
  - [x] `Invoke()` — executes `claude --skill <path>` via `exec.CommandContext`
  - [x] Task input serialized to JSON and passed via `cmd.Stdin` (safe from injection)
  - [x] Returns `TaskResult` with output on success, error message on failure
  - [x] `Health()` — `exec.LookPath("claude")` to check CLI availability
  - [x] `Checkpoint()` — returns empty checkpoint (stateless)
  - [x] `Resume()` — no-op (stateless)
  - [x] Compile-time check: `var _ Adapter = (*ClaudeCodeAdapter)(nil)`
- [x] Task 2: Claude Code adapter tests (AC: #2, #4, #5, #6)
  - [x] Create `internal/adapter/claude_code_test.go`
  - [x] `TestClaudeCodeAdapterDeclare` — verify name and task types
  - [x] `TestClaudeCodeAdapterHealth` — verify healthy or unavailable (env-dependent)
  - [x] `TestClaudeCodeAdapterCheckpoint` — verify empty checkpoint

## Dev Notes

### Architecture Compliance

- **Package:** `internal/adapter/` — co-located with other adapter implementations
- **Stdio transport:** Uses `os/exec` for CLI invocation per architecture decision (stdio transport for CLI-based agents)
- **Security:** Task input passed via stdin, not command arguments — prevents shell injection attacks
- **Stateless:** Claude Code skills are invoked per-task; checkpoint/resume are no-ops
- **Error handling:** Invoke captures both stdout and stderr via `CombinedOutput()`, returns structured `TaskResult` with error details on failure
- **Health check:** Uses `exec.LookPath` for zero-cost CLI detection without actually invoking Claude
- **Interface:** Implements full `Adapter` interface — compile-time verified
- **Naming:** `ClaudeCodeAdapter` (PascalCase), `claude_code.go` (snake_case file)

### Testing Strategy

- Declare and Checkpoint tests are deterministic (no external dependency)
- Health test accepts both "healthy" and "unavailable" since Claude CLI may not be in test environment
- Invoke not directly tested (requires Claude CLI) — integration tested via end-to-end flows

### References

- [Source: architecture.md#Project Structure — internal/adapter/claude_code.go]
- [Source: architecture.md#API & Communication Patterns — stdio transport]
- [Source: epics.md#Story 1.5]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Claude Code adapter wrapping the `claude` CLI as a Hive agent
- Stdin-based input passing for security (no command injection via args)
- Health check via `exec.LookPath` for lightweight CLI detection
- Stateless design — checkpoint/resume are no-ops
- 3 tests covering declare, health, and checkpoint
- Compile-time interface compliance verified

### Change Log

- 2026-04-16: Story 1.5 implemented — Claude Code adapter for CLI-based agent invocation

### File List

- internal/adapter/claude_code.go (new)
- internal/adapter/claude_code_test.go (new)
