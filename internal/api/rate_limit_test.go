package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_BurstAndRefill(t *testing.T) {
	rl := NewRateLimiter(3, 60) // 3 burst, 1/s refill

	// First 3 calls from the same IP pass.
	for i := 0; i < 3; i++ {
		require.True(t, rl.allow("10.0.0.1"), "burst token %d should succeed", i+1)
	}
	// 4th call denied — bucket empty.
	assert.False(t, rl.allow("10.0.0.1"), "bucket should be empty after 3 bursts")

	// Different IP: separate bucket.
	assert.True(t, rl.allow("10.0.0.2"), "distinct IPs must have independent buckets")
}

func TestRateLimiter_DefaultsOnBadInput(t *testing.T) {
	// Zero / negative inputs should fall back to safe defaults.
	rl := NewRateLimiter(0, 0)
	assert.True(t, rl.allow("1.1.1.1"))
}

func TestClientIP_PrefersXFFOnlyFromLoopback(t *testing.T) {
	// Direct peer (not loopback) with XFF → ignore XFF, trust socket.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	assert.Equal(t, "203.0.113.5", clientIP(req),
		"XFF from non-loopback peer must be ignored to prevent spoofing")

	// Loopback peer with XFF → trust the left-most XFF entry.
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:5678"
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")
	assert.Equal(t, "9.9.9.9", clientIP(req),
		"XFF from local proxy must be trusted and left-most value chosen")
}

func TestRateLimiter_MiddlewareReturns429(t *testing.T) {
	rl := NewRateLimiter(1, 60)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := rl.Middleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:80"
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "first request under limit")

	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "second request over limit")
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}
