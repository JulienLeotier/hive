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
type Scheduler struct {
	mu       sync.Mutex
	timers   map[string]*time.Ticker
	handler  WakeUpHandler
	stopChs  map[string]chan struct{}
}

// NewScheduler creates a heartbeat scheduler.
func NewScheduler(handler WakeUpHandler) *Scheduler {
	return &Scheduler{
		timers:  make(map[string]*time.Ticker),
		handler: handler,
		stopChs: make(map[string]chan struct{}),
	}
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
				if err := s.handler(context.Background(), agentName); err != nil {
					slog.Error("wake-up cycle failed", "agent", agentName, "error", err)
				}
			case <-stop:
				ticker.Stop()
				slog.Info("heartbeat stopped", "agent", agentName)
				return
			}
		}
	}()
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
