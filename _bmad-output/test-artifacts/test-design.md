# Test Design Document -- Hive

**Date:** 2026-04-16
**Scope:** System-level test plan covering all packages in the Hive orchestration platform
**Method:** Risk-based prioritization aligned with PRD functional requirements (FR1-FR130)

---

## 1. Risk Assessment by Package

| Package | Criticality | Risk Level | Rationale |
|---|---|---|---|
| `internal/storage` | High | Medium | Foundation layer -- all other packages depend on SQLite. Data corruption = total system failure. Current tests cover Open/Close/Migrations/WAL/Idempotency but lack CRUD and concurrent-access tests. |
| `internal/event` | High | Medium | Core event bus drives all orchestration. Failure here breaks task routing, agent coordination, and observability. Tests cover pub/sub, ordering, query. Missing: concurrent publisher stress, subscriber panic recovery verification, persistence-under-load. |
| `internal/task` | High | High | Task state machine is the heart of orchestration. Invalid state transitions could orphan work or double-execute. Tests cover happy path and events, but **missing: invalid transition rejection tests, concurrent state mutations, checkpoint-resume round-trip**. |
| `internal/agent` | High | Medium | Agent lifecycle management. Tests cover CRUD ops. Missing: UpdateHealth, concurrent registration, health-check timeout handling. |
| `internal/adapter` | High | Medium | Protocol boundary -- any adapter bug breaks agent communication. HTTP adapter well-tested (6 tests). Claude Code and MCP have minimal tests. Missing: timeout behavior, large payload, malformed JSON responses. |
| `internal/task/router` | High | High | Capability-based routing is a key differentiator. Tests cover basic matching. Missing: multi-agent tie-breaking, empty-capabilities edge case, performance with many agents. |
| `internal/resilience` | High | Medium | Circuit breaker prevents cascading failures (FR52-FR54). Well-tested (8 tests). Missing: integration with actual agent invocation, registry cleanup. |
| `internal/trust` | High | Medium | Graduated autonomy engine (FR63-FR69). Tests cover stats, promotion, manual set, no-auto-demote. Missing: full promotion ladder (supervised->guided->autonomous->trusted), threshold boundary tests. |
| `internal/auth/rbac` | Medium | Low | RBAC currently role-check only. Tests cover admin/operator/viewer/unknown. Missing: integration with HTTP middleware, permission caching. |
| `internal/api` | Medium | Medium | REST API surface. Tests cover list-agents, metrics, events, auth middleware. Missing: agent registration endpoint, task creation endpoint, error response format, pagination. |
| `internal/workflow` | Medium | Low | YAML parsing with DAG validation. Well-tested (8 tests including circular-dep detection). Missing: large workflow stress test. |
| `internal/autonomy` | Medium | Low | Agent behavioral plans. Tests cover YAML parsing and scheduler. Missing: integration with actual agent wake-up cycle. |
| `internal/config` | Low | Low | Configuration loading. Well-tested (6 tests). Solid coverage of defaults, YAML, env overrides, tilde expansion. |
| `internal/knowledge` | Medium | Medium | Shared knowledge layer (FR70-FR75). Tests cover record, search, list. Missing: decay/recency logic, concurrent writes. |
| `internal/webhook` | Medium | Low | Webhook dispatch (FR80-FR83). Tests cover matching, Slack/GitHub format. Missing: retry on failure, SSRF protection verification. |
| `internal/cost` | Low | Low | Cost tracking (FR101-FR104). Tests cover record, by-agent, daily. Missing: budget alerts. |
| `internal/ws` | Medium | Medium | WebSocket hub for dashboard. **No tests exist.** |
| `internal/cli` | Medium | Medium | User-facing CLI commands. **No tests exist** (tested via manual/E2E only). |
| `internal/dashboard` | Low | Low | Static asset embedding. No logic to test. |

---

## 2. Test Coverage Gaps

### Critical Gaps (must address before MVP)

1. **No integration tests exist.** All tests are unit-level within a single package. There is no test that validates the full orchestration flow: storage -> agent registration -> task creation -> routing -> invocation -> state transitions -> event emission.

2. **Task state machine lacks negative-path tests.** Current tests only verify happy-path transitions. No test verifies that `Complete()` rejects a task in `pending` state, or that `Start()` rejects a task in `running` state.

3. **No end-to-end test of circuit breaker + failover.** The circuit breaker is tested in isolation, but there is no test that proves a failing agent triggers the breaker and tasks get rerouted (FR52-FR54).

4. **Trust engine promotion ladder is not fully exercised.** Only supervised->guided promotion is tested. The full ladder (supervised->guided->autonomous->trusted) is untested.

5. **Router + Agent Manager integration is untested.** The router reads from the agents table but is tested with hand-inserted rows, not through the Manager API.

### Moderate Gaps (address in v0.2)

6. **WebSocket hub (`internal/ws`) has zero tests.**
7. **CLI commands have zero automated tests.** All CLI logic (init, agent add/remove, serve, logs, validate) is untested.
8. **API server endpoints are minimally tested** -- only GET endpoints, no POST/DELETE.
9. **Webhook retry and SSRF validation untested.**
10. **Knowledge search relevance ranking untested.**

