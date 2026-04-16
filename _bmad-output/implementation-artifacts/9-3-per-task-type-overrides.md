# Story 9.3: Per-Task-Type Overrides

Status: done

## Story

As a user,
I want certain task types to always require specific trust levels,
so that high-risk operations maintain human oversight regardless of agent track record.

## Acceptance Criteria

1. **Given** a trust override is configured (e.g., "financial-transactions: always Supervised")
   **When** a task of that type is routed
   **Then** the system enforces the override level regardless of the agent's earned trust

2. **Given** an override is applied
   **When** the task routing decision is made
   **Then** the override is logged with the task type and enforced level

3. **Given** no override is configured for a task type
   **When** the task is routed
   **Then** the agent's normal earned trust level applies

## Tasks / Subtasks

- [x] Task 1: Trust override configuration (AC: #1, #3)
  - [x] Define override as a map of task_type to required trust level in `Thresholds` struct
  - [x] Overrides configurable via YAML (e.g., `trust_overrides: {"financial-transactions": "supervised"}`)
  - [x] When evaluating trust for a task, check overrides map first
  - [x] If override exists, the effective trust level for that task type is the override value
- [x] Task 2: Override enforcement in routing (AC: #1, #2)
  - [x] Before routing a task, check if the task type has a trust override
  - [x] If the agent's trust level is below the override requirement, the task is not routed to that agent
  - [x] Log the override enforcement via slog with task_type and required_level
- [x] Task 3: Passthrough for unconfigured types (AC: #3)
  - [x] When no override exists for a task type, the agent's earned trust level is used as-is
  - [x] No additional processing or logging for non-overridden task types

## Dev Notes

### Architecture Compliance

- **Configuration-driven** — overrides are part of the `Thresholds` struct, loadable from YAML
- **slog** — override enforcement logged at info level for auditability
- **No new tables** — overrides are configuration, not persistent state; they live in `hive.yaml`

### Key Design Decisions

- Overrides are stored as a simple map (`map[string]string`) within the Thresholds config — this keeps them co-located with other trust configuration
- Override enforcement happens at the routing layer, not the trust engine — the engine computes earned trust, the router applies overrides when making assignment decisions
- Override levels use the same constants as earned levels (`supervised`, `guided`, `autonomous`, `trusted`) for consistency
- A task type with an override of `supervised` means even a `trusted` agent must operate under supervised constraints for that specific task type

### Integration Points

- `internal/trust/engine.go` — Thresholds struct extended with override map
- `internal/task/router.go` — checks trust overrides before routing decisions
- `internal/config/config.go` — trust_overrides in YAML configuration

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR65, FR68]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Trust overrides enforce minimum trust levels per task type
- Configured via YAML map in Thresholds struct
- Router checks overrides before agent assignment
- Passthrough for unconfigured task types (earned trust applies)
- Override enforcement logged for auditability

### Change Log

- 2026-04-16: Story 9.3 implemented — per-task-type trust overrides with routing enforcement

### File List

- internal/trust/engine.go (modified — Thresholds struct extended with override map)
- internal/task/router.go (modified — checks trust overrides during routing)
- internal/config/config.go (reference — YAML trust_overrides configuration)
