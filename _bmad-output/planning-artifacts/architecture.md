---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - '_bmad-output/planning-artifacts/prd.md'
  - '_bmad-output/planning-artifacts/product-brief-hive.md'
  - '_bmad-output/planning-artifacts/product-brief-hive-distillate.md'
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-04-16'
project_name: 'hive'
---

# Architecture Decision Document — Hive

_This document defines all architectural decisions, implementation patterns, and project structure for the Hive AI agent orchestration platform._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:** 56 FRs across 8 capability areas:
- Agent Management (FR1-FR7): Registration, auto-detection, hot-swap, health validation
- Workflow Definition (FR8-FR12): YAML workflows, DAG dependencies, event triggers, conditional routing
- Task Orchestration (FR13-FR18): Capability routing, parallel execution, checkpoint/resume
- Event System (FR19-FR23): Sub-200ms delivery, pub/sub, ordered event log
- Agent Adapter Protocol (FR24-FR28): 5-method interface, template generator, compliance tests
- Observability (FR29-FR33): Status, logs, metrics, decision tracing
- Agent Autonomy (FR43-FR51): Behavioral plans, wake-up cycles, self-assignment, idle detection
- Error Handling (FR52-FR56): Circuit breakers, auto-isolation, failover, retry policies

**Non-Functional Requirements:** 23 NFRs driving architecture:
- Event latency < 200ms p95 → in-process event bus, not external broker
- Zero external dependencies → embedded SQLite, embedded event bus, embedded frontend
- Single binary → Go with embedded assets
- Cross-platform → Go cross-compilation
- Crash recovery < 10s → checkpoint/resume with WAL-mode SQLite

**Scale & Complexity:**
- Primary domain: Developer tooling / agent orchestration
- Complexity: Medium (technically ambitious, no regulatory constraints)
- Estimated components: 8 core packages + CLI + dashboard

### Technical Constraints & Dependencies

- Single binary distribution (zero runtime dependencies)
- Embedded storage (SQLite, no external DB)
- Embedded event bus (in-process, no external broker)
- HTTP/JSON adapter protocol (universal compatibility)
- YAML configuration (human-readable agent plans)
- Cross-platform: macOS, Linux, Windows (arm64 + x64)

### Cross-Cutting Concerns

- Event delivery (touches every component)
- Agent health monitoring (orchestrator + adapters + dashboard)
- Checkpoint/resume (orchestrator + agents + storage)
- Logging with decision context (all components)
- Configuration management (CLI + orchestrator + agents)

## Technology Stack

### Language: Go

**Decision:** Go (latest stable: 1.24)

**Rationale:**
- Single binary compilation with zero runtime dependencies
- Excellent concurrency model (goroutines + channels) — perfect for event bus and parallel task execution
- Fast compilation and startup time
- Strong stdlib for HTTP servers, JSON, and CLI tooling
- Cross-compilation built-in (`GOOS`/`GOARCH`)
- CGO-free SQLite available (modernc.org/sqlite) — true single binary without C compiler
- Large ecosystem for the specific needs (CLI, HTTP, config)
- Faster development velocity than Rust with comparable deployment characteristics

**Alternatives considered:**
- **Rust**: Better raw performance and memory safety, but significantly slower development velocity for MVP. Hive's bottleneck is I/O (HTTP calls to agents), not CPU — Go's performance is more than sufficient. Can migrate hot paths to Rust later if needed.

### Storage: SQLite (embedded)

**Decision:** SQLite via `modernc.org/sqlite` (pure Go, no CGO)

**Rationale:**
- Zero external dependencies (no Postgres, no Redis)
- WAL mode for concurrent reads during writes
- Single file = easy backup, migration, debugging
- Proven at scale (used by billions of devices)
- Pure Go driver means true single-binary build

**Configuration:**
- WAL mode enabled by default
- Busy timeout: 5000ms
- Journal size limit: 64MB
- Auto-vacuum: incremental

### Event Bus: In-Process

**Decision:** Custom in-process event bus using Go channels + SQLite event store

**Rationale:**
- Zero external dependencies (no NATS, no Redis, no RabbitMQ)
- In-process = sub-millisecond delivery for local agents
- SQLite-backed event log for persistence and replay
- Architecture allows future swap to NATS via interface abstraction

