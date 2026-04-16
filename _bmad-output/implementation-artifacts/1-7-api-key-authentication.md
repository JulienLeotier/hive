# Story 1.7: API Key Authentication

Status: done

## Story

As a user,
I want agent-to-orchestrator communication secured by API keys,
so that only authorized agents can register and execute tasks in my hive.

## Acceptance Criteria

1. **Given** the hive server is running with API keys configured
   **When** an agent makes any API call without a valid API key
   **Then** the server responds with `401 Unauthorized` and a clear error message in the standard envelope

2. **Given** the user generates a key via `KeyManager.Generate()`
   **When** the key is generated
   **Then** the API key is returned with a `hive_` prefix and 64 hex characters
   **And** only a bcrypt hash is stored in the `api_keys` table in SQLite
   **And** a key prefix is stored for O(1) lookup (avoids O(N) bcrypt scans)
   **And** the raw key is never logged — only the key name is logged

3. **Given** an agent includes a valid API key in the `Authorization: Bearer <key>` header
   **When** it calls any orchestrator endpoint
   **Then** the server validates the key via prefix lookup + bcrypt comparison
   **And** the request is logged with the key name (not the key value)

4. **Given** no API keys exist in the database
   **When** any request is made to the API
   **Then** the request is allowed without authentication (dev mode)

5. **Given** API keys exist
   **When** a request is made with an invalid or missing Bearer token
   **Then** the server responds with `401 Unauthorized` including the error code `"UNAUTHORIZED"`

6. **Given** a stored API key
   **When** the user calls `KeyManager.Delete()` with the key name
   **Then** the key is removed from the database (NFR9, NFR10)

## Tasks / Subtasks

- [x] Task 1: Database schema for API keys (AC: #2)
  - [x] `api_keys` table in `001_initial.sql` with `id`, `name`, `key_hash`, `key_prefix`, `created_at`
  - [x] UNIQUE constraint on `name`
  - [x] Index on `key_prefix` for O(1) lookup
- [x] Task 2: KeyManager implementation (AC: #2, #3, #6)
  - [x] Create `internal/api/auth.go`
  - [x] `KeyManager` struct with `db *sql.DB`
  - [x] `NewKeyManager(db)` constructor
  - [x] `Generate(ctx, name)` — 32 random bytes, hex encode with `hive_` prefix, bcrypt hash, store prefix + hash
  - [x] `Validate(ctx, rawKey)` — extract prefix, lookup by prefix (O(1)), bcrypt compare, return key name
  - [x] `List(ctx)` — return key metadata (id, name, created_at) without hashes
  - [x] `Delete(ctx, name)` — remove key, fail if not found
  - [x] `HasKeys(ctx)` — check if any keys exist (fail-closed: returns true on DB error)
- [x] Task 3: Auth middleware (AC: #1, #4, #5)
  - [x] `AuthMiddleware(km)` — returns `func(http.Handler) http.Handler`
  - [x] Skip auth if no keys configured (dev mode via `HasKeys()`)
  - [x] Extract Bearer token from `Authorization` header
  - [x] Validate token via `KeyManager.Validate()`
  - [x] Return `401` with structured error envelope on missing/invalid key
  - [x] Log authenticated request with key name (not value) via `slog.Debug`
  - [x] Store key name in request context via `context.WithValue`
- [x] Task 4: Integration with API server (AC: #1, #3)
  - [x] `Server.Handler()` wraps all routes with `AuthMiddleware`
  - [x] `serve.go` creates `KeyManager` and passes to `NewServer()`
- [x] Task 5: Auth tests (AC: #1, #2, #3, #4, #5, #6)
  - [x] Create `internal/api/auth_test.go`
  - [x] `setupKeyManager()` with temp DB
  - [x] `TestGenerateAndValidate` — generate key, validate it, verify format
  - [x] `TestValidateWrongKey` — verify rejection of invalid key
  - [x] `TestValidateNoKeys` — verify rejection when no keys exist
  - [x] `TestListKeys` — verify metadata returned without hashes
  - [x] `TestDeleteKey` — verify removal
  - [x] `TestDeleteNonExistent` — verify error
  - [x] `TestHasKeys` — verify false when empty, true after generation
  - [x] `TestAuthMiddlewareNoKeysAllowsAll` — dev mode bypass
  - [x] `TestAuthMiddlewareRejectsWithoutKey` — 401 when keys exist but no header
  - [x] `TestAuthMiddlewareAcceptsValidKey` — 200 with valid Bearer token
  - [x] `TestAuthMiddlewareRejectsInvalidKey` — 401 with wrong Bearer token

## Dev Notes

### Architecture Compliance

- **Package:** `internal/api/` — auth is part of the API layer
- **Security:** Raw keys never stored — only bcrypt hashes. Raw key returned exactly once on generation.
- **No plaintext logging:** `slog.Info("api key generated", "name", name)` — logs name, never value (NFR10)
- **Prefix optimization:** First 21 chars (`hive_` + 16 hex) stored as `key_prefix` for O(1) DB lookup before expensive bcrypt comparison
- **Fail-closed:** `HasKeys()` returns `true` on DB error — requires auth when uncertain
- **Dev mode:** No keys configured = all requests allowed (zero-friction local development)
- **Middleware pattern:** Standard Go middleware `func(http.Handler) http.Handler` wrapping all API routes
- **Error format:** `{"data":null,"error":{"code":"UNAUTHORIZED","message":"..."}}` matches API envelope spec
- **Context propagation:** Key name stored in request context for downstream use
- **Key format:** `hive_` prefix + 64 hex chars (32 random bytes) — easily identifiable, high entropy
- **bcrypt cost:** Uses `bcrypt.DefaultCost` (currently 10) — balanced security/performance

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    key_hash TEXT NOT NULL,
    key_prefix TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
```

### Testing Strategy

- 11 tests covering generation, validation, listing, deletion, and all middleware paths
- Tests use `t.TempDir()` for isolated SQLite databases
- Middleware tests use `httptest.NewRecorder` and `httptest.NewRequest`
- Both happy path and error path covered for every operation

### References

- [Source: architecture.md#Authentication & Security]
- [Source: architecture.md#Data Architecture — api_keys table]
- [Source: epics.md#Story 1.7 — NFR9, NFR10]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- KeyManager with Generate, Validate, List, Delete, HasKeys methods
- Prefix-based O(1) key lookup before bcrypt comparison
- Auth middleware with dev mode bypass, Bearer token extraction, and structured error responses
- Fail-closed design: requires auth when DB is unreachable
- Raw keys never logged or stored — only bcrypt hashes persisted
- 11 tests covering all operations and middleware paths
- Integrated with API server via Handler() method

### Change Log

- 2026-04-16: Story 1.7 implemented — API key authentication with bcrypt hashing and middleware

### File List

- internal/api/auth.go (new)
- internal/api/auth_test.go (new)
- internal/api/server.go (modified — added KeyManager dependency and auth middleware)
- internal/cli/serve.go (modified — creates KeyManager and passes to server)
- internal/storage/migrations/001_initial.sql (modified — added api_keys table with key_prefix column and index)
