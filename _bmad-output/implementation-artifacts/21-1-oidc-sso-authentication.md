# Story 21.1: OIDC SSO Authentication

Status: done

## Story

As an enterprise admin,
I want users to authenticate via SSO (OpenID Connect),
so that access is managed through our identity provider.

## Acceptance Criteria

1. **Given** OIDC is configured in `hive.yaml` (issuer URL, client ID, client secret)
   **When** a user accesses the dashboard or API
   **Then** they are redirected to the OIDC provider for authentication

2. **Given** OIDC authentication succeeds
   **When** the provider redirects back to Hive
   **Then** a session is created with the user's identity and claims

3. **Given** a valid JWT token from the OIDC provider
   **When** it is included in API requests
   **Then** the server validates the token signature, expiry, and issuer

4. **Given** an expired or invalid JWT token
   **When** it is included in API requests
   **Then** the server responds with `401 Unauthorized`

## Tasks / Subtasks

- [x] Task 1: OIDC configuration (AC: #1)
  - [x] Add OIDC config fields to Config struct: issuer_url, client_id, client_secret, redirect_url, scopes
  - [x] Parse OIDC config from `hive.yaml` under `auth.oidc` section
  - [x] Environment variable overrides: `HIVE_AUTH_OIDC_ISSUER_URL`, etc.
  - [x] OIDC is optional -- when not configured, fall back to API key auth
- [x] Task 2: OIDC discovery and provider setup (AC: #1)
  - [x] Implement OIDC discovery via `.well-known/openid-configuration` endpoint
  - [x] Fetch and cache provider's JWKS (JSON Web Key Set) for token validation
  - [x] Configure authorization and token endpoints from discovery document
  - [x] Refresh JWKS periodically (default every 1h)
- [x] Task 3: Authentication flow (AC: #1, #2)
  - [x] Implement authorization code flow with PKCE
  - [x] Generate state and nonce for CSRF protection
  - [x] Handle callback: exchange code for tokens, validate ID token
  - [x] Extract user identity (sub, email, name) and groups from claims
  - [x] Create local session backed by secure cookie
- [x] Task 4: JWT validation middleware (AC: #3, #4)
  - [x] Create `OIDCMiddleware` in `internal/auth/` that validates Bearer tokens
  - [x] Validate: signature (against JWKS), expiry (exp claim), issuer (iss claim), audience (aud claim)
  - [x] Reject invalid/expired tokens with `401 Unauthorized` and clear error message
  - [x] Extract user identity from validated token for downstream authorization
  - [x] Support both API key and OIDC auth (API key checked first for backward compatibility)
- [x] Task 5: Session management (AC: #2)
  - [x] Create secure session with HttpOnly, Secure, SameSite cookies
  - [x] Session stores: user ID, email, groups, token expiry
  - [x] Session expiry matches OIDC token lifetime
  - [x] Logout endpoint clears session and redirects to OIDC provider logout
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test OIDC config parsing and defaults
  - [x] Test JWT validation with valid/expired/invalid tokens (mock JWKS)
  - [x] Test authentication flow with mock OIDC provider
  - [x] Test fallback to API key when OIDC not configured
  - [x] Test session creation and expiry

## Dev Notes

### Architecture Compliance

- OIDC is an optional enterprise feature -- system works with API key auth alone
- `internal/auth/` package handles both API key (existing) and OIDC authentication
- JWT validation uses standard library `crypto` packages -- no heavy OIDC library dependency
- Session data is minimal (user identity + groups) -- no sensitive data in cookies
- Uses `slog` for structured logging of authentication events (never logs tokens or secrets)

### Key Design Decisions

- Authorization code flow with PKCE (not implicit flow) for security best practice
- API key auth remains the primary method for agent-to-orchestrator communication; OIDC is for human users
- JWKS is cached and refreshed periodically to avoid per-request network calls
- Sessions use secure cookies with SameSite=Strict for CSRF protection
- User groups from OIDC claims feed into RBAC (Story 21.2)

### Integration Points

- internal/auth/oidc.go (new -- OIDC discovery, auth flow, JWT validation, session management)
- internal/auth/oidc_test.go (new -- OIDC tests with mock provider)
- internal/auth/rbac.go (reference -- user groups from OIDC feed into RBAC)
- internal/api/auth.go (modified -- added OIDC middleware alongside API key middleware)
- internal/api/server.go (modified -- OIDC callback route, logout route)
- internal/config/config.go (modified -- OIDC config fields)
- internal/config/config_test.go (modified -- OIDC config parsing tests)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 21 - Story 21.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR121]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- OIDC SSO with authorization code flow + PKCE for enterprise identity provider integration
- JWT validation against cached JWKS with signature, expiry, issuer, and audience checks
- Secure session management with HttpOnly/Secure/SameSite cookies
- Dual auth support: API key (agents) and OIDC (humans) coexist seamlessly
- OIDC is optional -- system falls back to API key auth when not configured
- User groups extracted from claims for RBAC integration (Story 21.2)

### Change Log

- 2026-04-16: Story 21.1 implemented -- OIDC SSO authentication with JWT validation and session management

### File List

- internal/auth/oidc.go (new -- OIDC discovery, authorization flow, JWT validation, session management)
- internal/auth/oidc_test.go (new -- mock OIDC provider tests, JWT validation, session tests)
- internal/api/auth.go (modified -- OIDC middleware alongside API key middleware)
- internal/api/server.go (modified -- OIDC callback and logout routes)
- internal/config/config.go (modified -- auth.oidc config section)
- internal/config/config_test.go (modified -- OIDC config parsing tests)
