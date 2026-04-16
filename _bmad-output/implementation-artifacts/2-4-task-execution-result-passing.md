# Story 2.4: Task Execution & Result Passing

Status: done

## Story

As a user,
I want tasks executed by agents with results passed to downstream tasks,
so that multi-step workflows produce cumulative results.

## Acceptance Criteria

1. **Given** a task is assigned to an agent **When** the orchestrator invokes the agent's `/invoke` endpoint with the task payload **Then** the agent processes the task and returns a `TaskResult`
2. **Given** an agent returns a successful result **When** the result is received **Then** the output is stored in the task's `output` field via `task.Store.Complete` **And** a `task.completed` event is emitted with the task ID
3. **Given** an agent returns a failure result **When** the error is received **Then** the task is marked failed via `task.Store.Fail` **And** a `task.failed` event is emitted with the error message
4. **Given** upstream tasks have completed **When** downstream tasks are created **Then** the downstream task input can include upstream task output (result passing via workflow engine)
5. **Given** the adapter protocol defines Invoke **When** a task is sent to an HTTP agent **Then** the HTTPAdapter POSTs to `/invoke` with Task JSON and deserializes the TaskResult response (FR15)

## Tasks / Subtasks

- [x] Task 1: Define Task and TaskResult types in adapter package (AC: #1, #5)
- [x] Task 2: Implement HTTPAdapter.Invoke with POST to /invoke endpoint (AC: #5)
- [x] Task 3: Implement task.Store.Complete to store output and emit event (AC: #2)
- [x] Task 4: Implement task.Store.Fail to store error and emit event (AC: #3)
- [x] Task 5: Wire result storage into task state machine transitions (AC: #2, #3)
- [x] Task 6: Write tests for invoke, complete with output, fail with error (AC: #1-#5)

## Dev Notes

- The Adapter interface defines `Invoke(ctx, Task) (TaskResult, error)` as the core execution method
- HTTPAdapter sends POST to `{baseURL}/invoke` with JSON body and reads TaskResult response
- Response body is capped at 10MB via `io.LimitReader` to prevent OOM from malicious agents
- Result passing between tasks is orchestrated at the workflow engine level (Story 3.3), not in the task package
- Task output is stored as a JSON string, allowing flexible result schemas per agent

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/adapter/adapter.go (new) -- Adapter interface, Task, TaskResult, HealthStatus, Checkpoint types
- internal/adapter/http.go (new) -- HTTPAdapter with Invoke, get/post/do helpers
- internal/adapter/http_test.go (new) -- HTTP adapter tests with httptest mock server
- internal/task/task.go (modified) -- Complete and Fail methods store output/error
- internal/task/task_test.go (modified) -- Tests for complete with output, fail with error
