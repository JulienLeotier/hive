# Story 6.5: Orchestration Decision Logging

Status: done

## Story

As a user,
I want every orchestration decision logged with reasoning,
so that I can understand why the system made specific choices.

## Acceptance Criteria

1. **Given** the system makes an orchestration decision (routing, failover, isolation, etc.)
   **When** the decision is made
   **Then** a structured log entry includes: decision type, input context, options considered, choice made, reasoning

2. **Given** decision logs exist
   **When** the user runs `hive logs --decisions`
   **Then** only decision-type events are displayed

3. **Given** structured logging is enabled
   **When** decision events are emitted
   **Then** log format uses slog structured fields for machine parseability

## Tasks / Subtasks

- [x] Task 1: Decision logging framework (AC: #1, #3)
  - [x] Define structured decision log format using `slog` fields: decision_type, context, options, choice, reason
  - [x] Log routing decisions: which agents were considered, which was selected, why
  - [x] Log failover decisions: original agent, replacement agent, failure reason
  - [x] Log isolation decisions: agent name, trigger (health/circuit), threshold
- [x] Task 2: Decision event types (AC: #1)
  - [x] Define decision event type prefix `decision.*` for event bus
  - [x] Emit `decision.routing`, `decision.failover`, `decision.isolation` events
  - [x] Include full reasoning context in event payload
- [x] Task 3: CLI filter for decisions (AC: #2)
  - [x] Add `--decisions` flag to `hive logs` command
  - [x] When set, automatically filter events by type prefix `decision.`
  - [x] Display decision-specific formatting: type, choice, reasoning summary
- [x] Task 4: Integration with orchestration components (AC: #1, #3)
  - [x] Add decision logging to task router (`FindCapableAgent`)
  - [x] Add decision logging to failover logic
  - [x] Add decision logging to agent isolation logic
  - [x] All logs use `slog.Info` with structured key-value fields

## Dev Notes

### Architecture Compliance

- Uses `log/slog` structured logging with typed fields — all decision logs are machine-parseable
- Decision events are persisted to the event bus for queryability via `hive logs --decisions`
- Decision type prefix `decision.*` enables clean filtering without mixing with operational events
- Structured fields follow slog conventions: `slog.String("key", "value")`, `slog.Any("options", list)`

### Key Design Decisions

- Decisions are both logged via slog (for structured log files) and published to the event bus (for CLI querying) — dual output ensures both real-time and historical access
- The `--decisions` flag is a convenience shortcut for `--type decision` on the logs command
- Routing decisions log all considered agents (not just the winner) to help debug capability matching issues
- Decision payloads include timestamps to track decision latency

### Integration Points

- `internal/task/router.go` — routing decision logging in `FindCapableAgent()`
- `internal/task/task.go` — failover decision logging
- `internal/agent/manager.go` — isolation decision logging in `UpdateHealth()`
- `internal/cli/logs.go` — `--decisions` flag mapping to type filter
- `internal/event/bus.go` — decision events stored via `Publish()`

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR33]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Decision logging integrated into routing, failover, and isolation decision points
- Dual output: slog structured logs for file/stdout and event bus for CLI querying
- CLI shortcut --decisions flag filters to decision.* event types
- Each decision log includes full context: options considered, choice made, reasoning

### Change Log

- 2026-04-16: Story 6.5 implemented — structured orchestration decision logging with CLI filter

### File List

- internal/task/router.go (modified — routing decision logging)
- internal/task/task.go (modified — failover decision logging)
- internal/agent/manager.go (modified — isolation decision logging)
- internal/cli/logs.go (modified — added --decisions flag)
- internal/event/bus.go (reference — Publish for decision events)
