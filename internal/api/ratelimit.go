package api

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// Rate limiter simple à token bucket par IP. Hive est un outil local
// single-user donc 120 req/min est plus que suffisant — mais si le
// dashboard est exposé derrière un reverse proxy accidentel, ça évite
// qu'un script externe puisse marteler /api/v1/projects en boucle.
//
// Algorithme : bucket de N tokens par IP, refill linéaire à rate/min.
// Pas de persistance, pas de partitionnement — un redémarrage du
// serveur reset les buckets, acceptable pour un outil local.

const (
	rateLimitBurst  = 1200         // tokens max par IP (= 20 req/sec burst)
	rateLimitRefill = time.Minute  // refill la totalité du bucket en 1 min
	rateLimitGC     = 10 * time.Minute
)

// localhostIPs sont exemptes de rate limit. Hive est un outil local
// single-user : l'operateur legitime hit localhost, et le dashboard
// Svelte peut burst sur events pendant un run BMAD (load+loadPhases+
// loadActivity sur chaque WS event x dizaines d'events/seconde).
// Rate-limiter cette IP casse l'UX pour rien. Les autres IPs (proxy,
// exposition accidentelle) restent cappees a rateLimitBurst/min.
var localhostIPs = map[string]bool{
	"127.0.0.1":             true,
	"::1":                   true,
	"localhost":             true,
	"0:0:0:0:0:0:0:1":       true,
}

type bucket struct {
	tokens   float64
	updated  time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*bucket),
		lastGC:  time.Now(),
	}
}

// allow returns true if ip has tokens left, consuming one. False means
// 429. Under the lock because bucket mutation races otherwise — the
// lock is per-limiter not per-ip, which is fine at 120 req/min scale.
func (l *rateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: rateLimitBurst - 1, updated: now}
		l.buckets[ip] = b
		l.maybeGC(now)
		return true
	}
	// Refill prorata du temps écoulé.
	elapsed := now.Sub(b.updated).Seconds()
	refill := (elapsed / rateLimitRefill.Seconds()) * float64(rateLimitBurst)
	b.tokens = min64(float64(rateLimitBurst), b.tokens+refill)
	b.updated = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *rateLimiter) maybeGC(now time.Time) {
	// Appelé sous le lock. GC les buckets inactifs depuis >10min pour
	// pas leaker de la mémoire si des IPs passent en one-shot.
	if now.Sub(l.lastGC) < rateLimitGC {
		return
	}
	cutoff := now.Add(-rateLimitGC)
	for ip, b := range l.buckets {
		if b.updated.Before(cutoff) {
			delete(l.buckets, ip)
		}
	}
	l.lastGC = now
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// rateLimitMiddleware wraps an http.Handler, rejecting requests from
// IPs over quota with 429. Appliqué à /api/ seulement — le dashboard
// statique et le WS ne subissent pas de limite.
func rateLimitMiddleware(l *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		// Localhost : aucun rate limit. Cas d'usage standard de Hive.
		if localhostIPs[ip] {
			next.ServeHTTP(w, r)
			return
		}
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			// G705: l'IP vient de headers client (tainted). On n'injecte
			// jamais la valeur brute dans le JSON — on la JSON-encode
			// pour neutraliser tout caractère spécial.
			payload, _ := json.Marshal(map[string]any{
				"error": map[string]string{
					"code":    "RATE_LIMITED",
					"message": "trop de requêtes pour " + ip,
				},
			})
			_, _ = w.Write(payload)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// Honore X-Forwarded-For si derrière un proxy (split + trim premier).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
