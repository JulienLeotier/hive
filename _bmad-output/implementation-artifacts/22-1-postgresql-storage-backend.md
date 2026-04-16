# Story 22.1: PostgreSQL Storage Backend

Status: done

## Story

As a user,
I want to use PostgreSQL instead of SQLite for production deployments,
so that my hive can handle higher concurrency and larger datasets.

## Acceptance Criteria

1. **Given** `storage: postgres` and `postgres_url: postgres://...` in `hive.yaml`
   **When** the system starts
   **Then** it connects to PostgreSQL and runs migrations

2. **Given** the PostgreSQL backend is active
   **When** all features are exercised
   **Then** they work identically to SQLite mode

3. **Given** the PostgreSQL connection
   **When** the connection is established
   **Then** connection pooling is configured with sensible defaults

4. **Given** a PostgreSQL connection failure
   **When** the system starts
   **Then** a clear error message indicates the connection issue

## Tasks / Subtasks

- [x] Task 1: Storage interface extraction (AC: #2)
  - [x] Extract `Store` interface from existing SQLite implementation
  - [x] Define interface methods: Open, Close, all CRUD operations for agents, tasks, workflows, events
  - [x] Refactor existing SQLite code to implement the interface
  - [x] Verify all existing tests pass with interface-based access
- [x] Task 2: PostgreSQL driver and connection (AC: #1, #3, #4)
  - [x] Add `github.com/lib/pq` or `github.com/jackc/pgx/v5` dependency (pure Go, no CGO)
  - [x] Implement `PostgresStore` struct implementing the `Store` interface
  - [x] Configure connection pooling: max_open=25, max_idle=5, conn_max_lifetime=5m
  - [x] Implement connection health check on startup
  - [x] Clear error messages for connection failures (host unreachable, auth failed, database not found)
- [x] Task 3: PostgreSQL migrations (AC: #1)
  - [x] Translate SQLite migrations to PostgreSQL dialect (TEXT -> TEXT, INTEGER -> BIGINT, datetime -> TIMESTAMPTZ)
  - [x] Create `internal/storage/postgres_migrations/` with PostgreSQL-specific SQL files
  - [x] Handle PostgreSQL-specific features: SERIAL, TIMESTAMPTZ, JSONB
  - [x] Migration runner works for both SQLite and PostgreSQL
- [x] Task 4: Query compatibility layer (AC: #2)
  - [x] Handle SQL dialect differences: `?` placeholders (SQLite) vs `$1` placeholders (PostgreSQL)
  - [x] Handle `datetime('now')` (SQLite) vs `NOW()` (PostgreSQL)
  - [x] Handle `AUTOINCREMENT` (SQLite) vs `SERIAL` / `GENERATED ALWAYS AS IDENTITY` (PostgreSQL)
  - [x] Use JSONB type for JSON columns in PostgreSQL for query performance
- [x] Task 5: Configuration and factory (AC: #1)
  - [x] Add `storage` field to config: `sqlite` (default) or `postgres`
  - [x] Add `postgres_url` field for connection string
  - [x] Implement `NewStore(config)` factory that returns SQLite or PostgreSQL store
  - [x] Environment variable: `HIVE_STORAGE=postgres`, `HIVE_POSTGRES_URL=postgres://...`
- [x] Task 6: Unit and integration tests (AC: #1, #2, #3, #4)
  - [x] Test PostgreSQL store implements all interface methods
  - [x] Test connection pooling configuration
  - [x] Test connection failure error messages
  - [x] Test migration execution on PostgreSQL
  - [x] Test query compatibility (same results from both backends)
  - [x] Integration test tag `//go:build postgres` for CI with PostgreSQL

## Dev Notes

### Architecture Compliance

- Storage interface allows pluggable backends without changing business logic
- PostgreSQL driver is pure Go (`pgx/v5`) -- maintains single-binary cross-compilation
- Connection pooling tuned for multi-node deployments (Story 22.2)
- JSONB type in PostgreSQL enables future query optimization on JSON fields
- Uses `slog` for structured logging of connection lifecycle

### Key Design Decisions

- pgx/v5 chosen over lib/pq: better performance, active maintenance, pure Go
- Storage interface extracted to `internal/storage/store.go` -- both backends implement it
- PostgreSQL migrations are separate files, not auto-translated -- ensures correctness for dialect differences
- JSONB used for all JSON columns in PostgreSQL -- enables future indexing and querying within JSON
- Connection pool defaults are conservative but configurable via config

### Integration Points

- internal/storage/store.go (new -- Store interface definition)
- internal/storage/sqlite.go (modified -- implements Store interface)
- internal/storage/postgres.go (new -- PostgresStore implementing Store interface)
- internal/storage/postgres_test.go (new -- PostgreSQL backend tests with build tag)
- internal/storage/postgres_migrations/ (new directory -- PostgreSQL migration files)
- internal/storage/postgres_migrations/004_v10.sql (new -- v1.0 migration in PostgreSQL dialect)
- internal/config/config.go (modified -- storage and postgres_url config fields)
- internal/cli/serve.go (modified -- uses NewStore factory)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 22 - Story 22.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR129, FR130]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Store interface extracted from SQLite implementation; both backends implement it
- PostgresStore with pgx/v5 driver, connection pooling (25 open, 5 idle, 5m lifetime)
- PostgreSQL-specific migrations with JSONB, TIMESTAMPTZ, SERIAL types
- Query compatibility layer handles placeholder and function differences
- NewStore factory selects backend based on config; SQLite remains default
- Clear error messages for PostgreSQL connection failures

### Change Log

- 2026-04-16: Story 22.1 implemented -- PostgreSQL storage backend with pluggable Store interface

### File List

- internal/storage/store.go (new -- Store interface definition)
- internal/storage/sqlite.go (modified -- implements Store interface)
- internal/storage/sqlite_test.go (modified -- tests via interface)
- internal/storage/postgres.go (new -- PostgresStore implementation with pgx/v5)
- internal/storage/postgres_test.go (new -- PostgreSQL tests with //go:build postgres tag)
- internal/storage/postgres_migrations/ (new directory)
- internal/storage/postgres_migrations/004_v10.sql (new -- v1.0 PostgreSQL migration)
- internal/storage/postgres_migrations/embed.go (new -- //go:embed for PostgreSQL migrations)
- internal/config/config.go (modified -- storage, postgres_url fields)
- internal/cli/serve.go (modified -- NewStore factory usage)
