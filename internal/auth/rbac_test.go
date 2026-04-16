package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminHasAllPermissions(t *testing.T) {
	assert.True(t, CheckPermission(RoleAdmin, "agents", "write"))
	assert.True(t, CheckPermission(RoleAdmin, "system", "admin"))
	assert.True(t, CheckPermission(RoleAdmin, "anything", "everything"))
}

func TestOperatorPermissions(t *testing.T) {
	assert.True(t, CheckPermission(RoleOperator, "agents", "read"))
	assert.True(t, CheckPermission(RoleOperator, "agents", "write"))
	assert.True(t, CheckPermission(RoleOperator, "events", "read"))
	assert.False(t, CheckPermission(RoleOperator, "system", "admin"))
	assert.False(t, CheckPermission(RoleOperator, "events", "delete"))
}

func TestViewerPermissions(t *testing.T) {
	assert.True(t, CheckPermission(RoleViewer, "agents", "read"))
	assert.True(t, CheckPermission(RoleViewer, "events", "read"))
	assert.False(t, CheckPermission(RoleViewer, "agents", "write"))
	assert.False(t, CheckPermission(RoleViewer, "agents", "delete"))
}

func TestUnknownRoleDenied(t *testing.T) {
	assert.False(t, CheckPermission(Role("unknown"), "agents", "read"))
}
