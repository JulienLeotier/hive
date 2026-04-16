# Story 16.4: Budget Alerts

Status: done

## Story

As a user,
I want to set budget alerts so I'm notified when spending exceeds thresholds,
so that I avoid runaway AI costs.

## Acceptance Criteria

1. **Given** a budget alert configured via `hive budget set --agent code-reviewer --daily-limit 10`
   **When** the agent's daily cost exceeds $10
   **Then** a `cost.alert` event is emitted

2. **Given** a `cost.alert` event is emitted
   **When** webhooks are configured for `cost.alert` events
   **Then** webhook notifications fire

3. **Given** a budget alert has triggered
   **When** the user runs `hive status`
   **Then** the alert is shown in the status output

4. **Given** no budget alert is configured for an agent
   **When** the agent incurs costs
   **Then** no alert is emitted regardless of the amount

5. **Given** a budget alert was already triggered today
   **When** additional costs are incurred for the same agent
   **Then** the alert is not re-emitted (one alert per day per agent)

## Tasks / Subtasks

- [x] Task 1: Budget alert types (AC: #1)
  - [x] Define `BudgetAlert` struct with AgentName, DailyLimit, LastTriggered fields
  - [x] Store budget alerts in `budget_alerts` table (created by v0.3 migration)
- [x] Task 2: Budget alert CRUD (AC: #1, #4)
  - [x] Implement `SetBudgetAlert(ctx, agentName, dailyLimit)` -- upsert into budget_alerts table
  - [x] Implement `GetBudgetAlert(ctx, agentName)` -- retrieve alert config for an agent
  - [x] Implement `ListBudgetAlerts(ctx)` -- list all configured alerts with current status
  - [x] Implement `RemoveBudgetAlert(ctx, agentName)` -- delete alert config
- [x] Task 3: Alert evaluation (AC: #1, #2, #5)
  - [x] After each cost recording, check if agent has a budget alert configured
  - [x] Compare daily cost (from `Tracker.DailyCostForAgent()`) against configured limit
  - [x] If limit exceeded and not already triggered today, emit `cost.alert` event
  - [x] Update `last_triggered` timestamp to prevent duplicate alerts
  - [x] Event payload includes: agent name, daily limit, current cost, overage amount
- [x] Task 4: CLI commands (AC: #1, #3)
  - [x] Create `budgetCmd` cobra command group
  - [x] `hive budget set --agent <name> --daily-limit <amount>` -- configure alert
  - [x] `hive budget list` -- show all budget alerts with current daily spend
  - [x] `hive budget remove --agent <name>` -- remove alert
  - [x] Integrate active alerts into `hive status` output
- [x] Task 5: Tests (AC: #1, #4, #5)
  - [x] Test alert triggers when daily cost exceeds limit
  - [x] Test alert does not trigger when cost is under limit
  - [x] Test alert does not re-trigger on same day
  - [x] Test no alert when no budget configured for agent

## Dev Notes

### Architecture Compliance

- Budget alert logic extends `internal/cost/tracker.go` -- keeps cost-related functionality co-located
- Alert evaluation hooks into the existing cost recording flow -- evaluated after each `Record()` call
- Uses `cost.alert` event type for consistency with the event bus naming conventions
- Budget alerts stored in dedicated `budget_alerts` table for clean separation from cost data
- CLI commands follow existing cobra subcommand patterns

### Key Design Decisions

- One-alert-per-day-per-agent deduplication prevents notification fatigue -- the `last_triggered` field tracks when the alert last fired
- Alert evaluation is synchronous after cost recording to ensure immediate notification -- the event bus handles async delivery to webhooks
- The daily limit is per-agent, not per-workflow -- this provides a clear spend cap at the agent level regardless of which workflows are running
- `cost.alert` event includes both the limit and current cost so webhook consumers can format appropriate messages
- Budget alert CRUD operations use upsert (INSERT OR REPLACE) to make `hive budget set` idempotent

### Integration Points

- `internal/cost/tracker.go` -- budget alert evaluation after `Record()`, CRUD methods
- `internal/cost/tracker_test.go` -- extended tests for budget alert behavior
- `internal/event/bus.go` -- `cost.alert` events published via event bus
- `internal/event/types.go` -- `CostAlert` constant (if added)
- `internal/cli/agent.go` -- budget alerts shown in `hive status`
- `internal/cli/` -- `budgetCmd` cobra command group
- `internal/webhook/` -- delivers `cost.alert` events to configured webhooks

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 16.4]
- [Source: _bmad-output/planning-artifacts/prd.md#FR103, FR104]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- BudgetAlert struct with AgentName, DailyLimit, LastTriggered fields
- CRUD methods: SetBudgetAlert, GetBudgetAlert, ListBudgetAlerts, RemoveBudgetAlert
- Alert evaluation runs after each cost Record() -- compares daily cost against limit
- Deduplication: one alert per day per agent via last_triggered timestamp
- cost.alert event emitted with agent name, limit, current cost, overage
- CLI: hive budget set/list/remove commands, alerts shown in hive status
- 4 tests covering trigger, under-limit, dedup, and no-config scenarios

### Change Log

- 2026-04-16: Story 16.4 implemented -- budget alerts with daily limit tracking and deduplication

### File List

- internal/cost/tracker.go (modified -- added budget alert CRUD and evaluation)
- internal/cost/tracker_test.go (modified -- added budget alert tests)
- internal/cli/agent.go (modified -- budget alerts in hive status)
- internal/cli/ (modified -- added budgetCmd cobra command group)
- internal/event/types.go (modified -- added CostAlert event constant)
