# Story 12.1: v0.2 Migration & Schema Update

Status: done

## Story

As a developer,
I want the v0.2 database schema migration,
so that all new tables are created on upgrade.

## Acceptance Criteria

1. **Given** an existing v0.1 database
   **When** the v0.2 binary starts
   **Then** migration 002 runs automatically creating: knowledge, trust_history, dialog_threads, dialog_messages, webhooks, costs tables

2. **Given** an existing v0.1 database with data
   **When** the migration runs
   **Then** all existing data in agents, events, tasks, workflows, api_keys tables is preserved

3. **Given** the migration has already run (version 2 in schema_versions)
   **When** the binary starts again
   **Then** the migration is skipped (idempotent)

4. **Given** a fresh installation with no database
   **When** the binary starts
   **Then** both migration 001 and 002 run sequentially, creating all tables

## Tasks / Subtasks

- [x] Task 1: Create v0.2 migration SQL (AC: #1, #2)
  - [x] Create `internal/storage/migrations/002_v02.sql`
  - [x] `knowledge` table: id (autoincrement), task_type, approach, outcome, context, embedding (BLOB), created_at
  - [x] `trust_history` table: id (ULID), agent_id, old_level, new_level, reason, criteria, created_at
  - [x] `dialog_threads` table: id (ULID), initiator_id, participant_id, topic, status, created_at
  - [x] `dialog_messages` table: id (autoincrement), thread_id, sender_id, content, created_at
  - [x] `webhooks` table: id (ULID), name (unique), url, type, event_filter, enabled, created_at
  - [x] `costs` table: id (autoincrement), agent_id, agent_name, workflow_id, task_id, cost (REAL), created_at
  - [x] All tables use `CREATE TABLE IF NOT EXISTS` for idempotency
  - [x] Add relevant indexes for query performance
- [x] Task 2: Migration runner compatibility (AC: #3, #4)
  - [x] Existing `Store.migrate()` in `internal/storage/sqlite.go` handles sequential migration execution
  - [x] Version tracking via `schema_versions` table prevents re-running
  - [x] `embed.go` in migrations package automatically includes new `.sql` files via `//go:embed *.sql`
- [x] Task 3: Verify existing data preservation (AC: #2)
  - [x] Migration only creates new tables — no ALTER TABLE or data modification on existing tables
  - [x] v0.1 tables untouched: agents, events, tasks, workflows, api_keys, schema_versions

## Dev Notes

### Architecture Compliance

- **Embedded migrations** — SQL files in `internal/storage/migrations/` are embedded via `//go:embed *.sql` and executed sequentially by version number
- **Transactional** — each migration runs in a SQLite transaction; rollback on failure prevents partial schema
- **Version tracking** — `schema_versions` table tracks applied versions; `parseVersion()` extracts version from filename prefix (e.g., `002` from `002_v02.sql`)
- **IF NOT EXISTS** — all CREATE TABLE statements use this clause for idempotency beyond the version check

### Key Design Decisions

- Migration file naming: `002_v02.sql` — version number prefix for ordering, descriptive suffix for readability
- All v0.2 tables created in a single migration file rather than one per table — keeps the migration count low and ensures atomic v0.2 upgrade
- No ALTER TABLE on existing v0.1 tables — v0.2 features use entirely new tables, maintaining backward compatibility
- `embedding` BLOB column on knowledge table is nullable and unused in v0.2 — reserved for v0.3 vector search
- `trust_history` uses ULID for primary key (TEXT), consistent with agents table
- `dialog_messages.thread_id` references `dialog_threads.id` — no explicit FOREIGN KEY constraint to keep SQLite compatibility simple, but the relationship is enforced by application logic
- `costs.cost` is REAL (float64) — sufficient precision for API pricing

### Integration Points

- `internal/storage/migrations/002_v02.sql` (new) — v0.2 schema migration
- `internal/storage/migrations/embed.go` — `//go:embed *.sql` auto-includes new migration
- `internal/storage/sqlite.go` — `migrate()` method handles sequential execution and version tracking

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 12.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- v0.2 migration creates 6 new tables: knowledge, trust_history, dialog_threads, dialog_messages, webhooks, costs
- All tables use CREATE TABLE IF NOT EXISTS for idempotency
- Existing v0.1 data preserved — no ALTER TABLE or data modifications
- Migration runs automatically on startup, tracked in schema_versions
- Embedded via //go:embed *.sql glob pattern

### Change Log

- 2026-04-16: Story 12.1 implemented — v0.2 database migration with 6 new tables

### File List

- internal/storage/migrations/002_v02.sql (new)
- internal/storage/migrations/embed.go (reference — auto-embeds new SQL files)
- internal/storage/sqlite.go (reference — migrate() handles versioned execution)
