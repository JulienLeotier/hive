# Story 15.3: NATS Connection Management

Status: done

## Story

As the system,
I want robust NATS connection handling,
so that the event bus recovers from network issues.

## Acceptance Criteria

1. **Given** a NATS connection is established
   **When** the connection drops
   **Then** the system automatically reconnects with exponential backoff

2. **Given** reconnection is in progress
   **When** events are published
   **Then** they are queued in the NATS client buffer and delivered after reconnection

3. **Given** the NATS connection state changes
   **When** `hive status` is run
   **Then** the connection state is reported (connected, reconnecting, disconnected)

4. **Given** the NATS server is unreachable at startup
   **When** the system starts with `event_bus: nats`
   **Then** it retries connection with exponential backoff and logs each attempt

5. **Given** a stable NATS connection
   **When** the system is running normally
   **Then** no reconnection overhead is incurred

## Tasks / Subtasks

- [x] Task 1: Connection options configuration (AC: #1, #4)
  - [x] Configure NATS client with reconnect options: max reconnects, reconnect wait, reconnect jitter
  - [x] Set exponential backoff: initial 1s, max 60s, with jitter
  - [x] Set reconnect buffer size for in-flight message buffering
  - [x] Configure connection name for identification in NATS monitoring
- [x] Task 2: Connection event handlers (AC: #1, #3)
  - [x] Register `DisconnectedErrHandler` -- logs disconnection with error reason
  - [x] Register `ReconnectedHandler` -- logs successful reconnection with server URL
  - [x] Register `ClosedHandler` -- logs final connection close
  - [x] Track connection state in `NATSBus` struct for status reporting
- [x] Task 3: Status integration (AC: #3)
  - [x] Add `ConnectionState()` method to `NATSBus` returning current state string
  - [x] Integrate with `hive status` to display NATS connection state
  - [x] Report: connected, reconnecting, disconnected, closed
- [x] Task 4: Startup retry (AC: #4)
  - [x] Configure NATS client with `nats.RetryOnFailedConnect(true)` for async initial connect
  - [x] Log connection attempts with slog at WARN level
  - [x] Set maximum initial connect time before failing hard
- [x] Task 5: Graceful shutdown (AC: #5)
  - [x] Drain pending messages before closing connection
  - [x] Set drain timeout to prevent indefinite shutdown blocking

## Dev Notes

### Architecture Compliance

- NATS client connection management is built into the `NATSBus` struct from Story 15.2
- Uses `nats.go` client library's built-in reconnection support rather than custom retry logic
- Connection state logged via `slog` at appropriate levels (INFO for connect, WARN for disconnect, ERROR for failed)
- Status reporting integrates with existing `hive status` command pattern

### Key Design Decisions

- Leverages `nats.go` client's native reconnection rather than implementing custom retry logic -- the NATS client handles buffering, reconnection, and subscription restoration automatically
- Exponential backoff starts at 1s and caps at 60s with jitter to prevent thundering herd on NATS server recovery
- `RetryOnFailedConnect(true)` allows the system to start even if NATS is temporarily unavailable -- events fall back to SQLite-only during disconnection
- Connection state is tracked internally via NATS callback handlers, making `ConnectionState()` a lightweight read with no network call
- Drain timeout is set to 5s to prevent shutdown from hanging on undeliverable messages

### Integration Points

- `internal/event/nats_bus.go` -- connection options and handlers in `NewNATSBus()`
- `internal/event/nats_bus.go` -- `ConnectionState()` method for status reporting
- `internal/cli/agent.go` -- `hive status` displays NATS connection state when backend is NATS
- `internal/config/config.go` -- NATS connection configuration fields

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 15.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- NATS connection configured with exponential backoff (1s-60s) and jitter
- Disconnect/Reconnect/Close handlers log state changes via slog
- ConnectionState() method returns current state for hive status integration
- RetryOnFailedConnect enables startup even when NATS is temporarily unavailable
- Graceful shutdown via drain with 5s timeout
- Reconnect buffer configured for in-flight message preservation

### Change Log

- 2026-04-16: Story 15.3 implemented -- robust NATS connection management with auto-reconnection

### File List

- internal/event/nats_bus.go (modified -- connection options, handlers, ConnectionState method)
- internal/cli/agent.go (modified -- hive status shows NATS connection state)
- internal/config/config.go (reference -- NATS configuration fields)
