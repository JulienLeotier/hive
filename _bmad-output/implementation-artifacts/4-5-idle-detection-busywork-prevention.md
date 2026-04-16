# Story 4.5: Idle Detection & Busywork Prevention

Status: done

## Story

As the system,
I want agents to idle gracefully when there's no work,
so that they don't waste compute generating unnecessary tasks.

## Acceptance Criteria

1. **Given** an agent wakes up and the observer finds no matching work **When** the plan evaluator runs **Then** the agent logs "idle -- no relevant work" and returns to sleep
2. **Given** an idle decision **When** it is recorded **Then** the idle decision is recorded as a normal event (`agent.idle`) via the event bus
3. **Given** agents that create tasks without backlog demand **When** detected **Then** they are flagged as "busywork generators"
4. **Given** busywork detection **When** evaluating agent behavior **Then** it uses a simple heuristic: tasks created without upstream trigger or backlog source (FR47, FR51)

## Tasks / Subtasks

- [x] Task 1: Define `idle` action in Plan ActionDef (when: "backlog.count == 0", do: "idle") (AC: #1)
- [x] Task 2: Implement idle path in WakeUpHandler -- log and return without action (AC: #1)
- [x] Task 3: Emit `agent.idle` event when agent idles (AC: #2)
- [x] Task 4: Design busywork detection heuristic (tasks created without trigger/backlog) (AC: #3, #4)
- [x] Task 5: Flag busywork pattern via event or log entry (AC: #3)

## Dev Notes

- The idle action is a first-class action in the plan state machine: `do: idle` in ActionDef
- When the WakeUpHandler finds no pending tasks matching the agent's capabilities, it takes the idle path
- Idle events use the existing event bus: `bus.Publish(ctx, "agent.idle", agentName, {reason: "no relevant work"})`
- Busywork detection heuristic: if an agent creates a task and there was no backlog demand or upstream trigger that motivated it, the task is flagged
- The anti_patterns field in AgentIdentity includes "generating busywork" as a declared constraint
- Idle is the safe default -- agents should err on the side of idling rather than inventing work
- This prevents runaway agent loops that waste LLM compute on self-generated tasks

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/autonomy/plan.go (dependency) -- ActionDef with `do: idle` support, AgentIdentity.AntiPatterns
- internal/autonomy/scheduler.go (modified) -- Idle path in WakeUpHandler, error logging on wake-up failure
- internal/event/types.go (dependency) -- Event types used for agent.idle events
