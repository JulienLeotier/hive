package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupKeyManager(t *testing.T) *KeyManager {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return NewKeyManager(store.DB)
}

func TestGenerateAndValidate(t *testing.T) {
	km := setupKeyManager(t)

	rawKey, err := km.Generate(context.Background(), "test-key")
	require.NoError(t, err)
	assert.True(t, len(rawKey) > 20)
	assert.Contains(t, rawKey, "hive_")

	// Validate with correct key
	name, valid := km.Validate(context.Background(), rawKey)
	assert.True(t, valid)
	assert.Equal(t, "test-key", name)
}

func TestValidateWrongKey(t *testing.T) {
	km := setupKeyManager(t)

	_, err := km.Generate(context.Background(), "real-key")
	require.NoError(t, err)

	name, valid := km.Validate(context.Background(), "hive_wrong_key_value")
	assert.False(t, valid)
	assert.Empty(t, name)
}

func TestValidateNoKeys(t *testing.T) {
	km := setupKeyManager(t)
	name, valid := km.Validate(context.Background(), "anything")
	assert.False(t, valid)
	assert.Empty(t, name)
}

func TestListKeys(t *testing.T) {
	km := setupKeyManager(t)

	_, err := km.Generate(context.Background(), "key-a")
	require.NoError(t, err)
	_, err = km.Generate(context.Background(), "key-b")
	require.NoError(t, err)

	keys, err := km.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, "key-a", keys[0].Name)
}

func TestDeleteKey(t *testing.T) {
	km := setupKeyManager(t)

	_, err := km.Generate(context.Background(), "to-delete")
	require.NoError(t, err)

	err = km.Delete(context.Background(), "to-delete")
	require.NoError(t, err)

	assert.False(t, km.HasKeys(context.Background()))
}

func TestDeleteNonExistent(t *testing.T) {
	km := setupKeyManager(t)
	err := km.Delete(context.Background(), "ghost")
	require.Error(t, err)
}

func TestHasKeys(t *testing.T) {
	km := setupKeyManager(t)
	assert.False(t, km.HasKeys(context.Background()))

	_, err := km.Generate(context.Background(), "first")
	require.NoError(t, err)
	assert.True(t, km.HasKeys(context.Background()))
}

func TestAuthMiddlewareNoKeysAllowsAll(t *testing.T) {
	km := setupKeyManager(t)
	handler := AuthMiddleware(km)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddlewareRejectsWithoutKey(t *testing.T) {
	km := setupKeyManager(t)
	_, err := km.Generate(context.Background(), "active-key")
	require.NoError(t, err)

	handler := AuthMiddleware(km)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddlewareAcceptsValidKey(t *testing.T) {
	km := setupKeyManager(t)
	rawKey, err := km.Generate(context.Background(), "valid-key")
	require.NoError(t, err)

	handler := AuthMiddleware(km)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddlewareRejectsInvalidKey(t *testing.T) {
	km := setupKeyManager(t)
	_, err := km.Generate(context.Background(), "real-key")
	require.NoError(t, err)

	handler := AuthMiddleware(km)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer hive_fake_key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
