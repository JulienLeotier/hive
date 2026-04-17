# NFR Assessment — Hive

**Date:** 2026-04-17
**Scope:** Backend Go (`internal/`), SvelteKit dashboard (`web/`), storage (SQLite+Postgres), WS hub, federation, OIDC auth
**Method:** BMAD testarch-nfr — 4 parallel domain subagents + aggregation

---

## Verdict

| Domain | Risk | Summary |
|---|---|---|
| **Security** | 🟠 MEDIUM | Good foundations (OIDC, RBAC, parameterized SQL) but production-blocking gaps: no TLS, tenant isolation not enforced at storage, WS unauthenticated |
| **Performance** | 🟠 MEDIUM | Correlated subqueries, sequential query chains, broadcast lock contention, aggressive polling across 16 pages |
| **Reliability** | 🟢 GOOD (7.8/10) | Circuit breakers, retries, checkpoints, WAL — solid. Gaps: NATS fire-and-forget, low API test coverage (28.7%) |
| **Scalability** | 🟠 WEAK | Multi-node affinity unenforced, WS hub single-node, no event retention |
| **Maintainability** | 🟢 GOOD | 28 clean packages, 44% test file ratio, 22 docs, no API contract generation |

---

## 1. Security (MEDIUM risk)

### 🔴 CRITICAL — must fix before any prod deploy

| # | Finding | Evidence | Action |
|---|---|---|---|
| S1 | **No TLS** — plaintext HTTP exposes credentials, tokens, events | `internal/api/server.go:346-349`, `internal/cli/serve.go:208` | Add `--cert`/`--key` flags or `HIVE_TLS_CERT`/`HIVE_TLS_KEY` env; use `ListenAndServeTLS` |
| S2 | **Missing security headers** (HSTS, X-Frame-Options, CSP, X-Content-Type-Options) | No middleware detected | Add headers middleware before mux |
| S3 | **Tenant isolation not enforced at storage layer** — only test files grep `WHERE tenant_id` | `internal/storage/postgres_e2e_test.go:62` is the only match; handlers must remember to filter | Audit every `handleList*` handler; add helper `QueryWithTenant(ctx, q, args)`; write cross-tenant-access integration tests |
| S4 | **WebSocket `/ws` has no auth** — anyone reaching the port can eavesdrop on broadcasts | `internal/ws/hub.go:66-126`, `internal/cli/serve.go:188` | Wrap `hub.HandleWS` with `AuthMiddleware` or validate session cookie in upgrade handler |

### 🟡 CONCERN — harden for enterprise

| # | Finding | Evidence | Action |
|---|---|---|---|
| S5 | `/api/v1/capabilities` unauth (federation discovery exposes architecture) | `internal/api/server.go:169`, `:311-343` | Optional IP allowlist or mTLS; document trust boundary |
| S6 | Federation mTLS certs stored plaintext in DB | `internal/federation/store.go:23-36` | Envelope-encrypt with KMS/age; rotate |
| S7 | No rate limiting on auth/login surface | Zero grep hits for throttle/limit in `internal/api/` | Per-IP + per-key token bucket on auth endpoints |

### 🟢 PASS — keep as is

- OIDC JWT validation (sig, iss, exp, sub) + 5min JWKS TTL — `internal/auth/oidc.go:150-227`
- RBAC middleware on all sensitive endpoints — `internal/api/server.go:159-190`
- Parameterized queries everywhere (zero `fmt.Sprintf` in SQL) — auth, federation, cost modules
- Secrets via YAML/env, never hardcoded — `internal/config/config.go:124-142`
- Response size limits (`io.LimitReader` on OIDC/federation/hivehub)
- SvelteKit XSS-safe (zero `{@html}` in app code)
- Command exec uses arg slices (no shell interpolation) — `internal/hivehub/push.go`
- Fail-closed auth: DB error → reject, not allow — `internal/api/auth.go:126-134`

---

## 2. Performance (MEDIUM risk)

### 🟡 CONCERN

