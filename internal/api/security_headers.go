package api

import "net/http"

// SecurityHeaders wraps h with a minimal set of defensive HTTP response
// headers. We ship conservative defaults that suit a first-party dashboard:
//
//   - X-Content-Type-Options: nosniff — stops MIME sniffing on asset routes
//   - X-Frame-Options: DENY          — prevents embedding in iframes (clickjack)
//   - Referrer-Policy: no-referrer   — we never need to leak the path outward
//   - Strict-Transport-Security     — only set when the request came over TLS
//   - Content-Security-Policy        — tight self-only policy for the SPA
//
// CSP is conservative on purpose: SvelteKit's static build serves JS/CSS from
// the same origin, so 'self' suffices. If a future feature needs eval or
// remote assets, relax this policy explicitly rather than silently.
func SecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "DENY")
		hdr.Set("Referrer-Policy", "no-referrer")
		hdr.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+ // Svelte scoped styles
				"img-src 'self' data:; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'")
		if r.TLS != nil {
			// 1 year, include subdomains. Only meaningful on HTTPS.
			hdr.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		h.ServeHTTP(w, r)
	})
}
