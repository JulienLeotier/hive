package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Request-level instrumentation. The existing /api/v1/metrics endpoint
// produces gauges (agent/task counts, breakers). This file adds the
// per-handler counters and latency buckets that let you answer "which
// endpoint is slow right now", which the gauges cannot.
//
// We keep the accounting in a package-level registry so it survives handler
// rebuilds and so a single /metrics scrape sees everything. The registry is
// intentionally simple (map + mutex + fixed histogram buckets) rather than
// pulling in prometheus/client_golang — the extra 10-15 deps aren't worth
// it for the handful of series we emit.

var metricsReg = &metricsRegistry{
	counters:  map[counterKey]uint64{},
	histogram: map[counterKey][len(histBucketsS)]uint64{},
	sums:      map[counterKey]float64{},
}

// histBucketsS is in seconds. Covers sub-ms to multi-second. Matches the
// default Prometheus histogram shape close enough for an orchestrator where
// p99 is usually under a few seconds and outliers up to 10s are noteworthy.
var histBucketsS = [...]float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

type counterKey struct {
	route  string
	status int
}

type metricsRegistry struct {
	mu        sync.Mutex
	counters  map[counterKey]uint64
	histogram map[counterKey][len(histBucketsS)]uint64
	sums      map[counterKey]float64
}

// Instrument wraps h so every request it serves is counted and its latency
// bucketed. routeLabel identifies the endpoint; pass "" to skip (e.g. for
// /healthz where labels aren't useful).
func Instrument(routeLabel string, h http.Handler) http.Handler {
	if routeLabel == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusCapturingWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		h.ServeHTTP(sw, r)
		metricsReg.observe(routeLabel, sw.status, time.Since(start).Seconds())
	})
}

func (m *metricsRegistry) observe(route string, status int, dur float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := counterKey{route: route, status: status}
	m.counters[k]++
	m.sums[k] += dur
	// Find the first bucket >= dur. Cumulative histogram: also increment
	// every bucket after that (Prometheus bucket semantics are "le" = less
	// than or equal).
	buckets := m.histogram[k]
	for i, upper := range histBucketsS {
		if dur <= upper {
			buckets[i]++
		}
	}
	m.histogram[k] = buckets
}

// writePromMetrics emits the full counter + histogram set in Prometheus text
// exposition format. Called by the /metrics handler after the gauge block.
func (m *metricsRegistry) writePromMetrics(w http.ResponseWriter) {
	m.mu.Lock()
	snapshot := struct {
		counters  map[counterKey]uint64
		histogram map[counterKey][len(histBucketsS)]uint64
		sums      map[counterKey]float64
	}{
		counters:  make(map[counterKey]uint64, len(m.counters)),
		histogram: make(map[counterKey][len(histBucketsS)]uint64, len(m.histogram)),
		sums:      make(map[counterKey]float64, len(m.sums)),
	}
	for k, v := range m.counters {
		snapshot.counters[k] = v
	}
	for k, v := range m.histogram {
		snapshot.histogram[k] = v
	}
	for k, v := range m.sums {
		snapshot.sums[k] = v
	}
	m.mu.Unlock()

	keys := make([]counterKey, 0, len(snapshot.counters))
	for k := range snapshot.counters {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].status < keys[j].status
	})

	fmt.Fprintf(w, "# HELP hive_http_requests_total HTTP requests by route and status\n# TYPE hive_http_requests_total counter\n")
	for _, k := range keys {
		fmt.Fprintf(w, "hive_http_requests_total{route=%q,status=\"%d\"} %d\n",
			k.route, k.status, snapshot.counters[k])
	}

	fmt.Fprintf(w, "# HELP hive_http_request_duration_seconds Request latency histogram\n# TYPE hive_http_request_duration_seconds histogram\n")
	for _, k := range keys {
		buckets := snapshot.histogram[k]
		total := snapshot.counters[k]
		for i, upper := range histBucketsS {
			fmt.Fprintf(w, "hive_http_request_duration_seconds_bucket{route=%q,status=\"%d\",le=%q} %d\n",
				k.route, k.status, formatFloat(upper), buckets[i])
		}
		fmt.Fprintf(w, "hive_http_request_duration_seconds_bucket{route=%q,status=\"%d\",le=\"+Inf\"} %d\n",
			k.route, k.status, total)
		fmt.Fprintf(w, "hive_http_request_duration_seconds_sum{route=%q,status=\"%d\"} %f\n",
			k.route, k.status, snapshot.sums[k])
		fmt.Fprintf(w, "hive_http_request_duration_seconds_count{route=%q,status=\"%d\"} %d\n",
			k.route, k.status, total)
	}
}

func formatFloat(v float64) string {
	s := fmt.Sprintf("%g", v)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

// statusCapturingWriter captures the status code for the metrics registry.
// http.ResponseWriter doesn't expose it otherwise.
type statusCapturingWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCapturingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.status = 200
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// PromHandler returns an unauthenticated handler that emits Prometheus
// metrics (gauges + counters + histograms). Intended to be mounted at
// /metrics for scrapers. Gauges come from the existing /api/v1/metrics
// Prometheus renderer; counters/histograms come from request instrumentation.
func (s *Server) PromHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		s.handleMetricsProm(w, r)
		metricsReg.writePromMetrics(w)
	})
}
