package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders_BasicHeadersSet(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SecurityHeaders(inner).ServeHTTP(w, req)

	h := w.Header()
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
	assert.Contains(t, h.Get("Content-Security-Policy"), "default-src 'self'")
	assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
}

func TestSecurityHeaders_HSTSOnlyOverTLS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Plain HTTP request — HSTS must NOT be emitted (browsers would cache
	// it even without TLS, bricking a future plaintext dev setup).
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SecurityHeaders(inner).ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"),
		"HSTS on plaintext HTTP is a trap — must only ship over TLS")

	// Simulate a TLS request by setting req.TLS.
	req = httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{}
	w = httptest.NewRecorder()
	SecurityHeaders(inner).ServeHTTP(w, req)
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=",
		"HSTS must be emitted once the request carries TLS state")
}

func TestSecurityHeaders_PassesThrough(t *testing.T) {
	// Ensure the wrapper doesn't swallow the response.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("tea"))
	})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SecurityHeaders(inner).ServeHTTP(w, req)
	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "tea", w.Body.String())
}
