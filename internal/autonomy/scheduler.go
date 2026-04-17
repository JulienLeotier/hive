package autonomy

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// WakeUpHandler is called when an agent wakes up.
type WakeUpHandler func(ctx context.Context, agentName string) error

// Scheduler manages heartbeat wake-up cycles for agents.
// Story 4.2: respects system load via a global semaphore (backpressure).
// If MaxConcurrent wakeups are already in flight, a tick is dropped with a
// backpressure log rather than queuing unbounded work.
type Scheduler struct {
	mu            sync.Mutex
	timers        map[string]*time.Ticker
	handler       WakeUpHandler
	stopChs       map[string]chan struct{}
	concurrency   chan struct{}
	maxConcurrent int
}

// NewScheduler creates a heartbeat scheduler with a default backpressure limit.
// MaxConcurrent defaults to 16 wake-up cycles in flight; call SetMaxConcurrent
// to tune.
func NewScheduler(handler WakeUpHandler) *Scheduler {
	s := &Scheduler{
		timers:  make(map[string]*time.Ticker),
		handler: handler,
		stopChs: make(map[string]chan struct{}),
	}
	s.SetMaxConcurrent(16)
	return s
}

// SetMaxConcurrent reconfigures the backpressure cap.
func (s *Scheduler) SetMaxConcurrent(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		n = 1
	}
	s.maxConcurrent = n
	s.concurrency = make(chan struct{}, n)
}

// Register starts heartbeat scheduling for an agent.
func (s *Scheduler) Register(agentName string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing if re-registering (delete from map first to prevent double-close)
	if stop, ok := s.stopChs[agentName]; ok {
		delete(s.stopChs, agentName)
		delete(s.timers, agentName)
		close(stop)
	}

	ticker := time.NewTicker(interval)
	stop := make(chan struct{})
	s.timers[agentName] = ticker
	s.stopChs[agentName] = stop

	go func() {
		slog.Info("heartbeat started", "agent", agentName, "interval", interval)
		for {
			select {
			case <-ticker.C:
				// Story 4.2: drop ticks when we're at capacity instead of queuing.
				select {
				case s.concurrency <- struct{}{}:
					go func() {
						defer func() { <-s.concurrency }()
						ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
						err := s.handler(ctx, agentName)
						cancel()
						if err != nil {
							slog.Error("wake-up cycle failed", "agent", agentName, "error", err)
						}
					}()
				default:
					slog.Warn("backpressure: wake-up skipped", "agent", agentName, "cap", s.maxConcurrent)
				}
			case <-stop:
				ticker.Stop()
				slog.Info("heartbeat stopped", "agent", agentName)
				return
			}
		}
	}()
}

// TriggerWakeUp fires a wake-up for the named agent outside the normal
// interval — lets the event bus push a wake-up when something interesting
// happens. Story 4.2 AC: "heartbeats can also be triggered by events".
// Respects the backpressure cap; dropped ticks are logged.
func (s *Scheduler) TriggerWakeUp(agentName string) {
	s.mu.Lock()
	_, registered := s.stopChs[agentName]
	s.mu.Unlock()
	if !registered {
		return
	}
	select {
	case s.concurrency <- struct{}{}:
		go func() {
			defer func() { <-s.concurrency }()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			err := s.handler(ctx, agentName)
			cancel()
			if err != nil {
				slog.Error("event-triggered wake-up failed", "agent", agentName, "error", err)
			}
		}()
	default:
		slog.Warn("backpressure: event wake-up dropped", "agent", agentName)
	}
}

// Unregister stops heartbeat scheduling for an agent.
func (s *Scheduler) Unregister(agentName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stop, ok := s.stopChs[agentName]; ok {
		close(stop)
		delete(s.timers, agentName)
		delete(s.stopChs, agentName)
	}
}

// StopAll stops all heartbeat timers.
func (s *Scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, stop := range s.stopChs {
		close(stop)
		delete(s.timers, name)
		delete(s.stopChs, name)
	}
}

// ActiveCount returns the number of agents with active heartbeats.
func (s *Scheduler) ActiveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.timers)
}
