# Story 21.2: RBAC Roles & Permissions

Status: done

## Story

As an admin,
I want to define roles with specific permissions,
so that users only access what they're authorized to.

## Acceptance Criteria

1. **Given** roles defined: admin (full access), operator (manage agents/workflows), viewer (read-only)
   **When** a user with "viewer" role tries to register an agent
   **Then** the request is rejected with `403 Forbidden`

2. **Given** a user authenticated via OIDC with group claims
   **When** the user accesses the API
   **Then** their role is determined by their OIDC groups mapped to Hive roles

3. **Given** roles are configurable
   **When** the admin updates `hive.yaml` with custom role definitions
   **Then** the system uses the updated role-permission mappings

4. **Given** API key authentication
   **When** the API key has an associated role
   **Then** permissions are enforced based on the key's role

## Tasks / Subtasks

- [x] Task 1: Role and permission model (AC: #1)
  - [x] Define `Role` struct with name and list of permissions
  - [x] Define built-in roles: admin (all), operator (agents, workflows, tasks), viewer (read-only)
  - [x] Define permissions: agents.read, agents.write, workflows.read, workflows.write, tasks.read, tasks.write, system.admin
  - [x] Create `roles` table in v1.0 migration for custom role persistence
- [x] Task 2: RBAC engine (AC: #1, #3)
  - [x] Implement `RBACEngine` in `internal/auth/rbac.go`
  - [x] Implement `HasPermission(userRole, permission)` -- checks role against permission
  - [x] Load role definitions from config with built-in defaults
  - [x] Support custom roles defined in `hive.yaml` under `auth.roles`
  - [x] Role hierarchy: admin > operator > viewer (admin inherits all lower permissions)
- [x] Task 3: RBAC middleware (AC: #1, #4)
  - [x] Create `RBACMiddleware(requiredPermission)` for API routes
  - [x] Extract user role from OIDC session or API key record
  - [x] Return 403 Forbidden with clear error message on permission denied
  - [x] Log authorization decisions with user identity and requested permission
- [x] Task 4: OIDC group-to-role mapping (AC: #2)
  - [x] Configure group-to-role mapping in `hive.yaml` under `auth.group_mapping`
  - [x] Map OIDC group claims (e.g., "hive-admins" -> admin, "hive-ops" -> operator)
  - [x] Default role for unmapped users: viewer
  - [x] Support multiple groups with highest-privilege wins
- [x] Task 5: API key role assignment (AC: #4)
  - [x] Add `role` field to api_keys table
  - [x] Modify `hive api-key generate` to accept `--role` flag (default: operator)
  - [x] Enforce RBAC on API key authenticated requests
- [x] Task 6: Route protection (AC: #1)
  - [x] Apply RBAC middleware to all API routes with appropriate permissions
  - [x] POST/PUT/DELETE endpoints require write permissions
  - [x] GET endpoints require read permissions
  - [x] Admin-only endpoints: API key management, federation, optimization
- [x] Task 7: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test built-in roles have correct permissions
  - [x] Test role hierarchy inheritance
  - [x] Test permission check with allowed and denied scenarios
  - [x] Test OIDC group-to-role mapping
  - [x] Test API key role enforcement
  - [x] Test custom role definitions from config

## Dev Notes

### Architecture Compliance

- `internal/auth/rbac.go` contains all RBAC logic, building on the existing auth package
- RBAC is enforced at the API middleware layer -- consistent across all endpoints
- Role definitions are configurable but have sensible defaults -- works out of the box
- Uses `slog` for structured logging of all authorization decisions
- Secrets (tokens, API keys) are never included in authorization log entries

### Key Design Decisions

- Three built-in roles with hierarchical permissions -- simple enough for most deployments
- Custom roles extend (don't replace) built-in roles -- prevents accidentally removing admin access
- OIDC group mapping uses highest-privilege wins when user has multiple groups
- API keys have roles too -- important for machine-to-machine access control
- Default role for unrecognized OIDC users is viewer (read-only) -- secure by default

### Integration Points

- internal/auth/rbac.go (modified -- RBACEngine, Role, Permission, middleware, group mapping)
- internal/auth/rbac_test.go (modified -- comprehensive RBAC tests)
- internal/api/auth.go (modified -- RBAC middleware integration)
- internal/api/server.go (modified -- RBAC middleware applied to all routes)
- internal/config/config.go (modified -- auth.roles and auth.group_mapping config)
- internal/cli/agent.go (reference -- API key --role flag)
- internal/storage/migrations/004_v10.sql (reference -- roles table, api_keys role column)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 21 - Story 21.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR122]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- RBACEngine with hierarchical roles: admin > operator > viewer
- Seven permissions covering agents, workflows, tasks, and system administration
- RBAC middleware enforces permissions on all API routes
- OIDC group-to-role mapping with highest-privilege-wins for multi-group users
- API keys support role assignment via --role flag
- Custom roles configurable via hive.yaml extending built-in defaults

### Change Log

- 2026-04-16: Story 21.2 implemented -- RBAC roles and permissions with OIDC group mapping and API key roles

### File List

- internal/auth/rbac.go (modified -- RBACEngine, roles, permissions, middleware, group mapping)
- internal/auth/rbac_test.go (modified -- role hierarchy, permission checks, group mapping tests)
- internal/api/auth.go (modified -- RBAC middleware integration)
- internal/api/server.go (modified -- RBAC middleware on all routes)
- internal/config/config.go (modified -- auth.roles, auth.group_mapping config sections)
- internal/config/config_test.go (modified -- RBAC config parsing tests)
- internal/storage/migrations/004_v10.sql (reference -- roles table, api_keys role column)
