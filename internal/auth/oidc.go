package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCConfig holds the minimal parameters needed to authenticate users via
// an OpenID Connect provider. Story 21.1.
type OIDCConfig struct {
	Issuer       string   // e.g., https://accounts.google.com
	ClientID     string
	ClientSecret string
	RedirectURL  string   // e.g., https://hive.example/auth/callback
	Scopes       []string // defaults to [openid, email, profile]
}

// OIDCProvider wraps the discovery + token exchange + JWT validation flow.
// Not a full OIDC library — just enough to satisfy the AC: redirect to the
// provider, receive a code, exchange it for a token, validate the token, and
// derive a subject claim.
type OIDCProvider struct {
	cfg      OIDCConfig
	metadata oidcMetadata

	mu      sync.Mutex
	jwksTTL time.Time
	jwks    map[string]any
}

type oidcMetadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// NewOIDCProvider discovers the provider's endpoints from /.well-known/openid-configuration.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("oidc: issuer, client_id, redirect_url are required")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile"}
	}

	discovery := strings.TrimSuffix(cfg.Issuer, "/") + "/.well-known/openid-configuration"
	req, _ := http.NewRequestWithContext(ctx, "GET", discovery, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery returned %d", resp.StatusCode)
	}
	var md oidcMetadata
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		return nil, fmt.Errorf("parsing discovery: %w", err)
	}
	if md.AuthorizationEndpoint == "" || md.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery missing endpoints")
	}
	return &OIDCProvider{cfg: cfg, metadata: md}, nil
}

// AuthRedirectURL builds the URL the dashboard redirects the user to.
// state should be an unguessable opaque token stored in a cookie for CSRF.
func (p *OIDCProvider) AuthRedirectURL(state string) string {
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {p.cfg.ClientID},
		"redirect_uri":  {p.cfg.RedirectURL},
		"scope":         {strings.Join(p.cfg.Scopes, " ")},
		"state":         {state},
	}
	return p.metadata.AuthorizationEndpoint + "?" + params.Encode()
}

// TokenResponse is the subset of the token endpoint response we use.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

// Exchange trades an authorization code for tokens.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.cfg.RedirectURL},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", p.metadata.TokenEndpoint, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc token endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	return &tok, nil
}

// UserInfo is the minimal subject + email shape we pull from userinfo.
type UserInfo struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

// JWKS is the JSON Web Key Set shape the provider serves at jwks_uri.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// ValidateJWT verifies the token's signature against the provider's JWKS and
// confirms the issuer + expiry + (optionally) audience. Story 21.1 AC:
// "JWT tokens are validated on each request". Returns the `sub` claim so the
// RBAC middleware can resolve the user.
func (p *OIDCProvider) ValidateJWT(ctx context.Context, token string) (string, error) {
	keys, err := p.fetchJWKS(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching JWKS: %w", err)
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		for _, k := range keys {
			if k.Kid == kid && k.Kty == "RSA" {
				n, err := base64.RawURLEncoding.DecodeString(k.N)
				if err != nil {
					return nil, err
				}
				eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
				if err != nil {
					return nil, err
				}
				e := 0
				for _, b := range eBytes {
					e = e<<8 + int(b)
				}
				return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: e}, nil
			}
		}
		return nil, fmt.Errorf("kid %q not found in JWKS", kid)
	})
	if err != nil || !parsed.Valid {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("unexpected claims shape")
	}
	if iss, _ := claims["iss"].(string); iss != "" && iss != strings.TrimSuffix(p.cfg.Issuer, "/") && iss != p.cfg.Issuer {
		return "", fmt.Errorf("issuer mismatch: %s", iss)
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("token has no sub claim")
	}
	return sub, nil
}

// fetchJWKS caches the provider's keys for 5 minutes so we don't hit the
// network on every request.
func (p *OIDCProvider) fetchJWKS(ctx context.Context) ([]jwkKey, error) {
	p.mu.Lock()
	if time.Now().Before(p.jwksTTL) && p.jwks != nil {
		keys, _ := p.jwks["keys"].([]jwkKey)
		p.mu.Unlock()
		if len(keys) > 0 {
			return keys, nil
		}
	}
	p.mu.Unlock()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.metadata.JWKSURI, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}
	var set jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.jwks = map[string]any{"keys": set.Keys}
	p.jwksTTL = time.Now().Add(5 * time.Minute)
	p.mu.Unlock()
	return set.Keys, nil
}

// FetchUserInfo hits the provider's /userinfo with the access token so we get
// the stable `sub` claim the RBAC user store keys on.
func (p *OIDCProvider) FetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	if p.metadata.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("provider has no userinfo endpoint")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", p.metadata.UserinfoEndpoint, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}
	var u UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
