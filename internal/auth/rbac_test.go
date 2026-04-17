package auth

import (
	"context"
	"testing"
)

func TestRoleContextRoundtrip(t *testing.T) {
	ctx := WithRole(context.Background(), RoleAdmin)
	r, ok := RoleFromContext(ctx)
	if !ok || r != RoleAdmin {
		t.Fatalf("RoleFromContext returned %q, %v; want admin, true", r, ok)
	}
}

func TestTenantContextRoundtrip(t *testing.T) {
	ctx := WithTenant(context.Background(), "default")
	s, ok := TenantFromContext(ctx)
	if !ok || s != "default" {
		t.Fatalf("TenantFromContext returned %q, %v; want default, true", s, ok)
	}
}

func TestMissingContextValuesAreSafe(t *testing.T) {
	if _, ok := RoleFromContext(context.Background()); ok {
		t.Fatal("expected no role on empty context")
	}
	if _, ok := TenantFromContext(context.Background()); ok {
		t.Fatal("expected no tenant on empty context")
	}
}
