# Story 21.4: Multi-Tenant Support

Status: done

## Story

As a platform operator,
I want to run multiple tenants on a single Hive deployment,
so that I can offer Hive as a service.

## Acceptance Criteria

1. **Given** multi-tenant mode is enabled in `hive.yaml`
   **When** tenants are created via `hive tenant create <name>`
   **Then** each tenant has isolated: agents, workflows, tasks, events, knowledge

2. **Given** multiple tenants exist
   **When** a user authenticated to tenant A queries agents
   **Then** only tenant A's agents are returned (tenant B's data is never visible)

3. **Given** multi-tenant mode
   **When** a task is created in tenant A
   **Then** it can only be routed to tenant A's agents

4. **Given** multi-tenant mode is disabled (default)
   **When** the system operates normally
   **Then** all data is in a single default tenant (backward compatible)

## Tasks / Subtasks

- [x] Task 1: Tenant data model (AC: #1)
  - [x] Define `Tenant` struct with ID, name, created_at, status
  - [x] Create `tenants` table in v1.0 migration
  - [x] Add `tenant_id` column to all data tables: agents, tasks, workflows, events, knowledge
  - [x] Default tenant ("default") created automatically in single-tenant mode
- [x] Task 2: Tenant management (AC: #1)
  - [x] Implement `TenantManager` in `internal/auth/` or `internal/tenant/` (new package)
  - [x] Implement `CreateTenant(name)` -- creates tenant with ULID
  - [x] Implement `ListTenants()` -- returns all tenants with status
  - [x] Implement `DeleteTenant(name)` -- soft-delete tenant and cascade data isolation
  - [x] Implement `GetTenant(name)` -- returns tenant details
- [x] Task 3: Tenant isolation middleware (AC: #2, #3)
  - [x] Create `TenantMiddleware` that injects tenant ID into request context
  - [x] Determine tenant from: OIDC claim (custom claim or group prefix), API key tenant association, explicit header
  - [x] All database queries scoped by tenant_id from context
  - [x] Reject requests with no tenant association in multi-tenant mode
- [x] Task 4: Query scoping (AC: #2, #3)
  - [x] Modify `agent.Manager` queries to include `WHERE tenant_id = ?`
  - [x] Modify `task.Store` queries to include tenant scoping
  - [x] Modify `workflow.Store` queries to include tenant scoping
  - [x] Modify `event.Bus` to scope event delivery by tenant
  - [x] Modify task router to only consider same-tenant agents
- [x] Task 5: Backward compatibility (AC: #4)
  - [x] When `multi_tenant: false` (default), use "default" tenant for all operations
  - [x] Tenant middleware is a no-op in single-tenant mode -- injects "default" tenant ID
  - [x] Existing single-tenant data migrated to "default" tenant automatically
  - [x] No behavior change for users who don't enable multi-tenancy
- [x] Task 6: CLI commands (AC: #1)
  - [x] Implement `hive tenant create <name>` command
  - [x] Implement `hive tenant list` command
  - [x] Implement `hive tenant delete <name>` command with confirmation
  - [x] Support `--json` output flag
- [x] Task 7: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test tenant creation and listing
  - [x] Test data isolation: tenant A cannot see tenant B's agents/tasks/events
  - [x] Test task routing scoped to tenant
  - [x] Test backward compatibility in single-tenant mode
  - [x] Test tenant middleware injection

## Dev Notes

### Architecture Compliance

- Multi-tenancy is implemented at the data layer (tenant_id on all tables) not the infrastructure layer
- Tenant isolation is enforced at the middleware level -- every query is scoped
- Single-tenant mode is the default for backward compatibility
- Uses `slog` with tenant_id in structured log fields for per-tenant observability
- No cross-tenant data access is possible -- enforced by SQL WHERE clauses

### Key Design Decisions

- Tenant ID is injected via context, not passed as function parameters -- keeps API signatures clean
- All existing tables get a tenant_id column (nullable, default "default") for migration compatibility
- Soft-delete for tenants -- data is retained but inaccessible, allowing recovery
- OIDC integration: tenant can be derived from a custom claim or group prefix (e.g., "tenant-acme" -> tenant "acme")
- Federation operates at the tenant level -- each tenant has independent federation links

### Integration Points

- internal/auth/tenant.go (new -- TenantManager, Tenant struct, CRUD operations)
- internal/auth/tenant_test.go (new -- tenant management and isolation tests)
- internal/api/server.go (modified -- TenantMiddleware on all routes)
- internal/agent/manager.go (modified -- tenant-scoped queries)
- internal/task/router.go (modified -- tenant-scoped routing)
- internal/task/task.go (modified -- tenant-scoped task queries)
- internal/event/bus.go (modified -- tenant-scoped event delivery)
- internal/workflow/workflow.go (modified -- tenant-scoped workflow queries)
- internal/cli/tenant.go (new -- tenant create/list/delete commands)
- internal/config/config.go (modified -- multi_tenant config field)
- internal/storage/migrations/004_v10.sql (reference -- tenants table, tenant_id columns)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 21 - Story 21.4]
- [Source: _bmad-output/planning-artifacts/prd.md#FR125]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Multi-tenant support with tenant_id scoping on all data tables
- TenantManager handles CRUD with ULID-based tenant IDs
- Tenant isolation enforced via middleware injecting tenant ID into context
- All database queries scoped by tenant_id -- zero cross-tenant data leakage
- Backward compatible: single-tenant mode uses "default" tenant transparently
- CLI commands for tenant create/list/delete with --json support

### Change Log

- 2026-04-16: Story 21.4 implemented -- multi-tenant support with data isolation and backward compatibility

### File List

- internal/auth/tenant.go (new -- TenantManager, Tenant struct, CRUD, tenant middleware)
- internal/auth/tenant_test.go (new -- tenant isolation, CRUD, backward compatibility tests)
- internal/api/server.go (modified -- TenantMiddleware on all routes)
- internal/agent/manager.go (modified -- tenant-scoped queries)
- internal/task/router.go (modified -- tenant-scoped routing)
- internal/task/task.go (modified -- tenant-scoped task queries)
- internal/event/bus.go (modified -- tenant-scoped event delivery)
- internal/workflow/workflow.go (modified -- tenant-scoped workflow queries)
- internal/cli/tenant.go (new -- tenant create/list/delete commands)
- internal/config/config.go (modified -- multi_tenant config field)
- internal/storage/migrations/004_v10.sql (reference -- tenants table, tenant_id columns)
