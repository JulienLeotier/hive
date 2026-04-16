# Story 5.1: Circuit Breaker

Status: done

## Story

As the system,
I want circuit breakers on all agent invocations,
so that failing agents don't cascade failures across the system.

## Acceptance Criteria

1. **Given** an agent fails 3 consecutive invocations (configurable threshold)
   **When** the circuit breaker trips
   **Then** subsequent invocations to that agent return immediately with "circuit open" error
   **And** a `agent.circuit_open` event is emitted

2. **Given** a tripped circuit breaker
   **When** 30 seconds elapse (configurable reset timeout)
   **Then** the circuit enters half-open state
   **And** the next invocation is a test -- success closes the circuit, failure reopens it

3. **Given** a half-open circuit breaker
   **When** the test invocation succeeds
   **Then** the circuit closes and normal traffic resumes

4. **Given** a half-open circuit breaker
   **When** the test invocation fails
   **Then** the circuit reopens for another reset timeout period

## Tasks / Subtasks

- [x] Task 1: CircuitBreaker struct and state machine (AC: #1, #2, #3, #4)
  - [x] Define `CircuitState` type with `StateClosed`, `StateOpen`, `StateHalfOpen` constants
  - [x] Create `CircuitBreaker` struct with mutex, state, failure count, threshold, reset timeout
  - [x] Implement `NewCircuitBreaker(agentName, threshold, resetTimeout)` constructor
  - [x] Implement `Allow()` — returns nil if request is allowed, error if circuit is open
  - [x] Implement `RecordSuccess()` — resets failure count, closes circuit if half-open
  - [x] Implement `RecordFailure()` — increments failure count, opens circuit at threshold
  - [x] Implement `State()` — returns current state with automatic half-open transition check
  - [x] Implement `Failures()` — returns current consecutive failure count
- [x] Task 2: BreakerRegistry for managing per-agent circuit breakers (AC: #1)
  - [x] Create `BreakerConfig` struct with `Threshold` and `ResetTimeout` fields
  - [x] Create `DefaultBreakerConfig()` returning threshold=3, resetTimeout=30s
  - [x] Create `BreakerRegistry` struct with mutex-protected map of breakers
  - [x] Implement `Get(agentName)` — returns existing breaker or creates new one with defaults
  - [x] Implement `AllStates()` — returns map of agent name to circuit state
- [x] Task 3: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test circuit starts closed and allows requests
  - [x] Test circuit trips after threshold consecutive failures
  - [x] Test circuit resets failure count on success
  - [x] Test circuit transitions to half-open after timeout
  - [x] Test half-open to closed on success
  - [x] Test half-open to open on failure
  - [x] Test registry creates breakers on demand and returns same instance
  - [x] Test registry AllStates returns correct states for all agents

## Dev Notes

### Architecture Compliance

- Thread-safe via `sync.Mutex` — circuit breakers are accessed from multiple goroutines during concurrent task execution
- Configurable threshold and reset timeout per agent via `BreakerConfig`
- Uses `slog` for state transition logging (circuit opened, half-open, closed)
- Event types `agent.circuit_open` defined in `internal/event/types.go` for integration with event bus
- Registry pattern allows the API server to query all circuit breaker states for the metrics endpoint

### Key Design Decisions

- Circuit breaker is a standalone package (`internal/resilience`) with no dependency on the event bus — event emission is handled by the caller (API server / task executor) to keep the breaker focused on state management
- `Allow()` performs the half-open timeout check inline, so callers get the latest state without polling
- `State()` also performs the timeout check for read-only state queries (used by metrics endpoint)

### Integration Points

- `internal/api/server.go` — creates `BreakerRegistry` and injects it into the API server for metrics reporting
- `internal/cli/serve.go` — initializes `BreakerRegistry` with `DefaultBreakerConfig()` on server startup
- `internal/event/types.go` — defines `AgentCircuitOpen` event constant

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Resilience Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR52]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- CircuitBreaker implements full closed/open/half-open state machine with configurable threshold and reset timeout
- BreakerRegistry provides thread-safe per-agent circuit breaker management with lazy initialization
- 8 unit tests covering all state transitions, registry behavior, and concurrent access patterns
- Integrated with API server metrics endpoint to expose circuit breaker states

### Change Log

- 2026-04-16: Story 5.1 implemented — circuit breaker pattern with registry and full test coverage

### File List

- internal/resilience/circuit_breaker.go (new)
- internal/resilience/circuit_breaker_test.go (new)
- internal/event/types.go (modified — added AgentCircuitOpen constant)
- internal/api/server.go (modified — added breakers to Server struct, metrics endpoint)
- internal/cli/serve.go (modified — initializes BreakerRegistry)
