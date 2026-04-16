# Story 4.4: Task Self-Assignment

Status: done

## Story

As an agent,
I want to claim tasks from the shared backlog based on my capabilities,
so that work gets done without a central dispatcher assigning me.

## Acceptance Criteria

1. **Given** the observer found pending tasks matching this agent's capabilities **When** the plan evaluator decides to take action **Then** the agent claims a task atomically using the task.Store.Assign method
2. **Given** the task is claimed **When** Assign succeeds **Then** the task status changes to `assigned` with this agent's ID **And** a `task.assigned` event is emitted
3. **Given** the task was already claimed by another agent **When** Assign is attempted **Then** RowsAffected returns 0 (task no longer in `pending` state) **And** the agent tries the next pending task
4. **Given** self-assignment occurs **When** the event is emitted **Then** it is recorded as a `task.assigned` event with the agent as source (FR46)

## Tasks / Subtasks

- [x] Task 1: Implement atomic task claiming via task.Store.Assign with SQL WHERE status=pending guard (AC: #1, #3)
- [x] Task 2: Emit task.assigned event on successful claim (AC: #2, #4)
- [x] Task 3: Handle concurrent claim conflict via RowsAffected check (AC: #3)
- [x] Task 4: Integrate self-assignment into WakeUpHandler flow (AC: #1)
- [x] Task 5: Write tests for assign, concurrent conflict, event emission (AC: #1-#4)

## Dev Notes

- Task self-assignment reuses the existing `task.Store.Assign` method -- no new code needed for the actual assignment
- Atomic claiming is guaranteed by the SQL UPDATE with `WHERE status = 'pending'` -- if another agent claimed first, RowsAffected is 0
- This is a pull-based model: agents pull tasks during wake-up rather than having a central dispatcher push tasks
- The WakeUpHandler coordinates: observe backlog -> find matching task -> attempt Assign -> on conflict try next
- SQLite's serialized writes ensure no double-claiming even with concurrent agents
- The task.assigned event payload includes both task_id and agent_id for audit trail

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/task/task.go (dependency) -- Store.Assign with atomic pending->assigned transition
- internal/task/task_test.go (dependency) -- TestTaskStateMachine covers assign flow
- internal/autonomy/scheduler.go (dependency) -- WakeUpHandler integration point for self-assignment