### Low Priority Gaps (address in v0.3+)

11. **Dashboard embedding has no tests** (low risk, just static files).
12. **Multi-node/cluster packages are stubs** (`cluster`, `federation`, `market`, `optimizer`, `audit`) with no tests.

---

## 3. Priority Areas for Additional Testing

### P0 -- Integration Tests (this sprint)

- Full orchestration flow: create storage -> register agent -> create task -> route -> invoke -> verify state transitions + events
- Circuit breaker integration: failing agent -> breaker opens -> tasks rerouted
- Trust engine promotion: run enough tasks to trigger full promotion ladder

### P1 -- Negative Path Unit Tests (this sprint)

- Task state machine: invalid transitions (e.g., pending->completed, running->assigned)
- Agent registration: unreachable agent, malformed capabilities response
- Router: no capable agent scenario with task creation (end-to-end)

### P2 -- API Endpoint Tests (next sprint)

- POST /api/v1/agents (register via API)
- POST /api/v1/tasks (create via API)
- DELETE /api/v1/agents/:name
- Error response format validation
- Authentication required for all mutating endpoints

### P3 -- Stress and Concurrency (v0.2)

- Concurrent task creation and assignment
- Event bus under high publish rate (1000 events/sec target per NFR4)
- Multiple agents competing for same task type

---

## 4. Test Strategy

### Unit Tests (existing + gaps)

**Scope:** Single package, mocked dependencies
**Location:** `*_test.go` files alongside source
**Framework:** Go stdlib `testing` + `testify/assert` + `testify/require`
**Database:** Each test creates a fresh `storage.Open(t.TempDir())` -- full isolation

**Current state by package:**

| Package | Test Count | Coverage Quality |
|---|---|---|
| storage | 7 | Good -- Open, Close, migrations, WAL, idempotent, schema tracking, permissions |
| config | 6 | Excellent -- defaults, YAML, env override, precedence, tilde expansion |
| adapter (HTTP) | 7 | Good -- all 5 protocol methods + error + interface compliance |
| adapter (Claude Code) | 3 | Minimal -- declare, health, checkpoint only |
| adapter (MCP) | 3 | Minimal -- declare with server, fallback, health delegation |
| agent/manager | 7 | Good -- register, duplicate, list, empty, remove, not-found, get-by-name |
| api/auth | 8 | Good -- generate, validate, list, delete, has-keys, middleware (4 cases) |
| api/server | 3 | Minimal -- list agents, metrics, events GET only |
| event/bus | 7 | Good -- publish, subscribe prefix, wildcard, ordering, query by type/source/limit/since |
| task/task | 7 | Good -- create, state machine, fail, checkpoint, list-by-workflow, list-pending, events emitted |
| task/router | 4 | Good -- find capable, different type, skip unhealthy, none available |
| trust/engine | 5 | Good -- stats empty/with-tasks, evaluate no-promote, promote to guided, manual set, no-auto-demote |
| resilience | 8 | Excellent -- all states, transitions, registry, all-states view |
| workflow/parser | 8 | Excellent -- valid, missing name, no tasks, duplicate, unknown dep, self-dep, circular, topological sort |
| autonomy/plan | 5 | Good -- identity parse, plan parse, missing fields, invalid state/transition |
| autonomy/scheduler | 4 | Good -- register, unregister, active count, stop-all |
| auth/rbac | 4 | Good -- admin, operator, viewer, unknown role |
| knowledge | 5 | Good -- record, failure, search, empty, limit, list-by-type |
| webhook | 6 | Good -- add/list, dispatch matching/non-matching, slack/github format, filter matching |
| cost | 2 | Minimal -- record+by-agent, daily cost |

### Integration Tests (new -- Deliverable B)

**Scope:** Cross-package flows using real SQLite and httptest servers
**Location:** `internal/integration/acceptance_test.go`
**Approach:** Tests compose multiple packages (storage, agent, event, task, resilience, trust) and verify the full orchestration lifecycle

### End-to-End Tests (future)

**Scope:** Full binary execution via CLI
**Approach:** Build the binary, run `hive init`, `hive add-agent`, `hive run`, validate output
**Blocked by:** CLI test harness not yet built

---

## 5. Non-Functional Requirements Test Coverage

| NFR | Current Coverage | Gap |
|---|---|---|
| NFR1 (event latency <200ms) | Not tested | Need benchmark test |
| NFR4 (1000 events/sec) | Not tested | Need load test |
| NFR5 (hot-swap zero data loss) | Not tested | Need integration test |
| NFR7 (event ordering) | Tested (TestEventOrdering) | Covered |
| NFR8 (ACID compliance) | Implicitly tested via SQLite WAL | Partially covered |
| NFR11 (payload validation) | Not tested | Need adapter fuzzing |
| NFR12 (file permissions) | TestDirectoryPermissions exists | Partial -- only checks dir exists |
