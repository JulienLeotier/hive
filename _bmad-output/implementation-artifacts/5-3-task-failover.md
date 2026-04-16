# Story 5.3: Task Failover

Status: done

## Story

As a user,
I want failed tasks automatically rerouted to healthy agents,
so that work completes despite individual agent failures.

## Acceptance Criteria

1. **Given** a task fails due to agent unavailability (not a business logic error)
   **When** failover triggers
   **Then** the system finds another capable, healthy agent

2. **Given** a failover is initiated
   **When** a replacement agent is found
   **Then** the task is reassigned with its last checkpoint (if available)
   **And** the new agent resumes from checkpoint or restarts the task

3. **Given** a failover occurs
   **When** the reassignment completes
   **Then** a `task.failover` event records the original agent, new agent, and reason

4. **Given** no capable healthy agent is available for failover
   **When** the failover attempt runs
   **Then** the task remains in failed state with a clear error message

## Tasks / Subtasks

- [x] Task 1: Failover logic in task store (AC: #1, #2, #4)
  - [x] Add `Reassign()` method to task store — resets task to `pending` state with original checkpoint preserved
  - [x] Integrate with `task.Router.FindCapableAgent()` to find a replacement agent
  - [x] Handle case where no replacement is available — task stays failed with descriptive error
- [x] Task 2: Checkpoint-aware reassignment (AC: #2)
  - [x] On failover, check if task has a checkpoint in the `checkpoint` column
  - [x] If checkpoint exists, pass it to the new agent's `/resume` endpoint
  - [x] If no checkpoint, restart the task from scratch with original input
- [x] Task 3: Failover event emission (AC: #3)
  - [x] Emit `task.failover` event with payload: task_id, original_agent, new_agent, reason
  - [x] Event type `TaskFailover` already defined in `internal/event/types.go`
- [x] Task 4: Integration with circuit breaker (AC: #1)
  - [x] When circuit breaker opens for an agent, trigger failover for that agent's running tasks
  - [x] Distinguish infrastructure failures (triggers failover) from business logic errors (no failover)
- [x] Task 5: Tests
  - [x] Test task reassignment resets status to pending
  - [x] Test failover event contains correct agent information
  - [x] Test checkpoint preservation during failover

## Dev Notes

### Architecture Compliance

- Failover is triggered by infrastructure failures (connection errors, timeouts, circuit open) — not by business logic errors returned by the agent
- Task checkpoint column in SQLite preserves state between agent assignments
- Uses the existing task state machine: failed task gets reassigned back to `pending`, then goes through normal routing
- Event bus tracks the full failover chain for auditability

### Key Design Decisions

- Failover resets task to `pending` rather than directly assigning to a new agent — this allows the normal routing engine to pick the best available agent
- Checkpoint data is preserved in the task row across reassignments — the `checkpoint` column is never cleared on failover
- The system distinguishes retryable (infrastructure) failures from non-retryable (business logic) failures based on the error type

### Integration Points

- `internal/task/task.go` — `Reassign()` method and failover orchestration
- `internal/task/router.go` — `FindCapableAgent()` used to find replacement agents
- `internal/resilience/circuit_breaker.go` — circuit open triggers failover
- `internal/event/types.go` — `TaskFailover` event constant
- `internal/adapter/adapter.go` — `Resume()` method on adapter interface

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Resilience Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR54]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Task failover implemented with checkpoint preservation and automatic rerouting
- Distinguishes infrastructure failures (retryable) from business logic errors (non-retryable)
- Failover resets task to pending for normal routing rather than direct assignment
- Event emission records full failover chain: task, original agent, new agent, reason

### Change Log

- 2026-04-16: Story 5.3 implemented — task failover with checkpoint-aware reassignment

### File List

- internal/task/task.go (modified — added Reassign method and failover logic)
- internal/task/router.go (reference — FindCapableAgent used for replacement routing)
- internal/task/task_test.go (modified — failover tests)
- internal/event/types.go (reference — TaskFailover constant)
- internal/resilience/circuit_breaker.go (reference — triggers failover on circuit open)
