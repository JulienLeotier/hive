package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// Health endpoints follow the container-orchestrator convention:
//
//   - /healthz (liveness) — does this process respond at all? Use for
//     "should the supervisor kill me?". Returns 200 as long as the handler
//     is reachable. No auth; any restart signal must not need credentials.
//
//   - /readyz (readiness) — can this instance serve traffic? Verifies the
//     storage backend is reachable with a 2s-capped ping. Use for
//     load-balancer membership. Also unauthenticated: probes come from the
//     LB's network, often without an identity.
//
// Both refuse to leak internal details on failure (generic messages) so an
// unauthenticated probe can't enumerate the stack.

// HealthHandler returns 200 unconditionally. Liveness only.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeHealthJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// ReadyHandler returns 200 when the DB responds to a ping within 2s, else 503.
func ReadyHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			writeHealthJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
				"reason": "storage_unreachable",
			})
			return
		}
		writeHealthJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func writeHealthJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
