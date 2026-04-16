# Story 4.2: Heartbeat Scheduler

Status: done

## Story

As a user,
I want agents to wake up on configurable schedules,
so that they check for work at appropriate intervals.

## Acceptance Criteria

1. **Given** an agent with a configured heartbeat interval (e.g., `heartbeat: 60s`) **When** the interval elapses **Then** the scheduler triggers the agent's wake-up cycle via the registered WakeUpHandler
2. **Given** multiple agents registered **When** each has a different heartbeat interval **Then** each agent wakes up independently on its own schedule
3. **Given** a registered agent **When** it is unregistered **Then** its heartbeat timer stops and no further wake-ups occur
4. **Given** all agents need to stop **When** StopAll is called **Then** all heartbeat timers are stopped cleanly (FR44)

## Tasks / Subtasks

- [x] Task 1: Define WakeUpHandler function type (AC: #1)
- [x] Task 2: Implement Scheduler struct with timer and stop channel maps (AC: #1, #2)
- [x] Task 3: Implement Register to start a goroutine with time.Ticker for heartbeat (AC: #1, #2)
- [x] Task 4: Implement Unregister to stop a single agent's heartbeat (AC: #3)
- [x] Task 5: Implement StopAll to stop all heartbeats (AC: #4)
- [x] Task 6: Implement ActiveCount for monitoring (AC: #2)
- [x] Task 7: Handle re-registration (stop old timer before starting new) (AC: #1)
- [x] Task 8: Write tests for register/wakeup, unregister, active count, stop all (AC: #1-#4)

## Dev Notes

- Scheduler uses `time.Ticker` per agent with a dedicated stop channel for clean shutdown
- Each agent's heartbeat runs in its own goroutine, select-ing between ticker.C and stop channel
- Re-registration (Register called twice for same agent) safely stops the old timer first
- WakeUpHandler errors are logged but do not stop the heartbeat -- agents continue waking up
- Mutex protects concurrent access to timers and stopChs maps
- ActiveCount is useful for status monitoring and testing
- The handler function receives agent name and context, allowing flexible wake-up logic

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/autonomy/scheduler.go (new) -- WakeUpHandler type, Scheduler struct with Register/Unregister/StopAll/ActiveCount
- internal/autonomy/scheduler_test.go (new) -- 4 tests: register and wake-up, unregister stops, active count, stop all
