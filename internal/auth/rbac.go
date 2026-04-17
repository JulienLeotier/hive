package auth

import "context"

// Hive is a single-user local tool — the multi-role RBAC system is
// gone. What remains is a tiny context plumbing so every request gets
// a synthetic admin role and a tenant id for queries that still scope
// by tenant.
//
// If you're bringing RBAC back, wire a real middleware in internal/api
// — don't resurrect the pre-pivot policy table.

type Role string

const RoleAdmin Role = "admin"

type contextKeyType string

const (
	ctxRoleKey   contextKeyType = "user_role"
	ctxTenantKey contextKeyType = "tenant_id"
)

func WithRole(ctx context.Context, role Role) context.Context {
	return context.WithValue(ctx, ctxRoleKey, role)
}

func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, tenantID)
}

func RoleFromContext(ctx context.Context) (Role, bool) {
	r, ok := ctx.Value(ctxRoleKey).(Role)
	return r, ok
}

func TenantFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(ctxTenantKey).(string)
	return s, ok
}
