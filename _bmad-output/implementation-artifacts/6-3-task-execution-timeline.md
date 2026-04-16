# Story 6.3: Task Execution Timeline

Status: done

## Story

As a user,
I want to see a task execution timeline for workflows,
so that I can understand execution flow and identify bottlenecks.

## Acceptance Criteria

1. **Given** a completed or running workflow
   **When** the user runs `hive logs --workflow <id> --timeline`
   **Then** output shows each task with: start time, end time, duration, agent, status

2. **Given** a workflow with parallel branches
   **When** the timeline is displayed
   **Then** parallel tasks are visually indicated

3. **Given** a workflow with sequential and parallel tasks
   **When** the timeline is rendered
   **Then** the critical path is highlighted

## Tasks / Subtasks

- [x] Task 1: Timeline flag on logs command (AC: #1)
  - [x] Add `--workflow` and `--timeline` flags to the logs command
  - [x] Query tasks for the given workflow via `task.Store.ListByWorkflow()`
  - [x] Display each task with start time, end time, duration, assigned agent, and status
- [x] Task 2: Duration calculation (AC: #1)
  - [x] Calculate duration from `started_at` to `completed_at` for completed tasks
  - [x] Show elapsed time for running tasks (from `started_at` to now)
  - [x] Show "pending" for tasks that haven't started
- [x] Task 3: Parallel task indication (AC: #2)
  - [x] Parse `depends_on` for each task to determine parallelism
  - [x] Use indentation or markers to visually indicate tasks running in parallel
  - [x] Group tasks by execution level (topological sort output)
- [x] Task 4: Critical path highlighting (AC: #3)
  - [x] Calculate the longest path through the DAG based on actual durations
  - [x] Mark critical path tasks with a visual indicator (e.g., `*` prefix)

## Dev Notes

### Architecture Compliance

- Timeline view builds on existing task data stored in SQLite (`tasks` table with `started_at`, `completed_at`)
- Uses `workflow.TopologicalSort()` to determine parallel execution levels
- Duration is computed from stored timestamps — no additional instrumentation needed
- CLI output is text-based; JSON output deferred to dashboard (v0.2)

### Key Design Decisions

- Timeline is a view mode on the `hive logs` command (`--timeline` flag) rather than a separate command — keeps the CLI surface area small
- Parallel tasks are grouped by topological level and displayed together
- Critical path is the longest chain of sequential task durations through the DAG
- Tasks that haven't started show "pending" instead of a duration

### Integration Points

- `internal/cli/logs.go` — timeline rendering logic triggered by `--timeline` flag
- `internal/task/task.go` — `ListByWorkflow()` for task data
- `internal/workflow/parser.go` — `TopologicalSort()` for parallel level detection
- `internal/workflow/workflow.go` — `GetByID()` for workflow metadata

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR31]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Timeline view shows tasks with start time, end time, duration, agent, and status
- Parallel tasks grouped by topological execution level
- Critical path calculated from actual task durations and marked with visual indicator
- Duration shows elapsed time for running tasks and "pending" for queued tasks

### Change Log

- 2026-04-16: Story 6.3 implemented — task execution timeline with parallel indication and critical path

### File List

- internal/cli/logs.go (modified — added --workflow and --timeline flags, timeline rendering)
- internal/task/task.go (reference — ListByWorkflow, task timestamps)
- internal/workflow/parser.go (reference — TopologicalSort for level grouping)
- internal/workflow/workflow.go (reference — GetByID for workflow metadata)
