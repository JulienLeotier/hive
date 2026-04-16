# Story 5.4: Agent Hot-Swap

Status: done

## Story

As a user,
I want to replace a running agent with zero downtime,
so that I can upgrade or switch agents without losing work.

## Acceptance Criteria

1. **Given** a running agent with in-progress tasks
   **When** the user runs `hive agent swap old-agent --to new-agent`
   **Then** in-progress tasks are checkpointed before the swap

2. **Given** the swap is initiated
   **When** checkpointing completes
   **Then** the old agent is gracefully disconnected
   **And** the new agent is registered and health-checked

3. **Given** the new agent passes health check
   **When** the swap finalizes
   **Then** checkpointed tasks are resumed on the new agent
   **And** zero tasks are lost in the swap process

4. **Given** the new agent fails health check during swap
   **When** the swap is aborted
   **Then** the old agent remains active and tasks continue normally

## Tasks / Subtasks

- [x] Task 1: Hot-swap orchestration logic (AC: #1, #2, #3)
  - [x] Implement swap sequence: checkpoint running tasks -> register new agent -> health check -> resume tasks -> remove old agent
  - [x] Use adapter `Checkpoint()` endpoint to capture task state before disconnection
  - [x] Use adapter `Resume()` endpoint to restore task state on new agent
- [x] Task 2: CLI command for agent swap (AC: #1)
  - [x] Add `hive agent swap <old-agent> --to <new-agent>` command via cobra
  - [x] Display progress: checkpointing, registering, health-checking, resuming
  - [x] Error handling with rollback on failure
- [x] Task 3: Abort on health check failure (AC: #4)
  - [x] If new agent fails health check, abort swap and keep old agent active
  - [x] Display clear error message with the health check failure reason
- [x] Task 4: Zero-loss guarantee (AC: #3)
  - [x] Checkpoint all running tasks before any state changes
  - [x] Only remove old agent after all tasks successfully resume on new agent
  - [x] Emit swap events for auditability

## Dev Notes

### Architecture Compliance

- Hot-swap follows the sequence: checkpoint -> register -> verify -> resume -> remove
- Uses the existing adapter protocol methods: `Checkpoint()` and `Resume()`
- Task checkpoints stored in SQLite `tasks.checkpoint` column survive the swap
- NFR5: Zero data loss 99.9% — achieved by checkpointing before any destructive action

### Key Design Decisions

- Swap is an atomic operation from the user's perspective — either it completes fully or rolls back
- The old agent is only removed after all tasks are verified running on the new agent
- Progress is displayed step-by-step in the CLI so the user can see exactly what's happening
- If the new agent URL/config is the same type as the old, capabilities are re-validated

### Integration Points

- `internal/agent/manager.go` — `Register()`, `Remove()`, `GetByName()` for agent lifecycle
- `internal/task/task.go` — `SaveCheckpoint()`, `GetByID()`, `ListByWorkflow()` for task state
- `internal/adapter/adapter.go` — `Checkpoint()`, `Resume()`, `Health()` adapter protocol
- `internal/cli/agent.go` — swap command registration

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Resilience Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR6, NFR5]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Hot-swap implemented as atomic checkpoint-register-verify-resume-remove sequence
- CLI command provides step-by-step progress output during swap
- Rollback on failure keeps old agent active with no task disruption
- Uses existing adapter Checkpoint/Resume protocol methods

### Change Log

- 2026-04-16: Story 5.4 implemented — agent hot-swap with zero-loss checkpointing

### File List

- internal/agent/manager.go (modified — swap orchestration logic)
- internal/cli/agent.go (modified — added swap subcommand)
- internal/task/task.go (reference — SaveCheckpoint, task queries)
- internal/adapter/adapter.go (reference — Checkpoint, Resume interface methods)
