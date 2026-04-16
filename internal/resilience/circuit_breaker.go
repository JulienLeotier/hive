package resilience

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state.
type CircuitState string

const (
	StateClosed   CircuitState = "closed"   // normal — requests flow through
	StateOpen     CircuitState = "open"     // tripped — requests rejected immediately
	StateHalfOpen CircuitState = "half-open" // testing — one request allowed through
)

// StateChangeHook is invoked whenever a breaker moves between states.
type StateChangeHook func(agentName string, from, to CircuitState)

// CircuitBreaker prevents cascading failures by stopping calls to unhealthy agents.
type CircuitBreaker struct {
	mu              sync.Mutex
	state           CircuitState
	failures        int
	threshold       int           // consecutive failures to trip
	resetTimeout    time.Duration // how long to wait before half-open
	lastFailureTime time.Time
	agentName       string
	onChange        StateChangeHook
}

// NewCircuitBreaker creates a circuit breaker for an agent.
func NewCircuitBreaker(agentName string, threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
		agentName:    agentName,
	}
}

// Allow checks if a request should be allowed through.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	from := cb.state
	var changed bool

	switch cb.state {
	case StateClosed:
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			slog.Info("circuit half-open", "agent", cb.agentName)
			changed = true
		} else {
			err := fmt.Errorf("circuit open for agent %s: %d consecutive failures", cb.agentName, cb.failures)
			cb.mu.Unlock()
			return err
		}
	case StateHalfOpen:
	}
	to := cb.state
	hook := cb.onChange
	cb.mu.Unlock()
	if changed && hook != nil {
		hook(cb.agentName, from, to)
	}
	return nil
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	from := cb.state
	cb.failures = 0
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		slog.Info("circuit closed", "agent", cb.agentName)
	}
	to := cb.state
	hook := cb.onChange
	cb.mu.Unlock()
	if from != to && hook != nil {
		hook(cb.agentName, from, to)
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	from := cb.state
	cb.failures++
	cb.lastFailureTime = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = StateOpen
		slog.Warn("circuit opened", "agent", cb.agentName, "failures", cb.failures)
	}
	to := cb.state
	hook := cb.onChange
	cb.mu.Unlock()
	if from != to && hook != nil {
		hook(cb.agentName, from, to)
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if open circuit should transition to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		cb.state = StateHalfOpen
	}
	return cb.state
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// BreakerRegistry manages circuit breakers for all agents.
type BreakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*CircuitBreaker
	defaults BreakerConfig
	onChange StateChangeHook
}

// BreakerConfig holds default circuit breaker settings.
type BreakerConfig struct {
	Threshold    int
	ResetTimeout time.Duration
}

// DefaultBreakerConfig returns sensible defaults.
func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{
		Threshold:    3,
		ResetTimeout: 30 * time.Second,
	}
}

// NewBreakerRegistry creates a registry with default settings.
func NewBreakerRegistry(cfg BreakerConfig) *BreakerRegistry {
	return &BreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		defaults: cfg,
	}
}

// OnStateChange installs a hook that fires when any breaker changes state.
// Existing breakers are retrofitted with the hook so late registration still works.
func (r *BreakerRegistry) OnStateChange(hook StateChangeHook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onChange = hook
	for _, cb := range r.breakers {
		cb.mu.Lock()
		cb.onChange = hook
		cb.mu.Unlock()
	}
}

// Get returns the circuit breaker for an agent, creating one if needed.
func (r *BreakerRegistry) Get(agentName string) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok := r.breakers[agentName]; ok {
		return cb
	}

	cb := NewCircuitBreaker(agentName, r.defaults.Threshold, r.defaults.ResetTimeout)
	cb.onChange = r.onChange
	r.breakers[agentName] = cb
	return cb
}

// AllStates returns the circuit state for all registered agents.
func (r *BreakerRegistry) AllStates() map[string]CircuitState {
	r.mu.Lock()
	defer r.mu.Unlock()

	states := make(map[string]CircuitState, len(r.breakers))
	for name, cb := range r.breakers {
		states[name] = cb.State()
	}
	return states
}
