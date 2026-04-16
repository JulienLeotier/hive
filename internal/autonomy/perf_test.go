package autonomy

import (
	"context"
	"testing"
	"time"
)

// TestObserverSnapshotP95 asserts Story 4.3 SLA: observation completes within
// 100ms. Seeds a plausible row count then runs 50 snapshots to get a distribution.
func TestObserverSnapshotP95(t *testing.T) {
	obs := setupObs(t)

	// Seed 200 pending tasks so the COUNT(*) queries have real work to do.
	for i := 0; i < 200; i++ {
		_, _ = obs.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, input)
			 VALUES (?, 'w','x','pending','{}')`, "t-"+string(rune(i%26+97))+string(rune(i/26+65)))
	}

	var worst time.Duration
	for i := 0; i < 50; i++ {
		start := time.Now()
		_, err := obs.Snapshot(context.Background(), "worker")
		if err != nil {
			t.Fatal(err)
		}
		d := time.Since(start)
		if d > worst {
			worst = d
		}
	}
	t.Logf("observer worst-case = %s", worst)
	if worst > 100*time.Millisecond {
		t.Fatalf("observer snapshot took %s, exceeds 100ms SLA", worst)
	}
}
