# Story 22.2: Multi-Node Clustering

Status: done

## Story

As a user,
I want to run multiple Hive nodes for high availability,
so that my hive survives node failures.

## Acceptance Criteria

1. **Given** multiple Hive nodes connected via NATS cluster
   **When** an agent registers on node A
   **Then** the registration is replicated to node B via NATS events

2. **Given** a multi-node cluster
   **When** a task is created on any node
   **Then** the task can be routed to agents on any node

3. **Given** a node failure
   **When** node A goes down
   **Then** node B continues operating with all remaining agents

4. **Given** a recovered node
   **When** node A comes back online
   **Then** it synchronizes state from the shared PostgreSQL database

## Tasks / Subtasks

- [x] Task 1: Node identity and discovery (AC: #1)
  - [x] Define `Node` struct with ID (ULID), address, status, last seen
  - [x] Each node generates a unique ID on startup
  - [x] Nodes announce themselves on NATS `hive.cluster.join` subject
  - [x] Implement node heartbeat on `hive.cluster.heartbeat` subject (every 5s)
  - [x] Detect node departure after 3 missed heartbeats
- [x] Task 2: ClusterManager core (AC: #1, #2)
  - [x] Create `ClusterManager` in `internal/cluster/node.go`
  - [x] Implement `Join()` -- announce node and start listening for peers
  - [x] Implement `Leave()` -- graceful departure announcement
  - [x] Track known nodes with health status
  - [x] Implement `Nodes()` -- returns list of known cluster members
- [x] Task 3: Event replication via NATS (AC: #1, #2)
  - [x] Replicate agent registration events across nodes via NATS
  - [x] Replicate task creation and assignment events
  - [x] Replicate workflow state changes
  - [x] Use NATS JetStream for at-least-once delivery guarantee
  - [x] Deduplicate events using event ID (ULID)
- [x] Task 4: Shared state via PostgreSQL (AC: #2, #4)
  - [x] All nodes read/write to shared PostgreSQL database (Story 22.1)
  - [x] PostgreSQL is the source of truth; NATS provides real-time notification
  - [x] On node recovery, state is loaded from PostgreSQL -- no manual sync needed
  - [x] Optimistic concurrency via version columns on key tables
- [x] Task 5: Node failure handling (AC: #3)
  - [x] Detect node failure via missed heartbeats
  - [x] Mark agents on failed node as degraded
  - [x] Emit `cluster.node.departed` event
  - [x] Tasks assigned to agents on failed node trigger failover (reuses Epic 5 logic)
  - [x] No data loss: all state is in PostgreSQL
- [x] Task 6: CLI cluster commands (AC: #1, #3, #4)
  - [x] Implement `hive cluster status` -- show all nodes with health
  - [x] Show per-node: ID, address, agent count, uptime, status
  - [x] Support `--json` output
- [x] Task 7: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test node join and discovery via NATS
  - [x] Test heartbeat and failure detection
  - [x] Test event replication across nodes
  - [x] Test state recovery from PostgreSQL on node restart
  - [x] Test agent failover on node departure

## Dev Notes

### Architecture Compliance

- `internal/cluster/node.go` manages node lifecycle and cluster membership
- NATS (from Epic 15) provides the inter-node communication layer
- PostgreSQL (from Story 22.1) provides shared persistent state
- Architecture is shared-nothing at the node level -- nodes communicate only via NATS and PostgreSQL
- Uses `slog` with node_id in structured log fields for per-node debugging

### Key Design Decisions

- NATS is the notification layer; PostgreSQL is the source of truth -- this simplifies recovery (just restart and read from DB)
- Event deduplication uses event ULIDs -- prevents double-processing of replicated events
- JetStream provides at-least-once delivery for critical cluster events
- Node heartbeat interval (5s) and failure threshold (3 missed = 15s) balance responsiveness with false-positive avoidance
- Optimistic concurrency (version columns) prevents lost updates when multiple nodes modify the same record

### Integration Points

- internal/cluster/node.go (modified -- ClusterManager, Node, join/leave/heartbeat, failure detection)
- internal/cluster/node_test.go (new -- clustering tests with mock NATS)
- internal/event/bus.go (modified -- NATS-backed event replication for cluster mode)
- internal/resilience/circuit_breaker.go (reference -- failover on node departure)
- internal/cli/cluster.go (new -- `hive cluster status` command)
- internal/config/config.go (modified -- cluster mode, node_id config fields)
- internal/storage/postgres.go (reference -- shared PostgreSQL for state sync)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 22 - Story 22.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR126, FR127, FR128]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- ClusterManager handles node join/leave/heartbeat with NATS-based discovery
- Event replication via NATS JetStream with ULID-based deduplication
- PostgreSQL is source of truth; NATS provides real-time notification layer
- Node failure detected after 3 missed heartbeats (15s); agents on failed node marked degraded
- Automatic state recovery from PostgreSQL on node restart
- `hive cluster status` CLI command shows all nodes with health and agent counts

### Change Log

- 2026-04-16: Story 22.2 implemented -- multi-node clustering with NATS discovery and PostgreSQL shared state

### File List

- internal/cluster/node.go (modified -- ClusterManager, Node struct, join/leave/heartbeat, failure detection)
- internal/cluster/node_test.go (new -- cluster lifecycle, heartbeat, failure, recovery tests)
- internal/event/bus.go (modified -- NATS event replication for cluster mode)
- internal/cli/cluster.go (new -- `hive cluster status` command)
- internal/config/config.go (modified -- cluster mode, node_id config)
- internal/storage/postgres.go (reference -- shared state for multi-node)
