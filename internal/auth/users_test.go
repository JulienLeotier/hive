package auth

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserStore(t *testing.T) *UserStore {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewUserStore(st.DB)
}

func TestUserUpsertAndGet(t *testing.T) {
	s := setupUserStore(t)
	ctx := context.Background()

	require.NoError(t, s.Upsert(ctx, UserRecord{Subject: "alice@example.com", Role: RoleAdmin}))
	got, err := s.Get(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role)
	assert.Equal(t, "default", got.TenantID)
}

func TestUserUpsertRejectsBadRole(t *testing.T) {
	s := setupUserStore(t)
	err := s.Upsert(context.Background(), UserRecord{Subject: "bob", Role: "god"})
	assert.Error(t, err)
}

func TestUserList(t *testing.T) {
	s := setupUserStore(t)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, UserRecord{Subject: "a", Role: RoleAdmin, TenantID: "t1"}))
	require.NoError(t, s.Upsert(ctx, UserRecord{Subject: "b", Role: RoleViewer, TenantID: "t2"}))

	users, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserDelete(t *testing.T) {
	s := setupUserStore(t)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, UserRecord{Subject: "a", Role: RoleViewer}))
	require.NoError(t, s.Delete(ctx, "a"))

	_, err := s.Get(ctx, "a")
	assert.Error(t, err)
}
