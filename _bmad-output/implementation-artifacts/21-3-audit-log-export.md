# Story 21.3: Audit Log Export

Status: done

## Story

As a compliance officer,
I want audit logs exported in standard formats,
so that I can meet regulatory requirements.

## Acceptance Criteria

1. **Given** the system has been running with events
   **When** the user runs `hive audit export --format json --since 30d --output audit.json`
   **Then** all system events, auth events, and agent actions are exported

2. **Given** the audit export command
   **When** `--format csv` is specified
   **Then** the export is in CSV format with headers

3. **Given** the audit export
   **When** the export runs
   **Then** it includes: event type, timestamp, actor (user/agent), action, target, result, IP address

4. **Given** sensitive data in events
   **When** the audit export runs
   **Then** secrets and tokens are redacted from the exported data

## Tasks / Subtasks

- [x] Task 1: Audit log aggregation (AC: #1, #3)
  - [x] Implement `AuditLogger` in `internal/audit/logger.go`
  - [x] Aggregate events from: event bus (system events), auth middleware (auth events), agent manager (agent actions)
  - [x] Normalize all events into audit record format: type, timestamp, actor, action, target, result, source_ip
  - [x] Store audit records with additional metadata for compliance
- [x] Task 2: JSON export (AC: #1, #3)
  - [x] Implement `ExportJSON(writer, timeRange, filters)` in audit logger
  - [x] Export as JSON array with one object per audit record
  - [x] Support time range filtering via `--since` flag (e.g., 30d, 7d, 24h)
  - [x] Support event type filtering via `--type` flag
- [x] Task 3: CSV export (AC: #2, #3)
  - [x] Implement `ExportCSV(writer, timeRange, filters)` in audit logger
  - [x] Include header row with column names
  - [x] Proper CSV escaping for fields containing commas or quotes
  - [x] Same filtering support as JSON export
- [x] Task 4: Sensitive data redaction (AC: #4)
  - [x] Implement `RedactSensitiveFields(record)` -- scrubs secrets from audit records
  - [x] Redact: API keys, tokens, passwords, certificate contents
  - [x] Replace sensitive values with `[REDACTED]`
  - [x] Redaction runs before export, not on storage -- full data preserved internally
- [x] Task 5: CLI command (AC: #1, #2)
  - [x] Implement `hive audit export` command
  - [x] Flags: `--format` (json|csv), `--since` (duration), `--output` (file path, default stdout), `--type` (event type filter)
  - [x] Progress indicator for large exports
  - [x] Error handling: invalid format, invalid time range, file write errors
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test JSON export format and content
  - [x] Test CSV export with headers and proper escaping
  - [x] Test time range filtering
  - [x] Test sensitive data redaction
  - [x] Test export to file and stdout

## Dev Notes

### Architecture Compliance

- `internal/audit/logger.go` is the central audit logging facility
- Audit records are derived from existing events -- no duplicate data collection
- Redaction happens at export time, preserving full data internally for system operation
- Export supports both file output and stdout for flexibility
- Uses `slog` for structured logging of export operations

### Key Design Decisions

- Audit export is read-only and non-destructive -- never modifies stored events
- Redaction is a separate pass, applied before serialization -- ensures no secrets leak in any format
- JSON and CSV are the two formats supported for v1.0 -- covers both programmatic and spreadsheet analysis
- Time range uses human-friendly duration format (30d, 7d, 24h) consistent with `hive logs`
- Large exports use streaming writes to handle months of data without memory issues

### Integration Points

- internal/audit/logger.go (modified -- AuditLogger, ExportJSON, ExportCSV, RedactSensitiveFields)
- internal/audit/logger_test.go (new -- export format, filtering, redaction tests)
- internal/cli/audit.go (new -- `hive audit export` command)
- internal/event/bus.go (reference -- event source for audit records)
- internal/api/auth.go (reference -- auth events for audit trail)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 21 - Story 21.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR123]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- AuditLogger aggregates system, auth, and agent events into normalized audit records
- JSON and CSV export with time range and event type filtering
- Sensitive data redaction: API keys, tokens, passwords, certificates replaced with [REDACTED]
- Streaming export for memory efficiency on large datasets
- CLI with --format, --since, --output, --type flags and progress indicator

### Change Log

- 2026-04-16: Story 21.3 implemented -- audit log export with JSON/CSV formats and sensitive data redaction

### File List

- internal/audit/logger.go (modified -- AuditLogger, ExportJSON, ExportCSV, RedactSensitiveFields, audit record normalization)
- internal/audit/logger_test.go (new -- export format, filtering, redaction, streaming tests)
- internal/cli/audit.go (new -- `hive audit export` command with format/since/output/type flags)
- internal/event/bus.go (reference -- event source for audit aggregation)
- internal/api/auth.go (reference -- auth events for audit trail)
