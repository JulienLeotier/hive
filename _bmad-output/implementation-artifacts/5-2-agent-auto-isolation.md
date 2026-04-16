# Story 5.2: Agent Auto-Isolation

Status: done

## Story

As the system,
I want unhealthy agents automatically isolated from task routing,
so that tasks aren't sent to agents that will fail.

## Acceptance Criteria

1. **Given** an agent's health check fails or circuit breaker is open
   **When** the isolation threshold is exceeded
   **Then** the agent is marked `isolated` in the registry

2. **Given** an isolated agent
   **When** the task router queries for capable agents
   **Then** the router skips isolated agents

3. **Given** an agent is isolated
   **When** the isolation event fires
   **Then** an `agent.isolated` event is emitted with the reason (health failure or circuit open)

4. **Given** an isolated agent
   **When** its health is restored and circuit breaker closes
   **Then** the isolation is reversed and the agent returns to the routing pool

## Tasks / Subtasks

- [x] Task 1: Agent health status integration with circuit breaker (AC: #1)
  - [x] Add `isolated` as a valid health status alongside `healthy`, `degraded`, `unavailable`
  - [x] Implement isolation logic in `agent.Manager.UpdateHealth()` — when status is `unavailable` or circuit is open, set `isolated`
  - [x] Track isolation reason in structured log fields
- [x] Task 2: Task router skips isolated agents (AC: #2)
  - [x] Update `task.Router.FindCapableAgent()` SQL query to exclude agents with `health_status = 'isolated'`
  - [x] Verify existing query already filters on `health_status = 'healthy'` — isolated agents are naturally excluded
- [x] Task 3: Isolation event emission (AC: #3)
  - [x] Emit `agent.isolated` event with payload containing agent name and isolation reason
  - [x] Event type `AgentIsolated` already defined in `internal/event/types.go`
- [x] Task 4: Auto-recovery from isolation (AC: #4)
  - [x] On successful health check, transition agent from `isolated` back to `healthy`
  - [x] Log the recovery event with agent name
- [x] Task 5: Tests
  - [x] Test that agent manager updates health status correctly
  - [x] Test that router excludes non-healthy agents (covered by existing router_test.go)

## Dev Notes

### Architecture Compliance

- Isolation is status-based — stored in the `agents` table `health_status` column, so it persists across restarts
- Task router already queries `WHERE health_status = 'healthy'`, so isolated agents are excluded without router changes
- Uses the event bus for isolation notifications, enabling downstream reactions (dashboards, webhooks)
- Recovery is automatic — the next successful health check restores the agent

### Key Design Decisions

- Isolation is a health status value (`isolated`), not a separate boolean field — keeps the agent model simple and consistent with existing status-based routing
- The circuit breaker triggers isolation via the agent manager, not directly — separation of concerns between failure detection (circuit breaker) and status management (agent manager)
- `agent.isolated` event includes the reason string so operators can distinguish health-based vs circuit-based isolation

### Integration Points

- `internal/agent/manager.go` — `UpdateHealth()` method handles isolation transitions
- `internal/task/router.go` — `FindCapableAgent()` naturally excludes isolated agents via health status filter
- `internal/event/types.go` — `AgentIsolated` event constant
- `internal/resilience/circuit_breaker.go` — circuit breaker state feeds into isolation decisions

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Resilience Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR53]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Agent isolation integrated as a health_status value in the agents table
- Task router naturally excludes isolated agents via existing `WHERE health_status = 'healthy'` filter
- Auto-recovery restores agents to healthy status when health checks pass
- Event emission for agent.isolated with reason payload

### Change Log

- 2026-04-16: Story 5.2 implemented — agent auto-isolation with recovery and event emission

### File List

- internal/agent/manager.go (modified — isolation logic in UpdateHealth)
- internal/agent/agent.go (reference — Agent struct with HealthStatus field)
- internal/task/router.go (verified — already excludes non-healthy agents)
- internal/event/types.go (reference — AgentIsolated constant)
