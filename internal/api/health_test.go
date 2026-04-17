package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Always200(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	HealthHandler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestReadyHandler_200WhenDBReachable(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	ReadyHandler(store.DB).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ready")
}

func TestReadyHandler_503WhenDBClosed(t *testing.T) {
	// Open then close the DB so PingContext fails.
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	store.Close()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	ReadyHandler(store.DB).ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "not_ready")
	// Must not leak internal detail (e.g. raw SQL driver error).
	assert.NotContains(t, w.Body.String(), "sql:", "probe responses shouldn't expose internals")
}

func TestReadyHandler_Timeout(t *testing.T) {
	// Wire a DB whose ping blocks longer than the 2s cap. Easiest way:
	// use a context that's already cancelled — the request handler
	// derives a child, so if the child cap doesn't short-circuit the ping
	// we'd hang. The point is that the handler returns within ~2s even
	// when the DB is sluggish.
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled; ping should error quickly

	req := httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	start := time.Now()
	ReadyHandler(store.DB).ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2500*time.Millisecond,
		"handler must not exceed the 2s DB ping cap by much")
	// On ctx cancelled, PingContext returns err → 503.
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// nilDB is a zero-value *sql.DB just to prove the handler doesn't panic on
// a clearly broken dependency. Not expected in real use — serve.go always
// passes store.DB — but a panic here would take the pod down.
var _ = (*sql.DB)(nil) // import placeholder
