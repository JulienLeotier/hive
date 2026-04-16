# Story 1.3: Agent Registration via CLI

Status: done

## Story

As a user,
I want to register, configure, and remove agents via the `hive` CLI,
so that I can manage which agents participate in my hive.

## Acceptance Criteria

1. **Given** a running hive with the storage layer initialized
   **When** the user runs `hive add-agent --name code-reviewer --type http --url http://localhost:8080`
   **Then** the system validates connectivity by calling the agent's `/health` endpoint
   **And** calls `/declare` to retrieve and store the agent's capabilities
   **And** stores the agent record in SQLite with status from health check
   **And** confirms registration with a success message showing agent name, ID, health, and capabilities

2. **Given** a registered agent
   **When** the user runs `hive remove-agent code-reviewer`
   **Then** the agent is removed from the registry
   **And** a confirmation message is printed

3. **Given** an attempt to register an agent with a duplicate name
   **When** the user runs `hive add-agent --name existing-agent ...`
   **Then** the registration fails with a clear error message

4. **Given** the agent manager
   **When** listing agents
   **Then** agents are returned sorted by name with all fields populated (FR1, FR3, FR5, FR7)

## Tasks / Subtasks

- [x] Task 1: Agent domain types (AC: #4)
  - [x] Create `internal/agent/agent.go` with `Agent` struct
  - [x] Fields: `ID`, `Name`, `Type`, `Config`, `Capabilities`, `HealthStatus`, `TrustLevel`, `CreatedAt`, `UpdatedAt`
  - [x] JSON struct tags for API serialization
- [x] Task 2: Agent Manager (AC: #1, #2, #3, #4)
  - [x] Create `internal/agent/manager.go` with `Manager` struct
  - [x] `NewManager(db)` constructor
  - [x] `Register(ctx, name, type, baseURL)` — health check, declare, store in SQLite
  - [x] `List(ctx)` — return all agents sorted by name (default limit 1000)
  - [x] `ListWithLimit(ctx, limit)` — configurable limit
  - [x] `Remove(ctx, name)` — delete agent, fail if not found
  - [x] `GetByName(ctx, name)` — retrieve single agent by name
  - [x] `UpdateHealth(ctx, name, status)` — update health status
  - [x] ULID generation for agent IDs via `oklog/ulid`
  - [x] Structured logging via `slog` for register/remove events
- [x] Task 3: Agent Manager tests (AC: #1, #2, #3, #4)
  - [x] Create `internal/agent/manager_test.go`
  - [x] `setupTestManager()` with temp SQLite DB and mock HTTP agent
  - [x] `TestRegisterAgent` — verify fields populated correctly
  - [x] `TestRegisterDuplicateNameFails` — verify unique constraint
  - [x] `TestListAgents` — verify sorted by name
  - [x] `TestListEmptyReturnsNil` — verify empty state
  - [x] `TestRemoveAgent` — verify removal
  - [x] `TestRemoveNonExistentFails` — verify error for missing agent
  - [x] `TestGetByName` — verify single lookup
  - [x] `TestGetByNameNotFound` — verify error for missing agent
- [x] Task 4: CLI commands (AC: #1, #2)
  - [x] Create `internal/cli/agent.go`
  - [x] `hive add-agent` — flags: `--name`, `--type` (default: http), `--url`
  - [x] Validates `--name` and `--url` are required
  - [x] Prints registration success with agent ID, health, capabilities
  - [x] `hive remove-agent [name]` — positional arg, `ExactArgs(1)`
  - [x] Prints removal confirmation
  - [x] Register commands in `init()` via `rootCmd.AddCommand()`

## Dev Notes

### Architecture Compliance

- **Package:** `internal/agent/` — owns Agent domain type and Manager
- **Database:** Direct SQL with `database/sql` prepared statements (no ORM)
- **IDs:** ULID via `github.com/oklog/ulid/v2` with `crypto/rand` entropy
- **Unique names:** SQLite UNIQUE constraint on `agents.name` prevents duplicates
- **Error wrapping:** `fmt.Errorf("context: %w", err)` consistently
- **Logging:** `slog.Info()` for registration and removal events
- **CLI pattern:** Cobra subcommands with flag parsing, loads config and opens storage per-command
- **Naming:** `Manager` (PascalCase), `NewManager` constructor pattern, `manager.go` (snake_case file)
- **Health validation on register:** Calls `/health` then `/declare` before storing — matches FR7 (validates connectivity and protocol compliance)

### Testing Strategy

- Tests use `t.TempDir()` for isolated SQLite databases
- Mock HTTP agent via `httptest.Server` responding to `/health` and `/declare`
- 8 test cases covering register, duplicate, list, remove, and lookup scenarios

### References

- [Source: architecture.md#Data Architecture — agents table]
- [Source: architecture.md#Naming Patterns — Go Code]
- [Source: epics.md#Story 1.3 — FR1, FR3, FR5, FR7]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Agent struct with full field set matching SQLite schema
- Manager with Register, List, ListWithLimit, Remove, GetByName, UpdateHealth methods
- CLI commands: `hive add-agent` with --name/--type/--url flags, `hive remove-agent` with positional arg
- 8 tests covering all CRUD operations and edge cases
- All tests use isolated temp databases and mock HTTP agents

### Change Log

- 2026-04-16: Story 1.3 implemented — agent registration, removal, and listing via CLI

### File List

- internal/agent/agent.go (new)
- internal/agent/manager.go (new)
- internal/agent/manager_test.go (new)
- internal/cli/agent.go (new)
