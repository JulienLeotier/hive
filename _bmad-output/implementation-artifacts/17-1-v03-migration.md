# Story 17.1: v0.3 Migration

Status: done

## Story

As a developer,
I want the v0.3 database migration adding costs and budget tables,
so that cost tracking persists.

## Acceptance Criteria

1. **Given** an existing v0.2 database
   **When** the v0.3 binary starts
   **Then** migration 003 runs creating: `costs`, `budget_alerts` tables

2. **Given** the migration runs
   **When** it completes
   **Then** existing data in all v0.1 and v0.2 tables is preserved

3. **Given** the migration has already been applied
   **When** the binary restarts
   **Then** the migration is idempotent -- it does not re-run or error

4. **Given** the costs table is created
   **When** cost data is recorded
   **Then** it stores: agent_id, agent_name, workflow_id, task_id, cost, created_at

5. **Given** the budget_alerts table is created
   **When** a budget alert is configured
   **Then** it stores: agent_name, daily_limit, last_triggered, created_at

## Tasks / Subtasks

- [x] Task 1: Create migration SQL file (AC: #1, #4, #5)
  - [x] Create `internal/storage/migrations/003_v03.sql`
  - [x] Define `costs` table: id (autoincrement), agent_id, agent_name, workflow_id, task_id, cost (REAL), created_at
  - [x] Define `budget_alerts` table: id (autoincrement), agent_name (UNIQUE), daily_limit (REAL), last_triggered, created_at
  - [x] Add indexes: `idx_costs_agent` on costs(agent_name), `idx_costs_date` on costs(created_at)
  - [x] Use `CREATE TABLE IF NOT EXISTS` for idempotency
- [x] Task 2: Verify migration system (AC: #2, #3)
  - [x] Verify existing migration system in `internal/storage/sqlite.go` picks up `003_v03.sql` automatically
  - [x] Confirm `schema_versions` table tracks version 3 after migration
  - [x] Verify v0.1 and v0.2 tables remain intact after migration
- [x] Task 3: Tests (AC: #1, #2, #3)
  - [x] Test migration 003 creates costs and budget_alerts tables
  - [x] Test migration is idempotent (running twice does not error)
  - [x] Test existing data in agents, events, tasks, workflows tables survives migration
  - [x] Test costs table accepts valid INSERT statements
  - [x] Test budget_alerts agent_name UNIQUE constraint

## Dev Notes

### Architecture Compliance

- Migration file follows existing naming convention: `NNN_description.sql` (e.g., `003_v03.sql`)
- Uses `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` for idempotency
- Migration is embedded via `//go:embed *.sql` in `internal/storage/migrations/embed.go`
- Existing migration runner in `internal/storage/sqlite.go` handles version tracking via `schema_versions` table
- No changes needed to migration infrastructure -- it auto-discovers new `.sql` files

### Key Design Decisions

- The `costs` table includes both `agent_id` and `agent_name` -- the ID is the canonical reference, the name is denormalized for query convenience (avoids JOINs in cost aggregation)
- `budget_alerts.agent_name` has a UNIQUE constraint since there should be exactly one budget alert per agent
- `last_triggered` is nullable (NULL means never triggered) -- simpler than using a sentinel date
- Indexes on `costs(agent_name)` and `costs(created_at)` optimize the most common queries: `ByAgent()` and `DailyCostForAgent()`
- The `costs.cost` column is REAL (float64) matching Go's `float64` for cost values

### Migration SQL

```sql
CREATE TABLE IF NOT EXISTS costs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    workflow_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    cost REAL NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_costs_agent ON costs(agent_name);
CREATE INDEX IF NOT EXISTS idx_costs_date ON costs(created_at);

CREATE TABLE IF NOT EXISTS budget_alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL UNIQUE,
    daily_limit REAL NOT NULL,
    last_triggered TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
```

### Integration Points

- `internal/storage/migrations/003_v03.sql` -- new migration file
- `internal/storage/migrations/embed.go` -- auto-embeds new SQL file
- `internal/storage/sqlite.go` -- existing migration runner handles version 3
- `internal/cost/tracker.go` -- uses costs and budget_alerts tables
- `internal/cost/tracker_test.go` -- tests create tables inline (test isolation)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 17.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Migration 003_v03.sql creates costs and budget_alerts tables
- Costs table: agent_id, agent_name, workflow_id, task_id, cost (REAL), created_at with indexes
- Budget_alerts table: agent_name (UNIQUE), daily_limit (REAL), last_triggered (nullable), created_at
- Idempotent via CREATE TABLE/INDEX IF NOT EXISTS
- Existing migration system auto-discovers new .sql files -- no infrastructure changes needed
- All existing data preserved across migration

### Change Log

- 2026-04-16: Story 17.1 implemented -- v0.3 database migration for costs and budget_alerts

### File List

- internal/storage/migrations/003_v03.sql (new -- costs and budget_alerts tables)
- internal/storage/migrations/embed.go (reference -- auto-embeds new migration)
- internal/storage/sqlite.go (reference -- migration runner)
- internal/storage/sqlite_test.go (modified -- added migration 003 tests)
