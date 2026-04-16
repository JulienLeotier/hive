# Story 13.3: AutoGen Adapter

Status: done

## Story

As a user,
I want to register Microsoft AutoGen agents with Hive,
so that I can include AutoGen conversations in multi-framework workflows.

## Acceptance Criteria

1. **Given** an AutoGen agent exposed via HTTP
   **When** the user runs `hive add-agent --type autogen --url http://localhost:8001`
   **Then** the adapter connects and maps AutoGen agent capabilities

2. **Given** a registered AutoGen agent
   **When** `Declare()` is called
   **Then** it attempts to retrieve capabilities from the AutoGen HTTP endpoint
   **And** falls back to a default `autogen-agent` task type if the endpoint is unavailable

3. **Given** a registered AutoGen agent
   **When** a task is routed to it
   **Then** tasks invoke AutoGen conversations via HTTP
   **And** results are returned through the standard Hive protocol

4. **Given** an AutoGen adapter
   **When** `Health()` is called
   **Then** it delegates to the underlying HTTP adapter's health check

5. **Given** an AutoGen adapter
   **When** `Checkpoint()` or `Resume()` is called
   **Then** it delegates to the underlying HTTP adapter

## Tasks / Subtasks

- [x] Task 1: AutoGen adapter struct (AC: #1, #2)
  - [x] Create `AutoGenAdapter` struct wrapping an `HTTPAdapter` and `Name` field
  - [x] Implement `NewAutoGenAdapter(baseURL, name)` constructor that creates inner HTTPAdapter
  - [x] Implement `Declare()` -- attempts HTTP declare, falls back to `autogen-agent` default
  - [x] Verify compile-time interface satisfaction with `var _ Adapter = (*AutoGenAdapter)(nil)`
- [x] Task 2: HTTP delegation (AC: #3, #4, #5)
  - [x] Implement `Invoke()` delegating to `http.Invoke()` for AutoGen API calls
  - [x] Implement `Health()` delegating to `http.Health()`
  - [x] Implement `Checkpoint()` delegating to `http.Checkpoint()`
  - [x] Implement `Resume()` delegating to `http.Resume()`
- [x] Task 3: Name override (AC: #2)
  - [x] Override the name from HTTP declare response with the user-provided name
  - [x] Ensure capabilities struct always carries the correct agent name

## Dev Notes

### Architecture Compliance

- Implements the `Adapter` interface from `internal/adapter/adapter.go`
- Delegates all protocol operations to the existing `HTTPAdapter` -- same pattern as LangChainAdapter
- AutoGen agents are expected to expose a standard HTTP API for conversation invocation
- Graceful fallback when AutoGen does not expose a `/declare` endpoint

### Key Design Decisions

- Follows the exact same composition pattern as `LangChainAdapter` -- wraps `HTTPAdapter` for transport
- AutoGen's multi-agent conversation model maps to a single Hive task invocation; the internal AutoGen conversation is opaque to the Hive orchestrator
- Default task type is `autogen-agent` to distinguish from other HTTP-based agents in capability routing
- The adapter does not attempt to manage AutoGen's internal agent topology -- it treats the AutoGen ensemble as a single unit

### Integration Points

- `internal/adapter/adapter.go` -- implements `Adapter` interface
- `internal/adapter/http.go` -- delegates all transport to `HTTPAdapter`
- `internal/cli/agent.go` -- `hive add-agent --type autogen` creates this adapter
- `internal/agent/manager.go` -- stores agent record after `Declare()` call

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 13.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR86, FR88]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- AutoGenAdapter wraps HTTPAdapter via composition for AutoGen HTTP API support
- Declare falls back to autogen-agent task type when endpoint unavailable
- All protocol methods delegate to HTTPAdapter -- Invoke, Health, Checkpoint, Resume
- User-provided name always overrides HTTP-declared name
- Compile-time interface satisfaction verified

### Change Log

- 2026-04-16: Story 13.3 implemented -- AutoGen adapter wrapping HTTPAdapter

### File List

- internal/adapter/autogen.go (new)
- internal/adapter/adapter.go (reference -- Adapter interface)
- internal/adapter/http.go (reference -- HTTPAdapter delegation target)
- internal/cli/agent.go (modified -- added autogen type to add-agent command)
- internal/agent/manager.go (reference -- agent registration flow)
