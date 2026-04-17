package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/metrics"
)

// metricsMiddleware wraps an http.Handler to record requests + latency
// in Prometheus. Path is normalised to keep label cardinality bounded :
// /api/v1/projects/prj_XXX → /api/v1/projects/:id. Sinon on exploserait
// le cardinalité avec une série time-series par projet_id.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusCapturingWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)
		path := normalisePath(r.URL.Path)
		status := strconv.Itoa(wrapped.status)
		metrics.APIRequests.WithLabelValues(r.Method, path, status).Inc()
		metrics.APIRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

type statusCapturingWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCapturingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// normalisePath collapse les segments d'ID pour que la cardinalité des
// labels reste bornée. On garde la forme littérale pour les paths
// racine et on remplace les segments qui ressemblent à un ID
// (contient _XX ou > 8 caractères alphanum) par ":id".
//
// Exemples :
//
//	/api/v1/projects/prj_01ABC/phases  → /api/v1/projects/:id/phases
//	/api/v1/gh/status                  → /api/v1/gh/status
//	/metrics                           → /metrics
func normalisePath(p string) string {
	if p == "" {
		return "/"
	}
	segments := strings.Split(p, "/")
	for i, s := range segments {
		if looksLikeID(s) {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

func looksLikeID(s string) bool {
	// Segment contenant un "_" et >= 10 chars = probablement un ULID/UUID.
	if len(s) >= 10 && strings.Contains(s, "_") {
		return true
	}
	// Segment >= 20 chars = très probablement un identifiant opaque.
	if len(s) >= 20 {
		return true
	}
	return false
}
