# Story 7.4: Adapter Template Generator

Status: done

## Story

As an adapter author,
I want to generate adapter boilerplate for my framework,
so that I can implement and contribute a new adapter quickly.

## Acceptance Criteria

1. **Given** the user runs `hive adapter-template my-framework`
   **When** generation completes
   **Then** a directory `my-framework/` is created with: adapter Go source file with interface stubs, protocol compliance test file, example configuration, README with contribution guide

2. **Given** the generated adapter
   **When** `go build` is run
   **Then** the adapter compiles successfully (with `TODO` implementations)

3. **Given** the generated test file
   **When** `go test` is run
   **Then** the compliance test suite passes with `TODO` implementations returning expected stub values

## Tasks / Subtasks

- [x] Task 1: Template generator command (AC: #1)
  - [x] Create `hive adapter-template <name>` cobra command
  - [x] Accept framework name as positional argument
  - [x] Create output directory with framework name
- [x] Task 2: Generated adapter source (AC: #1, #2)
  - [x] Generate Go source file implementing the `Adapter` interface
  - [x] Include stub implementations for: `Declare()`, `Invoke()`, `Health()`, `Checkpoint()`, `Resume()`
  - [x] Stubs return sensible defaults (e.g., Health returns "healthy", Declare returns empty capabilities)
  - [x] Include `TODO` comments at each implementation point
- [x] Task 3: Protocol compliance test (AC: #1, #3)
  - [x] Generate test file that validates the adapter implements all interface methods
  - [x] Test that Declare returns valid AgentCapabilities
  - [x] Test that Health returns valid HealthStatus
  - [x] Test that Invoke returns a TaskResult
  - [x] Tests pass with stub implementations
- [x] Task 4: Configuration and documentation (AC: #1)
  - [x] Generate example `config.yaml` for the adapter
  - [x] Generate `README.md` with: adapter overview, how to implement each method, how to run compliance tests, contribution guide

## Dev Notes

### Architecture Compliance

- Generated code follows the `Adapter` interface from `internal/adapter/adapter.go`
- Uses the same types: `AgentCapabilities`, `Task`, `TaskResult`, `HealthStatus`, `Checkpoint`
- Generated tests use `testing` + `testify` consistent with the rest of the codebase
- NFR17: Adapter authoring under 30 minutes — template provides 80% of the boilerplate

### Key Design Decisions

- Templates are Go `text/template` strings embedded in the binary — no external files needed
- Framework name is used for package name, file names, and struct names (sanitized to valid Go identifiers)
- Stub implementations are immediately compilable — the adapter author's job is to replace `TODO` stubs with real logic
- Compliance tests validate the interface contract, not the adapter's actual behavior

### Generated File Structure

```
my-framework/
  my_framework.go       -- Adapter implementation with interface stubs
  my_framework_test.go  -- Protocol compliance test suite
  config.yaml           -- Example configuration
  README.md             -- Implementation guide
```

### Integration Points

- `internal/adapter/adapter.go` — `Adapter` interface that generated code implements
- `internal/cli/root.go` — adapter-template command registration

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR25, NFR17]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Template generator creates complete adapter scaffold from a single command
- Generated Go source implements all 5 Adapter interface methods with TODO stubs
- Compliance test file validates interface contract with stub-compatible assertions
- Example config and README provide guidance for adapter authors

### Change Log

- 2026-04-16: Story 7.4 implemented — adapter template generator with compilable stubs

### File List

- internal/cli/root.go (modified — adapter-template command registration)
- internal/adapter/adapter.go (reference — Adapter interface definition)
