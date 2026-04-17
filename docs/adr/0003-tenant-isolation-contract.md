# ADR 0003 — Tenant Isolation Contract

**Status:** Accepted
**Date:** 2026-04-17
**Context:** Adversarial review A2, A6 / NFR S3

## Context

Hive is multi-tenant. Every agent, task, workflow, event, knowledge entry,
cost record, audit log row, budget alert, and cluster member carries a
`tenant_id` column. Handlers must filter every read by the caller's
tenant, or tenant A reads tenant B's data.

Before 2026-04-17, `auth.TenantFromContext` was defined but never called
from handlers. Every list endpoint leaked across tenants silently. We
discovered this in the NFR audit.

## Decision

1. **Every list handler on `/api/v1/*` filters by tenant.** The policy
   lives in `internal/api/tenant_filter.go`:

   - Non-empty tenant + any role → scope to that tenant.
   - Empty tenant + admin → no filter (cross-tenant view — admin ops
     tooling only; dev mode defaults here).
   - Empty tenant + non-admin → `AND 1=0` (no rows). Fail-closed.

2. **"default" is not a special value.** Earlier code defaulted unknown
   callers to the literal tenant `"default"`, which collides with a
   customer who names their tenant `"default"`. The new sentinel is
   empty string, which cannot be produced by a customer's config.

3. **Tables with a natural tenant get a `tenant_id` column.** Done for
   agents, tasks, workflows, events, knowledge, audit_log, costs,
   budget_alerts, cluster_members. Tables that are global by design
   (schema_versions, api_keys, federation_links) intentionally do not
   have one.

4. **Regression tests guard the contract.** `TestTenantIsolation_*` in
   `internal/api/` seeds two tenants' data and asserts a tenant-A caller
   never sees tenant-B rows across agents / tasks / events / workflows /
   knowledge / audit / cluster.

## Consequences

- API handlers MUST use `tenantFilter(ctx, alias)` when writing a new
  list endpoint. PR reviewers check for this.
- `federation_links` is intentionally global — federation is an
  infrastructure concern, not a tenant concern. If federation ever
  becomes per-tenant, a migration + ADR amendment is required.
- CLI commands (`hive add-agent`, `hive run`, etc.) have their own
  tenant handling via `--tenant` flags; they bypass the HTTP RBAC flow
  and operate as `admin + explicit tenant`. Ops tooling SHOULD NOT run
  as `admin + empty tenant` in production.