**Design:**
- `EventBus` interface with `Publish()`, `Subscribe()`, `Replay()` methods
- Default implementation: Go channels for in-process delivery
- All events persisted to SQLite `events` table before delivery
- Strict ordering via auto-increment ID

### Frontend: Svelte (embedded)

**Decision:** Svelte 5 for dashboard UI, compiled and embedded in Go binary

**Rationale:**
- Smallest bundle size of any modern framework (~5KB runtime)
- Compiles to vanilla JS (no virtual DOM overhead)
- Fast build times
- Simple mental model (reactive assignments, no hooks)
- Embedded in Go binary via `embed` package — single binary serves both API and UI

**Build pipeline:**
- Svelte builds to `internal/dashboard/dist/`
- Go `//go:embed` directive bundles static assets into binary
- Development: Vite dev server with proxy to Go API
- Production: Single binary serves everything

### CLI Framework: Cobra

**Decision:** `github.com/spf13/cobra` for CLI

**Rationale:**
- De facto standard for Go CLIs (kubectl, docker, gh all use it)
- Built-in shell completion (bash, zsh, fish)
- Subcommand architecture matches Hive's command structure
- Integrated help generation

### Configuration: YAML

**Decision:** YAML via `gopkg.in/yaml.v3`

**Rationale:**
- Human-readable (operators edit agent PLAN.yaml by hand)
- Supports comments (critical for documenting agent behavior)
- Widely understood format
- Maps cleanly to Go structs

### Testing: Go stdlib + testify

**Decision:** `testing` package + `github.com/stretchr/testify`

**Rationale:**
- Go's built-in testing is sufficient for most needs
- testify adds readable assertions and mocking
- Table-driven tests for adapter protocol compliance
- `httptest` for HTTP handler testing

### Build & Release: GoReleaser

**Decision:** GoReleaser for cross-platform binary builds

**Rationale:**
- Automated cross-compilation for all target platforms
- Homebrew tap generation
- GitHub release automation
- Checksum and signing support

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
1. Go as primary language ✅
2. SQLite as embedded storage ✅
3. In-process event bus with SQLite persistence ✅
4. HTTP/JSON adapter protocol ✅
5. Agent behavioral plan format (YAML state machine) ✅

**Important Decisions (Shape Architecture):**
6. Svelte for dashboard ✅
7. Cobra for CLI ✅
8. Event sourcing for all state changes ✅
9. Interface-based abstractions for pluggable components ✅

**v0.2 Decisions (Now Active):**
10. WebSocket for real-time dashboard updates ✅
11. Svelte 5 dashboard embedded in Go binary ✅
12. Trust engine with SQLite-backed scoring ✅
13. Knowledge layer with sqlite-vec for vector search ✅
14. Gorilla/websocket for WS transport ✅

**v0.3 Decisions (Now Active):**
15. NATS as pluggable event bus backend via EventBus interface ✅
16. HiveHub as Git-backed template registry (GitHub repo) ✅
17. Framework adapters: CrewAI (Python subprocess), LangChain (HTTP), AutoGen (HTTP), OpenAI (API) ✅
18. Lightweight local embeddings for knowledge search (bag-of-words TF-IDF → upgrade later) ✅
19. Cost tracking via agent capabilities `cost_per_run` field ✅

**v1.0 Decisions (Now Active):**
20. Market-based allocation: auction engine with bid/ask protocol ✅
21. Cross-hive federation: mTLS + capability discovery protocol ✅
22. Self-optimization: pattern analyzer using historical event data ✅
23. PostgreSQL support via pgx driver (pluggable behind storage interface) ✅
24. OIDC SSO via coreos/go-oidc ✅
25. RBAC with casbin policy engine ✅

**All decisions resolved. No deferred items.**

### Data Architecture

**Database Schema (SQLite):**

