# Story 10.1: Knowledge Store & CRUD

Status: done

## Story

As a developer,
I want a knowledge store backed by SQLite,
so that learned patterns persist across restarts.

## Acceptance Criteria

1. **Given** the v0.2 migration has run (002_v02.sql)
   **When** a task completes (success or failure)
   **Then** the approach and outcome are stored in the knowledge table

2. **Given** a knowledge entry is recorded
   **When** it is stored
   **Then** it includes: task_type, approach description, outcome, context JSON

3. **Given** knowledge entries exist
   **When** the store counts entries
   **Then** it returns the correct total count

4. **Given** knowledge entries of different task types exist
   **When** entries are listed by type
   **Then** only entries matching the specified task type are returned in reverse chronological order

## Tasks / Subtasks

- [x] Task 1: Knowledge Store implementation (AC: #1, #2, #3, #4)
  - [x] Create `internal/knowledge/store.go` with `Store` struct backed by `*sql.DB`
  - [x] Define `Entry` struct: ID, TaskType, Approach, Outcome, Context, CreatedAt
  - [x] Implement `NewStore(db)` constructor with default 90-day maxAge
  - [x] Implement `Record(ctx, taskType, approach, outcome, ctxJSON)` — inserts entry into knowledge table
  - [x] Implement `Count(ctx)` — returns total knowledge entries count
  - [x] Implement `ListByType(ctx, taskType)` — returns entries filtered by task type, ordered by created_at DESC
- [x] Task 2: Knowledge table schema (AC: #1)
  - [x] `knowledge` table: id (autoincrement), task_type, approach, outcome, context, embedding (BLOB for future), created_at
  - [x] Table created by v0.2 migration
- [x] Task 3: Unit tests (AC: #2, #3, #4)
  - [x] Test Record and Count (insert one, count = 1)
  - [x] Test Record failure outcome (verify outcome field stored correctly)
  - [x] Test ListByType returns only matching entries
  - [x] Test ListByType returns correct count with mixed types

## Dev Notes

### Architecture Compliance

- **Direct SQL** — no ORM, uses `database/sql` with `ExecContext` and `QueryContext`
- **slog** — debug-level logging on knowledge recording
- **Time handling** — `created_at` stored as SQLite datetime text, parsed with Go `time.Parse`
- **COALESCE** — used in queries for nullable `context` field to prevent scan errors
- **Package isolation** — `internal/knowledge` depends only on `database/sql` — no circular dependencies

### Key Design Decisions

- Knowledge entries are immutable once recorded — no update or delete operations (append-only log of learned patterns)
- The `embedding` column exists in the schema as BLOB but is not populated in v0.2 — reserved for vector similarity search in v0.3
- `maxAge` is set to 90 days by default and used by the Search method (Story 10.2) to exclude stale entries
- Context field is stored as raw JSON string — flexible schema for different task types
- `ListByType` returns in reverse chronological order — most recent approaches appear first

### Integration Points

- `internal/knowledge/store.go` — knowledge CRUD operations
- `internal/knowledge/store_test.go` — unit tests for CRUD
- `internal/storage/migrations/` — knowledge table schema in v0.2 migration

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR70, FR71]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Knowledge store with Record, Count, ListByType operations
- Entry struct captures task_type, approach, outcome, context JSON
- Append-only pattern — no update/delete operations
- 90-day default maxAge for knowledge decay (used by Search in Story 10.2)
- 4 unit tests covering recording, counting, failure outcomes, and type filtering

### Change Log

- 2026-04-16: Story 10.1 implemented — knowledge store CRUD with SQLite persistence

### File List

- internal/knowledge/store.go (new)
- internal/knowledge/store_test.go (new)
