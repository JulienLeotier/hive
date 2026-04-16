# Story 23.2: v1.0 End-to-End Test

Status: done

## Story

As a developer,
I want a comprehensive E2E test covering all v1.0 features,
so that I'm confident the full platform works.

## Acceptance Criteria

1. **Given** the full v1.0 system
   **When** the E2E test runs
   **Then** it exercises market allocation (auction, bidding, winner selection)

2. **Given** the full v1.0 system
   **When** the E2E test runs
   **Then** it exercises federation (mock peer, capability exchange, cross-hive routing)

3. **Given** the full v1.0 system
   **When** the E2E test runs
   **Then** it exercises optimization analysis (pattern detection, recommendations)

4. **Given** the full v1.0 system
   **When** the E2E test runs
   **Then** it exercises RBAC enforcement (role-based access, permission denied)

5. **Given** the full v1.0 system
   **When** the E2E test runs
   **Then** it exercises multi-tenant isolation (tenant creation, data separation)

6. **Given** all E2E test scenarios
   **When** the test suite completes
   **Then** all assertions pass

## Tasks / Subtasks

- [x] Task 1: E2E test infrastructure (AC: #6)
  - [x] Create `internal/e2e/v10_test.go` with `//go:build e2e` tag
  - [x] Setup: start full Hive server with all v1.0 features enabled
  - [x] Helper functions: create agents, submit tasks, wait for completion
  - [x] Teardown: clean database, stop server
  - [x] Timeout per test scenario: 60s
- [x] Task 2: Market allocation E2E (AC: #1)
  - [x] Register 3 agents with different capabilities and token balances
  - [x] Create workflow with `allocation: market`
  - [x] Verify auction opens, agents bid, winner selected
  - [x] Verify token deduction on winning agent
  - [x] Verify `task.auction.won` event emitted
- [x] Task 3: Federation E2E (AC: #2)
  - [x] Start mock federation peer (HTTP server with mTLS)
  - [x] Connect to mock peer via `federation.Connect()`
  - [x] Verify capability exchange succeeds
  - [x] Create task requiring capability only on mock peer
  - [x] Verify cross-hive routing and result return
  - [x] Verify `task.federated` event emitted
- [x] Task 4: Optimization E2E (AC: #3)
  - [x] Execute multiple workflows to generate historical data
  - [x] Run pattern analyzer
  - [x] Verify findings are generated (at least one type)
  - [x] Verify recommendations are generated from findings
  - [x] Verify recommendations include estimated impact
- [x] Task 5: RBAC E2E (AC: #4)
  - [x] Generate API key with "viewer" role
  - [x] Verify viewer can GET /api/v1/agents (read allowed)
  - [x] Verify viewer cannot POST /api/v1/agents (write denied, 403)
  - [x] Generate API key with "admin" role
  - [x] Verify admin can access all endpoints
- [x] Task 6: Multi-tenant E2E (AC: #5)
  - [x] Enable multi-tenant mode
  - [x] Create tenant A and tenant B
  - [x] Register agent in tenant A
  - [x] Verify tenant B cannot see tenant A's agent
  - [x] Create task in tenant A; verify it routes to tenant A's agent only
  - [x] Verify events are scoped to tenant
- [x] Task 7: Full pipeline E2E (AC: #1, #2, #3, #4, #5, #6)
  - [x] Single comprehensive test: register agents -> create market workflow -> run workflow -> verify auction -> check optimization data -> verify RBAC -> verify tenant isolation
  - [x] Verify no panics or unhandled errors throughout
  - [x] Assert all events are properly ordered and typed

## Dev Notes

### Architecture Compliance

- E2E tests are behind `//go:build e2e` tag to avoid running in unit test suites
- Tests use real Hive server (in-process) with SQLite for isolation
- Mock federation peer is a minimal HTTP server with mTLS for realistic testing
- All assertions use `testify/assert` and `testify/require` for clear failure messages
- Tests are independent: each scenario sets up and tears down its own state

### Key Design Decisions

- In-process server (not subprocess) for faster startup and easier debugging
- SQLite (not PostgreSQL) for E2E tests by default -- PostgreSQL E2E requires additional tag `//go:build e2e,postgres`
- Mock federation peer is lightweight: only implements capability exchange and task proxy
- Each scenario has a 60s timeout to prevent hung tests
- Full pipeline test runs last to catch integration issues between features

### Integration Points

- internal/e2e/v10_test.go (new -- all v1.0 E2E test scenarios)
- internal/e2e/helpers_test.go (new -- test helper functions for server setup, agent creation, task submission)
- internal/e2e/mock_federation_test.go (new -- mock federation peer for cross-hive testing)
- internal/market/auction.go (reference -- market allocation under test)
- internal/federation/protocol.go (reference -- federation under test)
- internal/optimizer/analyzer.go (reference -- optimization under test)
- internal/auth/rbac.go (reference -- RBAC under test)
- internal/auth/tenant.go (reference -- multi-tenant under test)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 23 - Story 23.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Comprehensive E2E test suite covering all v1.0 features behind //go:build e2e tag
- Market allocation E2E: auction lifecycle with 3 agents, bidding, token deduction
- Federation E2E: mock peer with mTLS, capability exchange, cross-hive task routing
- Optimization E2E: historical data generation, pattern analysis, recommendation verification
- RBAC E2E: viewer denied write access, admin granted full access
- Multi-tenant E2E: tenant creation, data isolation, scoped routing and events
- Full pipeline integration test exercising all features in sequence

### Change Log

- 2026-04-16: Story 23.2 implemented -- v1.0 E2E test suite with 7 test scenarios covering all features

### File List

- internal/e2e/v10_test.go (new -- v1.0 E2E test scenarios)
- internal/e2e/helpers_test.go (new -- test infrastructure and helper functions)
- internal/e2e/mock_federation_test.go (new -- mock federation peer with mTLS)
