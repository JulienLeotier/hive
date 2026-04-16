# Story 19.3: Cross-Hive Task Routing

Status: done

## Story

As the system,
I want to route tasks to federated agents when local agents can't handle them,
so that the network effect increases available capabilities.

## Acceptance Criteria

1. **Given** a task requires capability "data-analysis" and no local agent has it
   **When** a federated hive has an agent with that capability
   **Then** the task is proxied to the federated hive via the federation protocol

2. **Given** a task is routed to a federated hive
   **When** the task completes
   **Then** results are returned to the originating hive

3. **Given** a cross-hive task routing
   **When** the task is dispatched
   **Then** a `task.federated` event records the cross-hive routing with peer URL

4. **Given** both local and remote agents have the required capability
   **When** the router selects an agent
   **Then** local agents are preferred over federated agents

## Tasks / Subtasks

- [x] Task 1: Federated task proxy (AC: #1, #2)
  - [x] Implement `ProxyTask(peerURL, task)` in federation protocol
  - [x] Serialize task payload for cross-hive transmission over mTLS
  - [x] Implement task result callback -- receive result from remote hive
  - [x] Handle timeouts on cross-hive task execution (configurable, default 5m)
  - [x] Map remote task status to local task state machine
- [x] Task 2: Router integration (AC: #1, #4)
  - [x] Modify task router to check federated capabilities when no local agent matches
  - [x] Implement local-first preference: always try local agents before federated
  - [x] Select best federated peer when multiple peers have the capability
  - [x] Create `FederatedRouting` decision log entry for observability
- [x] Task 3: Result handling (AC: #2)
  - [x] Receive proxied task results via federation endpoint
  - [x] Update local task record with remote execution results
  - [x] Transition task through normal state machine (running -> completed/failed)
  - [x] Pass results to downstream tasks in the originating workflow
- [x] Task 4: Event integration (AC: #3)
  - [x] Emit `task.federated` event with: task ID, peer URL, remote capability, routing reason
  - [x] Emit `task.federated.completed` when remote result is received
  - [x] Emit `task.federated.failed` on remote execution failure or timeout
- [x] Task 5: Error handling and resilience (AC: #1, #2)
  - [x] Handle federation link going down during task execution
  - [x] Retry task on another federated peer if available
  - [x] Fall back to `task.unroutable` if no federated peer can handle it
  - [x] Circuit breaker integration for frequently failing federated peers
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test task proxying to federated peer
  - [x] Test result callback and local state update
  - [x] Test local-first routing preference
  - [x] Test timeout handling on cross-hive tasks
  - [x] Test fallback when federation link drops mid-execution
  - [x] Test event emission for federated routing

## Dev Notes

### Architecture Compliance

- Cross-hive task routing extends the existing task router with a federated fallback path
- Task proxying uses the same mTLS transport established in Story 19.1
- Results follow the normal task state machine -- downstream tasks see no difference between local and federated execution
- Circuit breakers from Epic 5 are applied to federated peers to prevent routing to unreliable partners
- Uses `slog` for structured logging of all cross-hive routing decisions

### Key Design Decisions

- Local-first is the default and only routing preference for federation -- minimizes latency and data exposure
- Cross-hive tasks have a longer default timeout (5m vs 30s for local) to account for network latency
- Task payload is the same format locally and remotely -- no translation layer needed
- Failed federated tasks retry on other peers before declaring unroutable
- Federation routing creates an explicit decision log entry for audit trail

### Integration Points

- internal/federation/protocol.go (modified -- ProxyTask, result callback, timeout handling)
- internal/federation/protocol_test.go (modified -- proxying, result handling, resilience tests)
- internal/task/router.go (modified -- federated fallback path, local-first preference)
- internal/task/router_test.go (modified -- federated routing tests)
- internal/event/types.go (modified -- federated task event constants)
- internal/resilience/circuit_breaker.go (reference -- applied to federated peers)
- internal/api/server.go (modified -- federation task proxy and result callback endpoints)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 19 - Story 19.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR112]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Cross-hive task proxying over mTLS with configurable timeout (default 5m)
- Task router extended with federated fallback: local-first, then best federated peer
- Results returned via callback and integrated into normal task state machine
- Federation events emitted for routing, completion, and failure
- Circuit breakers applied to federated peers for resilience
- Retry on alternate peers before declaring task unroutable

### Change Log

- 2026-04-16: Story 19.3 implemented -- cross-hive task routing with local-first preference and resilient proxying

### File List

- internal/federation/protocol.go (modified -- ProxyTask, result callback, timeout, retry)
- internal/federation/protocol_test.go (modified -- cross-hive proxying, result handling, resilience tests)
- internal/task/router.go (modified -- federated fallback path, local-first routing)
- internal/task/router_test.go (modified -- federated routing preference and fallback tests)
- internal/event/types.go (modified -- federated task event constants)
- internal/api/server.go (modified -- federation task proxy and callback endpoints)
