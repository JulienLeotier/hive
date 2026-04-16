# Story 15.2: NATS Backend Implementation

Status: done

## Story

As a user,
I want to configure NATS as my event bus backend,
so that multiple Hive nodes can share the same event stream.

## Acceptance Criteria

1. **Given** a NATS server is running
   **When** the user sets `event_bus: nats` and `nats_url: nats://localhost:4222` in `hive.yaml`
   **Then** events are published to and subscribed from NATS subjects

2. **Given** NATS is configured as the event bus
   **When** a component calls `Publish()`
   **Then** the event is published to a NATS subject matching the event type (e.g., `hive.task.created`)

3. **Given** NATS is configured as the event bus
   **When** a component calls `Subscribe(prefix)`
   **Then** the subscriber receives events from NATS subjects matching the prefix pattern

4. **Given** NATS is the event bus
   **When** events are published
   **Then** event ordering is maintained per-subject

5. **Given** the NATS backend
   **When** `Close()` is called
   **Then** the NATS connection is drained and closed gracefully

## Tasks / Subtasks

- [x] Task 1: NATS bus struct (AC: #1)
  - [x] Create `NATSBus` struct in `internal/event/nats_bus.go`
  - [x] Accept NATS connection URL and optional credentials
  - [x] Implement `NewNATSBus(url)` constructor that connects to NATS
  - [x] Add compile-time interface check: `var _ EventBus = (*NATSBus)(nil)`
- [x] Task 2: Publish to NATS (AC: #2, #4)
  - [x] Implement `Publish()` -- marshal event payload to JSON and publish to NATS subject
  - [x] Map event types to NATS subjects with `hive.` prefix (e.g., `task.created` -> `hive.task.created`)
  - [x] Return the published event with generated ID
  - [x] Persist events to SQLite in addition to NATS for query support
- [x] Task 3: Subscribe from NATS (AC: #3)
  - [x] Implement `Subscribe()` -- create NATS subscription on matching subject pattern
  - [x] Map prefix patterns to NATS wildcard subjects (e.g., `task.*` -> `hive.task.*`)
  - [x] Deserialize incoming NATS messages to `Event` structs before calling subscriber
- [x] Task 4: Query support (AC: #1)
  - [x] Implement `Query()` -- delegates to SQLite for historical queries (NATS is fire-and-forget)
  - [x] Events are dual-written: NATS for real-time delivery, SQLite for persistence and queries
- [x] Task 5: Graceful close (AC: #5)
  - [x] Implement `Close()` -- drain NATS connection and close
- [x] Task 6: Configuration integration (AC: #1)
  - [x] Add `EventBus` and `NatsURL` fields to `Config` struct
  - [x] Factory function selects backend based on `event_bus` config value
- [x] Task 7: Tests (AC: #1, #2, #3)
  - [x] Test NATS bus implements EventBus interface
  - [x] Test publish and subscribe with embedded NATS server or mock
  - [x] Test subject mapping from event types to NATS subjects

## Dev Notes

### Architecture Compliance

- Implements the `EventBus` interface defined in Story 15.1
- Uses `github.com/nats-io/nats.go` client library -- the only new dependency for NATS support
- Dual-write pattern: events go to both NATS (real-time delivery) and SQLite (persistence/queries)
- NATS subjects use `hive.` prefix to namespace Hive events on shared NATS servers
- Config-driven backend selection via `event_bus` field in `hive.yaml`

### Key Design Decisions

- Dual-write (NATS + SQLite) rather than NATS-only -- this preserves full query capability via the existing SQLite `Query()` implementation while adding real-time multi-node delivery
- NATS is used for fan-out delivery only, not for event storage -- JetStream persistence is not required and would add complexity
- Subject mapping uses dot notation consistent with both NATS conventions and the existing Hive event type format
- The factory function in config returns `EventBus` interface, so callers never know which backend is active
- `Close()` uses NATS drain to ensure in-flight messages are delivered before disconnecting

### Integration Points

- `internal/event/nats_bus.go` -- `NATSBus` struct implementing `EventBus`
- `internal/event/types.go` -- `EventBus` interface (from Story 15.1)
- `internal/event/bus.go` -- existing `Bus` as SQLite delegate for dual-write queries
- `internal/config/config.go` -- `EventBus` and `NatsURL` config fields
- `internal/cli/serve.go` -- factory function selects backend at startup

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 15.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR95, FR96]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- NATSBus implements EventBus interface with nats.go client library
- Dual-write pattern: NATS for real-time multi-node delivery, SQLite for persistence and queries
- Subject mapping: event types prefixed with `hive.` for NATS namespace
- Subscribe maps prefix patterns to NATS wildcard subjects
- Graceful close via NATS drain
- Config fields added: event_bus (embedded/nats), nats_url
- Factory function selects backend at startup

### Change Log

- 2026-04-16: Story 15.2 implemented -- NATS event bus backend with dual-write pattern

### File List

- internal/event/nats_bus.go (new -- NATSBus implementing EventBus)
- internal/event/types.go (reference -- EventBus interface)
- internal/event/bus.go (reference -- SQLite bus used as delegate for queries)
- internal/config/config.go (modified -- added EventBus, NatsURL fields)
- internal/cli/serve.go (modified -- factory function for backend selection)
