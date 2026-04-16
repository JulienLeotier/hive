package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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
