# Architecture Adversarial Review — Hive

**Date:** 2026-04-17
**Method:** BMAD adversarial lens, Explore subagent
**Scope:** tenant model, federation, cluster, event bus, storage, API, extensibility
**Exclusions:** NFRs (see `nfr-assessment-2026-04-17.md`)

---

## Design flaws (nommés — `TRIAGE` indique ce qui est traité dans ce pass)

| # | Finding | Evidence | Triage |
|---|---|---|---|
| A1 | **Event bus split-brain** — SQLite is source of truth locally, NATS is async gossip. Publish failure = event persisted on node A, never seen on node B. No ordering guarantee, no vector clocks. Supervisor on each node can independently re-assign the same orphaned task. | `internal/event/bus.go:54-82`, `internal/event/nats.go:87-101`, `task/supervisor.go:72-79` | **DEFERRED** — needs design doc (JetStream? Lamport clocks?) |
| A2 | **Tenant boundary gaps** — handlers filter correctly now (post-fix), but `federation_links`, `budget_alerts`, `cluster_members`, agent templates, and workflow defs have no `tenant_id`. Cross-tenant enumeration via peer lookup possible. | `storage/migrations/006_v10_features.sql:36-48,79-85`, `003_v03_budget_alerts.sql:3-11` | **PARTIAL** — fix highest-impact tables (budget_alerts, cluster_members) now; federation needs its own model |
| A3 | **Federation trust model undefined** — `/api/v1/capabilities` unauth; no cert rotation/revocation primitive; `federation/store.go:List()` filter on `status='active'` is easily spoofable. | `internal/api/server.go:169`, `/federation/store.go` | **DEFERRED** — requires dedicated key mgmt + rotation story |
| A4 | **Proxy response forgery** — federated peer returns `{task_id, status, output}` trusted blindly. No signature, no nonce, no peer identity check on the response. | `federation/proxy.go:82-87` | **DEFERRED** — protocol change across federation pairs |
| A5 | **Event source spoofing** — any `operator` can POST `/api/v1/events` with arbitrary `source`. Poisons audit + event log. | `internal/api/server.go` `handleEmitEvent` | **FIXING NOW** |
| A6 | **"default" tenant as sentinel + fallback** — time bomb when a real customer names a tenant `"default"`. | `internal/api/server.go:218-237`, `/auth/rbac.go:99` | **FIXING NOW** — use empty sentinel, fail-closed |
| A7 | **SQLite in prod** — single writer lock, `SQLITE_BUSY` under concurrent load. Not documented as dev-only. | `internal/storage/sqlite.go` | **FIXING NOW** — startup warning + doc |
| A8 | **cluster_members has no tenant** — node roster leaks across tenants. | `migrations/006:79-85` | Addressed via A2 plan |

## Open design questions (team)

- **Federation ownership**: two hives both declare `task.translate` — who owns? Task-hop TTL to prevent routing loops?
- **PickAgent determinism**: `cluster/node.go:156-182` sorts lexicographically. Balances or early-alphabet bias?
- **Adapter crash isolation**: Invoke() panic → task stuck `running` forever?
- **Workflow cycles**: `workflow/engine.go` doesn't validate DAG acyclicity.

## Compliments

- Tenant filter helper is fail-closed; all dashboard handlers wired.
- RBAC policy matrix is unexported → no runtime privilege escalation.
- Parameterized SQL everywhere; LIKE ESCAPE correct on event type filter.
- Event bus recovers panics in subscribers (`bus.go:159-166`).
