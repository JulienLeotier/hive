package api

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hijackableWriter is a ResponseWriter that also implements http.Hijacker.
// Used in tests to prove the instrumentation wrapper forwards Hijack().
type hijackableWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
}

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijackCalled = true
	return nil, nil, nil // nil, nil is fine for the assert; we're only proving the call happened
}

// TestInstrument_ForwardsHijacker guards against a regression where
// statusCapturingWriter doesn't implement http.Hijacker. Without this,
// WebSocket upgrades via gorilla/websocket return 500 because the
// upgrader can't hijack the underlying connection.
func TestInstrument_ForwardsHijacker(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, ok := w.(http.Hijacker)
		require.True(t, ok, "instrumented writer must implement http.Hijacker")
		_, _, err := h.Hijack()
		require.NoError(t, err)
	})

	wrapped := Instrument("/ws", inner)

	rec := &hijackableWriter{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest("GET", "/ws", nil)
	wrapped.ServeHTTP(rec, req)

	assert.True(t, rec.hijackCalled, "Hijack() on the underlying writer must be reached")
}

// TestInstrument_RecordsMetrics sanity checks that wrapping still captures
// status + latency for a normal request.
func TestInstrument_RecordsMetrics(t *testing.T) {
	// Fresh registry state isn't exposed, so we just prove the round-trip
	// doesn't panic and writes the expected status.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("short and stout"))
	})
	wrapped := Instrument("/test", inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "short and stout", rec.Body.String())
}
