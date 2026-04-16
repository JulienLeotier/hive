# Story 7.3: hive run -- Terminal Output

Status: done

## Story

As a user,
I want clear, real-time terminal output when running workflows,
so that I can see what's happening without checking logs.

## Acceptance Criteria

1. **Given** a valid workflow and registered agents
   **When** the user runs `hive run`
   **Then** terminal shows: workflow start, each task dispatched (agent + capability), task results (success/failure + summary), workflow completion summary with duration

2. **Given** the user wants minimal output
   **When** they run `hive run --quiet`
   **Then** only the final result is displayed (suppresses all progress output)

3. **Given** the user wants machine-readable output
   **When** they run `hive run --json`
   **Then** structured progress events are output as JSON

## Tasks / Subtasks

- [x] Task 1: Workflow execution with live output (AC: #1)
  - [x] Parse `hive.yaml` via `workflow.ParseFile()`
  - [x] Topologically sort tasks for execution order
  - [x] For each execution level, dispatch tasks to agents via router
  - [x] Print progress: `[START] workflow "name"`, `[TASK] dispatching "task-name" to agent "agent-name"`, `[OK] task "task-name" completed (1.2s)`, `[DONE] workflow completed in 5.3s`
- [x] Task 2: Event subscription for real-time updates (AC: #1)
  - [x] Subscribe to `task.*` and `workflow.*` events on the event bus
  - [x] Map events to terminal output in real-time
  - [x] Show task failures with error summary: `[FAIL] task "task-name" failed: <reason>`
- [x] Task 3: Quiet mode (AC: #2)
  - [x] Implement `--quiet` flag that suppresses all progress output
  - [x] Only display final workflow result (success/failure + summary)
- [x] Task 4: JSON output mode (AC: #3)
  - [x] Implement `--json` flag for structured JSON progress events
  - [x] Each event is a JSON line (JSONL format) for streaming consumption
  - [x] Events include: type, task, agent, timestamp, duration, status

## Dev Notes

### Architecture Compliance

- Workflow execution uses `workflow.ParseFile()` for config and `workflow.TopologicalSort()` for execution order
- Task routing via `task.Router.FindCapableAgent()` — capability-based, not agent-name-based
- Event bus subscription provides real-time updates without polling
- Duration tracking uses Go's `time.Since()` from workflow start

### Key Design Decisions

- Output format uses bracketed prefixes (`[START]`, `[TASK]`, `[OK]`, `[FAIL]`, `[DONE]`) for easy scanning
- JSON mode outputs JSONL (one JSON object per line) for streaming-friendly consumption by other tools
- Quiet mode is useful for CI/CD where only the exit code and final result matter
- Parallel tasks at the same topological level are dispatched concurrently with goroutines

### Output Format Examples

```
[START] workflow "code-review"
[TASK]  dispatching "review" to agent "code-reviewer" (code-review)
[OK]    task "review" completed (2.1s)
[TASK]  dispatching "summarize" to agent "summarizer" (summarize)
[OK]    task "summarize" completed (0.8s)
[DONE]  workflow "code-review" completed in 2.9s (2 tasks)
```

### Integration Points

- `internal/cli/root.go` — `hive run` command would be registered here or as a dedicated file
- `internal/workflow/parser.go` — `ParseFile()`, `TopologicalSort()` for workflow loading
- `internal/task/router.go` — `FindCapableAgent()` for routing
- `internal/task/task.go` — `Create()`, `Assign()`, `Start()`, `Complete()` state machine
- `internal/event/bus.go` — `Subscribe()` for real-time event delivery

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR29]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Workflow execution with real-time bracketed terminal output showing task dispatch, completion, and failures
- Quiet mode (--quiet) suppresses progress, showing only final result
- JSON mode (--json) outputs JSONL for programmatic consumption
- Parallel task dispatch at same topological level via goroutines
- Duration tracking for individual tasks and overall workflow

### Change Log

- 2026-04-16: Story 7.3 implemented — hive run with real-time terminal output

### File List

- internal/cli/root.go (modified — run command registration)
- internal/workflow/parser.go (reference — ParseFile, TopologicalSort)
- internal/workflow/workflow.go (reference — workflow store, status updates)
- internal/task/router.go (reference — FindCapableAgent)
- internal/task/task.go (reference — task state machine)
- internal/event/bus.go (reference — Subscribe for real-time events)
