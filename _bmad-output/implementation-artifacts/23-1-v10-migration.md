# Story 23.1: v1.0 Migration

Status: done

## Story

As a developer,
I want the v1.0 database migration,
so that all new tables are created on upgrade.

## Acceptance Criteria

1. **Given** an existing v0.3 database
   **When** the v1.0 binary starts
   **Then** migration 004 runs creating: bids, federation_links, optimizations, tenants, roles tables

2. **Given** existing v0.3 data
   **When** migration 004 runs
   **Then** all existing data is preserved

3. **Given** migration 004
   **When** it runs multiple times
   **Then** it is idempotent (no errors on re-run)

4. **Given** the v1.0 migration for PostgreSQL
   **When** `storage: postgres` is configured
   **Then** the PostgreSQL-dialect migration runs with equivalent schema

## Tasks / Subtasks

- [x] Task 1: SQLite migration 004 (AC: #1, #2, #3)
  - [x] Create `internal/storage/migrations/004_v10.sql`
  - [x] CREATE TABLE IF NOT EXISTS bids (id, task_id, agent_id, price, estimated_duration, status, created_at)
  - [x] CREATE TABLE IF NOT EXISTS federation_links (id, peer_url, cert_path, status, capabilities, last_seen, created_at)
  - [x] CREATE TABLE IF NOT EXISTS optimizations (id, run_id, type, severity, affected_entity, data, status, created_at)
  - [x] CREATE TABLE IF NOT EXISTS tenants (id, name, status, created_at)
  - [x] CREATE TABLE IF NOT EXISTS roles (id, name, permissions, created_at)
  - [x] ALTER TABLE agents ADD COLUMN token_balance INTEGER DEFAULT 100
  - [x] ALTER TABLE agents ADD COLUMN node_id TEXT
  - [x] ALTER TABLE agents ADD COLUMN tenant_id TEXT DEFAULT 'default'
  - [x] ALTER TABLE tasks ADD COLUMN tenant_id TEXT DEFAULT 'default'
  - [x] ALTER TABLE workflows ADD COLUMN tenant_id TEXT DEFAULT 'default'
  - [x] ALTER TABLE events ADD COLUMN tenant_id TEXT DEFAULT 'default'
  - [x] ALTER TABLE api_keys ADD COLUMN role TEXT DEFAULT 'operator'
  - [x] Create indexes on tenant_id columns and bids.task_id
  - [x] All CREATE/ALTER statements use IF NOT EXISTS / IF NOT EXISTS patterns for idempotency
- [x] Task 2: PostgreSQL migration 004 (AC: #4)
  - [x] Create `internal/storage/postgres_migrations/004_v10.sql`
  - [x] Equivalent schema using PostgreSQL types: JSONB, TIMESTAMPTZ, SERIAL
  - [x] Use DO $$ BEGIN ... EXCEPTION ... END $$ for idempotent ALTER TABLE
  - [x] Same table and column structure as SQLite migration
- [x] Task 3: Migration runner update (AC: #1, #3)
  - [x] Register migration 004 in the migration runner
  - [x] Verify migration version tracking in schema_versions table
  - [x] Ensure migration only runs once (version check before execution)
- [x] Task 4: Data preservation verification (AC: #2)
  - [x] Test migration on database with v0.3 data
  - [x] Verify all existing agents, tasks, workflows, events are intact after migration
  - [x] Verify existing records get default tenant_id = 'default'
  - [x] Verify existing API keys get default role = 'operator'
- [x] Task 5: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test migration creates all new tables
  - [x] Test migration is idempotent (run twice, no errors)
  - [x] Test existing data preserved with default values
  - [x] Test new columns have correct defaults
  - [x] Test index creation

## Dev Notes

### Architecture Compliance

- Migration follows the same pattern as 001_initial.sql, 002_v02.sql, 003_v03.sql
- All DDL statements use IF NOT EXISTS for idempotency
- SQLite and PostgreSQL migrations are separate files for dialect correctness
- Migration version tracked in schema_versions table to prevent re-execution
- Uses `slog` for migration progress logging

### Key Design Decisions

- Default tenant_id = 'default' ensures backward compatibility -- existing data belongs to the default tenant
- Default role = 'operator' for existing API keys maintains current access levels
- Default token_balance = 100 gives existing agents a starting balance for market participation
- ALTER TABLE with defaults ensures existing rows get appropriate values without NULL issues
- Indexes on tenant_id columns are critical for multi-tenant query performance

### Migration Schema (SQLite)

```sql
-- Bids table for market allocation
CREATE TABLE IF NOT EXISTS bids (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    price INTEGER NOT NULL,
    estimated_duration INTEGER NOT NULL,
    status TEXT DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_bids_task ON bids(task_id);

-- Federation links
CREATE TABLE IF NOT EXISTS federation_links (
    id TEXT PRIMARY KEY,
    peer_url TEXT NOT NULL UNIQUE,
    cert_path TEXT NOT NULL,
    status TEXT DEFAULT 'disconnected',
    capabilities TEXT DEFAULT '{}',
    last_seen TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Optimizations
CREATE TABLE IF NOT EXISTS optimizations (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    affected_entity TEXT NOT NULL,
    data TEXT DEFAULT '{}',
    status TEXT DEFAULT 'new',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Tenants
CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Roles
CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    permissions TEXT NOT NULL DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Column additions (wrapped in error-tolerant blocks for idempotency)
-- token_balance, node_id, tenant_id on agents
-- tenant_id on tasks, workflows, events
-- role on api_keys
```

### Integration Points

- internal/storage/migrations/004_v10.sql (new -- SQLite v1.0 migration)
- internal/storage/postgres_migrations/004_v10.sql (new -- PostgreSQL v1.0 migration)
- internal/storage/sqlite.go (reference -- migration runner registers 004)
- internal/storage/postgres.go (reference -- PostgreSQL migration runner registers 004)
- internal/storage/migrations/embed.go (reference -- embeds new migration file)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 23 - Story 23.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Migration 004 creates 5 new tables: bids, federation_links, optimizations, tenants, roles
- Adds columns to existing tables: token_balance, node_id, tenant_id on agents; tenant_id on tasks/workflows/events; role on api_keys
- All statements use IF NOT EXISTS for idempotency
- Existing data preserved with sensible defaults (tenant_id='default', role='operator', token_balance=100)
- Both SQLite and PostgreSQL dialect migrations with equivalent schema
- Indexes on tenant_id columns and bids.task_id for query performance

### Change Log

- 2026-04-16: Story 23.1 implemented -- v1.0 database migration with new tables and column additions

### File List

- internal/storage/migrations/004_v10.sql (new -- SQLite v1.0 migration)
- internal/storage/migrations/embed.go (modified -- embeds 004_v10.sql)
- internal/storage/postgres_migrations/004_v10.sql (new -- PostgreSQL v1.0 migration)
- internal/storage/postgres_migrations/embed.go (modified -- embeds 004_v10.sql)
- internal/storage/sqlite.go (modified -- migration runner registers version 4)
- internal/storage/sqlite_test.go (modified -- migration 004 tests)
- internal/storage/postgres.go (modified -- PostgreSQL migration runner registers version 4)
