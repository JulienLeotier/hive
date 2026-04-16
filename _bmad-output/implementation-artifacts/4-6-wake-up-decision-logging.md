# Story 4.6: Wake-Up Decision Logging

Status: done

## Story

As a user,
I want every agent wake-up decision logged with full reasoning,
so that I can audit and debug autonomous agent behavior.

## Acceptance Criteria

1. **Given** an agent completes a wake-up cycle **When** the cycle finishes (action taken or idle) **Then** the system logs: agent ID, timestamp, what was observed (backlog count, events count), what was decided (action/idle), why (plan state + matching rule), duration
2. **Given** wake-up decision logs **When** the user queries them **Then** logs are queryable via `hive logs --agent <name>` with type and time filters
3. **Given** structured logging **When** decisions are logged **Then** they use slog structured fields for machine-parseable output (FR48)

## Tasks / Subtasks

- [x] Task 1: Add slog structured logging to WakeUpHandler for decision context (AC: #1, #3)
- [x] Task 2: Log agent name, cycle result (action/idle), and errors (AC: #1)
- [x] Task 3: Emit events for wake-up decisions via event bus (AC: #1, #2)
- [x] Task 4: Wire `hive logs` command to query agent-specific events (AC: #2)
- [x] Task 5: Support --agent and --type filters in logs command (AC: #2)

## Dev Notes

- Wake-up decision logging uses `log/slog` with structured fields per architecture spec
- The scheduler logs at INFO level for heartbeat start/stop, and ERROR level for wake-up failures
- Decision events are persisted via the event bus, making them queryable via `hive logs`
- The `hive logs` CLI command supports `--agent` (maps to source filter), `--type` (event type prefix), `--since` (time range), `--limit` (result count)
- JSON output is supported via `--json` flag for machine parsing and CI integration
- Events include: agent name (source), observation summary, decision (action type or idle), and any errors
- The combination of slog (real-time logging) and event bus (persistent queryable log) provides both operational and audit capabilities

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/autonomy/scheduler.go (modified) -- slog.Info/Error for heartbeat lifecycle and wake-up decisions
- internal/cli/logs.go (new) -- hive logs command with --type, --agent, --since, --limit, --json filters
- internal/event/bus.go (dependency) -- Query method used by logs command
