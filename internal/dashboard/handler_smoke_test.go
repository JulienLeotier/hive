package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServesPrerenderedRoute(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/projects", "/events", "/audit"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, p, nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status %d", p, rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "<!doctype html>") {
			t.Fatalf("%s: not html: %q", p, body[:min(80, len(body))])
		}
		if !strings.Contains(body, `./_app/immutable/entry/start.`) {
			t.Fatalf("%s: expected prerendered html with relative asset paths, got index fallback", p)
		}
	}
}

func TestHandler_SPAFallback(t *testing.T) {
	h := Handler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent-route", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `/_app/immutable/entry/start.`) {
		t.Fatalf("expected index fallback (absolute paths), got: %q", body[:min(200, len(body))])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