| # | Finding | Evidence | Impact | Action |
|---|---|---|---|---|
| P1 | **Correlated subqueries** in auctions handler (2 subqueries × N rows) | `internal/api/dashboard_handlers.go:135-136` | 200+ extra queries per page load | Rewrite as LEFT JOIN + aggregation |
| P2 | **4 sequential queries per poll** in `handleCosts` (5s interval ⇒ ~180 queries/min/user) | `internal/api/server.go:493-559` | Multiplies DB load linearly with users | Single CTE or materialized view w/ 10-30s TTL |
| P3 | **Broadcast holds mutex across all writes** — slow client blocks all | `internal/ws/hub.go:129-155` | Cascade failure under network stress | Per-client goroutine + buffered channel + write deadline + evict-after-N-misses |
| P4 | **16 pages poll at 5-10s** → 4-8 req/s per active user | `setInterval(load, 5000)` across `web/src/routes/*` | Scales linearly with concurrent dashboards | Raise to 15-30s; WS already pushes real-time; add ETag/304 caching |
| P5 | **No pagination on list endpoints** — `/api/v1/tasks` returns up to 500 rows, others unbounded | `internal/api/server.go:393` (hard LIMIT 500) | Large deployments: memory + network spikes | Add `?limit=N&offset=N` (or cursor pagination) to all list endpoints |
| P6 | Missing index on `bids(id)` / composite `(id, auction_id)` for `winner_bid_id` subquery | `internal/storage/migrations/*` has only `idx_bids_auction` | Full table scan on winner lookup | Add migration |

### 🟢 PASS

- Graceful shutdown timeout 5s — `internal/api/server.go:744`
- Request handlers use `r.Context()` (inherits client deadline)
- Bundle budget: 1 MB JS+CSS / 3 MB total, CI-enforced — `internal/dashboard/bundle_test.go`

---

## 3. Reliability (GOOD — 7.8/10)

### 🟢 STRONG

- **Error wrapping** systematic (`fmt.Errorf("...%w", err)`) — `internal/storage/sqlite.go:27-45`
- **Panic recovery** on subscriber callbacks — `internal/event/bus.go:159-166`
- **Circuit breaker** 3-state with configurable threshold + resetTimeout — `internal/resilience/circuit_breaker.go`
- **Retry with exponential backoff + jitter** — `internal/resilience/retry.go` (3 attempts, 200ms→2s, 20% jitter, context-aware abort)
- **HealthWatcher** auto-reassigns tasks from isolated agents — `internal/health_watcher.go:53-71`
- **Timeouts** configured everywhere: HTTP client 30s/120s, federation 10s, shutdown 5s
- **SQLite WAL + foreign keys ON** — `internal/storage/sqlite.go:59-62`
- **Checkpoint/resume supervisor** with stale detection — `internal/task/supervisor.go:16-45`

### 🟡 GAPS

| # | Finding | Evidence | Action |
|---|---|---|---|
| R1 | NATS event bridge is fire-and-forget (`_, _ = nats.Publish(...)`) | `internal/cli/serve.go:80` | Fall back to local bus on publish failure; alert on sustained failures |
| R2 | Low API test coverage (28.7%) — auth + error paths under-exercised | Coverage reports per commit `8848059` | Raise coverage gate for `internal/api` to 60% |
| R3 | `TestNATSTwoBusesShareEvents` failing | `internal/event/bus_test.go` | Investigate race; may indicate real message-loss bug |
| R4 | No `/metrics` (Prometheus) or OpenTelemetry tracing | Zero grep for metrics/otel | Add `github.com/prometheus/client_golang`; export per-handler latency + counters |
| R5 | No explicit health check endpoint | (implicit via breakers) | Add `GET /healthz` (liveness) + `/readyz` (checks DB + federation) |

---

## 4. Scalability (WEAK)

### 🔴 Multi-node deployments will break

