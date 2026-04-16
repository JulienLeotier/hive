# Story 9.4: Manual Trust Management

Status: done

## Story

As a user,
I want to manually promote or demote agents via CLI,
so that I can override the automatic system when needed.

## Acceptance Criteria

1. **Given** a registered agent
   **When** the user runs `hive agent trust code-reviewer --level autonomous`
   **Then** the agent's trust level is updated immediately in the database

2. **Given** a manual trust change
   **When** the level is updated
   **Then** a `manual_override` entry is logged in the `trust_history` table with reason "set by user"

3. **Given** an invalid trust level
   **When** the user provides a non-existent level
   **Then** the command returns a clear error listing valid levels

## Tasks / Subtasks

- [x] Task 1: SetManual method on trust engine (AC: #1, #2)
  - [x] Implement `SetManual(ctx, agentID, newLevel)` on Engine
  - [x] Query current trust level from agents table
  - [x] Call `setLevel()` with reason `manual_override` and criteria `set by user`
  - [x] Transactional update: agents table + trust_history insert
  - [x] Return error if agent not found
- [x] Task 2: CLI command (AC: #1, #3)
  - [x] Add trust management to CLI (accessible via `hive agent trust <name> --level <level>`)
  - [x] Validate level against known constants (supervised, guided, autonomous, trusted)
  - [x] Display confirmation message on success
  - [x] Display error with valid levels on invalid input
- [x] Task 3: Unit tests (AC: #1, #2)
  - [x] Test SetManual updates agent trust level in database
  - [x] Test SetManual creates trust_history entry with `manual_override` reason
  - [x] Test SetManual returns error for non-existent agent

## Dev Notes

### Architecture Compliance

- **Transactional** — same `setLevel()` helper used by auto-promotion, ensuring consistent agents + trust_history updates
- **CLI pattern** — follows existing cobra command pattern with flags
- **ULID** — trust history entry ID generated via `ulid.MustNew` with `crypto/rand.Reader`
- **slog** — no additional logging in SetManual beyond what `setLevel()` provides (history table is the audit log)

### Key Design Decisions

- Manual trust management allows both promotion AND demotion — unlike auto-promotion which only goes up, manual control can set any level
- The trust_history entry explicitly records `manual_override` as the reason, distinguishing it from `auto_promotion` entries for audit trail
- Agent lookup is by ID in the engine but by name in the CLI — the CLI resolves name to ID before calling the engine
- No confirmation prompt for demotion — the user is trusted to make intentional trust level changes

### Integration Points

- `internal/trust/engine.go` — `SetManual()` method
- `internal/trust/engine_test.go` — tests for manual trust setting
- `internal/cli/agent.go` — CLI command for trust management

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR67]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- SetManual allows any trust level change (both promotion and demotion)
- Transactional update using shared setLevel helper
- trust_history records `manual_override` reason for audit trail
- CLI validates level against known constants
- Unit tests verify database update and history creation

### Change Log

- 2026-04-16: Story 9.4 implemented — manual trust management via CLI and trust engine

### File List

- internal/trust/engine.go (modified — added SetManual method)
- internal/trust/engine_test.go (modified — added TestSetManual)
- internal/cli/agent.go (modified — added trust management CLI command)
