# Story 1.2: Agent Adapter Protocol — HTTP Implementation

Status: done

## Story

As an adapter author,
I want a clear protocol interface and HTTP adapter implementation,
so that I can connect any HTTP-based agent to Hive in under 20 lines.

## Acceptance Criteria

1. **Given** the `Adapter` Go interface with `Declare()`, `Invoke()`, `Health()`, `Checkpoint()`, `Resume()` methods
   **When** an adapter author implements the HTTP adapter for their agent
   **Then** the implementation requires fewer than 20 lines of configuration for a basic agent

2. **Given** the HTTP adapter implementation
   **When** it communicates with an agent's endpoints
   **Then** the adapter uses HTTP/JSON with the agent's `/declare`, `/invoke`, `/health`, `/checkpoint`, `/resume` endpoints

3. **Given** the adapter protocol types
   **When** defining `AgentCapabilities`, `Task`, `TaskResult`, `HealthStatus`, `Checkpoint` structs
   **Then** all types use JSON struct tags and are serializable for transport

4. **Given** the `HTTPAdapter` implementation
   **When** an HTTP error (4xx/5xx) is returned from an agent
   **Then** the adapter returns a descriptive error including the HTTP status code and response body

5. **Given** the `HTTPAdapter` Health method
   **When** the agent is unreachable
   **Then** it returns `HealthStatus{Status: "unavailable"}` without erroring (graceful degradation)

6. **Given** the adapter interface
   **When** compile-time interface compliance is checked
   **Then** `var _ Adapter = (*HTTPAdapter)(nil)` compiles successfully (FR24, FR26-FR28)

## Tasks / Subtasks

- [x] Task 1: Define Adapter interface and protocol types (AC: #1, #3)
  - [x] Create `internal/adapter/adapter.go` with `Adapter` interface (5 methods)
  - [x] Define `AgentCapabilities` struct with `Name`, `TaskTypes`, `CostPerRun` fields
  - [x] Define `Task` struct with `ID`, `Type`, `Input` fields
  - [x] Define `TaskResult` struct with `TaskID`, `Status`, `Output`, `Error` fields
  - [x] Define `HealthStatus` struct with `Status`, `Message` fields
  - [x] Define `Checkpoint` struct with `Data` field
- [x] Task 2: Implement HTTP adapter (AC: #2, #4, #5)
  - [x] Create `internal/adapter/http.go` with `HTTPAdapter` struct
  - [x] `NewHTTPAdapter(baseURL)` constructor with 30s default timeout
  - [x] `Declare()` — GET `/declare`, decode `AgentCapabilities`
  - [x] `Invoke()` — POST `/invoke` with `Task` body, decode `TaskResult`
  - [x] `Health()` — GET `/health`, return `unavailable` on error (no error propagation)
  - [x] `Checkpoint()` — GET `/checkpoint`, decode `Checkpoint`
  - [x] `Resume()` — POST `/resume` with `Checkpoint` body
  - [x] Shared `get()`, `post()`, `do()` helpers with 10MB response limit
- [x] Task 3: HTTP adapter tests (AC: #6)
  - [x] Create `internal/adapter/http_test.go`
  - [x] `newMockAgent()` — `httptest.Server` responding to all 5 endpoints
  - [x] `TestHTTPAdapterDeclare` — verify capabilities decoded
  - [x] `TestHTTPAdapterHealth` — verify healthy status
  - [x] `TestHTTPAdapterHealthUnreachable` — verify graceful unavailable
  - [x] `TestHTTPAdapterInvoke` — verify task result
  - [x] `TestHTTPAdapterCheckpoint` — verify checkpoint data
  - [x] `TestHTTPAdapterResume` — verify resume success
  - [x] `TestHTTPAdapterHTTPError` — verify error includes HTTP status
  - [x] Compile-time interface check: `var _ Adapter = (*HTTPAdapter)(nil)`

## Dev Notes

### Architecture Compliance

- **Package:** `internal/adapter/` — owns the Adapter interface and all implementations
- **Interface design:** `Adapter` interface defined in the adapter package per architecture (5 methods: `Declare`, `Invoke`, `Health`, `Checkpoint`, `Resume`)
- **HTTP client:** Uses `net/http` stdlib with configurable `http.Client` and 30s default timeout
- **Response safety:** 10MB response body limit via `io.LimitReader` prevents OOM from malicious agents
- **Error wrapping:** All errors wrapped with `fmt.Errorf("method: %w", err)` per project conventions
- **JSON transport:** HTTP/JSON protocol as specified in architecture — all types use `json` struct tags
- **Graceful health:** `Health()` never returns an error — unreachable agents return `HealthStatus{Status: "unavailable"}` instead of failing
- **Naming:** PascalCase for exported types (`HTTPAdapter`, `AgentCapabilities`), camelCase for unexported helpers (`get`, `post`, `do`)

### Testing Strategy

- Mock agent via `httptest.Server` with all 5 protocol endpoints
- Tests cover happy path, error path, and unreachable agent scenarios
- Compile-time interface check ensures `HTTPAdapter` satisfies `Adapter`

### References

- [Source: architecture.md#API & Communication Patterns — Adapter Protocol]
- [Source: architecture.md#Core Architectural Decisions — HTTP/JSON adapter protocol]
- [Source: epics.md#FR24, FR26-FR28]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Adapter interface defined with 5 methods matching the architecture spec
- Protocol types (`AgentCapabilities`, `Task`, `TaskResult`, `HealthStatus`, `Checkpoint`) with JSON tags
- HTTP adapter implementation with GET/POST helpers and 10MB response limit
- 7 tests covering all protocol methods plus error handling and unreachable agents
- Compile-time interface compliance verified

### Change Log

- 2026-04-16: Story 1.2 implemented — adapter protocol interface and HTTP implementation

### File List

- internal/adapter/adapter.go (new)
- internal/adapter/http.go (new)
- internal/adapter/http_test.go (new)
