package resilience

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second)
	assert.Equal(t, StateClosed, cb.State())
	assert.NoError(t, cb.Allow())
}

func TestCircuitBreakerTripsAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State()) // not yet

	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State()) // tripped!
	assert.Error(t, cb.Allow())
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 30*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // reset

	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.Failures())
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())
	assert.NoError(t, cb.Allow()) // test request allowed
}

func TestCircuitBreakerHalfOpenToClosedOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	require.Equal(t, StateHalfOpen, cb.State())
	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreakerHalfOpenToOpenOnFailure(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	require.Equal(t, StateHalfOpen, cb.State())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestBreakerRegistryCreatesOnDemand(t *testing.T) {
	reg := NewBreakerRegistry(DefaultBreakerConfig())

	cb1 := reg.Get("agent-a")
	cb2 := reg.Get("agent-a")
	assert.Same(t, cb1, cb2) // same instance

	cb3 := reg.Get("agent-b")
	assert.NotSame(t, cb1, cb3) // different agent
}

func TestBreakerRegistryAllStates(t *testing.T) {
	reg := NewBreakerRegistry(DefaultBreakerConfig())

	reg.Get("healthy").RecordSuccess()
	cb := reg.Get("failing")
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	states := reg.AllStates()
	assert.Equal(t, StateClosed, states["healthy"])
	assert.Equal(t, StateOpen, states["failing"])
}
