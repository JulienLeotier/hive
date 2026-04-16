# Story 8.5: WebSocket Hub

Status: done

## Story

As a developer,
I want a WebSocket hub that broadcasts events to connected dashboard clients,
so that the dashboard updates in real-time.

## Acceptance Criteria

1. **Given** the API server is running with WebSocket support
   **When** a dashboard client connects to `/ws`
   **Then** it receives all events as they are published

2. **Given** an event is published via the event bus
   **When** the WebSocket hub broadcasts it
   **Then** event delivery to WebSocket is under 100ms from publication (NFR25)

3. **Given** a client connection becomes stale or fails
   **When** the hub detects the failure via write error
   **Then** the stale connection is closed and removed from the client registry

4. **Given** multiple dashboard clients are connected
   **When** an event is broadcast
   **Then** all connected clients receive the event concurrently

## Tasks / Subtasks

- [x] Task 1: WebSocket Hub struct and lifecycle (AC: #1, #3, #4)
  - [x] Create `internal/ws/hub.go` with `Hub` struct containing mutex-protected client map
  - [x] Define `client` struct wrapping `websocket.Conn` with per-client write mutex
  - [x] Implement `NewHub()` constructor
  - [x] Implement `HandleWS()` — upgrades HTTP to WebSocket, registers client, starts read loop for disconnect detection
  - [x] Implement `Broadcast()` — sends event JSON to all clients, removes failed connections
  - [x] Implement `ClientCount()` — returns number of connected clients
- [x] Task 2: WebSocket upgrader and origin checking (AC: #1)
  - [x] Configure `gorilla/websocket.Upgrader` with custom `CheckOrigin`
  - [x] Allow localhost and 127.0.0.1 origins by default
  - [x] Support configurable `AllowedOrigins` for production deployments
- [x] Task 3: Event broadcast integration (AC: #2)
  - [x] Hub's `Broadcast()` serializes events as JSON matching the event structure: `{id, type, source, payload, created_at}`
  - [x] Failed writes close the connection and remove from client registry
  - [x] Per-client write mutex prevents concurrent writes to the same connection

## Dev Notes

### Architecture Compliance

- **gorilla/websocket** — mature WebSocket library for Go, handles protocol upgrade and frame management
- **Thread-safe** — `sync.Mutex` on client map, per-client `sync.Mutex` on write operations prevents data races
- **Minimal coupling** — Hub depends only on `event.Event` struct, no dependency on event bus or API server
- **Client lifecycle** — read loop goroutine detects disconnects, cleanup removes client from map and closes connection

### Key Design Decisions

- Hub uses a `map[*client]bool` pattern rather than a slice — O(1) removal of stale clients
- Per-client write mutex (`wmu`) prevents concurrent `WriteJSON` calls which would cause gorilla/websocket panics
- Read loop exists solely for disconnect detection — the hub is unidirectional (server → client)
- Failed broadcasts are collected in a `failed` slice, then removed after iteration to avoid map mutation during iteration
- Origin checking allows localhost by default for development; `AllowedOrigins` package variable for production configuration

### Integration Points

- `internal/ws/hub.go` — WebSocket hub implementation
- `internal/cli/serve.go` — mounts WebSocket handler at `/ws` route
- `internal/event/types.go` — `Event` struct used for broadcast serialization
- `web/src/routes/events/+page.svelte` — client-side WebSocket consumer

### References

- [Source: _bmad-output/planning-artifacts/prd.md#NFR25]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.5]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- WebSocket hub manages concurrent client connections with thread-safe map
- Broadcast sends event JSON to all clients, automatically cleans up stale connections
- Per-client write mutex prevents concurrent write panics
- Origin checking allows localhost by default with configurable AllowedOrigins
- Read loop goroutine detects client disconnects and triggers cleanup
- ClientCount helper for monitoring connected dashboard sessions

### Change Log

- 2026-04-16: Story 8.5 implemented — WebSocket hub with broadcast, client management, and origin checking

### File List

- internal/ws/hub.go (new)
- internal/cli/serve.go (modified — mounts WebSocket handler at /ws)
- internal/event/types.go (reference — Event struct for broadcast)
