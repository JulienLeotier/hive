# Story 2.6: Task Checkpoint & Resume

Status: done

## Story

As a user,
I want long-running tasks to checkpoint their state,
so that work isn't lost if an agent fails or is replaced.

## Acceptance Criteria

1. **Given** a task is running **When** the checkpoint interval elapses **Then** the orchestrator calls the agent's `/checkpoint` endpoint via the Adapter interface
2. **Given** a checkpoint is retrieved from an agent **When** it is saved **Then** the serialized state is stored in the task's `checkpoint` field in SQLite
3. **Given** an agent fails and the task is reassigned **When** a new agent receives the task **Then** the new agent's `/resume` endpoint is called with the checkpoint data
4. **Given** the Adapter protocol **When** Checkpoint and Resume are called **Then** they use GET `/checkpoint` and POST `/resume` respectively (FR17, FR18, NFR5)

## Tasks / Subtasks

- [x] Task 1: Define Checkpoint type in adapter package (AC: #4)
- [x] Task 2: Implement Adapter.Checkpoint and Adapter.Resume in interface (AC: #4)
- [x] Task 3: Implement HTTPAdapter.Checkpoint (GET /checkpoint) (AC: #1, #4)
- [x] Task 4: Implement HTTPAdapter.Resume (POST /resume with checkpoint body) (AC: #3, #4)
- [x] Task 5: Implement task.Store.SaveCheckpoint to persist to SQLite (AC: #2)
- [x] Task 6: Implement task.Store.GetByID returning checkpoint data (AC: #2)
- [x] Task 7: Write tests for checkpoint save/retrieve round-trip (AC: #1-#4)

## Dev Notes

- Checkpoint is a simple `struct { Data any }` allowing agents to serialize arbitrary state
- SaveCheckpoint updates the `checkpoint` column directly without state transition (task stays `running`)
- GetByID reads the checkpoint field, making it available for resume after reassignment
- HTTPAdapter.Checkpoint calls GET on `/checkpoint` and unmarshals the response
- HTTPAdapter.Resume calls POST on `/resume` with the Checkpoint JSON body
- Actual periodic checkpoint scheduling (timer-based) is handled at the orchestration layer
- The checkpoint/resume flow enables zero-data-loss agent hot-swap (Story 5.4)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/adapter/adapter.go (modified) -- Checkpoint type, Adapter interface with Checkpoint/Resume methods
- internal/adapter/http.go (modified) -- HTTPAdapter.Checkpoint (GET), HTTPAdapter.Resume (POST)
- internal/task/task.go (modified) -- SaveCheckpoint method, GetByID returns checkpoint field
- internal/task/task_test.go (modified) -- TestSaveCheckpoint round-trip test
