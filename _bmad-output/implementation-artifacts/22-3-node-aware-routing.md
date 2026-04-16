# Story 22.3: Node-Aware Routing

Status: done

## Story

As the system,
I want task routing to prefer local agents over remote ones,
so that latency is minimized.

## Acceptance Criteria

1. **Given** agents exist on multiple nodes
   **When** a task is routed
   **Then** the router prefers agents on the same node

2. **Given** no local agent has the required capability
   **When** a task is routed
   **Then** the router falls back to remote agents on other nodes

3. **Given** routing preference is configurable
   **When** `routing: best-fit` is set in `hive.yaml`
   **Then** the router selects the best agent regardless of node location

4. **Given** routing preference is configurable
   **When** `routing: local-first` is set (default)
   **Then** the router prefers local agents, falling back to remote

## Tasks / Subtasks

- [x] Task 1: Node-aware agent registry (AC: #1, #2)
  - [x] Track which node each agent is registered on (node_id column in agents table)
  - [x] ClusterManager provides current node ID for local agent identification
  - [x] Agent manager exposes `ListByNode(nodeID)` and `ListLocal()` methods
- [x] Task 2: Local-first routing strategy (AC: #1, #4)
  - [x] Implement `LocalFirstRouter` that extends existing capability-based routing
  - [x] Filter candidates to same-node agents first
  - [x] If local candidates exist, select best match among them
  - [x] If no local candidates, expand to all cluster agents
  - [x] Log routing decision: "local" or "remote" with latency annotation
- [x] Task 3: Best-fit routing strategy (AC: #3)
  - [x] Implement `BestFitRouter` that considers all agents regardless of node
  - [x] Selection criteria: capability match, health, load, trust -- node location is not a factor
  - [x] Used when latency is less important than optimal agent selection
- [x] Task 4: Configuration (AC: #3, #4)
  - [x] Add `routing` field to config: `local-first` (default) or `best-fit`
  - [x] Allow per-workflow routing override in workflow YAML
  - [x] Validate routing configuration in `hive validate`
- [x] Task 5: Latency tracking (AC: #1)
  - [x] Track task dispatch latency per agent (local vs. remote)
  - [x] Include latency data in metrics endpoint
  - [x] Log routing decisions with estimated latency impact
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test local-first routing prefers same-node agents
  - [x] Test local-first routing falls back to remote agents
  - [x] Test best-fit routing ignores node location
  - [x] Test per-workflow routing override
  - [x] Test latency tracking accuracy

## Dev Notes

### Architecture Compliance

- Node-aware routing extends the existing task router with location awareness
- Routing strategy is pluggable: local-first and best-fit implement the same interface
- Local-first is the default for single-node backward compatibility (all agents are "local")
- Uses `slog` for structured logging of routing decisions with node context

### Key Design Decisions

- Local-first is the default because most deployments will benefit from lower latency
- Best-fit is available for workloads where agent quality matters more than latency
- Node location is stored in the agents table (node_id) for query efficiency
- Per-workflow override allows mixed strategies in the same cluster
- Latency tracking provides data for future optimization recommendations (Epic 20 integration)

### Integration Points

- internal/task/router.go (modified -- LocalFirstRouter, BestFitRouter, node-aware candidate filtering)
- internal/task/router_test.go (modified -- node-aware routing tests)
- internal/cluster/node.go (reference -- CurrentNodeID for local agent identification)
- internal/agent/manager.go (modified -- ListByNode, ListLocal methods)
- internal/agent/manager_test.go (modified -- node-scoped listing tests)
- internal/config/config.go (modified -- routing config field)
- internal/workflow/parser.go (modified -- per-workflow routing override parsing)
- internal/api/server.go (reference -- latency data in metrics endpoint)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 22 - Story 22.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR128]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Node-aware routing with LocalFirstRouter (default) and BestFitRouter strategies
- Local-first prefers same-node agents then falls back to remote; best-fit ignores node
- Agent manager extended with ListByNode and ListLocal for efficient node-scoped queries
- Per-workflow routing override supported in workflow YAML
- Latency tracking on task dispatch for metrics and optimization data
- Backward compatible: single-node deployments treat all agents as local

### Change Log

- 2026-04-16: Story 22.3 implemented -- node-aware routing with local-first and best-fit strategies

### File List

- internal/task/router.go (modified -- LocalFirstRouter, BestFitRouter, node-aware filtering)
- internal/task/router_test.go (modified -- node-aware routing preference and fallback tests)
- internal/cluster/node.go (reference -- CurrentNodeID)
- internal/agent/manager.go (modified -- ListByNode, ListLocal methods, node_id tracking)
- internal/agent/manager_test.go (modified -- node-scoped listing tests)
- internal/config/config.go (modified -- routing strategy config)
- internal/workflow/parser.go (modified -- per-workflow routing override)
