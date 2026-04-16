# Story 4.3: State Observer

Status: done

## Story

As an agent,
I want to observe the current state of the world at each wake-up,
so that I can make informed decisions about what to do.

## Acceptance Criteria

1. **Given** an agent's wake-up cycle is triggered **When** the observer runs **Then** it gathers pending tasks in the shared backlog matching agent capabilities
2. **Given** the observer runs **When** gathering context **Then** it includes recent events since last wake-up and current workflow states
3. **Given** the observation context **When** presented to the agent's plan evaluator **Then** it contains all information needed for the agent to decide (act or idle)
4. **Given** observation runs **When** querying the system **Then** it completes efficiently using existing query methods (FR45)

## Tasks / Subtasks

- [x] Task 1: Design observation context as input to WakeUpHandler (AC: #3)
- [x] Task 2: Leverage task.Store.ListPending for backlog observation (AC: #1)
- [x] Task 3: Leverage event.Bus.Query for recent events observation (AC: #2)
- [x] Task 4: Wire observation into scheduler's WakeUpHandler pattern (AC: #4)

## Dev Notes

- The state observer is not a separate file but is implemented through the WakeUpHandler callback pattern
- The WakeUpHandler receives the agent name and uses existing queries (task.ListPending, event.Bus.Query) to gather observation context
- This design follows the architecture's agent wake-up flow: Scheduler -> Wake -> Observer reads state/backlog -> Plan evaluates
- Observation data includes: pending tasks matching capabilities, recent events, and workflow status
- The handler pattern allows each agent's observation to be customized based on its Plan.States[].Observe configuration
- Observation queries are efficient -- they use indexed SQLite columns (status, type, created_at)
- No separate observer.go file was needed -- the WakeUpHandler closure pattern in the scheduler provides equivalent functionality with less indirection

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/autonomy/scheduler.go (modified) -- WakeUpHandler receives context for observation
- internal/task/task.go (dependency) -- ListPending used for backlog observation
- internal/event/bus.go (dependency) -- Query used for recent events observation
