# Story 2.1: Event Bus & Persistence

Status: done

## Story

As a developer,
I want an in-process event bus that persists all events to SQLite,
so that the system has reliable, ordered event delivery with replay capability.

## Acceptance Criteria

1. **Given** the event bus is initialized on application startup **When** any component calls `eventBus.Publish(event)` **Then** the event is persisted to the `events` table before delivery to subscribers **And** events are delivered to matching subscribers within 200ms p95 (NFR1)
2. **Given** events are being published **When** the event bus delivers events **Then** event ordering is strictly maintained via auto-increment ID (NFR7)
3. **Given** a subscriber registers for a type prefix **When** events matching that prefix are published **Then** subscribers receive all matching events (e.g., `task.*` matches `task.created`) **And** a wildcard `*` subscription receives all events
4. **Given** agents emit custom events via the adapter protocol **When** events are published **Then** they follow the standard Event struct format with type, source, payload, and timestamp (FR16, FR19-FR22)
5. **Given** persisted events exist **When** the Query method is called with filter options **Then** events can be filtered by type prefix, source, time range, and limit **And** results are ordered chronologically by ID

## Tasks / Subtasks

- [x] Task 1: Define Event types and Subscriber type (AC: #4)
- [x] Task 2: Implement Bus struct with SQLite-backed Publish (AC: #1, #2)
- [x] Task 3: Implement Subscribe with prefix matching and wildcard support (AC: #3)
- [x] Task 4: Implement Query with type/source/since/limit filters (AC: #5)
- [x] Task 5: Add panic recovery in subscriber delivery (AC: #1)
- [x] Task 6: Define all event type constants (task.*, agent.*, workflow.*) (AC: #4)
- [x] Task 7: Write comprehensive tests for publish, subscribe, ordering, query (AC: #1-#5)

## Dev Notes

- Event bus uses synchronous in-process delivery via Go function calls (not channels) for simplicity and guaranteed ordering
- All events are persisted to SQLite `events` table with `INSERT` before subscriber delivery
- Subscriber matching uses `strings.HasPrefix` for prefix-based routing
- `safeCall` wrapper recovers from panics in subscriber callbacks to prevent one bad subscriber from crashing delivery
- Query supports LIKE-based type prefix matching with proper escaping of SQL wildcards
- Event type constants use dot notation per architecture spec: `task.created`, `agent.registered`, `workflow.started`
- ULID is not used for event IDs -- auto-increment INTEGER provides strict ordering guarantee (NFR7)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/event/types.go (new) -- Event struct, Subscriber type, event type constants
- internal/event/bus.go (new) -- Bus struct with Publish, Subscribe, Query, deliver, safeCall
- internal/event/bus_test.go (new) -- 8 tests covering publish/subscribe/ordering/query
