package api

import (
	"context"

	"github.com/JulienLeotier/hive/internal/auth"
)

// tenantFilter returns a SQL fragment and args appropriate for filtering a
// multi-tenant table by tenant_id. Three cases:
//
//  1. Context has a non-empty tenant → scope to that tenant.
//  2. Empty tenant but caller is admin → no filter (cross-tenant view).
//     This is the dev-mode path (no user store attached) and the explicit
//     admin-superuser path for ops tooling.
//  3. Empty tenant and caller is not admin → fail closed (no rows).
//
// A6 guard: previously we defaulted unknown callers to tenant "default", which
// collides with a real customer who names their tenant "default" — they could
// see dev-mode rows, or dev-mode queries could leak their data. Now the
// sentinel is an empty string combined with an admin role, which cannot be
// produced by a customer's config.
//
// Usage:
//
//	clause, args := tenantFilter(ctx, "t")
//	query := "SELECT ... FROM tasks t WHERE 1=1" + clause
//	rows, err := db.QueryContext(ctx, query, args...)
//
// Pass the table alias (or empty string for unqualified column). The helper
// intentionally emits " AND ..." so it composes with an existing WHERE 1=1.
func tenantFilter(ctx context.Context, alias string) (string, []any) {
	tenant, _ := auth.TenantFromContext(ctx)
	if tenant != "" {
		col := "tenant_id"
		if alias != "" {
			col = alias + "." + col
		}
		return " AND " + col + " = ?", []any{tenant}
	}
	if role, ok := auth.RoleFromContext(ctx); ok && role == auth.RoleAdmin {
		return "", nil // admin cross-tenant view
	}
	return " AND 1=0", nil // fail closed
}

// tenantFromCtx returns the tenant; empty string means "not scoped" (either
// admin cross-tenant or a misconfigured non-admin — callers should decide).
func tenantFromCtx(ctx context.Context) string {
	t, _ := auth.TenantFromContext(ctx)
	return t
}

// requireTenantScope returns the tenant + ok=true for scoped callers, or
// empty+true for admins (opted-in cross-tenant view). Non-admins with no
// tenant get ok=false so the handler can reject with 403. Use this for
// endpoints that pass the tenant to a manager that can filter itself.
func requireTenantScope(ctx context.Context) (string, bool) {
	tenant, _ := auth.TenantFromContext(ctx)
	if tenant != "" {
		return tenant, true
	}
	if role, ok := auth.RoleFromContext(ctx); ok && role == auth.RoleAdmin {
		return "", true
	}
	return "", false
}
