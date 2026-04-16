package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

// APIKey represents a stored API key (hash only, never the raw key).
type APIKey struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// KeyManager handles API key generation, storage, and validation.
type KeyManager struct {
	db *sql.DB
}

// NewKeyManager creates a key manager backed by the given database.
func NewKeyManager(db *sql.DB) *KeyManager {
	return &KeyManager{db: db}
}

// Generate creates a new API key, stores the bcrypt hash, and returns the raw key.
// The raw key is only returned once — it is never stored.
func (km *KeyManager) Generate(ctx context.Context, name string) (rawKey string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating random key: %w", err)
	}
	rawKey = "hive_" + hex.EncodeToString(raw)

	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing key: %w", err)
	}

	entropy := rand.Reader
	id := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(entropy, 0))

	_, err = km.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, name, key_hash) VALUES (?, ?, ?)`,
		id.String(), name, string(hash),
	)
	if err != nil {
		return "", fmt.Errorf("storing key %s: %w", name, err)
	}

	slog.Info("api key generated", "name", name)
	return rawKey, nil
}

// Validate checks if the provided raw key matches any stored key hash.
// Returns the key name if valid, empty string if invalid.
func (km *KeyManager) Validate(ctx context.Context, rawKey string) (string, bool) {
	rows, err := km.db.QueryContext(ctx, `SELECT name, key_hash FROM api_keys`)
	if err != nil {
		return "", false
	}
	defer rows.Close()

	for rows.Next() {
		var name, hash string
		if err := rows.Scan(&name, &hash); err != nil {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(rawKey)); err == nil {
			return name, true
		}
	}
	return "", false
}

// List returns all API key metadata (without hashes).
func (km *KeyManager) List(ctx context.Context) ([]APIKey, error) {
	rows, err := km.db.QueryContext(ctx, `SELECT id, name, created_at FROM api_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		var created string
		if err := rows.Scan(&k.ID, &k.Name, &created); err != nil {
			continue
		}
		k.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Delete removes an API key by name.
func (km *KeyManager) Delete(ctx context.Context, name string) error {
	result, err := km.db.ExecContext(ctx, `DELETE FROM api_keys WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("key %s not found", name)
	}
	return nil
}

// HasKeys returns true if any API keys exist in the database.
func (km *KeyManager) HasKeys(ctx context.Context) bool {
	var count int
	km.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&count)
	return count > 0
}

// AuthMiddleware returns an HTTP middleware that validates Bearer tokens.
// If no API keys exist, all requests are allowed (dev mode).
func AuthMiddleware(km *KeyManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip auth if no keys configured (dev mode)
			if !km.HasKeys(ctx) {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"data":null,"error":{"code":"UNAUTHORIZED","message":"Missing or invalid Authorization header. Use: Bearer <api-key>"}}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			keyName, valid := km.Validate(ctx, token)
			if !valid {
				http.Error(w, `{"data":null,"error":{"code":"UNAUTHORIZED","message":"Invalid API key"}}`, http.StatusUnauthorized)
				return
			}

			slog.Debug("authenticated request", "key_name", keyName, "path", r.URL.Path)
			ctx = context.WithValue(ctx, ctxKeyName, keyName)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type contextKey string

const ctxKeyName contextKey = "api_key_name"

// randomInt is a helper to avoid importing math/rand for ULID entropy.
func init() {
	// Ensure crypto/rand works for ULID generation
	_ = new(big.Int)
}
