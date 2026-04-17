package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIdP stands up a real HTTP server that implements the bits of OIDC the
// provider needs: discovery, token endpoint, userinfo, and JWKS. Lets us
// exercise the full OIDCProvider flow without a hosted IdP. Story 21.1.
type mockIdP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string
	issued string // id token issued on /token
}

func newMockIdP(t *testing.T) *mockIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	m := &mockIdP{key: key, kid: "test-key-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 base,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Redirect back with the code the caller wants.
		http.Redirect(w, r, r.URL.Query().Get("redirect_uri")+"?code=test-code&state="+r.URL.Query().Get("state"), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tok := m.signIDToken(t, "alice@example.com")
		m.issued = tok
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access",
			"id_token":     tok,
			"token_type":   "Bearer",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sub":   "alice@example.com",
			"email": "alice@example.com",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		nBytes := key.N.Bytes()
		// RSA exponent 65537 = 0x010001 → base64url(AQAB)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kid": m.kid,
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(nBytes),
				"e":   "AQAB",
			}},
		})
	})
	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockIdP) signIDToken(t *testing.T, sub string) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": m.server.URL,
		"sub": sub,
		"aud": "hive",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = m.kid
	signed, err := tok.SignedString(m.key)
	require.NoError(t, err)
	return signed
}

func (m *mockIdP) close() { m.server.Close() }

// TestOIDCFullFlow exercises discovery + auth redirect + token exchange +
// userinfo + JWT validation. Story 21.1 end-to-end.
func TestOIDCFullFlow(t *testing.T) {
	idp := newMockIdP(t)
	defer idp.close()

	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		Issuer:       idp.server.URL,
		ClientID:     "hive",
		ClientSecret: "secret",
		RedirectURL:  "http://hive.test/auth/callback",
	})
	require.NoError(t, err)

	// Auth redirect URL must point at the mock IdP's /authorize.
	redirect := provider.AuthRedirectURL("state-xyz")
	assert.Contains(t, redirect, "/authorize")
	assert.Contains(t, redirect, "state=state-xyz")

	// Token exchange returns an id_token signed by the IdP.
	tok, err := provider.Exchange(context.Background(), "test-code")
	require.NoError(t, err)
	assert.NotEmpty(t, tok.IDToken)

	// Userinfo returns the subject.
	info, err := provider.FetchUserInfo(context.Background(), tok.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", info.Subject)

	// JWT validation against the IdP's JWKS returns the sub claim.
	sub, err := provider.ValidateJWT(context.Background(), tok.IDToken)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", sub)
}

// TestOIDCValidateJWTRejectsWrongIssuer ensures we don't accept tokens signed
// by a different issuer even if the key happens to match.
func TestOIDCValidateJWTRejectsWrongIssuer(t *testing.T) {
	idp := newMockIdP(t)
	defer idp.close()

	provider, err := NewOIDCProvider(context.Background(), OIDCConfig{
		Issuer:      idp.server.URL,
		ClientID:    "hive",
		RedirectURL: "http://hive.test/cb",
	})
	require.NoError(t, err)

	// Forge a token with a bogus issuer but signed by the same key.
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": "https://evil.example.com",
		"sub": "mallory",
		"exp": now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = idp.kid
	signed, err := tok.SignedString(idp.key)
	require.NoError(t, err)

	_, err = provider.ValidateJWT(context.Background(), signed)
	assert.Error(t, err)
}
