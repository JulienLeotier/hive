# Story 20.3: Auto-Tuning

Status: done

## Story

As the system,
I want to automatically apply approved optimizations,
so that performance improves without manual intervention.

## Acceptance Criteria

1. **Given** the user approves an optimization via `hive optimize --apply`
   **When** the next workflow run executes
   **Then** the approved optimizations are applied (e.g., prefer faster agent, parallelize tasks)

2. **Given** an optimization has been applied
   **When** subsequent workflow runs complete
   **Then** results are compared to pre-optimization baseline

3. **Given** an optimization is applied
   **When** the system applies it
   **Then** a `system.optimization.applied` event is logged

4. **Given** an applied optimization degrades performance
   **When** the comparison detects regression
   **Then** the optimization is automatically rolled back

## Tasks / Subtasks

- [x] Task 1: Optimization application engine (AC: #1, #3)
  - [x] Define `OptimizationAction` struct with type, parameters, status, applied_at, baseline
  - [x] Implement `ApplyOptimization(recommendation)` -- converts recommendation to runtime config change
  - [x] Implement agent preference optimization: set routing weight for preferred agent
  - [x] Implement parallelization optimization: modify workflow DAG to remove unnecessary dependencies
  - [x] Implement heartbeat tuning: update agent heartbeat interval
  - [x] Emit `system.optimization.applied` event with optimization details
- [x] Task 2: Baseline capture (AC: #2)
  - [x] Record pre-optimization metrics: average duration, p95 duration, failure rate per workflow
  - [x] Store baseline in optimizations table linked to the applied optimization
  - [x] Capture sufficient history (last 10 runs) for meaningful comparison
- [x] Task 3: Post-optimization comparison (AC: #2, #4)
  - [x] Compare post-optimization metrics against stored baseline after configurable sample size (default 5 runs)
  - [x] Calculate percentage improvement/regression for each metric
  - [x] Generate comparison report accessible via `hive optimize --status`
- [x] Task 4: Auto-rollback (AC: #4)
  - [x] Detect performance regression: post-optimization metrics worse than baseline by threshold (default 10%)
  - [x] Automatically revert the optimization to previous configuration
  - [x] Emit `system.optimization.rolled_back` event with regression data
  - [x] Mark optimization as `rolled_back` in storage
- [x] Task 5: CLI integration (AC: #1)
  - [x] Add `--apply` flag to `hive optimize` -- applies all recommended optimizations
  - [x] Add `--apply-id <id>` to apply a specific recommendation
  - [x] Add `--status` flag to show applied optimizations with comparison results
  - [x] Add `--rollback <id>` to manually rollback a specific optimization
  - [x] Confirmation prompt before applying (bypass with `--yes`)
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test optimization application for each type (agent preference, parallelization, heartbeat)
  - [x] Test baseline capture from historical data
  - [x] Test comparison calculation and improvement detection
  - [x] Test auto-rollback on regression detection
  - [x] Test manual rollback via CLI

## Dev Notes

### Architecture Compliance

- Auto-tuning modifies runtime configuration, not source YAML files -- changes are reversible and don't alter user's workflow definition
- Optimizations are applied as runtime overrides that take precedence over YAML config
- Rollback restores the previous runtime state, not a "default" state
- All applied optimizations are persisted for audit trail and comparison
- Uses `slog` for structured logging of all optimization actions

### Key Design Decisions

- Optimizations require explicit approval (`--apply`) -- no fully automatic changes without user consent
- Baseline is captured at apply time from recent history, not from a predetermined target
- Comparison requires a minimum sample size (5 runs) before evaluating -- avoids reacting to noise
- Regression threshold (10%) is configurable -- prevents rolling back on minor fluctuations
- Runtime overrides are stored separately from workflow config to maintain clean separation

### Integration Points

- internal/optimizer/analyzer.go (modified -- ApplyOptimization, BaselineCapture, Comparison, AutoRollback)
- internal/optimizer/analyzer_test.go (modified -- apply, baseline, comparison, rollback tests)
- internal/cli/optimize.go (modified -- --apply, --apply-id, --status, --rollback flags)
- internal/task/router.go (modified -- respects runtime routing weight overrides)
- internal/workflow/workflow.go (modified -- respects runtime DAG overrides)
- internal/autonomy/scheduler.go (modified -- respects runtime heartbeat overrides)
- internal/event/types.go (modified -- optimization applied/rolled_back event constants)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 20 - Story 20.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR117, FR120]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Auto-tuning applies three optimization types: agent preference, DAG parallelization, heartbeat interval
- Baseline captured from last 10 workflow runs at optimization apply time
- Post-optimization comparison after 5 runs with percentage improvement/regression calculation
- Auto-rollback on regression exceeding 10% threshold with event emission
- CLI supports --apply, --apply-id, --status, --rollback with confirmation prompt
- All changes are runtime overrides, not modifications to user's YAML files

### Change Log

- 2026-04-16: Story 20.3 implemented -- auto-tuning with baseline comparison and automatic rollback on regression

### File List

- internal/optimizer/analyzer.go (modified -- ApplyOptimization, BaselineCapture, Comparison, AutoRollback)
- internal/optimizer/analyzer_test.go (modified -- optimization apply, comparison, and rollback tests)
- internal/cli/optimize.go (modified -- --apply, --apply-id, --status, --rollback flags)
- internal/task/router.go (modified -- runtime routing weight overrides)
- internal/workflow/workflow.go (modified -- runtime DAG overrides)
- internal/autonomy/scheduler.go (modified -- runtime heartbeat overrides)
- internal/event/types.go (modified -- optimization applied/rolled_back event constants)
