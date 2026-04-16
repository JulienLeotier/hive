# Story 2.2: Task State Machine

Status: done

## Story

As the system,
I want tasks with a well-defined state machine,
so that task lifecycle is predictable and debuggable.

## Acceptance Criteria

1. **Given** a task is created **When** it enters the system **Then** it starts in `pending` state with a ULID identifier **And** a `task.created` event is emitted
2. **Given** a task progresses through its lifecycle **When** state transitions occur **Then** it follows: `pending` -> `assigned` -> `running` -> `completed` | `failed` **And** each transition emits the corresponding event (`task.assigned`, `task.started`, `task.completed`, `task.failed`)
3. **Given** a task state transition is attempted **When** the task is not in the expected source state **Then** the transition fails with a clear error message
4. **Given** task input and output **When** the task is created or completed **Then** input/output is stored as JSON strings in SQLite
5. **Given** a running task **When** a checkpoint is saved **Then** the serialized state is stored in the task's `checkpoint` field
6. **Given** tasks exist **When** queried by workflow ID or by pending status **Then** correct filtered results are returned ordered by creation time

## Tasks / Subtasks

- [x] Task 1: Define Task struct with all fields and status constants (AC: #1)
- [x] Task 2: Implement Store with Create method (pending state, ULID, event emission) (AC: #1, #4)
- [x] Task 3: Implement Assign transition (pending -> assigned) with guard (AC: #2, #3)
- [x] Task 4: Implement Start transition (assigned -> running) with guard (AC: #2, #3)
- [x] Task 5: Implement Complete transition (running -> completed) with output storage (AC: #2, #4)
- [x] Task 6: Implement Fail transition (running -> failed) with error message (AC: #2)
- [x] Task 7: Implement SaveCheckpoint for running tasks (AC: #5)
- [x] Task 8: Implement GetByID, ListByWorkflow, ListPending queries (AC: #6)
- [x] Task 9: Write tests for full lifecycle, invalid transitions, checkpoint, queries (AC: #1-#6)

## Dev Notes

- Task Store depends on event.Bus for emitting state change events
- State transition guards use SQL WHERE clause on current status -- `RowsAffected() == 0` means invalid transition
- ULID generated via `ulid.MustNew` with `crypto/rand` reader for uniqueness
- `depends_on` stored as JSON array string for DAG dependency tracking
- Timestamps use SQLite `datetime('now')` for consistency
- ListPending supports optional task type filter for capability-based querying

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/task/task.go (new) -- Task struct, Store, Create/Assign/Start/Complete/Fail/SaveCheckpoint/GetByID/ListByWorkflow/ListPending
- internal/task/task_test.go (new) -- 7 tests covering lifecycle, failure, checkpoint, queries, event emission
