package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Rate limiting guards the auth surface. A brute-force attacker can otherwise
// burn through API keys (bcrypt verification is cheap on success, slow but
// unbounded over time) or hammer the OIDC callback state check.
//
// Design choices:
//
//   - Token bucket per client IP. Simple, no external dep. Cleanup on a timer
//     keeps the map bounded.
//   - Default 20 tokens / 60s refill — loose enough for a real user who's
//     debugging, tight enough that a script can't try 10k keys in a minute.
//   - Ignored in dev mode (no keys configured): there's nothing to brute
//     force, and local dev loops would hit the limit constantly.
//   - IP extraction prefers X-Forwarded-For only when the request came via
//     loopback, matching what a TLS-terminating proxy would inject. Trusting
//     XFF on every request is the classic spoofing foot-gun.

// RateLimiter returns a middleware that caps requests per client IP.
// capacity = max burst, refillPerMinute = tokens added per minute.
type RateLimiter struct {
	capacity         int
	refillPerMinute  int
	mu               sync.Mutex
	buckets          map[string]*bucket
	lastCleanup      time.Time
	cleanupThreshold time.Duration
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimiter builds a per-IP token bucket. Pass 0 for either field to get
// the defaults (20 burst, 60/min refill).
func NewRateLimiter(capacity, refillPerMinute int) *RateLimiter {
	if capacity <= 0 {
		capacity = 20
	}
	if refillPerMinute <= 0 {
		refillPerMinute = 60
	}
	return &RateLimiter{
		capacity:         capacity,
		refillPerMinute:  refillPerMinute,
		buckets:          make(map[string]*bucket),
		lastCleanup:      time.Now(),
		cleanupThreshold: 10 * time.Minute,
	}
}

// Middleware returns an http middleware that rejects with 429 when the
// client IP's bucket is empty.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{tokens: float64(rl.capacity), lastSeen: now}
		rl.buckets[ip] = b
	} else {
		// Refill since lastSeen. refillPerMinute / 60 = tokens per second.
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * float64(rl.refillPerMinute) / 60.0
		if b.tokens > float64(rl.capacity) {
			b.tokens = float64(rl.capacity)
		}
		b.lastSeen = now
	}

	// Opportunistic cleanup so a stream of unique IPs can't blow the map.
	if now.Sub(rl.lastCleanup) > rl.cleanupThreshold {
		for k, v := range rl.buckets {
			if now.Sub(v.lastSeen) > rl.cleanupThreshold {
				delete(rl.buckets, k)
			}
		}
		rl.lastCleanup = now
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// clientIP prefers the socket remote addr. Only honours X-Forwarded-For when
// the direct peer is loopback (i.e. a local reverse proxy). This avoids
// trusting spoofed headers from clients hitting the server directly.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if isLoopback(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the left-most, which is the original client per RFC 7239.
			for i, c := range xff {
				if c == ',' {
					return trimSpace(xff[:i])
				}
			}
			return trimSpace(xff)
		}
	}
	return host
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// trimSpace is a tiny dep-free trim since we only ever see ASCII whitespace
// in XFF. Avoids importing "strings" for one call site.
func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
