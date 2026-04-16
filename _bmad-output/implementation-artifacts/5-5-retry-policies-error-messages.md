# Story 5.5: Retry Policies & Error Messages

Status: done

## Story

As a user,
I want configurable retry policies and clear error messages,
so that transient failures are handled automatically and I understand persistent failures.

## Acceptance Criteria

1. **Given** a task fails with a retryable error
   **When** retry policy is configured (e.g., `retries: 3, backoff: exponential`)
   **Then** the system retries with configured backoff (1s, 2s, 4s)
   **And** each retry emits a `task.retry` event

2. **Given** a task exhausts all retries
   **When** the final retry fails
   **Then** the task transitions to `failed` with aggregated error context

3. **Given** any error in the system
   **When** it's surfaced to the user (CLI or logs)
   **Then** the error message includes: what went wrong, which agent/task was involved, a suggested remediation action
   **And** never includes secrets, tokens, or full stack traces

## Tasks / Subtasks

- [x] Task 1: Retry policy configuration (AC: #1)
  - [x] Define retry policy struct with max retries, backoff strategy (constant, linear, exponential), base delay
  - [x] Support per-agent and per-task-type retry configuration in hive.yaml
  - [x] Default policy: 3 retries, exponential backoff, 1s base delay
- [x] Task 2: Retry execution logic (AC: #1, #2)
  - [x] Implement retry loop with backoff calculation
  - [x] Emit `task.retry` event on each retry attempt with attempt number and delay
  - [x] On final failure, transition task to `failed` with full retry history in output
  - [x] Event type `TaskRetry` already defined in `internal/event/types.go`
- [x] Task 3: Structured error messages (AC: #3)
  - [x] Create error formatting that includes: context (what happened), entity (which agent/task), remediation (suggested fix)
  - [x] Sanitize errors to strip secrets, API keys, and bearer tokens before logging or display
  - [x] Use slog structured fields for machine-parseable error context
- [x] Task 4: Secret scrubbing (AC: #3)
  - [x] Implement secret pattern detection (Bearer tokens, API keys, connection strings)
  - [x] Scrub matched patterns before any log output or CLI display
  - [x] Never include raw stack traces in user-facing errors

## Dev Notes

### Architecture Compliance

- Retry policies are configurable per agent and per task type, defaulting to sensible values
- Exponential backoff prevents retry storms: delay = baseDelay * 2^(attempt-1)
- Secret scrubbing enforces NFR10 (secrets never logged) at the error formatting layer
- All retries are tracked via `task.retry` events for full observability
- Error messages follow a consistent format: "what: which: remediation suggestion"

### Key Design Decisions

- Retry is handled at the task execution layer, not the adapter layer — this ensures retry events are emitted and policies are consistent across all adapter types
- Backoff strategies: `constant` (same delay), `linear` (delay * attempt), `exponential` (delay * 2^attempt)
- Secret scrubbing uses regex patterns for common secret formats and replaces with `[REDACTED]`
- Error remediation suggestions are mapped from common error codes (connection refused -> check agent URL, timeout -> increase timeout)

### Integration Points

- `internal/task/task.go` — retry loop wraps task invocation
- `internal/event/types.go` — `TaskRetry` event constant
- `internal/config/config.go` — retry policy configuration in hive.yaml
- `internal/adapter/adapter.go` — `Invoke()` is the retried operation

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Resilience Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR55, FR56, NFR10]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Configurable retry policies with constant, linear, and exponential backoff strategies
- Secret scrubbing strips Bearer tokens, API keys, and connection strings from all error output
- Structured error messages include what/which/remediation for every user-facing error
- Each retry attempt emits a task.retry event for full observability

### Change Log

- 2026-04-16: Story 5.5 implemented — retry policies with exponential backoff and secret-safe error messages

### File List

- internal/task/task.go (modified — retry loop and error formatting)
- internal/task/task_test.go (modified — retry and error message tests)
- internal/config/config.go (modified — retry policy configuration)
- internal/event/types.go (reference — TaskRetry constant)
