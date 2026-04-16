# Story 18.1: Auction Engine

Status: done

## Story

As a user,
I want tasks allocated through an auction where agents bid,
so that the best agent for each task is selected automatically.

## Acceptance Criteria

1. **Given** a task is created with `allocation_strategy: "market"`
   **When** capable agents are notified
   **Then** each agent submits a bid (price + estimated duration)

2. **Given** bids have been submitted for a task
   **When** the bidding window closes
   **Then** the system selects the winner based on configured strategy (lowest cost, fastest, best reputation)

3. **Given** an auction completes
   **When** a winner is selected
   **Then** a `task.auction.won` event is emitted with bid details

4. **Given** a task with market allocation and no bids received
   **When** the bidding window expires
   **Then** the task falls back to capability-based routing with a `task.auction.no_bids` event

## Tasks / Subtasks

- [x] Task 1: Bid struct and auction data model (AC: #1)
  - [x] Define `Bid` struct with agent ID, price, estimated duration, reputation score, timestamp
  - [x] Define `Auction` struct with task ID, bids, status (open/closed/awarded), bidding window duration
  - [x] Create bids table in v1.0 migration for persistence
- [x] Task 2: AuctionEngine core (AC: #1, #2, #3)
  - [x] Create `AuctionEngine` struct in `internal/market/auction.go`
  - [x] Implement `OpenAuction(taskID, biddingWindow)` -- creates auction and notifies capable agents
  - [x] Implement `SubmitBid(taskID, agentID, price, estimatedDuration)` -- validates and stores bid
  - [x] Implement `CloseAuction(taskID)` -- evaluates bids and selects winner per strategy
  - [x] Implement bidding window timer with automatic close
- [x] Task 3: Winner selection strategies (AC: #2)
  - [x] `LowestCost` strategy -- selects bid with lowest price
  - [x] `Fastest` strategy -- selects bid with shortest estimated duration
  - [x] `BestReputation` strategy -- selects bid from agent with highest trust level
  - [x] `Balanced` strategy -- weighted score across cost, speed, and reputation
- [x] Task 4: Fallback routing (AC: #4)
  - [x] Detect when bidding window closes with zero bids
  - [x] Emit `task.auction.no_bids` event
  - [x] Fall back to standard capability-based routing via `task.Router`
- [x] Task 5: Event integration (AC: #3)
  - [x] Emit `task.auction.opened` when auction starts
  - [x] Emit `task.auction.bid` for each bid received
  - [x] Emit `task.auction.won` with winning bid details (agent, price, duration)
  - [x] Emit `task.auction.no_bids` on empty auction fallback
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test auction open/close lifecycle
  - [x] Test bid submission and validation
  - [x] Test each winner selection strategy
  - [x] Test fallback when no bids received
  - [x] Test bidding window expiry auto-close
  - [x] Test concurrent bid submission safety

## Dev Notes

### Architecture Compliance

- `internal/market/auction.go` is the core package for market-based allocation
- Thread-safe via `sync.Mutex` -- auctions are accessed from multiple goroutines as agents submit bids concurrently
- Uses `slog` for structured logging of auction lifecycle events
- Event types defined in `internal/event/types.go` for integration with the event bus
- Bids persisted to SQLite `bids` table created by v1.0 migration (`004_v10.sql`)

### Key Design Decisions

- Bidding window is configurable per auction (default 5s) to balance speed vs. participation
- Winner selection strategy is configured at the workflow level in `hive.yaml`, not per task
- Fallback to capability-based routing ensures tasks are never stuck when no agents bid
- AuctionEngine has no direct dependency on event bus -- event emission is handled by the caller to keep market logic focused

### Integration Points

- `internal/market/auction.go` -- AuctionEngine, Bid, Auction types, selection strategies
- `internal/task/router.go` -- modified to check allocation_strategy before routing; delegates to AuctionEngine for market tasks
- `internal/event/types.go` -- added auction event constants
- `internal/api/server.go` -- bid submission endpoint for agents
- `internal/cli/serve.go` -- initializes AuctionEngine on server startup

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 18 - Story 18.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR105, FR106]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- AuctionEngine implements full open/bid/close/award lifecycle with configurable bidding window
- Four winner selection strategies: LowestCost, Fastest, BestReputation, Balanced (weighted composite)
- Fallback to capability-based routing when no bids received within window
- Auction events emitted at each lifecycle stage for observability
- Thread-safe bid submission with mutex-protected auction state
- Bids persisted to SQLite for audit trail and analytics

### Change Log

- 2026-04-16: Story 18.1 implemented -- auction engine with bidding, selection strategies, and fallback routing

### File List

- internal/market/auction.go (modified -- AuctionEngine, Bid, Auction structs, selection strategies)
- internal/market/auction_test.go (new -- auction lifecycle, bid submission, strategy, fallback tests)
- internal/task/router.go (modified -- allocation_strategy check, AuctionEngine delegation)
- internal/event/types.go (modified -- added auction event constants)
- internal/api/server.go (modified -- bid submission endpoint)
- internal/cli/serve.go (modified -- AuctionEngine initialization)
