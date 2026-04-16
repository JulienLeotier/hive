# Story 20.1: Pattern Analyzer

Status: done

## Story

As the system,
I want to analyze historical execution data for optimization patterns,
so that I can identify bottlenecks and inefficiencies.

## Acceptance Criteria

1. **Given** the system has executed multiple workflows
   **When** the analyzer runs (triggered by `hive optimize` or on schedule)
   **Then** it identifies slow agents (p95 duration)

2. **Given** historical execution data
   **When** the analyzer runs
   **Then** it identifies underutilized agents

3. **Given** workflow execution history
   **When** the analyzer runs
   **Then** it identifies sequential tasks that could be parallelized

4. **Given** task failure data
   **When** the analyzer runs
   **Then** it identifies frequently failing task types

5. **Given** analysis findings
   **When** the analysis completes
   **Then** findings are stored for recommendation generation (Story 20.2)

## Tasks / Subtasks

- [x] Task 1: Analyzer data model (AC: #5)
  - [x] Define `Finding` struct with type, severity, affected entity, data, timestamp
  - [x] Define `AnalysisReport` struct containing findings grouped by category
  - [x] Create `optimizations` table in v1.0 migration for persisting findings
- [x] Task 2: PatternAnalyzer core (AC: #1, #2, #3, #4)
  - [x] Create `PatternAnalyzer` struct in `internal/optimizer/analyzer.go`
  - [x] Implement `Analyze(timeRange)` -- runs all analysis passes and returns AnalysisReport
  - [x] Accept storage dependencies for querying historical data
- [x] Task 3: Slow agent detection (AC: #1)
  - [x] Query task durations grouped by agent over the analysis window
  - [x] Calculate p50, p95, p99 duration per agent per task type
  - [x] Flag agents where p95 exceeds 2x the median for that task type
  - [x] Include comparison data: agent X is Nx slower than agent Y
- [x] Task 4: Underutilization detection (AC: #2)
  - [x] Calculate agent idle rate: (wake-ups with no action) / (total wake-ups)
  - [x] Flag agents with idle rate above configurable threshold (default 60%)
  - [x] Include heartbeat interval suggestion based on actual work frequency
- [x] Task 5: Parallelization opportunities (AC: #3)
  - [x] Analyze workflow DAGs for sequential tasks with no data dependency
  - [x] Detect patterns where task B reads no output from task A but is sequenced after it
  - [x] Estimate time savings from parallelizing identified task pairs
- [x] Task 6: Failure pattern detection (AC: #4)
  - [x] Calculate failure rate by task type over the analysis window
  - [x] Flag task types with failure rate above threshold (default 10%)
  - [x] Identify common error patterns via error message grouping
  - [x] Correlate failures with specific agents to distinguish agent vs. task issues
- [x] Task 7: Finding persistence (AC: #5)
  - [x] Store analysis findings in optimizations table
  - [x] Tag findings with analysis run ID for grouping
  - [x] Track finding status: new, acknowledged, applied, dismissed
- [x] Task 8: Unit tests (AC: #1, #2, #3, #4, #5)
  - [x] Test slow agent detection with synthetic duration data
  - [x] Test underutilization detection with idle rate calculations
  - [x] Test parallelization opportunity detection in sample DAGs
  - [x] Test failure pattern identification
  - [x] Test finding persistence and retrieval

## Dev Notes

### Architecture Compliance

- `internal/optimizer/analyzer.go` is the core analysis engine
- All analysis queries run on historical data in SQLite -- no impact on live execution
- Findings are structured data, not free text -- enables programmatic consumption by Story 20.3
- Uses `slog` for structured logging of analysis progress and findings
- Analysis is triggered on demand (`hive optimize`) or on schedule -- not continuous

### Key Design Decisions

- Analysis runs as a batch operation, not streaming -- simpler implementation and no overhead during normal operation
- p95 duration is the primary metric for slow agent detection (not average, which masks outliers)
- Parallelization detection uses conservative heuristics: only flags tasks with zero data dependency
- Failure correlation with agents helps distinguish "this task type always fails" from "this agent fails at everything"
- Finding status lifecycle allows users to track which recommendations they've acted on

### Integration Points

- internal/optimizer/analyzer.go (modified -- PatternAnalyzer, Finding, AnalysisReport, all detection passes)
- internal/optimizer/analyzer_test.go (new -- tests for each analysis pass)
- internal/task/task.go (reference -- task duration and status queries)
- internal/agent/manager.go (reference -- agent idle rate and wake-up data)
- internal/workflow/workflow.go (reference -- DAG structure for parallelization analysis)
- internal/storage/migrations/004_v10.sql (reference -- optimizations table)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 20 - Story 20.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR116]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- PatternAnalyzer implements four analysis passes: slow agents, underutilization, parallelization, failure patterns
- Slow agent detection uses p95 duration with per-task-type comparison between agents
- Underutilization flags agents above 60% idle rate with heartbeat tuning suggestion
- Parallelization detector identifies sequential tasks with no data dependency and estimates time savings
- Failure patterns correlated with agents to distinguish agent vs. task-type issues
- Findings persisted to optimizations table with status lifecycle tracking

### Change Log

- 2026-04-16: Story 20.1 implemented -- pattern analyzer with four analysis passes and finding persistence

### File List

- internal/optimizer/analyzer.go (modified -- PatternAnalyzer, Finding, AnalysisReport, detection passes)
- internal/optimizer/analyzer_test.go (new -- tests for all analysis passes and finding persistence)
- internal/task/task.go (reference -- task duration queries)
- internal/agent/manager.go (reference -- agent idle rate data)
- internal/workflow/workflow.go (reference -- DAG structure queries)
- internal/storage/migrations/004_v10.sql (reference -- optimizations table)
