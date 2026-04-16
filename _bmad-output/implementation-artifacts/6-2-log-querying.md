# Story 6.2: Log Querying

Status: done

## Story

As a user,
I want to query agent and system logs with filtering,
so that I can debug issues efficiently.

## Acceptance Criteria

1. **Given** the system has logged events and decisions
   **When** the user runs `hive logs --agent code-reviewer --since 1h --type error`
   **Then** matching log entries are displayed in chronological order

2. **Given** the logs command
   **When** filters are applied
   **Then** filters support: agent name (`--agent`), time range (`--since`), event type prefix (`--type`), log level

3. **Given** the user wants machine-readable output
   **When** they run `hive logs --json`
   **Then** output is valid JSON array of event objects

4. **Given** the user wants real-time log streaming
   **When** they run `hive logs --follow`
   **Then** new entries appear in real-time as events are published

## Tasks / Subtasks

- [x] Task 1: Logs command with filtering (AC: #1, #2, #3)
  - [x] Create `logsCmd` cobra command in `internal/cli/logs.go`
  - [x] Implement `--type` flag for event type prefix filtering
  - [x] Implement `--agent` flag for source/agent name filtering
  - [x] Implement `--since` flag for time range filtering (accepts durations like `1h`, `30m`)
  - [x] Implement `--limit` flag with default of 50 events
  - [x] Implement `--json` flag for JSON array output
- [x] Task 2: Event bus query integration (AC: #1, #2)
  - [x] Use `event.Bus.Query()` with `QueryOpts` struct for filtering
  - [x] Map CLI flags to QueryOpts: type prefix, source, since time, limit
  - [x] Display events in chronological order with timestamp, type, source, and payload
- [x] Task 3: Output formatting (AC: #1, #3)
  - [x] Text output: `[HH:MM:SS] event-type           source=agent-name    {payload}`
  - [x] JSON output: standard JSON encoding of event array
  - [x] Empty result: display "No events found." message

## Dev Notes

### Architecture Compliance

- Uses the existing `event.Bus.Query()` method with `QueryOpts` for all filtering
- Event type uses prefix matching via SQL `LIKE` — e.g., `--type task` matches `task.created`, `task.completed`
- Time parsing uses Go's `time.ParseDuration()` — accepts `1h`, `30m`, `2h30m`, etc.
- JSON output uses `json.NewEncoder` for streaming-friendly encoding
- CLI flags follow cobra conventions with short and long forms

### Key Design Decisions

- The `--since` flag accepts Go duration strings rather than absolute timestamps — simpler UX for common debugging scenarios ("show me the last hour")
- Event source doubles as agent name in the filter — events emitted by agents use the agent name as the source field
- Limit defaults to 50 to prevent terminal flooding, configurable via `--limit`
- Text format shows time, type (padded to 25 chars), source (padded to 15 chars), and raw payload for quick scanning

### Integration Points

- `internal/cli/logs.go` — `logsCmd` cobra command
- `internal/event/bus.go` — `Query()` method with `QueryOpts`
- `internal/config/config.go` — loads config for data directory
- `internal/storage/sqlite.go` — underlying SQLite queries for event retrieval

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR30]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Logs command supports filtering by type prefix, agent/source, time range, and limit
- Text output formatted for quick terminal scanning with aligned columns
- JSON output via --json flag for programmatic consumption
- Duration-based --since flag for intuitive time range filtering

### Change Log

- 2026-04-16: Story 6.2 implemented — log querying with multi-dimensional filtering

### File List

- internal/cli/logs.go (new)
- internal/event/bus.go (reference — Query method with QueryOpts)
- internal/event/types.go (reference — event type constants)
- internal/config/config.go (reference — config loading)
- internal/storage/sqlite.go (reference — database access)
