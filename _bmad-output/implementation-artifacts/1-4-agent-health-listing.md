# Story 1.4: Agent Health & Listing

Status: done

## Story

As a user,
I want to list all registered agents and see their health status,
so that I know which agents are available and functioning.

## Acceptance Criteria

1. **Given** one or more registered agents
   **When** the user runs `hive status`
   **Then** the output shows a table with: agent name, type, health status, and trust level
   **And** the total agent count is displayed

2. **Given** the `hive status` command
   **When** the `--json` flag is provided
   **Then** output is JSON-encoded array of agent objects for machine-readable format

3. **Given** no registered agents
   **When** the user runs `hive status`
   **Then** a helpful message is shown: "No agents registered. Use 'hive add-agent' to register one."

4. **Given** the API server is running
   **When** a client calls `GET /api/v1/agents`
   **Then** the response returns all agents in the standard API envelope `{"data": [...], "error": null}`

5. **Given** the API server is running
   **When** a client calls `GET /api/v1/metrics`
   **Then** the response includes agent counts by status (healthy, degraded, unavailable) and circuit breaker states (FR4)

## Tasks / Subtasks

- [x] Task 1: `hive status` CLI command (AC: #1, #2, #3)
  - [x] Add `statusCmd` to `internal/cli/agent.go`
  - [x] Tabular output with columns: NAME, TYPE, HEALTH, TRUST (padded formatting)
  - [x] `--json` flag for JSON output via `json.NewEncoder`
  - [x] Empty state message when no agents registered
  - [x] Register via `rootCmd.AddCommand(statusCmd)` in `init()`
- [x] Task 2: API server foundation (AC: #4, #5)
  - [x] Create `internal/api/server.go` with `Server` struct
  - [x] Dependencies: `agent.Manager`, `event.Bus`, `resilience.BreakerRegistry`, `KeyManager`
  - [x] `NewServer()` constructor wires all dependencies
  - [x] Standard response envelope: `Response{Data, Error}` and `Error{Code, Message}`
  - [x] `writeJSON()` and `writeError()` helpers
  - [x] Route registration: `GET /api/v1/agents`, `GET /api/v1/events`, `GET /api/v1/metrics`
  - [x] `Handler()` returns `http.Handler` with auth middleware applied
  - [x] `Start(addr)` starts the HTTP server
  - [x] `Serve()` background server with graceful shutdown
- [x] Task 3: API handlers (AC: #4, #5)
  - [x] `handleListAgents` — delegates to `agentMgr.List()`, returns agents in envelope
  - [x] `handleListEvents` — query events with type/source/since filters
  - [x] `handleMetrics` — agent counts by health status, circuit breaker states, timestamp
- [x] Task 4: `hive serve` CLI command
  - [x] Create `internal/cli/serve.go`
  - [x] Loads config, opens storage, wires all dependencies
  - [x] Serves API routes under `/api/` with auth middleware
  - [x] Serves dashboard at `/` (static embedded assets)
  - [x] Graceful shutdown on SIGINT/SIGTERM
- [x] Task 5: API server tests (AC: #4, #5)
  - [x] Create `internal/api/server_test.go`
  - [x] `setupServer()` — creates server with temp DB and all dependencies
  - [x] `TestListAgentsEndpoint` — verify 200 OK and response envelope
  - [x] `TestMetricsEndpoint` — verify agent counts and circuit breaker data
  - [x] `TestEventsEndpoint` — verify event query with type filter

## Dev Notes

### Architecture Compliance

- **API envelope:** `{"data": ..., "error": null}` matches architecture spec exactly
- **Error format:** `{"data": null, "error": {"code": "...", "message": "..."}}` with uppercase error codes
- **Routes:** Prefixed with `/api/v1/` per architecture conventions
- **CLI output:** Tabular format with column headers for human readability, `--json` for scripting
- **Auth middleware:** Applied via `Handler()` method — all API routes are authenticated when keys exist
- **Dependencies:** Server receives all dependencies via constructor injection (no globals)
- **Serve command:** Wires together storage, agent manager, event bus, circuit breakers, key manager, and dashboard
- **Graceful shutdown:** Signal handling with 5s timeout per production best practices

### Testing Strategy

- Tests use `httptest.NewRecorder` for handler testing without network
- `setupServer()` creates full dependency graph with temp database
- Tests verify response format, status codes, and data structure

### References

- [Source: architecture.md#API & Communication Patterns — Internal API]
- [Source: architecture.md#Project Structure — internal/api/]
- [Source: epics.md#Story 1.4 — FR4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- `hive status` command with tabular output, --json flag, and empty state message
- API server with standard response envelope and 3 endpoint handlers
- Metrics endpoint with agent health counts and circuit breaker states
- `hive serve` command wiring all dependencies with graceful shutdown
- 3 API server tests verifying response format and data

### Change Log

- 2026-04-16: Story 1.4 implemented — agent health listing via CLI and API server with metrics

### File List

- internal/cli/agent.go (modified — added statusCmd)
- internal/api/server.go (new)
- internal/api/server_test.go (new)
- internal/cli/serve.go (new)
