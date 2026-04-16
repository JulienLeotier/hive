# Story 13.2: LangChain/LangGraph Adapter

Status: done

## Story

As a user,
I want to register LangChain and LangGraph agents with Hive,
so that I can orchestrate my LangChain chains and graphs.

## Acceptance Criteria

1. **Given** a LangChain agent exposed via HTTP (LangServe)
   **When** the user runs `hive add-agent --type langchain --url http://localhost:8000`
   **Then** the adapter connects to the LangServe endpoint and maps available chains to Hive capabilities

2. **Given** a registered LangChain agent
   **When** `Declare()` is called
   **Then** it attempts to retrieve capabilities from the LangServe endpoint
   **And** falls back to a default `langchain-chain` task type if the endpoint is unavailable

3. **Given** a registered LangChain agent
   **When** a task is routed to it
   **Then** tasks invoke specific chains via the LangServe HTTP API
   **And** results are returned through the standard Hive protocol

4. **Given** a LangChain adapter
   **When** `Health()` is called
   **Then** it delegates to the underlying HTTP adapter's health check

5. **Given** a LangChain adapter
   **When** `Checkpoint()` or `Resume()` is called
   **Then** it delegates to the underlying HTTP adapter

## Tasks / Subtasks

- [x] Task 1: LangChain adapter struct (AC: #1, #2)
  - [x] Create `LangChainAdapter` struct wrapping an `HTTPAdapter` and `Name` field
  - [x] Implement `NewLangChainAdapter(baseURL, name)` constructor that creates inner HTTPAdapter
  - [x] Implement `Declare()` -- attempts HTTP declare, falls back to `langchain-chain` default
  - [x] Verify compile-time interface satisfaction with `var _ Adapter = (*LangChainAdapter)(nil)`
- [x] Task 2: HTTP delegation (AC: #3, #4, #5)
  - [x] Implement `Invoke()` delegating to `http.Invoke()` for LangServe API calls
  - [x] Implement `Health()` delegating to `http.Health()`
  - [x] Implement `Checkpoint()` delegating to `http.Checkpoint()`
  - [x] Implement `Resume()` delegating to `http.Resume()`
- [x] Task 3: Name override (AC: #2)
  - [x] Override the name from HTTP declare response with the user-provided name
  - [x] Ensure capabilities struct always carries the correct agent name

## Dev Notes

### Architecture Compliance

- Implements the `Adapter` interface from `internal/adapter/adapter.go`
- Delegates all protocol operations to the existing `HTTPAdapter` -- avoids code duplication
- LangServe endpoints follow standard HTTP/JSON patterns that the HTTPAdapter already supports
- Graceful fallback when LangServe does not expose a `/declare` endpoint

### Key Design Decisions

- The adapter wraps `HTTPAdapter` via composition rather than inheritance -- clean Go idiom that reuses all HTTP transport logic
- LangServe does not have a standard capability declaration endpoint, so the adapter defaults to `langchain-chain` as the task type when `/declare` is not available
- The user-provided name always overrides whatever the HTTP endpoint returns, ensuring consistent naming in the Hive registry
- LangGraph agents exposed via LangServe are handled identically -- the adapter does not distinguish between chains and graphs

### Integration Points

- `internal/adapter/adapter.go` -- implements `Adapter` interface
- `internal/adapter/http.go` -- delegates all transport to `HTTPAdapter`
- `internal/cli/agent.go` -- `hive add-agent --type langchain` creates this adapter
- `internal/agent/manager.go` -- stores agent record after `Declare()` call

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 13.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR85, FR88]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- LangChainAdapter wraps HTTPAdapter via composition for full LangServe HTTP support
- Declare falls back to langchain-chain task type when endpoint unavailable
- All protocol methods delegate to HTTPAdapter -- Invoke, Health, Checkpoint, Resume
- User-provided name always overrides HTTP-declared name
- Compile-time interface satisfaction verified

### Change Log

- 2026-04-16: Story 13.2 implemented -- LangChain/LangGraph adapter wrapping HTTPAdapter

### File List

- internal/adapter/langchain.go (new)
- internal/adapter/adapter.go (reference -- Adapter interface)
- internal/adapter/http.go (reference -- HTTPAdapter delegation target)
- internal/cli/agent.go (modified -- added langchain type to add-agent command)
- internal/agent/manager.go (reference -- agent registration flow)
