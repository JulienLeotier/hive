# Story 9.1: Trust Level Tracking

Status: done

## Story

As a user,
I want each agent to have a tracked trust level that reflects its performance,
so that I can progressively grant more autonomy to reliable agents.

## Acceptance Criteria

1. **Given** an agent is registered
   **When** it completes tasks
   **Then** the system tracks: total tasks completed, success rate, error rate, consecutive successes

2. **Given** an agent has completed tasks
   **When** the trust engine queries stats
   **Then** it returns accurate counts for total tasks, successes, failures, and error rate

3. **Given** an agent with no completed tasks
   **When** stats are queried
   **Then** all counters return zero and error rate is 0.0

4. **Given** the agents table
   **When** an agent is registered
   **Then** it starts with a default trust level of `scripted` (lowest autonomy)

## Tasks / Subtasks

- [x] Task 1: Trust levels and thresholds definition (AC: #1, #4)
  - [x] Define trust level constants: `supervised`, `guided`, `autonomous`, `trusted`
  - [x] Create `Thresholds` struct with configurable promotion criteria per level
  - [x] Implement `DefaultThresholds()` returning sensible defaults:
    - Guided: 50 tasks, <10% error
    - Autonomous: 200 tasks, <5% error
    - Trusted: 500 tasks, <2% error
- [x] Task 2: Trust Engine with stats tracking (AC: #1, #2, #3)
  - [x] Create `internal/trust/engine.go` with `Engine` struct backed by `*sql.DB`
  - [x] Define `AgentStats` struct: TotalTasks, Successes, Failures, ErrorRate, CurrentLevel
  - [x] Implement `NewEngine(db, thresholds)` constructor
  - [x] Implement `GetStats(ctx, agentID)` — queries agents table for trust level, tasks table for completion counts
  - [x] ErrorRate calculated as `failures / total_tasks` (0 when no tasks)
- [x] Task 3: Trust history table (AC: #1)
  - [x] `trust_history` table schema: id (ULID), agent_id, old_level, new_level, reason, criteria, created_at
  - [x] Table created by v0.2 migration
- [x] Task 4: Unit tests (AC: #2, #3)
  - [x] Test GetStats with no tasks (all zeros)
  - [x] Test GetStats with mixed completed/failed tasks (correct counts and error rate)

## Dev Notes

### Architecture Compliance

- **Direct SQL** — queries `agents` and `tasks` tables directly, no ORM
- **ULID** — trust history entries use ULID for IDs, consistent with project convention
- **crypto/rand** — ULID entropy source uses `crypto/rand.Reader` for secure randomness
- **slog** — structured logging for trust promotions with agent_id, levels, task count, error rate
- **Package isolation** — `internal/trust` depends only on `database/sql` and `ulid` — no circular dependencies

### Key Design Decisions

- Trust levels form a linear hierarchy: supervised < guided < autonomous < trusted — simpler than a lattice model and sufficient for the current use cases
- Stats are computed from the tasks table on demand rather than cached — ensures accuracy at the cost of slightly higher query latency (acceptable for the low volume of trust evaluations)
- Default trust level is `scripted` in the database schema but the trust engine evaluates against `supervised` as the starting evaluation point
- `Thresholds` struct is YAML-serializable for future configuration via `hive.yaml`

### Integration Points

- `internal/trust/engine.go` — trust level tracking and stats computation
- `internal/trust/engine_test.go` — unit tests for stats tracking
- `internal/storage/migrations/001_initial.sql` — agents table with `trust_level` column
- `internal/agent/agent.go` — agent struct includes TrustLevel field

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR63]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Four trust levels defined: supervised, guided, autonomous, trusted
- Configurable thresholds with sensible defaults (50/200/500 tasks at 10%/5%/2% error)
- GetStats queries tasks table for live performance metrics
- AgentStats struct provides total tasks, successes, failures, error rate, current level
- Trust history table tracks all level changes with reason and criteria

### Change Log

- 2026-04-16: Story 9.1 implemented — trust level tracking with stats computation and history table

### File List

- internal/trust/engine.go (new)
- internal/trust/engine_test.go (new)
