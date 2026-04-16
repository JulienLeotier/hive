# Story 18.3: Token Economy

Status: done

## Story

As the system,
I want agents to accumulate internal tokens based on task completions,
so that the market has price signals for optimal allocation.

## Acceptance Criteria

1. **Given** an agent completes a task
   **When** the result is accepted
   **Then** the agent earns tokens proportional to task value

2. **Given** an agent with a token balance
   **When** the user runs `hive agent stats <name>`
   **Then** bid history, win rate, and token balance are displayed

3. **Given** the token economy is active
   **When** agents bid on tasks
   **Then** bids are constrained by token balance (agents cannot bid more than they have)

4. **Given** an auction is won
   **When** the task starts execution
   **Then** the bid amount is deducted from the winning agent's token balance

## Tasks / Subtasks

- [x] Task 1: Token balance tracking (AC: #1)
  - [x] Add `token_balance` field to agents table (INTEGER DEFAULT 100 -- starting balance)
  - [x] Implement `CreditTokens(agentID, amount)` in agent manager
  - [x] Implement `DebitTokens(agentID, amount)` in agent manager
  - [x] Token changes are atomic SQLite transactions
  - [x] Emit `agent.tokens.credited` and `agent.tokens.debited` events
- [x] Task 2: Token earning on task completion (AC: #1)
  - [x] Calculate token reward based on task value (configured in workflow YAML or default)
  - [x] Credit tokens to agent upon `task.completed` event
  - [x] Higher-value tasks earn proportionally more tokens
  - [x] Failed tasks earn zero tokens
- [x] Task 3: Bid constraints (AC: #3, #4)
  - [x] Validate bid amount does not exceed agent's token balance
  - [x] Reject bids exceeding balance with clear error
  - [x] Debit winning bid amount when auction closes and task is assigned
  - [x] Refund tokens if task fails due to system error (not agent error)
- [x] Task 4: Agent stats CLI command (AC: #2)
  - [x] Implement `hive agent stats <name>` subcommand
  - [x] Display: token balance, total bids, auctions won, win rate, total tokens earned
  - [x] Query bid history from bids table
  - [x] Support `--json` output flag
- [x] Task 5: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test token credit and debit operations
  - [x] Test atomic balance updates under concurrent access
  - [x] Test bid rejection when balance insufficient
  - [x] Test token earning on task completion
  - [x] Test token refund on system-error task failure
  - [x] Test agent stats output correctness

## Dev Notes

### Architecture Compliance

- Token balance stored in the `agents` table (new column) for transactional consistency with agent state
- All token operations use SQLite transactions to prevent negative balances under concurrency
- Token economy is opt-in: only active when workflows use `allocation: market`
- Agent stats command follows existing CLI patterns with `--json` support
- Uses `slog` for structured logging of all token movements

### Key Design Decisions

- Starting balance of 100 tokens gives new agents enough to participate in several auctions
- Token rewards are proportional to task value, not flat -- incentivizes agents to take on complex work
- Bid amount is deducted at assignment, not completion -- creates real cost to winning an auction
- Refund on system-error failures (but not agent-error) prevents punishing agents for infrastructure issues
- Token balance is an integer to avoid floating-point rounding issues

### Integration Points

- internal/agent/manager.go -- CreditTokens, DebitTokens, GetStats methods
- internal/market/auction.go -- bid validation against token balance, debit on award
- internal/cli/agent.go -- `hive agent stats` subcommand
- internal/event/types.go -- token event constants
- internal/storage/migrations/004_v10.sql -- token_balance column on agents table

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 18 - Story 18.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR108, FR109]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Token balance tracking with atomic credit/debit operations on the agents table
- Token earning proportional to task value on completion; zero for failed tasks
- Bid validation rejects bids exceeding agent's current balance
- Token deduction at auction award; refund on system-error failures
- `hive agent stats` command shows balance, bid history, win rate with --json support
- All token operations are SQLite-transactional for concurrent safety

### Change Log

- 2026-04-16: Story 18.3 implemented -- token economy with balance tracking, bid constraints, and agent stats CLI

### File List

- internal/agent/manager.go (modified -- CreditTokens, DebitTokens, GetStats methods)
- internal/agent/manager_test.go (modified -- token operation and concurrency tests)
- internal/market/auction.go (modified -- bid balance validation, token debit on award)
- internal/market/auction_test.go (modified -- bid constraint and refund tests)
- internal/cli/agent.go (modified -- added `stats` subcommand)
- internal/event/types.go (modified -- token event constants)
- internal/storage/migrations/004_v10.sql (reference -- token_balance column)