```sql
-- Agents registered in the hive
CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,           -- 'claude-code', 'mcp', 'http', etc.
    config TEXT NOT NULL,         -- JSON: adapter config
    capabilities TEXT NOT NULL,   -- JSON: declared capabilities
    plan TEXT,                    -- JSON: behavioral plan (PLAN.yaml parsed)
    health_status TEXT DEFAULT 'unknown',
    trust_level TEXT DEFAULT 'scripted',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Event log (append-only, event sourcing)
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,           -- 'task.created', 'agent.health', etc.
    source TEXT NOT NULL,         -- agent ID or 'system'
    payload TEXT NOT NULL,        -- JSON event data
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_events_type ON events(type);
CREATE INDEX idx_events_source ON events(source);

-- Tasks and their state
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT DEFAULT 'pending', -- pending, assigned, running, completed, failed
    agent_id TEXT,
    input TEXT NOT NULL,           -- JSON
    output TEXT,                   -- JSON
    checkpoint TEXT,               -- JSON: serialized state for resume
    depends_on TEXT,               -- JSON: array of task IDs
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT
);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_workflow ON tasks(workflow_id);

-- Workflows
CREATE TABLE workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    config TEXT NOT NULL,          -- JSON: parsed hive.yaml
    status TEXT DEFAULT 'idle',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Shared knowledge layer (v0.2)
CREATE TABLE knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL,
    approach TEXT NOT NULL,
    outcome TEXT NOT NULL,         -- 'success' or 'failure'
    context TEXT,                  -- JSON
    embedding BLOB,               -- vector embedding for similarity search
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_knowledge_task_type ON knowledge(task_type);
CREATE INDEX idx_knowledge_outcome ON knowledge(outcome);

-- Trust history (v0.2)
CREATE TABLE trust_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    old_level TEXT NOT NULL,
    new_level TEXT NOT NULL,
    reason TEXT NOT NULL,          -- 'auto_promotion', 'manual_override', 'demotion'
    criteria TEXT,                 -- JSON: metrics that triggered change
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_trust_history_agent ON trust_history(agent_id);

-- Agent dialog threads (v0.2)
CREATE TABLE dialog_threads (
    id TEXT PRIMARY KEY,
    initiator_agent_id TEXT NOT NULL,
    participant_agent_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    status TEXT DEFAULT 'active',  -- 'active', 'completed'
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE TABLE dialog_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id TEXT NOT NULL REFERENCES dialog_threads(id),
    sender_agent_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX idx_dialog_messages_thread ON dialog_messages(thread_id);

-- Webhook configurations (v0.2)
CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    type TEXT NOT NULL,            -- 'slack', 'github', 'email', 'generic'
    event_filter TEXT,             -- JSON: event types to notify on
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
```

**Data validation:** Go struct tags + custom validators at API boundary. No ORM — direct SQL with `database/sql` and prepared statements.

**Migration approach:** Embedded SQL migrations in Go binary, applied on startup. Version tracked in `schema_versions` table.

### Authentication & Security

**MVP scope (single-node, local):**
- API key authentication for adapter-to-orchestrator communication
- API keys stored hashed in SQLite
- Secrets never logged (redacted in all log output)
- File permissions: `0600` for config, `0700` for data directory
- No TLS for local development; TLS required in production config

**Deferred:** mTLS, RBAC, SSO (enterprise features)

### API & Communication Patterns

**Internal API (orchestrator ↔ dashboard):**
- REST/JSON over HTTP
- Routes prefixed `/api/v1/`
- Standard response envelope: `{"data": ..., "error": null}`
- Error format: `{"data": null, "error": {"code": "AGENT_NOT_FOUND", "message": "..."}}`

**Adapter Protocol (orchestrator ↔ agents):**
- HTTP/JSON (default), stdio (CLI agents)
- 5 endpoints per agent: `/declare`, `/invoke`, `/health`, `/checkpoint`, `/resume`
- Request timeout: configurable per agent (default 30s)
- Circuit breaker: open after 3 consecutive failures, half-open after 30s

**WebSocket (v0.2 — dashboard real-time):**
- Endpoint: `/ws` for dashboard event streaming
- Library: `github.com/gorilla/websocket`
- Clients subscribe to event types (same prefix matching as event bus)
- Server pushes events to connected clients as they occur
- Heartbeat ping/pong every 30s to detect stale connections

**Event protocol:**
- Events are JSON objects: `{"id": 1, "type": "task.created", "source": "system", "payload": {...}, "timestamp": "..."}`
- Event types use dot notation: `agent.registered`, `task.completed`, `workflow.started`
- Subscribers register via type prefix match: `task.*` subscribes to all task events

### Infrastructure & Deployment

**Local development:** `go run ./cmd/hive` — single command, zero setup
**Production:** Single binary + data directory
**Docker:** Multi-stage build → scratch image (~15MB)
**CI/CD:** GitHub Actions → GoReleaser → GitHub Releases + Homebrew tap

## Implementation Patterns & Consistency Rules

