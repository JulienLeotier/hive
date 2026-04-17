package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitAllowsUnderBurst(t *testing.T) {
	l := newRateLimiter()
	for i := 0; i < rateLimitBurst; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("should allow request %d under burst", i)
		}
	}
}

func TestRateLimitBlocksAfterBurst(t *testing.T) {
	l := newRateLimiter()
	for i := 0; i < rateLimitBurst; i++ {
		l.allow("9.9.9.9")
	}
	if l.allow("9.9.9.9") {
		t.Fatal("request past burst should be rejected")
	}
}

func TestRateLimitIsolatedPerIP(t *testing.T) {
	l := newRateLimiter()
	for i := 0; i < rateLimitBurst; i++ {
		l.allow("5.5.5.5")
	}
	if !l.allow("6.6.6.6") {
		t.Fatal("other IP must have its own bucket")
	}
}

func TestRateLimitMiddleware429(t *testing.T) {
	l := newRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := rateLimitMiddleware(l, next)

	for i := 0; i < rateLimitBurst; i++ {
		req := httptest.NewRequest("GET", "/x", nil)
		req.RemoteAddr = "10.0.0.1:11111"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("req %d: want 200, got %d", i, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "10.0.0.1:11111"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After missing: %q", w.Header().Get("Retry-After"))
	}
}

func TestClientIPRespectsXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if ip := clientIP(req); ip != "1.2.3.4" {
		t.Fatalf("XFF split: got %q", ip)
	}
}

func TestClientIPStripsPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "192.168.1.42:5555"
	if ip := clientIP(req); ip != "192.168.1.42" {
		t.Fatalf("port strip: got %q", ip)
	}
}
