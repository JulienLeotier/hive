# Story 18.2: Allocation Strategies

Status: done

## Story

As a user,
I want to configure different allocation strategies per workflow,
so that I can optimize for cost, speed, or quality depending on the use case.

## Acceptance Criteria

1. **Given** a workflow with `allocation: market`
   **When** tasks are created
   **Then** the system uses the auction engine for agent selection

2. **Given** a workflow with `allocation: round-robin`
   **When** tasks are created
   **Then** the system distributes tasks evenly across capable agents

3. **Given** a workflow with `allocation: capability-match`
   **When** tasks are created
   **Then** the system uses the existing best-fit capability-based routing

4. **Given** a workflow with an invalid allocation strategy
   **When** `hive validate` runs
   **Then** the validation reports the invalid strategy with valid options listed

## Tasks / Subtasks

- [x] Task 1: Strategy interface and registry (AC: #1, #2, #3)
  - [x] Define `AllocationStrategy` interface with `Select(task, candidates) (agentID, error)` method
  - [x] Implement `MarketStrategy` wrapping AuctionEngine from Story 18.1
  - [x] Implement `RoundRobinStrategy` tracking last-assigned index per capability group
  - [x] Implement `CapabilityMatchStrategy` wrapping existing router logic
  - [x] Create `StrategyRegistry` mapping strategy names to implementations
- [x] Task 2: Workflow-level configuration (AC: #1, #2, #3)
  - [x] Add `allocation` field to workflow YAML schema
  - [x] Parse allocation strategy in `workflow.Parser`
  - [x] Pass strategy to task router on workflow execution
  - [x] Default to `capability-match` when no strategy specified
- [x] Task 3: Round-robin implementation (AC: #2)
  - [x] Track per-capability-group assignment counter
  - [x] Select next capable agent in round-robin order
  - [x] Skip unhealthy or isolated agents
  - [x] Thread-safe counter with mutex protection
- [x] Task 4: Validation support (AC: #4)
  - [x] Add allocation strategy validation to `hive validate` command
  - [x] Report invalid strategies with list of valid options: market, round-robin, capability-match
  - [x] Validate strategy is consistent with agent capabilities
- [x] Task 5: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test MarketStrategy delegates to AuctionEngine
  - [x] Test RoundRobinStrategy distributes evenly
  - [x] Test RoundRobinStrategy skips unhealthy agents
  - [x] Test CapabilityMatchStrategy matches existing routing behavior
  - [x] Test validation rejects invalid strategy names
  - [x] Test default strategy is capability-match

## Dev Notes

### Architecture Compliance

- Strategy pattern via `AllocationStrategy` interface allows pluggable allocation without modifying the core task router
- Round-robin state is in-memory (resets on restart) -- acceptable since it's a distribution optimization, not a correctness requirement
- Strategy selection is at the workflow level, not task level -- simplifies configuration and reasoning
- Uses `slog` for logging strategy selection decisions

### Key Design Decisions

- Strategy interface is minimal: `Select(task, []Agent) (string, error)` -- returns chosen agent ID
- MarketStrategy wraps AuctionEngine rather than reimplementing -- avoids duplication with 18.1
- Round-robin tracks assignment per capability group, not globally -- ensures even distribution within each capability set
- Default strategy remains capability-match to maintain backward compatibility with existing workflows

### Integration Points

- internal/market/auction.go -- MarketStrategy wraps AuctionEngine
- internal/task/router.go -- modified to accept and use AllocationStrategy interface
- internal/workflow/parser.go -- modified to parse `allocation` field from YAML
- internal/cli/validate.go -- modified to validate allocation strategy
- internal/workflow/workflow.go -- Workflow struct extended with AllocationStrategy field

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 18 - Story 18.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR107]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- AllocationStrategy interface with three implementations: Market, RoundRobin, CapabilityMatch
- StrategyRegistry maps string names to strategy instances for runtime lookup
- Workflow YAML extended with `allocation` field defaulting to capability-match
- Round-robin tracks per-capability-group counters with mutex protection
- Validation reports invalid strategies with clear error listing valid options

### Change Log

- 2026-04-16: Story 18.2 implemented -- pluggable allocation strategies with workflow-level configuration

### File List

- internal/market/auction.go (modified -- added AllocationStrategy interface, MarketStrategy, RoundRobinStrategy, CapabilityMatchStrategy, StrategyRegistry)
- internal/market/auction_test.go (modified -- strategy tests for all three implementations)
- internal/task/router.go (modified -- accepts AllocationStrategy, delegates selection)
- internal/task/router_test.go (modified -- tests for strategy-based routing)
- internal/workflow/parser.go (modified -- parses allocation field)
- internal/workflow/parser_test.go (modified -- tests allocation parsing and defaults)
- internal/cli/validate.go (modified -- validates allocation strategy name)