### Naming Patterns

**Go Code:**
- Packages: lowercase, single word (`agent`, `event`, `task`, `workflow`)
- Exported types: PascalCase (`AgentManager`, `EventBus`, `TaskRouter`)
- Unexported: camelCase (`handleEvent`, `routeTask`)
- Interfaces: verb-noun or -er suffix (`Adapter`, `Router`, `Publisher`)
- Files: snake_case (`agent_manager.go`, `event_bus.go`)
- Test files: `*_test.go` co-located with source

**Database:**
- Tables: plural snake_case (`agents`, `events`, `tasks`)
- Columns: snake_case (`agent_id`, `created_at`, `health_status`)
- Indexes: `idx_{table}_{column}` (`idx_events_type`)

**API:**
- Endpoints: plural nouns (`/api/v1/agents`, `/api/v1/tasks`)
- URL params: kebab-case for multi-word (`/api/v1/agent-types`)
- Query params: snake_case (`?agent_id=abc&status=running`)
- JSON fields: snake_case (`{"agent_id": "...", "health_status": "..."}`)

**Events:**
- Type: dot notation, past tense (`task.created`, `agent.registered`, `workflow.completed`)
- Payload fields: snake_case matching DB schema

**Config (YAML):**
- Keys: snake_case (`heartbeat_interval`, `max_retries`)
- Agent IDs: kebab-case (`code-reviewer`, `data-analyst`)

### Structure Patterns

- Tests co-located with source (`foo.go` + `foo_test.go`)
- One package per domain concept (`internal/agent/`, `internal/event/`, `internal/task/`)
- Interfaces defined in consumer package, not provider
- No `utils` or `helpers` packages — put functions where they're used
- Config loaded once at startup, passed via dependency injection

### Format Patterns

**API Response:**
```json
{"data": {"id": "task-123", "status": "completed"}, "error": null}
```

**API Error:**
```json
{"data": null, "error": {"code": "AGENT_UNAVAILABLE", "message": "Agent code-reviewer is not responding"}}
```

**Timestamps:** ISO 8601 UTC (`2026-04-16T14:30:00Z`) everywhere

**IDs:** ULID (sortable, URL-safe, no collision) via `github.com/oklog/ulid`

### Process Patterns

**Error handling:**
- Return `error` from all fallible functions (Go convention)
- Wrap errors with context: `fmt.Errorf("routing task %s: %w", taskID, err)`
- HTTP handlers return structured error responses, never stack traces
- Circuit breaker for all external calls (agent invocations)

**Logging:**
- Structured logging via `log/slog` (stdlib, Go 1.21+)
- Levels: DEBUG, INFO, WARN, ERROR
- Always include: `agent_id`, `task_id`, `workflow_id` when available
- Decision logging: every orchestration decision logged at INFO with reasoning

**Agent lifecycle:**
```
Register → Health Check → Ready → [Wake → Observe → Decide → Act → Record → Sleep] → Deregister
```

### Enforcement Guidelines

**All code MUST:**
- Pass `go vet` and `golangci-lint` with zero warnings
- Have test coverage for all exported functions
- Use the error wrapping pattern consistently
- Use structured logging (slog), never fmt.Println
- Define interfaces at point of use, not point of implementation

## Project Structure & Boundaries

### Complete Project Directory Structure

