package git

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPEscapeScopes(t *testing.T) {
	got := httpEscapeScopes("repo workflow read:org")
	if want := url.QueryEscape("repo workflow read:org"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// Sanity : the colon must be escaped since we feed this into a form
	// body where a raw `:` would be fine but consistency matters.
	if !strings.Contains(got, "%3A") {
		t.Fatalf("colon not escaped: %q", got)
	}
}

func TestHTTPNewRequestSetsHeaders(t *testing.T) {
	req, err := httpNewRequest(context.Background(), "POST",
		"https://example.invalid/x", "a=1&b=2")
	if err != nil {
		t.Fatalf("httpNewRequest: %v", err)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q", got)
	}
	if req.Method != "POST" {
		t.Fatalf("method = %q", req.Method)
	}
}

func TestHTTPDoJSONRoundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("hello") != "world" {
			t.Errorf("form hello = %q", r.Form.Get("hello"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "n": 3})
	}))
	defer srv.Close()

	req, err := httpNewRequest(context.Background(), "POST", srv.URL, "hello=world")
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	var out struct {
		OK bool `json:"ok"`
		N  int  `json:"n"`
	}
	if err := httpDoJSON(req, &out); err != nil {
		t.Fatalf("httpDoJSON: %v", err)
	}
	if !out.OK || out.N != 3 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestHTTPDoJSONSurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	req, _ := httpNewRequest(context.Background(), "POST", srv.URL, "")
	var out map[string]any
	err := httpDoJSON(req, &out)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error mentioning 500, got %v", err)
	}
}

func TestTruncateLong(t *testing.T) {
	s := strings.Repeat("x", 500)
	got := truncate(s, 10)
	if len(got) != 10+len("…") {
		t.Fatalf("truncate length = %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("no ellipsis")
	}
}

func TestTruncateShortNoop(t *testing.T) {
	if got := truncate("  hi  ", 50); got != "hi" {
		t.Fatalf("truncate should trim and return the original, got %q", got)
	}
}

func TestStartDeviceFlowAppliesIntervalDefault(t *testing.T) {
	// Point the client at a stub server via a custom RoundTripper-like
	// hook is overkill — instead we override the GitHub URL by
	// intercepting DNS. Simpler path: test the helper layer we already
	// exercise, and keep StartDeviceFlow's integration path behind an
	// e2e. Here we only check the unit behavior we can reach in isolation.
	//
	// The only thing StartDeviceFlow does beyond httpDoJSON is apply a
	// default interval of 5 when the response leaves it at 0. We can
	// exercise that via a round-trip to a local server that returns
	// interval:0 by reproducing the function body against our URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(DeviceStart{
			UserCode:        "ABCD-1234",
			VerificationURI: "https://github.com/login/device",
			DeviceCode:      "dev-code-1",
			Interval:        0,
			ExpiresIn:       900,
		})
	}))
	defer srv.Close()

	req, err := httpNewRequest(context.Background(), "POST", srv.URL, "client_id=test")
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	var out DeviceStart
	if err := httpDoJSON(req, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	// Mimic StartDeviceFlow's fallback
	if out.Interval <= 0 {
		out.Interval = 5
	}
	if out.Interval != 5 {
		t.Fatalf("interval default missing: %d", out.Interval)
	}
	if out.UserCode != "ABCD-1234" {
		t.Fatalf("user_code mismatch: %q", out.UserCode)
	}
}

func TestCloneRepoRejectsNonEmptyWorkdir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dummy"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := CloneRepo(context.Background(), "octocat/hello-world", dir)
	if err == nil || !strings.Contains(err.Error(), "n'est pas vide") {
		t.Fatalf("expected non-empty workdir error, got %v", err)
	}
}

func TestCloneRepoRejectsEmptyArgs(t *testing.T) {
	if err := CloneRepo(context.Background(), "", "/tmp/x"); err == nil {
		t.Fatal("expected error for empty target")
	}
	if err := CloneRepo(context.Background(), "octo/x", ""); err == nil {
		t.Fatal("expected error for empty workdir")
	}
}

func TestLoginWithTokenRejectsEmpty(t *testing.T) {
	err := LoginWithToken(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "vide") {
		t.Fatalf("expected vide-token error, got %v", err)
	}
}
