package task

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
)

// AdapterResolver looks up an adapter for a given agent id. Returns nil if the
// agent has no live adapter available (e.g., already unavailable).
type AdapterResolver func(agentID string) adapter.Adapter

// CheckpointSupervisor periodically:
//   1. Calls /checkpoint on each running task's agent and persists the snapshot
//      (Story 2.6 AC: "the orchestrator calls the agent's /checkpoint endpoint").
//   2. Detects running tasks whose checkpoint is stale and reassigns them via
//      the router; when a resolver is configured, the new agent's /resume is
//      called with the most recent checkpoint so work continues without loss.
type CheckpointSupervisor struct {
	store    *Store
	router   *Router
	interval time.Duration
	maxAge   time.Duration

	resolver AdapterResolver

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

// WithAdapterResolver wires live adapters so the supervisor can actively poll
// /checkpoint and invoke /resume on reassignment.
func (s *CheckpointSupervisor) WithAdapterResolver(r AdapterResolver) *CheckpointSupervisor {
	s.resolver = r
	return s
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
				if err := s.Poll(ctx); err != nil {
					slog.Error("checkpoint poll failed", "error", err)
				}
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

// Sweep runs one reassignment pass on stale running tasks.
// When an adapter resolver is configured, the detected checkpoint is handed
// off via /resume on the next-assigned agent (done at claim time downstream).
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

// Poll asks every running task's agent for a fresh checkpoint and persists it.
// Tasks without a live adapter are skipped (they'll be swept as stale later).
func (s *CheckpointSupervisor) Poll(ctx context.Context) error {
	if s.resolver == nil {
		return nil
	}
	rows, err := s.store.db.QueryContext(ctx,
		`SELECT id, COALESCE(agent_id,'') FROM tasks WHERE status = 'running'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type runningTask struct{ id, agentID string }
	var running []runningTask
	for rows.Next() {
		var r runningTask
		if err := rows.Scan(&r.id, &r.agentID); err == nil && r.agentID != "" {
			running = append(running, r)
		}
	}

	for _, r := range running {
		a := s.resolver(r.agentID)
		if a == nil {
			continue
		}
		cp, err := a.Checkpoint(ctx)
		if err != nil {
			slog.Debug("checkpoint poll failed", "task_id", r.id, "error", err)
			continue
		}
		data, err := json.Marshal(cp.Data)
		if err != nil {
			continue
		}
		if err := s.store.SaveCheckpoint(ctx, r.id, string(data)); err != nil {
			slog.Error("persisting checkpoint", "task_id", r.id, "error", err)
		}
	}
	return nil
}

// ResumeOnAgent hands a persisted checkpoint to a fresh adapter via /resume.
// Called after reassignment so the new agent picks up where the old one left off.
func (s *CheckpointSupervisor) ResumeOnAgent(ctx context.Context, taskID string, a adapter.Adapter) error {
	t, err := s.store.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t.Checkpoint == "" {
		return nil
	}
	var data any
	if err := json.Unmarshal([]byte(t.Checkpoint), &data); err != nil {
		data = t.Checkpoint
	}
	return a.Resume(ctx, adapter.Checkpoint{Data: data})
}
