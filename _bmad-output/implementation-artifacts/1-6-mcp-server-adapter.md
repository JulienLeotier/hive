# Story 1.6: MCP Server Adapter

Status: done

## Story

As a user,
I want to register MCP (Model Context Protocol) servers with Hive,
so that I can orchestrate MCP tools as part of my agent workflows.

## Acceptance Criteria

1. **Given** an MCP server running locally or remotely
   **When** a `MCPAdapter` is created with the server URL
   **Then** the adapter connects to the MCP server via HTTP (delegating to the HTTP adapter)

2. **Given** the `MCPAdapter`
   **When** `Declare()` is called and the MCP server responds
   **Then** it retrieves the server's capabilities and overrides the name with the configured agent name

3. **Given** the `MCPAdapter`
   **When** `Declare()` is called and the MCP server is unreachable
   **Then** it falls back to returning generic `"mcp-tool"` capabilities with the configured name

4. **Given** the `MCPAdapter`
   **When** `Invoke()` is called with a task
   **Then** it delegates to the HTTP adapter to invoke the MCP server
   **And** wraps any HTTP errors in a structured `TaskResult` with `"failed"` status

5. **Given** the `MCPAdapter`
   **When** `Health()`, `Checkpoint()`, or `Resume()` is called
   **Then** it delegates directly to the underlying HTTP adapter

6. **Given** the `MCPAdapter`
   **When** compile-time interface compliance is checked
   **Then** `var _ Adapter = (*MCPAdapter)(nil)` compiles successfully

## Tasks / Subtasks

- [x] Task 1: MCP adapter implementation (AC: #1, #2, #3, #4, #5)
  - [x] Create `internal/adapter/mcp.go`
  - [x] `MCPAdapter` struct with `ServerURL`, `Name`, and embedded `http *HTTPAdapter`
  - [x] `NewMCPAdapter(serverURL, name)` constructor — creates internal HTTPAdapter
  - [x] `Declare()` — tries HTTP adapter's Declare, overrides name; falls back to generic `mcp-tool` capabilities on error
  - [x] `Invoke()` — delegates to HTTP adapter, wraps errors in `TaskResult{Status: "failed"}`
  - [x] `Health()` — delegates to HTTP adapter
  - [x] `Checkpoint()` — delegates to HTTP adapter
  - [x] `Resume()` — delegates to HTTP adapter
  - [x] Compile-time check: `var _ Adapter = (*MCPAdapter)(nil)`
- [x] Task 2: MCP adapter tests (AC: #2, #3, #5, #6)
  - [x] Create `internal/adapter/mcp_test.go`
  - [x] `TestMCPAdapterDeclareWithServer` — mock HTTP server returning capabilities, verify name override
  - [x] `TestMCPAdapterDeclareFallback` — unreachable server, verify fallback to generic capabilities
  - [x] `TestMCPAdapterHealthDelegates` — verify delegation to HTTP adapter via mock agent

## Dev Notes

### Architecture Compliance

- **Package:** `internal/adapter/` — co-located with HTTP and Claude Code adapters
- **Composition:** MCP adapter composes the HTTP adapter rather than inheriting — Go's idiomatic delegation pattern
- **Graceful fallback:** `Declare()` never fails — returns generic capabilities when server is unreachable
- **Name override:** Always uses the configured name, not the server-reported name, for consistent identity
- **Error wrapping:** `Invoke()` catches HTTP errors and returns them as `TaskResult{Status: "failed"}` instead of propagating
- **Protocol reuse:** MCP servers are expected to expose the same `/declare`, `/invoke`, `/health` endpoints as standard HTTP agents — the MCP adapter adds name handling and error wrapping
- **Interface:** Implements full `Adapter` interface — compile-time verified
- **Naming:** `MCPAdapter` (PascalCase), `mcp.go` (snake_case file)

### Testing Strategy

- Tests use `httptest.Server` for MCP server simulation
- Fallback test uses unreachable `localhost:1` to verify graceful degradation
- Reuses `newMockAgent()` from `http_test.go` for health delegation test
- 3 tests covering server-connected, fallback, and delegation scenarios

### References

- [Source: architecture.md#Project Structure — internal/adapter/mcp.go]
- [Source: architecture.md#API & Communication Patterns — HTTP/JSON transport]
- [Source: epics.md#Story 1.6]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- MCP adapter composing the HTTP adapter for protocol delegation
- Graceful fallback to generic `mcp-tool` capabilities when server is unreachable
- Name override ensures consistent agent identity regardless of server response
- Error wrapping on Invoke returns structured TaskResult instead of raw errors
- 3 tests covering connected, fallback, and delegation scenarios
- Compile-time interface compliance verified

### Change Log

- 2026-04-16: Story 1.6 implemented — MCP server adapter with HTTP delegation and graceful fallback

### File List

- internal/adapter/mcp.go (new)
- internal/adapter/mcp_test.go (new)
