package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
)

// Reassigner is the subset of task.Router the watcher needs.
type Reassigner interface {
	ReassignAgentTasks(ctx context.Context, agentName, reason string) (int, error)
}

// HealthWatcher reacts to circuit breaker state changes:
//   - Open   → mark agent unavailable, reassign its in-flight tasks (Story 5.2 + 5.3).
//   - Closed → mark agent healthy again.
//   - HalfOpen → mark agent degraded so the UI can show the recovery attempt.
type HealthWatcher struct {
	mgr        *Manager
	reassigner Reassigner
	bus        *event.Bus
}

// NewHealthWatcher builds a watcher. The manager is required; reassigner and bus are optional.
func NewHealthWatcher(mgr *Manager, reassigner Reassigner, bus *event.Bus) *HealthWatcher {
	return &HealthWatcher{mgr: mgr, reassigner: reassigner, bus: bus}
}

// Hook returns the callback to install on the BreakerRegistry.
// The breaker fires from whichever goroutine tripped it, with no ambient
// context, so we build one here with a 5s cap — long enough for the DB
// writes to settle, short enough that a stuck backend doesn't pin a
// goroutine on every breaker transition.
func (w *HealthWatcher) Hook() resilience.StateChangeHook {
	return func(agentName string, from, to resilience.CircuitState) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		switch to {
		case resilience.StateOpen:
			// Story 5.1 AC: "a `agent.circuit_open` event is emitted".
			if w.bus != nil {
				_, _ = w.bus.Publish(ctx, event.AgentCircuitOpen, agentName, map[string]any{
					"from": string(from), "to": string(to),
				})
			}
			w.isolate(ctx, agentName)
		case resilience.StateClosed:
			w.restore(ctx, agentName)
		case resilience.StateHalfOpen:
			_ = w.mgr.UpdateHealth(ctx, agentName, "degraded")
		}
		slog.Info("breaker state changed", "agent", agentName, "from", from, "to", to)
	}
}

func (w *HealthWatcher) isolate(ctx context.Context, agentName string) {
	if err := w.mgr.UpdateHealth(ctx, agentName, "unavailable"); err != nil {
		slog.Error("isolate: update health", "agent", agentName, "error", err)
	}
	if w.bus != nil {
		_, _ = w.bus.Publish(ctx, event.AgentIsolated, agentName, map[string]string{"reason": "circuit_open"})
	}
	if w.reassigner == nil {
		return
	}
	n, err := w.reassigner.ReassignAgentTasks(ctx, agentName, "agent isolated")
	if err != nil {
		slog.Error("isolate: reassign tasks", "agent", agentName, "error", err)
		return
	}
	if n > 0 {
		slog.Warn("reassigned in-flight tasks", "agent", agentName, "count", n)
	}
}

func (w *HealthWatcher) restore(ctx context.Context, agentName string) {
	if err := w.mgr.UpdateHealth(ctx, agentName, "healthy"); err != nil {
		slog.Error("restore: update health", "agent", agentName, "error", err)
	}
	if w.bus != nil {
		_, _ = w.bus.Publish(ctx, event.AgentHealthUp, agentName, map[string]string{"via": "breaker_closed"})
	}
}
