# Story 20.2: Optimization Recommendations

Status: done

## Story

As a user,
I want to see actionable optimization recommendations,
so that I can improve my hive's performance.

## Acceptance Criteria

1. **Given** the analyzer has identified slow agent patterns
   **When** the user runs `hive optimize`
   **Then** recommendations include "Agent X is 3x slower than Agent Y for code-review tasks"

2. **Given** the analyzer has identified parallelization opportunities
   **When** the user runs `hive optimize`
   **Then** recommendations include "Tasks A and B in workflow W could run in parallel"

3. **Given** the analyzer has identified underutilized agents
   **When** the user runs `hive optimize`
   **Then** recommendations include "Agent Z has 40% idle rate -- consider reducing heartbeat interval"

4. **Given** any recommendation
   **When** it is displayed
   **Then** it includes estimated impact (time savings, efficiency gain)

## Tasks / Subtasks

- [x] Task 1: Recommendation engine (AC: #1, #2, #3, #4)
  - [x] Define `Recommendation` struct with type, description, impact, affected entities, suggested action
  - [x] Implement `GenerateRecommendations(findings)` -- transforms findings into actionable recommendations
  - [x] Calculate estimated impact for each recommendation type
- [x] Task 2: Slow agent recommendations (AC: #1, #4)
  - [x] Generate comparative recommendation: "Agent X is Nx slower than Agent Y for <task-type>"
  - [x] Estimate time savings: projected reduction if slower agent is replaced or deprioritized
  - [x] Suggest action: prefer faster agent via allocation strategy or investigate root cause
- [x] Task 3: Parallelization recommendations (AC: #2, #4)
  - [x] Generate parallelization recommendation: "Tasks A and B in workflow W could run in parallel"
  - [x] Estimate time savings: sum of shorter task's duration (would overlap instead of sequential)
  - [x] Suggest action: remove dependency or restructure workflow YAML
- [x] Task 4: Underutilization recommendations (AC: #3, #4)
  - [x] Generate utilization recommendation with idle percentage
  - [x] Suggest heartbeat interval based on actual work frequency
  - [x] Estimate efficiency gain: reduced wake-up overhead
- [x] Task 5: CLI output (AC: #1, #2, #3, #4)
  - [x] Implement `hive optimize` command displaying all recommendations
  - [x] Group recommendations by category (performance, parallelism, utilization, reliability)
  - [x] Show estimated impact next to each recommendation
  - [x] Support `--json` output for programmatic consumption
  - [x] Support `--category <type>` filter to show specific recommendation types
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test recommendation generation from slow agent findings
  - [x] Test recommendation generation from parallelization findings
  - [x] Test recommendation generation from underutilization findings
  - [x] Test impact estimation calculations
  - [x] Test CLI output formatting

## Dev Notes

### Architecture Compliance

- Recommendations are generated from findings (Story 20.1), not from raw data -- clear separation of analysis and presentation
- `hive optimize` is the single entry point: runs analysis then generates recommendations
- Impact estimates are projections based on historical data, clearly labeled as estimates
- Uses `slog` for structured logging of recommendation generation

### Key Design Decisions

- Recommendations are human-readable text with structured data -- supports both CLI display and programmatic consumption
- Impact is expressed in concrete units (seconds saved, percentage improvement) not abstract scores
- Recommendations are grouped by category for scanability
- The `--apply` flag is reserved for Story 20.3 (auto-tuning) -- this story is read-only analysis

### Integration Points

- internal/optimizer/analyzer.go (modified -- GenerateRecommendations, Recommendation struct)
- internal/optimizer/analyzer_test.go (modified -- recommendation generation tests)
- internal/cli/optimize.go (new -- `hive optimize` command with category filter and JSON output)
- internal/event/types.go (modified -- optimization event constants)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 20 - Story 20.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR118, FR119]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Recommendation engine transforms analyzer findings into actionable recommendations with estimated impact
- Three recommendation types: slow agent comparison, parallelization opportunity, underutilization suggestion
- Impact expressed in concrete units: seconds saved, percentage efficiency gain
- `hive optimize` CLI command with category filtering and --json output
- Recommendations grouped by category: performance, parallelism, utilization, reliability

### Change Log

- 2026-04-16: Story 20.2 implemented -- optimization recommendations with impact estimates and CLI output

### File List

- internal/optimizer/analyzer.go (modified -- GenerateRecommendations, Recommendation struct, impact calculations)
- internal/optimizer/analyzer_test.go (modified -- recommendation generation and impact tests)
- internal/cli/optimize.go (new -- `hive optimize` command, category filter, JSON output)
- internal/event/types.go (modified -- optimization event constants)
