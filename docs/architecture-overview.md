# Architecture Overview

Hive is a single-binary AI agent orchestration platform written in Go. It coordinates agents from any framework through the Agent Adapter Protocol.

## High-Level Architecture

```
User (CLI) --> API Server --> Workflow Engine --> Task Router --> Adapter --> Agent
                 |                  |                              |
            Dashboard          Event Bus <-----------------  Event emission
                                   |
                            SQLite (events, tasks, agents)
```

## Core Components

### Event Bus (`internal/event/`)

The event bus is the backbone of Hive. All state changes flow through it.

- **Implementation**: In-process Go struct with subscriber map (`bus.go`)
- **Persistence**: Events are written to SQLite before delivery (guaranteed delivery)
- **Subscription**: Prefix-based matching -- subscribing to `"task"` matches `"task.created"`, `"task.completed"`, etc.
- **Delivery**: Synchronous per-event, panic-recovered per subscriber (`safeCall()`)
- **Query**: SQL-based filtering by type prefix, source, time range (`bus.Query()`)

Event types are defined in `internal/event/types.go` using dot notation:
- `agent.registered`, `agent.removed`, `agent.health.up`, `agent.health.down`
- `task.created`, `task.assigned`, `task.started`, `task.completed`, `task.failed`
- `workflow.started`, `workflow.completed`, `workflow.failed`

### Adapter Layer (`internal/adapter/`)

The `Adapter` interface in `adapter.go` defines 5 methods: `Declare`, `Invoke`, `Health`, `Checkpoint`, `Resume`. Each agent type has its own adapter implementation:

| Adapter | Transport | File |
|---|---|---|
| HTTPAdapter | HTTP/JSON | `http.go` |
| ClaudeCodeAdapter | stdio subprocess | `claude_code.go` |
| MCPAdapter | HTTP (delegated) | `mcp.go` |
| OpenAIAdapter | OpenAI Assistants API | `openai.go` |
| LangChainAdapter | HTTP (LangServe) | `langchain.go` |
| CrewAIAdapter | Python subprocess | `crewai.go` |
| AutoGenAdapter | HTTP | `autogen.go` |

### Task Router (`internal/task/router.go`)

`FindCapableAgent()` queries all healthy agents and matches their declared `TaskTypes` against the requested task type. First healthy match wins.

### Task State Machine (`internal/task/task.go`)

Tasks follow a strict state machine: `pending` --> `assigned` --> `running` --> `completed`/`failed`. Each transition emits an event. The `Store` type manages persistence and enforces valid transitions.

### Agent Manager (`internal/agent/manager.go`)

Handles registration, listing, removal, and health updates. On `Register()`, validates the agent by calling its `Health` and `Declare` endpoints. Agents start with trust level `scripted`.

### Workflow Engine (`internal/workflow/`)

- **Parser** (`parser.go`): Parses YAML workflow configs, validates names, types, dependency references, and detects cycles using Kahn's algorithm
- **Topological Sort**: Groups tasks into parallel execution levels via `TopologicalSort()`
- **Store** (`workflow.go`): Persists workflows and manages status transitions with event emission

### Autonomy System (`internal/autonomy/`)

- **Scheduler** (`scheduler.go`): Manages heartbeat wake-up cycles per agent using `time.Ticker`. Calls a `WakeUpHandler` on each tick.
- **Plan** (`plan.go`): Parses `PLAN.yaml` behavioral state machines. Each plan has states, observations, actions, and transitions.
- **Identity** (`plan.go`): Parses `AGENT.yaml` with agent name, role, capabilities, constraints, anti-patterns.

### Trust Engine (`internal/trust/engine.go`)

Graduated autonomy with 4 levels: `supervised` --> `guided` --> `autonomous` --> `trusted`. Promotion is based on completed task count and error rate. Only promotes, never auto-demotes. All level changes logged to `trust_history` table.

### Resilience (`internal/resilience/circuit_breaker.go`)

Three-state circuit breaker pattern: `closed` (normal) --> `open` (tripped after N failures) --> `half-open` (testing). `BreakerRegistry` manages one breaker per agent with configurable threshold (default: 3) and reset timeout (default: 30s).

### API Server (`internal/api/`)

REST API at `/api/v1/`:
- `GET /api/v1/agents` -- list agents
- `GET /api/v1/events` -- query events (params: `type`, `source`, `since`)
- `GET /api/v1/metrics` -- agent health counts and circuit breaker states

Auth via Bearer token middleware. No keys configured = dev mode (all requests allowed).

### WebSocket Hub (`internal/ws/hub.go`)

Broadcasts events to connected dashboard clients in real-time. Uses `gorilla/websocket`. Origin checking allows localhost by default; additional origins configurable via `AllowedOrigins`.

### Storage (`internal/storage/sqlite.go`)

SQLite with WAL mode, auto-migrations on startup. Migrations are embedded SQL files. Tables: `agents`, `events`, `tasks`, `workflows`, `knowledge`, `trust_history`, `webhooks`, `api_keys`, `costs`, `audit_log`.

### Dashboard (`web/` + `internal/dashboard/`)

Svelte 5 SPA compiled to static assets, embedded in the Go binary via `//go:embed dist/*`. SPA routing falls back to `index.html` for unknown paths.

## Key Design Decisions

- **Single binary**: Go + embedded SQLite (modernc.org/sqlite, pure Go) + embedded Svelte assets
- **Event sourcing**: All state changes emit events; event log is the source of truth
- **Interface-based boundaries**: Adapter, EventBus, storage interfaces allow future swaps
- **No circular dependencies**: Enforced by Go compiler's package model
- **IDs**: ULID (sortable, URL-safe) via `oklog/ulid`
- **Logging**: Structured `log/slog` throughout

## Data Flow: Agent Wake-Up

```
Scheduler (ticker) --> Wake agent --> Observer reads state/backlog
                                        |
                                  Plan evaluates --> Action (invoke / idle / escalate)
                                        |
                                  Event emitted --> SQLite
```

## Package Dependency Graph

```
cmd/hive --> internal/cli
internal/cli --> internal/config, internal/storage, internal/agent, internal/event,
                 internal/api, internal/dashboard, internal/resilience, internal/workflow
internal/api --> internal/agent, internal/event, internal/resilience
internal/task --> internal/event, internal/adapter
internal/agent --> internal/adapter
internal/autonomy --> (standalone, YAML parsing)
internal/trust --> (standalone, DB queries)
```