```
hive/
├── README.md
├── LICENSE
├── go.mod
├── go.sum
├── .goreleaser.yaml
├── .github/
│   └── workflows/
│       ├── ci.yaml
│       └── release.yaml
├── cmd/
│   └── hive/
│       └── main.go                  # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent types and interfaces
│   │   ├── agent_test.go
│   │   ├── manager.go               # Agent lifecycle management (FR1-FR7)
│   │   ├── manager_test.go
│   │   ├── registry.go              # Agent registration and discovery
│   │   └── registry_test.go
│   ├── adapter/
│   │   ├── adapter.go               # Adapter interface (FR24-FR28)
│   │   ├── adapter_test.go
│   │   ├── http.go                  # HTTP adapter implementation
│   │   ├── http_test.go
│   │   ├── stdio.go                 # Stdio adapter (CLI agents)
│   │   ├── stdio_test.go
│   │   ├── claude_code.go           # Claude Code adapter
│   │   ├── claude_code_test.go
│   │   ├── mcp.go                   # MCP server adapter
│   │   └── mcp_test.go
│   ├── event/
│   │   ├── bus.go                   # EventBus interface + in-process impl (FR19-FR23)
│   │   ├── bus_test.go
│   │   ├── store.go                 # SQLite event persistence
│   │   ├── store_test.go
│   │   ├── types.go                 # Event type definitions
│   │   └── types_test.go
│   ├── task/
│   │   ├── task.go                  # Task types and state machine
│   │   ├── task_test.go
│   │   ├── router.go                # Capability-based routing (FR13-FR14)
│   │   ├── router_test.go
│   │   ├── executor.go              # Task execution + checkpoint/resume (FR15-FR18)
│   │   └── executor_test.go
│   ├── workflow/
│   │   ├── workflow.go              # Workflow types and DAG (FR8-FR12)
│   │   ├── workflow_test.go
│   │   ├── parser.go                # YAML workflow parser
│   │   ├── parser_test.go
│   │   ├── engine.go                # Workflow execution engine
│   │   └── engine_test.go
│   ├── autonomy/
│   │   ├── plan.go                  # Agent behavioral plan (PLAN.yaml) (FR43-FR51)
│   │   ├── plan_test.go
│   │   ├── scheduler.go             # Heartbeat scheduler + wake-up cycles
│   │   ├── scheduler_test.go
│   │   ├── observer.go              # State observation for agent decisions
│   │   └── observer_test.go
│   ├── resilience/
│   │   ├── circuit_breaker.go       # Circuit breaker pattern (FR52-FR53)
│   │   ├── circuit_breaker_test.go
│   │   ├── failover.go              # Agent failover logic (FR54)
│   │   └── failover_test.go
│   ├── trust/                       # v0.2: Graduated autonomy engine
│   │   ├── engine.go                # Trust level tracking + auto-promotion (FR63-FR69)
│   │   ├── engine_test.go
│   │   ├── scorer.go                # Performance scoring (success rate, error rate)
│   │   └── scorer_test.go
│   ├── knowledge/                   # v0.2: Shared knowledge layer
│   │   ├── store.go                 # Knowledge CRUD + vector search (FR70-FR75)
│   │   ├── store_test.go
│   │   ├── embedding.go             # Text-to-vector embedding
│   │   └── embedding_test.go
│   ├── dialog/                      # v0.2: Agent-to-agent collaboration
│   │   ├── thread.go                # Dialog thread management (FR76-FR79)
│   │   └── thread_test.go
│   ├── webhook/                     # v0.2: Notification integrations
│   │   ├── dispatcher.go            # Webhook delivery + retry (FR80-FR83)
│   │   ├── dispatcher_test.go
│   │   ├── slack.go                 # Slack format
│   │   └── github.go                # GitHub format
│   ├── ws/                          # v0.2: WebSocket for dashboard
│   │   ├── hub.go                   # Connection hub + broadcast
│   │   └── hub_test.go
│   ├── storage/
│   │   ├── sqlite.go                # SQLite connection + migrations
│   │   ├── sqlite_test.go
│   │   ├── migrations/
│   │   │   ├── 001_initial.sql
│   │   │   ├── 002_v02_knowledge_trust_dialog_webhook.sql  # v0.2
│   │   │   └── embed.go             # Embedded migrations
│   │   └── queries.go               # Prepared SQL queries
│   ├── api/
│   │   ├── server.go                # HTTP server setup
│   │   ├── server_test.go
│   │   ├── routes.go                # Route registration
│   │   ├── middleware.go             # Auth, logging, CORS middleware
│   │   ├── handlers_agent.go        # /api/v1/agents handlers
│   │   ├── handlers_task.go         # /api/v1/tasks handlers
│   │   ├── handlers_workflow.go     # /api/v1/workflows handlers
│   │   ├── handlers_event.go        # /api/v1/events handlers
│   │   └── handlers_metrics.go      # /api/v1/metrics handlers
│   ├── cli/
│   │   ├── root.go                  # Root command + global flags
│   │   ├── init.go                  # hive init
│   │   ├── agent.go                 # hive add-agent, remove-agent, agent swap
│   │   ├── run.go                   # hive run
│   │   ├── status.go                # hive status
│   │   ├── logs.go                  # hive logs
│   │   ├── validate.go              # hive validate
│   │   └── template.go              # hive adapter-template
│   ├── config/
│   │   ├── config.go                # Configuration loading + validation
│   │   └── config_test.go
│   └── dashboard/
│       ├── embed.go                 # //go:embed dist/*
│       └── dist/                    # Svelte build output (gitignored, built before Go build)
├── web/                             # Svelte dashboard source
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.js
│   ├── src/
│   │   ├── app.html
│   │   ├── app.css
│   │   ├── lib/
│   │   │   ├── api.ts               # API client
│   │   │   ├── types.ts             # Shared types
│   │   │   └── stores.ts            # Svelte stores
│   │   └── routes/
│   │       ├── +layout.svelte
│   │       ├── +page.svelte          # Dashboard home
│   │       ├── agents/
│   │       │   └── +page.svelte      # Agent list + health
│   │       ├── tasks/
│   │       │   └── +page.svelte      # Task flow view
│   │       └── events/
│   │           └── +page.svelte      # Event timeline
│   └── static/
├── templates/                       # Hive project templates
│   ├── code-review/
│   │   ├── hive.yaml
│   │   └── README.md
│   ├── content-pipeline/
│   │   ├── hive.yaml
│   │   └── README.md
│   └── research/
│       ├── hive.yaml
│       └── README.md
├── protocol/                        # Agent Adapter Protocol spec
│   ├── spec.md                      # Protocol specification document
│   └── testkit/
│       ├── compliance_test.go       # Protocol compliance test suite
│       └── mock_agent.go            # Mock agent for testing
├── docs/
│   ├── quickstart.md
│   ├── adapter-guide.md
│   ├── configuration.md
│   └── contributing.md
└── Makefile                         # build, test, lint, dev targets
```