| # | Finding | Evidence | Action |
|---|---|---|---|
| SC1 | `LocalNodeID` affinity not enforced — agents stamped but no re-routing if node dies | `internal/agent/manager.go` (`LocalNodeID` var), `internal/cluster/node.go:73-76` | Implement `RoutingMode` handler; add task rebalance on node death |
| SC2 | WS hub state is single-node in-memory — clients on node A don't see events broadcast on node B | `internal/ws/hub.go:24-27` | Bridge hub through NATS: subscribe local hub to NATS, publish on broadcast |
| SC3 | **No retention policy** on events/tasks/costs — tables grow unbounded | Zero grep hits for `DELETE FROM ... WHERE created_at <` | Add retention job (e.g. events > 90 days, completed tasks > 30 days); configurable per-table |
| SC4 | 318 uses of `context.TODO()`/`Background()` — many workers spawned without deadline | Grep in `internal/` | Audit each; propagate `r.Context()` or derive with timeout |
| SC5 | No worker pool tuning — only 2 semaphore usages (`autonomy/scheduler`, `workflow/engine`) | Grep | Expose `max_workers` per component in config |

---

## 5. Maintainability (GOOD)

### 🟢 Strengths

- **28 packages** in `internal/` with clean layering, no circular imports
- **44% test file ratio** (68 tests / 156 files), **31% test line ratio**
- **3 integration test suites** + real-infrastructure tests (commit `047eebd`)
- **22 docs** in `docs/` (architecture, federation, multi-node, configuration, etc.)
- **14 direct deps** in `go.mod` — well-chosen, no zombies
- Config structured YAML + env, sane defaults (port 8233, ~/.hive/data)

### 🟡 Gaps

| # | Finding | Action |
|---|---|---|
| M1 | **No API contract generation** — Go types + TS types hand-written, risk of drift | Generate OpenAPI from Go handlers (go-swagger or chi/openapi); emit TS client |
| M2 | **Frontend has no response DTOs** — `apiGet<T>` is generic but each page inlines `type Task = { ... }` | Centralize in `web/src/lib/types.ts` (generated from OpenAPI ideally) |
| M3 | **No runtime validation** on API responses (frontend) | Zod/Valibot schemas for critical responses |
| M4 | **No ADRs** (decision records) for clustering / affinity / federation strategy | `docs/adr/0001-*.md` incremental |
| M5 | **Root README is 3 lines** | Expand with quickstart + link to `docs/` |
| M6 | **No feature flags** — no canary/gradual rollout | Lightweight flag store (DB-backed or config-driven) |

---

## Priority Actions (rolled up)

### 🔴 Before production (CRITICAL)

1. **TLS + security headers** (S1, S2)
2. **Tenant isolation audit + integration tests** (S3)
3. **WebSocket authentication** (S4)
4. **Fix `handleListAuctions` subqueries** (P1) — quick win, 95% latency reduction
5. **Hub.Broadcast per-client goroutines** (P3) — prevents cascade under network stress

### 🟠 Next sprint (HIGH)

6. Federation cert at-rest encryption (S6)
7. Rate limiting on auth (S7)
8. Retention policies on events/tasks/costs (SC3)
9. Health check endpoints `/healthz` + `/readyz` (R5)
10. Prometheus `/metrics` + basic tracing (R4)
11. NATS bridge for multi-node WS hub (SC2)
12. Investigate failing NATS test (R3)

### 🟡 Technical debt (MEDIUM)

13. Raise poll intervals to 15-30s + ETag caching (P4)
14. Combine `handleCosts` queries into CTE (P2)
15. Add `?limit/offset` pagination to all list endpoints (P5)
16. Missing indexes on `bids` (P6)
17. Raise `internal/api` test coverage to ≥60% (R2)
18. Enforce `RoutingMode` + task rebalance on node death (SC1)
19. Audit `context.TODO()` usage (SC4)
20. OpenAPI generation + shared TS types (M1, M2, M3)
21. Expand README + add ADRs (M4, M5)
22. Feature flags for gradual rollout (M6)

---

## Methodology

- **Skill:** BMAD `bmad-testarch-nfr`
- **Subagents:** 4 parallel Explore agents (security / performance / reliability / scalability+maintainability)
- **Evidence standard:** every finding cites at least one `path:line`
- **Scope excluded:** UI design, feature completeness (covered in Epic 8 AC audit 2026-04-17)
