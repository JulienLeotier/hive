# Story 13.1: CrewAI Adapter

Status: done

## Story

As a user,
I want to register CrewAI agents with Hive,
so that I can orchestrate my CrewAI crews alongside other frameworks.

## Acceptance Criteria

1. **Given** a CrewAI project at a local path
   **When** the user runs `hive add-agent --type crewai --path ./my-crew`
   **Then** the adapter detects CrewAI crew configuration and maps crew capabilities to the Hive protocol

2. **Given** a registered CrewAI agent
   **When** a task is routed to it
   **Then** tasks are invoked by running the CrewAI crew via Python subprocess
   **And** the subprocess receives the task input as JSON

3. **Given** a CrewAI adapter
   **When** the health endpoint is called
   **Then** it verifies Python is available in PATH
   **And** returns `unavailable` if Python is not found

4. **Given** a CrewAI adapter
   **When** `Declare()` is called
   **Then** it returns capabilities with the agent name and `crewai-crew` task type

5. **Given** a CrewAI subprocess execution
   **When** the crew fails
   **Then** the adapter returns a `failed` result with the combined stdout/stderr error output

## Tasks / Subtasks

- [x] Task 1: CrewAI adapter struct (AC: #1, #4)
  - [x] Create `CrewAIAdapter` struct with `ProjectPath` and `Name` fields
  - [x] Implement `NewCrewAIAdapter(projectPath, name)` constructor
  - [x] Implement `Declare()` returning `crewai-crew` task type
  - [x] Verify compile-time interface satisfaction with `var _ Adapter = (*CrewAIAdapter)(nil)`
- [x] Task 2: Subprocess invocation (AC: #2, #5)
  - [x] Implement `Invoke()` using `exec.CommandContext` to run `python -m crewai run --input <json>`
  - [x] Set working directory to `ProjectPath`
  - [x] Marshal task input to JSON for subprocess stdin
  - [x] Parse combined output and return as `TaskResult`
  - [x] Handle subprocess errors with descriptive error messages
- [x] Task 3: Health check (AC: #3)
  - [x] Implement `Health()` checking Python availability via `exec.LookPath("python")`
  - [x] Return `healthy` if Python found, `unavailable` otherwise
- [x] Task 4: Checkpoint/Resume stubs (AC: #1)
  - [x] Implement `Checkpoint()` and `Resume()` as no-ops (CrewAI does not support checkpointing)

## Dev Notes

### Architecture Compliance

- Implements the `Adapter` interface from `internal/adapter/adapter.go` (Declare, Invoke, Health, Checkpoint, Resume)
- Uses `exec.CommandContext` for subprocess invocation with context cancellation support
- No CGO dependencies -- subprocess-based integration keeps the Go binary pure
- Error messages include both the Go error and subprocess output for debugging

### Key Design Decisions

- CrewAI is invoked via `python -m crewai run` subprocess rather than FFI -- this avoids Python runtime dependencies in the Go binary and supports any CrewAI version
- The adapter maps all CrewAI capabilities to a single `crewai-crew` task type since CrewAI crews are treated as atomic units
- Checkpoint/Resume are no-ops because CrewAI manages its own internal state; the Hive checkpoint protocol does not map cleanly to CrewAI's execution model

### Integration Points

- `internal/adapter/adapter.go` -- implements `Adapter` interface
- `internal/adapter/http.go` -- sibling adapter for comparison
- `internal/cli/agent.go` -- `hive add-agent --type crewai` creates this adapter
- `internal/agent/manager.go` -- stores agent record after `Declare()` call

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 13.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR84, FR88]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- CrewAIAdapter implemented with subprocess-based Python invocation
- Declare returns crewai-crew task type, Health checks for Python in PATH
- Invoke runs `python -m crewai run --input <json>` with context cancellation
- Checkpoint/Resume implemented as no-ops
- Compile-time interface satisfaction verified

### Change Log

- 2026-04-16: Story 13.1 implemented -- CrewAI adapter with subprocess invocation

### File List

- internal/adapter/crewai.go (new)
- internal/adapter/adapter.go (reference -- Adapter interface)
- internal/cli/agent.go (modified -- added crewai type to add-agent command)
- internal/agent/manager.go (reference -- agent registration flow)
