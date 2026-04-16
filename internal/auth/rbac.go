package auth

import (
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

// Policy maps roles to their allowed permissions.
var Policy = map[Role][]Permission{
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
	perms, ok := Policy[role]
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
				role = RoleViewer // default to most restrictive
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

const ctxRoleKey contextKeyType = "user_role"
