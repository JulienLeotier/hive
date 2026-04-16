package event

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestEventBusPublishP95 asserts Story 2.1 SLA: events delivered to matching
// subscribers within 200ms p95. Runs 200 publishes and measures end-to-end
// latency (publish → subscriber invocation).
func TestEventBusPublishP95(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := NewBus(st.DB)
	latencies := make([]time.Duration, 0, 200)
	done := make(chan time.Duration, 200)

	bus.Subscribe("perf", func(e Event) {
		done <- time.Since(e.CreatedAt)
	})

	for i := 0; i < 200; i++ {
		start := time.Now()
		_, err := bus.Publish(context.Background(), "perf.tick", "bench", map[string]int{"i": i})
		require.NoError(t, err)
		// Wait for delivery (subscribers are synchronous in Bus.deliver).
		select {
		case <-done:
			latencies = append(latencies, time.Since(start))
		case <-time.After(time.Second):
			t.Fatalf("delivery stalled at i=%d", i)
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[int(float64(len(latencies))*0.95)]
	t.Logf("event bus p95 = %s (n=%d)", p95, len(latencies))
	if p95 > 200*time.Millisecond {
		t.Fatalf("p95 latency %s exceeds 200ms SLA", p95)
	}
}
