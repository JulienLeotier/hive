# Story 2.3: Capability-Based Task Routing

Status: done

## Story

As a user,
I want tasks automatically routed to the right agent based on capabilities,
so that I don't have to manually assign every task.

## Acceptance Criteria

1. **Given** a task with a required type (e.g., `code-review`) **When** the task enters the routing engine **Then** the system matches the task type against registered agent capabilities' `task_types` array
2. **Given** multiple agents with matching capabilities **When** routing occurs **Then** the system selects the first healthy agent (ordered by name) **And** unhealthy/unavailable agents are skipped
3. **Given** a capable and healthy agent is found **When** the task is routed **Then** the agent ID and name are returned for assignment
4. **Given** no capable agent is available **When** routing is attempted **Then** empty strings are returned (task remains pending) **And** no error is raised (task.unroutable handled by caller)
5. **Given** an agent is marked `unavailable` or `degraded` **When** routing occurs **Then** only agents with `health_status = 'healthy'` are considered

## Tasks / Subtasks

- [x] Task 1: Implement Router struct backed by SQL database (AC: #1)
- [x] Task 2: Implement FindCapableAgent querying healthy agents and matching task_types (AC: #1, #2, #5)
- [x] Task 3: Parse AgentCapabilities JSON from agents table capabilities column (AC: #1)
- [x] Task 4: Skip unhealthy agents in routing (AC: #5)
- [x] Task 5: Return empty on no match (AC: #4)
- [x] Task 6: Write tests for routing, type matching, unhealthy skip, no-match (AC: #1-#5)

## Dev Notes

- Router queries `agents` table directly, filtering by `health_status = 'healthy'`
- Agent capabilities stored as JSON in `capabilities` column, unmarshaled to `adapter.AgentCapabilities`
- Routing strategy is simple first-match ordered by name -- future stories may add load balancing
- The router does not assign the task -- it only finds the best agent. Assignment is the caller's responsibility via task.Store.Assign
- Integration with event bus for `task.unroutable` events is handled at the orchestration layer, not in the router itself

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/task/router.go (new) -- Router struct, FindCapableAgent method
- internal/task/router_test.go (new) -- 4 tests covering match, different types, unhealthy skip, no match
