package task

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CheckpointSupervisor periodically scans for running tasks whose checkpoint
// has gone stale and reassigns them via the router.
type CheckpointSupervisor struct {
	store    *Store
	router   *Router
	interval time.Duration
	maxAge   time.Duration

	mu   sync.Mutex
	stop chan struct{}
}

// NewCheckpointSupervisor builds a supervisor. interval is how often to scan,
// maxAge is the threshold after which a running task without a fresh checkpoint
// is considered stale and reassigned.
func NewCheckpointSupervisor(store *Store, router *Router, interval, maxAge time.Duration) *CheckpointSupervisor {
	return &CheckpointSupervisor{
		store:    store,
		router:   router,
		interval: interval,
		maxAge:   maxAge,
	}
}

// Start launches the supervisor loop. Call Stop to halt.
func (s *CheckpointSupervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return
	}
	s.stop = make(chan struct{})
	stopCh := s.stop
	s.mu.Unlock()

	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		slog.Info("checkpoint supervisor started", "interval", s.interval, "max_age", s.maxAge)
		for {
			select {
			case <-t.C:
				if n, err := s.Sweep(ctx); err != nil {
					slog.Error("checkpoint sweep failed", "error", err)
				} else if n > 0 {
					slog.Warn("reassigned stale tasks", "count", n)
				}
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop halts the supervisor loop.
func (s *CheckpointSupervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
}

// Sweep runs one detection pass. Exposed for tests and manual triggers.
func (s *CheckpointSupervisor) Sweep(ctx context.Context) (int, error) {
	stale, err := s.store.StaleRunningTasks(ctx, s.maxAge)
	if err != nil {
		return 0, err
	}
	reassigned := 0
	for _, t := range stale {
		if err := s.router.Reassign(ctx, t.ID, "checkpoint stale"); err != nil {
			slog.Error("reassigning stale task", "task_id", t.ID, "error", err)
			continue
		}
		reassigned++
	}
	return reassigned, nil
}
