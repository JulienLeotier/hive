# Story 9.2: Auto-Promotion Engine

Status: done

## Story

As a user,
I want agents automatically promoted when they meet configured thresholds,
so that trust evolves without manual intervention.

## Acceptance Criteria

1. **Given** an agent has trust thresholds configured (e.g., "Guided after 50 tasks, <10% error")
   **When** the agent meets the threshold criteria
   **Then** the system promotes the agent to the next trust level

2. **Given** an agent is promoted
   **When** the promotion occurs
   **Then** the system logs the promotion with criteria details in the `trust_history` table
   **And** the promotion includes: old level, new level, reason (`auto_promotion`), criteria string

3. **Given** an agent's trust level is higher than what stats would justify (e.g., manually promoted)
   **When** the auto-promotion engine evaluates
   **Then** it never auto-demotes — only promotes upward

4. **Given** an agent does not meet any promotion threshold
   **When** the engine evaluates
   **Then** no promotion occurs and the current level is returned unchanged

## Tasks / Subtasks

- [x] Task 1: Evaluate method — auto-promotion logic (AC: #1, #3, #4)
  - [x] Implement `Evaluate(ctx, agentID)` on Engine — returns (promoted bool, newLevel string, err)
  - [x] Call `GetStats()` to get current performance metrics
  - [x] Implement `calculateTargetLevel(stats)` — determines highest earned level based on thresholds
  - [x] Compare target level against current level using `levelRank()` ordinal helper
  - [x] Only promote (target > current), never demote (target <= current)
- [x] Task 2: Level persistence and history logging (AC: #2)
  - [x] Implement `setLevel(ctx, agentID, oldLevel, newLevel, reason, criteria)` — transactional update
  - [x] Update `agents.trust_level` in same transaction as `trust_history` insert
  - [x] Use ULID for trust_history entry ID
  - [x] Log promotion via slog with agent_id, from, to, tasks, error_rate
- [x] Task 3: Level ranking helper (AC: #3)
  - [x] Implement `levelRank(level)` — maps level string to ordinal: supervised=0, guided=1, autonomous=2, trusted=3
  - [x] Used by Evaluate to prevent demotion
- [x] Task 4: Unit tests (AC: #1, #3, #4)
  - [x] Test no promotion when insufficient tasks
  - [x] Test promotion to guided when thresholds met (50 tasks, <10% error)
  - [x] Test never auto-demotes (trusted agent with poor stats stays trusted)

## Dev Notes

### Architecture Compliance

- **Transactional** — level update and history insert happen in the same SQLite transaction (`BeginTx` / `Commit`) to prevent inconsistency
- **ULID** — trust history entries use `ulid.MustNew` with `crypto/rand.Reader`
- **slog** — structured info-level log on each promotion
- **Pure function** — `calculateTargetLevel` is a pure function of stats and thresholds, easily testable

### Key Design Decisions

- Auto-promotion is one-directional only — the engine will never automatically demote an agent. This prevents oscillation where an agent is promoted/demoted repeatedly around a threshold boundary
- `Evaluate()` is called by the orchestrator after each task completion — the caller decides when to trigger evaluation
- The engine checks all threshold levels in descending order (trusted first, then autonomous, guided, supervised) to find the highest earned level
- Criteria string stored in trust_history includes both task count and error rate for auditability

### Integration Points

- `internal/trust/engine.go` — `Evaluate()`, `setLevel()`, `calculateTargetLevel()`, `levelRank()`
- `internal/trust/engine_test.go` — tests for promotion, no-promotion, and no-demotion
- `internal/storage/migrations/` — trust_history table schema

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR64, FR66, FR69]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Evaluate method computes target level from stats and promotes if higher than current
- Transactional setLevel updates agents table and inserts trust_history in one transaction
- Never auto-demotes — levelRank comparison ensures only upward movement
- calculateTargetLevel checks thresholds in descending order for highest earned level
- 3 unit tests covering promotion, no-promotion, and no-demotion scenarios

### Change Log

- 2026-04-16: Story 9.2 implemented — auto-promotion engine with transactional persistence and demotion prevention

### File List

- internal/trust/engine.go (modified — added Evaluate, setLevel, calculateTargetLevel, levelRank)
- internal/trust/engine_test.go (modified — added promotion and demotion tests)
