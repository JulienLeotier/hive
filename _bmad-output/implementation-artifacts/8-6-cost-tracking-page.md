# Story 8.6: Cost Tracking Page

Status: done

## Story

As a user,
I want to see cost tracking per agent and per workflow,
so that I can manage my AI spend.

## Acceptance Criteria

1. **Given** agents declare `cost_per_run` in capabilities
   **When** the cost page is displayed
   **Then** it shows cost per agent (total and recent), cost per workflow, and cost trend over time

2. **Given** tasks have been completed with associated costs
   **When** the cost tracker queries the database
   **Then** costs are aggregated by agent name with total cost and task count

3. **Given** cost entries exist for today
   **When** daily cost is queried for a specific agent
   **Then** the system returns the correct sum for today's costs only

## Tasks / Subtasks

- [x] Task 1: Cost Tracker implementation (AC: #1, #2, #3)
  - [x] Create `internal/cost/tracker.go` with `Tracker` struct backed by `*sql.DB`
  - [x] Define `Entry` struct: agent_id, agent_name, workflow_id, task_id, cost, created_at
  - [x] Define `Summary` struct: agent_name, total_cost, task_count
  - [x] Implement `NewTracker(db)` constructor
  - [x] Implement `Record()` — inserts cost entry for a completed task
  - [x] Implement `ByAgent()` — returns cost summaries grouped by agent, ordered by highest cost
  - [x] Implement `DailyCostForAgent()` — returns today's total cost for a specific agent
- [x] Task 2: Cost table schema (AC: #2)
  - [x] `costs` table with: id (autoincrement), agent_id, agent_name, workflow_id, task_id, cost (REAL), created_at
  - [x] Table created by v0.2 migration
- [x] Task 3: Unit tests (AC: #2, #3)
  - [x] Test Record and ByAgent aggregation (2 agents, verify ordering by highest cost)
  - [x] Test DailyCostForAgent returns correct daily sum
  - [x] Test that ByAgent returns correct task counts per agent

## Dev Notes

### Architecture Compliance

- **Direct SQL** — no ORM, uses `database/sql` with prepared statements for cost queries
- **slog** — debug-level logging on cost recording
- **Package isolation** — `internal/cost` has no dependency on event bus or API server; the caller records costs after task completion
- **Aggregation** — `ByAgent()` uses SQL `SUM(cost)` and `COUNT(*)` with `GROUP BY` for efficient server-side aggregation

### Key Design Decisions

- Cost tracking is decoupled from task execution — the cost `Record()` call is made by the orchestrator after a task completes, passing the agent's declared `cost_per_run`
- `ByAgent()` orders by total cost descending — most expensive agents appear first for quick identification of high-spend agents
- `DailyCostForAgent()` uses SQLite's `date()` function for date comparison — no timezone handling needed for MVP (UTC assumed)
- Cost is stored as `REAL` (float64) — sufficient precision for API pricing (typically $0.001-$10 per call)

### Integration Points

- `internal/cost/tracker.go` — cost tracking implementation
- `internal/cost/tracker_test.go` — unit tests for recording and aggregation
- `internal/storage/migrations/` — costs table created by v0.2 migration

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR60]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.6]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Cost tracker records per-task costs and aggregates by agent
- ByAgent returns summaries ordered by highest cost first
- DailyCostForAgent supports daily budget monitoring
- 3 unit tests covering recording, aggregation, and daily cost queries
- Costs table schema with agent_id, workflow_id, task_id, cost fields

### Change Log

- 2026-04-16: Story 8.6 implemented — cost tracker with recording, aggregation, and daily cost queries

### File List

- internal/cost/tracker.go (new)
- internal/cost/tracker_test.go (new)