### Architectural Boundaries

**API Boundaries:**
- External: `/api/v1/*` — dashboard + external integrations
- Adapter: Agent-facing HTTP endpoints (orchestrator calls agents, not reverse)
- CLI: Talks to API server (even locally) — CLI is a client

**Package Boundaries:**
- `internal/` — all application code, not importable externally
- Each package owns its types and interfaces
- Cross-package communication via interfaces, not concrete types
- No circular dependencies (enforced by Go compiler)

**Data Boundaries:**
- Only `internal/storage/` touches SQLite directly
- Other packages use repository interfaces
- Events are the source of truth — state can be rebuilt from event log

### Requirements to Structure Mapping

| FR Category | Primary Package | Files |
|---|---|---|
| Agent Management (FR1-7) | `internal/agent/` | manager.go, registry.go |
| Workflow Definition (FR8-12) | `internal/workflow/` | workflow.go, parser.go, engine.go |
| Task Orchestration (FR13-18) | `internal/task/` | router.go, executor.go |
| Event System (FR19-23) | `internal/event/` | bus.go, store.go |
| Adapter Protocol (FR24-28) | `internal/adapter/` | adapter.go, http.go, stdio.go |
| Observability (FR29-33) | `internal/api/` | handlers_metrics.go, handlers_event.go |
| Agent Autonomy (FR43-51) | `internal/autonomy/` | plan.go, scheduler.go, observer.go |
| Error Handling (FR52-56) | `internal/resilience/` | circuit_breaker.go, failover.go |
| Dashboard (FR57-62) | `internal/api/` + `internal/ws/` + `web/` | server.go, hub.go, Svelte app |
| Graduated Autonomy (FR63-69) | `internal/trust/` | engine.go, scorer.go |
| Knowledge Layer (FR70-75) | `internal/knowledge/` | store.go, embedding.go |
| Agent Dialog (FR76-79) | `internal/dialog/` | thread.go |
| Webhooks (FR80-83) | `internal/webhook/` | dispatcher.go, slack.go, github.go |
| Framework Adapters (FR84-88) | `internal/adapter/` | crewai.go, langchain.go, autogen.go, openai.go |
| HiveHub (FR89-93) | `internal/hivehub/` | registry.go, publish.go, install.go |
| NATS Event Bus (FR94-97) | `internal/event/` | nats.go (implements EventBus interface) |
| Enhanced Knowledge (FR98-100) | `internal/knowledge/` | embedding.go (upgraded) |
| Cost Management (FR101-104) | `internal/cost/` | tracker.go, alerts.go |
| Market Allocation (FR105-109) | `internal/market/` | auction.go, bidder.go, economy.go |
| Cross-Hive Federation (FR110-115) | `internal/federation/` | protocol.go, discovery.go, proxy.go |
| Self-Optimization (FR116-120) | `internal/optimizer/` | analyzer.go, recommender.go |
| Enterprise Auth (FR121-122) | `internal/auth/` | oidc.go, rbac.go |
| Audit & Compliance (FR123-124) | `internal/audit/` | logger.go, exporter.go |
| Multi-Node (FR125-130) | `internal/cluster/` | node.go, replication.go |

