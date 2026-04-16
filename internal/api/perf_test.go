package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStatusEndpointP95 approximates Story 6.1 SLA: status responds within
// 500ms. The status CLI queries the same data, but the /api/v1/metrics
// endpoint is the HTTP reflection — measure it as a stand-in for the CLI.
func TestStatusEndpointP95(t *testing.T) {
	srv := setupServer(t)

	var worst time.Duration
	for i := 0; i < 50; i++ {
		start := time.Now()
		req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", w.Code)
		}
		d := time.Since(start)
		if d > worst {
			worst = d
		}
	}
	t.Logf("metrics endpoint worst-case = %s", worst)
	if worst > 500*time.Millisecond {
		t.Fatalf("metrics took %s, exceeds 500ms SLA", worst)
	}
}
