package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Role defines a user role with permissions.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Permission defines an action on a resource.
type Permission struct {
	Resource string // "agents", "workflows", "tasks", "events", "system"
	Action   string // "read", "write", "delete", "admin"
}

// policy maps roles to their allowed permissions. Unexported to prevent runtime mutation.
var policy = map[Role][]Permission{
	RoleAdmin: {
		{Resource: "*", Action: "*"},
	},
	RoleOperator: {
		{Resource: "agents", Action: "read"},
		{Resource: "agents", Action: "write"},
		{Resource: "workflows", Action: "read"},
		{Resource: "workflows", Action: "write"},
		{Resource: "tasks", Action: "read"},
		{Resource: "tasks", Action: "write"},
		{Resource: "events", Action: "read"},
	},
	RoleViewer: {
		{Resource: "agents", Action: "read"},
		{Resource: "workflows", Action: "read"},
		{Resource: "tasks", Action: "read"},
		{Resource: "events", Action: "read"},
	},
}

// CheckPermission verifies if a role has a specific permission.
func CheckPermission(role Role, resource, action string) bool {
	perms, ok := policy[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if (p.Resource == "*" || p.Resource == resource) && (p.Action == "*" || p.Action == action) {
			return true
		}
	}
	return false
}

// RBACMiddleware enforces role-based access control on HTTP handlers.
func RBACMiddleware(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get role from context (set by auth middleware)
			role, ok := r.Context().Value(ctxRoleKey).(Role)
			if !ok {
				// No role in context — deny access (fail-closed)
				http.Error(w, `{"error":{"code":"FORBIDDEN","message":"no role in request context"}}`,
					http.StatusForbidden)
				return
			}

			if !CheckPermission(role, resource, action) {
				http.Error(w, fmt.Sprintf(`{"error":{"code":"FORBIDDEN","message":"role %s cannot %s %s"}}`, role, action, resource),
					http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type contextKeyType string

const (
	ctxRoleKey   contextKeyType = "user_role"
	ctxTenantKey contextKeyType = "tenant_id"
)

// WithRole stashes a role in the context so RBACMiddleware can read it.
// Called by a resolver middleware after authentication.
func WithRole(ctx context.Context, role Role) context.Context {
	return context.WithValue(ctx, ctxRoleKey, role)
}

// WithTenant stashes a tenant id in the context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, tenantID)
}

// RoleFromContext returns the role stored by WithRole, if any.
func RoleFromContext(ctx context.Context) (Role, bool) {
	r, ok := ctx.Value(ctxRoleKey).(Role)
	return r, ok
}

// TenantFromContext returns the tenant id stored by WithTenant, if any.
func TenantFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(ctxTenantKey).(string)
	return s, ok
}