### Data Flow

```
User (CLI) → API Server → Workflow Engine → Task Router → Adapter → Agent
                ↑              ↓                            ↓
           Dashboard      Event Bus ←←←←←←←←←←←←←←←← Event emission
                              ↓
                        SQLite (events, tasks, agents)
```

**Agent Wake-Up Flow:**
```
Scheduler (cron) → Wake agent → Observer reads state/backlog
                                    ↓
                              Plan evaluates → Action (invoke task / idle / escalate)
                                    ↓
                              Event emitted → Logged to SQLite
```

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
- Go + SQLite (modernc.org/sqlite) = true single binary, no CGO ✅
- Go channels + SQLite event store = reliable in-process event bus ✅
- Cobra CLI + Go HTTP server = CLI calls API for all operations ✅
- Svelte + Go embed = dashboard served from single binary ✅

**Pattern Consistency:**
- snake_case in DB, JSON, YAML — consistent across all data layers ✅
- PascalCase for Go types, camelCase for unexported — standard Go conventions ✅
- Dot notation for events — consistent with established patterns ✅

**Structure Alignment:**
- Package-per-domain matches FR categories ✅
- Interface-based boundaries enable future swaps (event bus, storage) ✅
- Test co-location follows Go conventions ✅

### Requirements Coverage Validation ✅

**All 56 FRs mapped to specific packages and files** (see mapping table above)

**NFR Coverage:**
- NFR1 (200ms event latency): In-process Go channels ✅
- NFR5 (hot-swap zero loss): Checkpoint/resume in task executor ✅
- NFR8 (ACID): SQLite WAL mode ✅
- NFR13 (single binary): Go + embed + modernc.org/sqlite ✅
- NFR14 (zero dependencies): All embedded ✅
- NFR16 (5 min onboarding): CLI scaffolding + templates ✅

### Implementation Readiness ✅

**Confidence Level:** HIGH

**Key Strengths:**
- Every technology choice is proven and stable
- Go's concurrency model is a natural fit for event-driven orchestration
- Single binary deployment eliminates entire classes of operational issues
- Interface-based architecture allows component swap without rewrite

**Areas for Future Enhancement:**
- WebSocket support for real-time dashboard (v0.2)
- NATS integration for multi-node event bus (v0.4)
- PostgreSQL option for enterprise-scale storage (v0.4)

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] 56 functional requirements analyzed and mapped
- [x] 23 non-functional requirements addressed
- [x] Scale and complexity assessed (medium)
- [x] Cross-cutting concerns identified (events, health, logging, config)

**✅ Architectural Decisions**
- [x] Language: Go 1.24
- [x] Storage: SQLite (modernc.org/sqlite)
- [x] Event bus: In-process (Go channels + SQLite)
- [x] Frontend: Svelte 5 (embedded in binary)
- [x] CLI: Cobra
- [x] Config: YAML
- [x] Build: GoReleaser

**✅ Implementation Patterns**
- [x] Naming conventions (Go, DB, API, events, config)
- [x] Structure patterns (package-per-domain, co-located tests)
- [x] Communication patterns (events, API, adapters)
- [x] Process patterns (error handling, logging, agent lifecycle)

**✅ Project Structure**
- [x] Complete directory tree with all files
- [x] FR-to-package mapping
- [x] Data flow diagram
- [x] Agent wake-up flow

### Implementation Handoff

**First implementation priority:**

```bash
# 1. Initialize Go module
go mod init github.com/julienvadic/hive

# 2. Create directory structure
# 3. Implement storage layer (SQLite + migrations)
# 4. Implement event bus
# 5. Implement adapter interface + HTTP adapter
# 6. Implement task router
# 7. Implement CLI (init, add-agent, run, status)
# 8. Implement workflow parser + engine
# 9. Add autonomy scheduler
# 10. Build dashboard
```

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently across all components
- Respect package boundaries — no circular dependencies
- All exported functions must have tests
- All orchestration decisions must be logged with structured context
