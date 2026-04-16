# Story 8.4: Event Timeline

Status: done

## Story

As a user,
I want a real-time event timeline with filtering,
so that I can debug issues and understand system behavior.

## Acceptance Criteria

1. **Given** events are being published
   **When** the events page is displayed
   **Then** it shows events in reverse chronological order with: type, source, payload preview, timestamp

2. **Given** the events page is displayed
   **When** the user enters a type filter (e.g., "task")
   **Then** only events matching the type prefix are shown

3. **Given** the events page is open
   **When** new events are published in the system
   **Then** they appear in real-time via WebSocket without page refresh

4. **Given** no events exist
   **When** the events page is displayed
   **Then** it shows an empty state message: "No events yet."

## Tasks / Subtasks

- [x] Task 1: Events page component (AC: #1, #2, #3, #4)
  - [x] Create `web/src/routes/events/+page.svelte` with Svelte 5 runes
  - [x] Define `Event` type with id, type, source, payload, created_at fields
  - [x] Implement `loadEvents()` fetching from `/api/v1/events` with optional type filter
  - [x] Implement `connectWS()` for real-time event streaming via WebSocket at `/ws`
  - [x] WebSocket auto-reconnect on close (3-second reconnect delay)
  - [x] New events prepended to list, capped at 100 entries in UI
  - [x] Type filter input with "Filter" button
  - [x] Timeline layout with left border accent, timestamp, type, source, payload preview
  - [x] Empty state message when no events exist
- [x] Task 2: WebSocket protocol integration (AC: #3)
  - [x] WebSocket URL constructed from current page protocol (ws/wss) and host
  - [x] Incoming messages parsed as JSON event objects
  - [x] Events match the same structure as API response: `{id, type, source, payload, created_at}`

## Dev Notes

### Architecture Compliance

- **Svelte 5 runes** — `$state` for events array and filter state, `$effect` for lifecycle
- **WebSocket** — connects to `/ws` endpoint served by `internal/ws/hub.go` for real-time push
- **Hybrid data loading** — initial load via REST API, then real-time updates via WebSocket
- **Auto-reconnect** — WebSocket `onclose` handler reconnects after 3 seconds for resilience

### Key Design Decisions

- Timeline layout (not table) provides a visual chronological flow with left border accent — better for event streams than tabular data
- Events capped at 100 in the UI to prevent memory growth — older events scroll off
- WebSocket reconnect is unconditional on close — handles both server restarts and network interruptions
- Type filter sends a new API request (server-side filtering) for initial load, then WebSocket pushes all events (client must filter if needed)
- Payload shown in truncated `<code>` element (max-width 300px) with ellipsis for long payloads

### Integration Points

- `web/src/routes/events/+page.svelte` — event timeline page with WebSocket
- `internal/api/server.go` — `handleListEvents` for initial event load
- `internal/ws/hub.go` — `HandleWS` for WebSocket upgrade, `Broadcast` for real-time push
- `internal/event/bus.go` — `Query()` for filtered event retrieval

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR59, FR61]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Event timeline page shows events in reverse chronological order with type, source, payload, timestamp
- Real-time updates via WebSocket connection to `/ws` endpoint
- Auto-reconnect on WebSocket close with 3-second delay
- Type prefix filter for narrowing event view
- Timeline UI with left border accent and truncated payload preview
- Events capped at 100 entries in browser memory

### Change Log

- 2026-04-16: Story 8.4 implemented — real-time event timeline with WebSocket push and type filtering

### File List

- web/src/routes/events/+page.svelte (new)
- internal/ws/hub.go (reference — WebSocket hub for real-time broadcast)
- internal/api/server.go (reference — handleListEvents endpoint)
- internal/event/bus.go (reference — Query for initial load)
