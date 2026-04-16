# Story 16.3: Cost Tracker

Status: done

## Story

As a user,
I want to see how much each agent and workflow costs,
so that I can manage my AI spend effectively.

## Acceptance Criteria

1. **Given** agents declare `cost_per_run` in their capabilities
   **When** tasks complete
   **Then** the system accumulates cost per agent and per workflow in the costs table

2. **Given** cost data has been recorded
   **When** the user runs `hive status --costs`
   **Then** a cost breakdown is shown with: agent name, total cost, task count

3. **Given** cost data exists
   **When** costs are queried by agent
   **Then** results are sorted by total cost descending (highest spend first)

4. **Given** a specific agent
   **When** daily cost is queried
   **Then** the system returns the total cost for that agent for the current day

5. **Given** no cost data exists
   **When** `hive status --costs` is run
   **Then** a helpful message indicates no cost data is available yet

## Tasks / Subtasks

- [x] Task 1: Cost entry types (AC: #1)
  - [x] Define `Entry` struct with AgentID, AgentName, WorkflowID, TaskID, Cost, CreatedAt fields
  - [x] Define `Summary` struct with AgentName, TotalCost, TaskCount for aggregated views
- [x] Task 2: Cost tracker core (AC: #1, #3, #4)
  - [x] Create `Tracker` struct in `internal/cost/tracker.go` wrapping `*sql.DB`
  - [x] Implement `NewTracker(db)` constructor
  - [x] Implement `Record(ctx, agentID, agentName, workflowID, taskID, cost)` -- INSERT into costs table
  - [x] Implement `ByAgent(ctx)` -- SELECT with GROUP BY agent_name, ORDER BY SUM(cost) DESC
  - [x] Implement `DailyCostForAgent(ctx, agentName)` -- SUM where date matches today
- [x] Task 3: Task completion integration (AC: #1)
  - [x] Hook into task completion event to record cost from agent's declared `cost_per_run`
  - [x] Subscribe to `task.completed` events via event bus
  - [x] Look up agent's `cost_per_run` from capabilities and record the cost
- [x] Task 4: CLI integration (AC: #2, #5)
  - [x] Add `--costs` flag to `hive status` command
  - [x] Display cost table with columns: AGENT, TOTAL COST, TASKS
  - [x] Show "No cost data recorded yet" when costs table is empty
  - [x] Support `--json` flag for machine-readable cost output
- [x] Task 5: Tests (AC: #1, #3, #4)
  - [x] Test Record stores cost entry correctly
  - [x] Test ByAgent returns sorted summaries with correct aggregation
  - [x] Test DailyCostForAgent returns today's cost only
  - [x] Test empty state returns zero cost

## Dev Notes

### Architecture Compliance

- `internal/cost/tracker.go` -- standalone package with `Tracker` struct wrapping `*sql.DB`
- Uses direct SQL with prepared statements -- no ORM, consistent with project patterns
- Cost data stored in `costs` table created by v0.3 migration (Story 17.1)
- Structured logging via `slog` for cost recording events
- Tests use `storage.Open(t.TempDir())` pattern for isolated test databases

### Key Design Decisions

- Cost tracker is a passive recorder -- it does not enforce limits (that is Story 16.4 Budget Alerts)
- The `ByAgent()` aggregation runs in SQL rather than Go -- efficient for large datasets
- `DailyCostForAgent()` uses SQLite's `date()` function for date comparison, avoiding timezone issues
- The tracker subscribes to `task.completed` events rather than being called directly from the task executor -- loose coupling via the event bus
- Test setup creates the costs table inline since the v0.3 migration file may not exist at test time

### Integration Points

- `internal/cost/tracker.go` -- `Tracker` struct with Record, ByAgent, DailyCostForAgent
- `internal/cost/tracker_test.go` -- unit tests with isolated SQLite databases
- `internal/cli/agent.go` -- `--costs` flag on `hive status` command
- `internal/event/bus.go` -- subscribes to `task.completed` events
- `internal/adapter/adapter.go` -- `AgentCapabilities.CostPerRun` field provides cost data

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 16.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR101, FR102]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Tracker struct with Record, ByAgent, DailyCostForAgent methods
- Entry and Summary types for cost data representation
- ByAgent returns summaries sorted by total cost descending
- DailyCostForAgent uses SQLite date() for accurate daily aggregation
- Integrated with task.completed events via event bus subscription
- CLI --costs flag on hive status with table and JSON output
- 3 unit tests covering record, aggregation, and daily cost queries

### Change Log

- 2026-04-16: Story 16.3 implemented -- cost tracker with per-agent and per-workflow tracking

### File List

- internal/cost/tracker.go (new -- Tracker with Record, ByAgent, DailyCostForAgent)
- internal/cost/tracker_test.go (new -- unit tests for cost tracking)
- internal/cli/agent.go (modified -- added --costs flag to hive status)
- internal/adapter/adapter.go (reference -- CostPerRun field in AgentCapabilities)
