# Story 1.1: Project Bootstrap & Storage Layer

Status: review

## Story

As a developer,
I want to initialize the Hive Go module with embedded SQLite storage,
so that I have a solid foundation to build all other features on.

## Acceptance Criteria

1. **Go module initialized** with `github.com/JulienLeotier/hive` module path and all dependencies resolved
2. **SQLite database** created at `~/.hive/data/hive.db` on first run with WAL mode enabled
3. **Schema migrations** run automatically on startup, creating tables: `agents`, `events`, `tasks`, `workflows`, `api_keys`, `schema_versions`
4. **Data directory** created at `~/.hive/data/` with `0700` permissions if it doesn't exist
5. **Configuration** loads from `hive.yaml` in current directory with environment variable overrides (`HIVE_*` prefix)
6. **CLI skeleton** responds to `hive version` and `hive --help`
7. **Structured logging** via `log/slog` with configurable level (default: INFO)
8. **All tests pass** with `go test ./...`

## Tasks / Subtasks

- [x] Task 1: Go module & dependencies (AC: #1)
  - [x] `go mod init github.com/JulienLeotier/hive`
  - [x] Add dependencies: `modernc.org/sqlite`, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/oklog/ulid/v2`, `github.com/stretchr/testify`
  - [x] `go mod tidy`
- [x] Task 2: Configuration loader (AC: #5)
  - [x] Create `internal/config/config.go` with `Config` struct
  - [x] YAML loading from `hive.yaml` (current dir)
  - [x] Environment variable overrides with `HIVE_` prefix (e.g., `HIVE_LOG_LEVEL=debug`)
  - [x] Sensible defaults (log_level: info, data_dir: ~/.hive/data, port: 8233)
  - [x] Create `internal/config/config_test.go`
- [x] Task 3: SQLite storage layer (AC: #2, #3, #4)
  - [x] Create `internal/storage/sqlite.go` with `Store` struct
  - [x] Open/create database at configured data dir path
  - [x] Enable WAL mode, set busy_timeout=5000, journal_size_limit=64MB
  - [x] Create `internal/storage/migrations/001_initial.sql` with all tables
  - [x] Create `internal/storage/migrations/embed.go` with `//go:embed *.sql`
  - [x] Run migrations on `Store.Open()`, track in `schema_versions` table
  - [x] Data directory creation with `0700` permissions
  - [x] Create `internal/storage/sqlite_test.go` (test open, migrate, close)
- [x] Task 4: CLI skeleton (AC: #6, #7)
  - [x] Create `internal/cli/root.go` with root command, `--log-level` flag, slog initialization
  - [x] Create `internal/cli/version.go` with `hive version` command (prints version, Go version, OS/arch)
  - [x] Create `cmd/hive/main.go` entry point
  - [x] Version injected via ldflags at build time (`-X main.version=...`)
- [x] Task 5: Makefile (AC: #8)
  - [x] `make build` — compile binary to `./hive`
  - [x] `make test` — run all tests
  - [x] `make lint` — run `go vet` + `golangci-lint`
  - [x] `make dev` — build and run with debug logging

## Dev Notes

### Architecture Compliance

- **Language:** Go 1.24+ (already installed on system: go1.26.0)
- **SQLite driver:** `modernc.org/sqlite` — pure Go, NO CGO. This is critical for single-binary cross-compilation
- **DO NOT** use `mattn/go-sqlite3` — it requires CGO and breaks single-binary builds
- **CLI framework:** `github.com/spf13/cobra` — use subcommand pattern
- **IDs:** ULID via `github.com/oklog/ulid/v2` — import and verify it compiles, used in later stories
- **Logging:** `log/slog` from stdlib (Go 1.21+) — JSON handler for production, text handler for dev
- **Config:** `gopkg.in/yaml.v3` — struct tags for YAML mapping
- **Testing:** `testing` + `github.com/stretchr/testify/assert` + `github.com/stretchr/testify/require`

### Database Schema (001_initial.sql)

```sql
CREATE TABLE IF NOT EXISTS schema_versions (
    version INTEGER PRIMARY KEY,
    applied_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    capabilities TEXT NOT NULL DEFAULT '{}',
    plan TEXT,
    health_status TEXT DEFAULT 'unknown',
    trust_level TEXT DEFAULT 'scripted',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    agent_id TEXT,
    input TEXT NOT NULL DEFAULT '{}',
    output TEXT,
    checkpoint TEXT,
    depends_on TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow ON tasks(workflow_id);

CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    status TEXT DEFAULT 'idle',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    key_hash TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
```

### Naming Conventions (from Architecture)

- **Packages:** lowercase single word (`config`, `storage`, `cli`)
- **Exported types:** PascalCase (`Store`, `Config`)
- **Unexported:** camelCase (`openDB`, `runMigrations`)
- **Files:** snake_case (`sqlite.go`, `config_test.go`)
- **Tests:** co-located (`foo.go` + `foo_test.go`)

### Project Structure (files to create in this story)

```
hive/
├── cmd/hive/main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   ├── sqlite_test.go
│   │   └── migrations/
│   │       ├── 001_initial.sql
│   │       └── embed.go
│   └── cli/
│       ├── root.go
│       └── version.go
├── go.mod
├── go.sum
└── Makefile
```

### Anti-Patterns to Avoid

- **DO NOT** create a `utils` or `helpers` package — put functions where they're used
- **DO NOT** use an ORM — direct SQL with `database/sql` and prepared statements
- **DO NOT** create tables for features not in this story (knowledge table is v0.2)
- **DO NOT** add HTTP server in this story — that's Story 1.2+
- **DO NOT** hardcode paths — use config for all file locations
- **DO NOT** use `fmt.Println` for logging — use `slog` everywhere

### Config Struct Shape

```go
type Config struct {
    LogLevel string `yaml:"log_level" env:"HIVE_LOG_LEVEL"`
    DataDir  string `yaml:"data_dir" env:"HIVE_DATA_DIR"`
    Port     int    `yaml:"port" env:"HIVE_PORT"`
}
```

Defaults: `log_level: "info"`, `data_dir: "~/.hive/data"`, `port: 8233`

### Testing Requirements

- `config_test.go`: Test YAML loading, env overrides, defaults
- `sqlite_test.go`: Test database open, migration execution, table existence, WAL mode enabled
- Use `t.TempDir()` for test databases — never touch real `~/.hive/`
- All tests must pass with `go test ./...`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Technology Stack]
- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR36, FR37]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Go module initialized with all 5 dependencies (modernc.org/sqlite, cobra, yaml.v3, ulid/v2, testify)
- Config loader: YAML + env overrides + tilde expansion + sensible defaults. 6 tests pass.
- SQLite store: WAL mode, auto-migration, schema_versions tracking, 0700 dir permissions. 7 tests pass.
- CLI skeleton: root command with --log-level flag, version subcommand with ldflags injection.
- Makefile: build, test, lint, dev, clean targets.
- All 13 tests pass. go vet clean. Binary builds to 15MB single file.

### Change Log

- 2026-04-16: Story 1.1 implemented — project bootstrap with config, storage, CLI, and Makefile

### File List

- cmd/hive/main.go (new)
- internal/config/config.go (new)
- internal/config/config_test.go (new)
- internal/storage/sqlite.go (new)
- internal/storage/sqlite_test.go (new)
- internal/storage/migrations/001_initial.sql (new)
- internal/storage/migrations/embed.go (new)
- internal/cli/root.go (new)
- internal/cli/version.go (new)
- go.mod (new)
- go.sum (new)
- Makefile (new)
