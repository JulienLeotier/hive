---
stepsCompleted: ['step-01-validate', 'step-02-design-epics', 'step-03-create-stories', 'step-04-final-validation']
inputDocuments:
  - '_bmad-output/planning-artifacts/prd.md'
  - '_bmad-output/planning-artifacts/architecture.md'
---

# Hive - Epic Breakdown

## Overview

This document decomposes the Hive PRD (56 FRs, 23 NFRs) and Architecture decisions into 7 epics and 42 implementable stories, organized by user value.

## Requirements Inventory

### Functional Requirements

- FR1: User can register an agent from any supported framework via CLI
- FR2: User can auto-detect agent type and capabilities from project structure
- FR3: User can manually configure agent capabilities via YAML
- FR4: User can list all registered agents with health status and capabilities
- FR5: User can remove a registered agent from the hive
- FR6: User can hot-swap a running agent without losing task state
- FR7: System validates agent connectivity and protocol compliance on registration
- FR8: User can define multi-agent workflows in YAML format
- FR9: User can specify task dependencies as a DAG
- FR10: User can define event triggers that initiate workflows
- FR11: User can define conditional routing based on task results
- FR12: System validates workflow configuration before execution
- FR13: System routes tasks to capable agents based on capabilities and availability
- FR14: System executes independent tasks in parallel when DAG allows
- FR15: System passes task results between agents as defined in workflow
- FR16: System emits events for all state changes
- FR17: System checkpoints in-progress task state at configurable intervals
- FR18: System resumes tasks from checkpoint after agent replacement or failure
- FR19: System delivers events to subscribed agents within 200ms p95
- FR20: Agents can subscribe to specific event types
- FR21: Agents can emit custom events
- FR22: System maintains ordered event log for replay and debugging
- FR23: User can query event history via CLI
- FR24: Adapter author can implement protocol in under 20 lines for basic HTTP agents
- FR25: Adapter author can generate boilerplate via CLI template command
- FR26: Adapter author can run protocol compliance test suite
- FR27: System supports HTTP/JSON, WebSocket, and stdio transport
- FR28: Adapters declare agent capabilities in structured format
- FR29: User can view real-time hive status
- FR30: User can query agent-specific logs with filtering
- FR31: User can view task execution timeline
- FR32: System exposes metrics endpoint for external monitoring
- FR33: System logs all orchestration decisions with reasoning context
- FR34: User can scaffold a new hive project via hive init
- FR35: User can select from pre-built hive templates
- FR36: System supports environment-based configuration overrides
- FR37: System stores all state in embedded SQLite
- FR43: User can define agent behavioral plan as YAML state machine
- FR44: System executes agent wake-up cycles on configurable schedules
- FR45: Agent observes shared state, event history, and backlog at each wake-up
- FR46: Agent can self-assign tasks from shared backlog
- FR47: Agent can choose to idle when no relevant work exists
- FR48: System logs each agent's wake-up decision
- FR49: User can define agent identity and constraints via AGENT.yaml
- FR50: User can inspect and modify agent behavior by editing YAML
- FR51: System monitors for busywork patterns
- FR52: System implements circuit breaker pattern for failing agents
- FR53: System auto-isolates agents exceeding failure thresholds
- FR54: System reroutes queued tasks from isolated agents
- FR55: System provides clear error messages with remediation
- FR56: User can configure retry policies per agent or task type

### Non-Functional Requirements

- NFR1: Event latency < 200ms p95
- NFR2: CLI commands respond < 500ms
- NFR5: Hot-swap zero data loss 99.9%
- NFR6: Crash recovery < 10s from checkpoint
- NFR7: Strict event ordering
- NFR8: SQLite ACID compliance
- NFR9: API key + mTLS auth support
- NFR10: Secrets never logged
- NFR13: Single binary, cross-platform
- NFR14: Zero external dependencies
- NFR16: First workflow < 5 minutes
- NFR17: Adapter authoring < 30 minutes
- NFR20: Shell completion (bash, zsh, fish)

### Additional Requirements (Architecture)

- Go 1.24 with modernc.org/sqlite (pure Go, no CGO)
- Cobra CLI framework
- Structured logging via log/slog
- ULID for all entity IDs
- GoReleaser for cross-platform builds
- Svelte 5 dashboard (embedded in binary) — deferred to post-MVP

### UX Design Requirements

No UX design document — CLI-first product. Dashboard deferred to v0.2.

### FR Coverage Map

| FR | Epic | Story |
|---|---|---|
| FR1, FR3, FR5, FR7 | Epic 1 | 1.3 |
| FR2 | Epic 7 | 7.2 |
| FR4 | Epic 1 | 1.4 |
| FR6 | Epic 5 | 5.4 |
| FR8 | Epic 3 | 3.1 |
| FR9 | Epic 3 | 3.2 |
| FR10 | Epic 3 | 3.4 |
| FR11 | Epic 3 | 3.5 |
| FR12 | Epic 3 | 3.6 |
| FR13 | Epic 2 | 2.3 |
| FR14 | Epic 2 | 2.5 |
| FR15 | Epic 2 | 2.4 |
| FR16, FR19-FR22 | Epic 2 | 2.1 |
| FR17, FR18 | Epic 2 | 2.6 |
| FR23 | Epic 6 | 6.1 |
| FR24, FR26-FR28 | Epic 1 | 1.2 |
| FR25 | Epic 7 | 7.4 |
| FR29 | Epic 6 | 6.1 |
| FR30 | Epic 6 | 6.2 |
| FR31 | Epic 6 | 6.3 |
| FR32 | Epic 6 | 6.4 |
| FR33 | Epic 6 | 6.5 |
| FR34, FR35 | Epic 7 | 7.1 |
| FR36, FR37 | Epic 1 | 1.1 |
| FR43, FR49, FR50 | Epic 4 | 4.1 |
| FR44 | Epic 4 | 4.2 |
| FR45 | Epic 4 | 4.3 |
| FR46 | Epic 4 | 4.4 |
| FR47, FR51 | Epic 4 | 4.5 |
| FR48 | Epic 4 | 4.6 |
| FR52 | Epic 5 | 5.1 |
| FR53 | Epic 5 | 5.2 |
| FR54 | Epic 5 | 5.3 |
| FR55, FR56 | Epic 5 | 5.5 |
| NFR9 | Epic 1 | 1.7 |

## Epic List

### Epic 1: Agent Registration & Communication
Users can register AI agents from any framework and verify they're connected and healthy, with secure communication.
**FRs covered:** FR1-FR7, FR24, FR26-FR28, FR36, FR37, NFR9

### Epic 2: Event-Driven Task Orchestration
Users can route tasks to the right agent and get results back, with full event tracking.
**FRs covered:** FR13-FR22

### Epic 3: Workflow Definition & Execution
Users can define multi-agent workflows in YAML and run them end-to-end.
**FRs covered:** FR8-FR12

### Epic 4: Agent Autonomy & Self-Direction
Agents know what to do at each wake-up without human direction.
**FRs covered:** FR43-FR51

### Epic 5: Resilience & Self-Healing
The system self-heals when agents fail, without losing work.
**FRs covered:** FR6, FR52-FR56

### Epic 6: Observability & Monitoring
Users can see what's happening in their hive and debug issues.
**FRs covered:** FR23, FR29-FR33

### Epic 7: CLI & Developer Experience
Users can go from zero to first orchestrated workflow in under 5 minutes.
**FRs covered:** FR2, FR25, FR34, FR35

---

## Epic 1: Agent Registration & Communication

Users can register AI agents from any framework (Claude Code, MCP, HTTP) and verify they're connected, healthy, and ready to receive work.

### Story 1.1: Project Bootstrap & Storage Layer

As a developer,
I want to initialize the Hive Go module with embedded SQLite storage,
So that I have a solid foundation to build all other features on.

**Acceptance Criteria:**

**Given** a fresh Go module with `go mod init`
**When** the application starts
**Then** it initializes a SQLite database in `~/.hive/data/hive.db` with WAL mode enabled
**And** runs embedded SQL migrations to create `agents`, `events`, `tasks`, `workflows` tables
**And** creates the data directory with `0700` permissions if it doesn't exist
**And** configuration loads from `hive.yaml` with environment variable overrides (FR36, FR37)

### Story 1.2: Agent Adapter Protocol — HTTP Implementation

As an adapter author,
I want a clear protocol interface and HTTP adapter implementation,
So that I can connect any HTTP-based agent to Hive in under 20 lines.

**Acceptance Criteria:**

**Given** the `Adapter` Go interface with `Declare()`, `Invoke()`, `Health()`, `Checkpoint()`, `Resume()` methods
**When** an adapter author implements the HTTP adapter for their agent
**Then** the implementation requires fewer than 20 lines of configuration for a basic agent
**And** the adapter communicates via HTTP/JSON with the agent's endpoints
**And** the adapter supports stdio transport for CLI-based agents
**And** a protocol compliance test suite validates any adapter implementation (FR24, FR26-FR28)

### Story 1.3: Agent Registration via CLI

As a user,
I want to register, configure, and remove agents via the `hive` CLI,
So that I can manage which agents participate in my hive.

**Acceptance Criteria:**

**Given** a running hive with the storage layer initialized
**When** the user runs `hive add-agent --name code-reviewer --type http --url http://localhost:8080`
**Then** the system validates connectivity by calling the agent's `/health` endpoint
**And** calls `/declare` to retrieve and store the agent's capabilities
**And** stores the agent record in SQLite with status `healthy`
**And** confirms registration with a success message showing agent name and capabilities

**Given** a registered agent
**When** the user runs `hive remove-agent code-reviewer`
**Then** the agent is removed from the registry
**And** any queued tasks for that agent are returned to the unassigned pool

**Given** a registered agent
**When** the user runs `hive add-agent --name reviewer --config ./agent.yaml`
**Then** the system reads capabilities from the YAML config file (FR1, FR3, FR5, FR7)

### Story 1.4: Agent Health & Listing

As a user,
I want to list all registered agents and see their health status,
So that I know which agents are available and functioning.

**Acceptance Criteria:**

**Given** one or more registered agents
**When** the user runs `hive status`
**Then** the output shows a table with: agent name, type, health status, capabilities, last health check time
**And** health is refreshed by calling each agent's `/health` endpoint
**And** agents that fail health check are marked as `degraded` or `unavailable`
**And** output supports `--json` flag for machine-readable format (FR4)

### Story 1.5: Claude Code Adapter

As a user,
I want to register Claude Code agents (skills/workflows) with Hive,
So that I can orchestrate my existing Claude Code agents alongside other frameworks.

**Acceptance Criteria:**

**Given** a Claude Code skill or workflow at a local path
**When** the user runs `hive add-agent --type claude-code --path ./my-skill`
**Then** the adapter auto-detects the Claude Code agent's capabilities from its skill definition
**And** wraps the Claude Code invocation in the Hive adapter protocol
**And** the agent can receive tasks and return results through the standard protocol

### Story 1.6: MCP Server Adapter

As a user,
I want to register MCP (Model Context Protocol) servers with Hive,
So that I can orchestrate MCP tools as part of my agent workflows.

**Acceptance Criteria:**

**Given** an MCP server running locally or remotely
**When** the user runs `hive add-agent --type mcp --url stdio:///path/to/mcp-server` or `--url http://localhost:3000`
**Then** the adapter connects to the MCP server and retrieves its tool list
**And** maps MCP tools to Hive capabilities
**And** the agent can receive tasks that invoke specific MCP tools and return results

### Story 1.7: API Key Authentication

As a user,
I want agent-to-orchestrator communication secured by API keys,
So that only authorized agents can register and execute tasks in my hive.

**Acceptance Criteria:**

**Given** the hive server is running
**When** an agent makes any API call without a valid API key
**Then** the server responds with `401 Unauthorized` and a clear error message

**Given** the user runs `hive api-key generate --name my-agent-key`
**When** the key is generated
**Then** the API key is displayed once to the user
**And** only a bcrypt hash is stored in the `api_keys` table in SQLite
**And** the key is never logged or stored in plaintext

**Given** an agent includes a valid API key in the `Authorization: Bearer <key>` header
**When** it calls any orchestrator endpoint (`/declare`, `/invoke`, `/health`, etc.)
**Then** the server validates the key hash and allows the request
**And** the request is logged with the key name (not the key value) (NFR9, NFR10)

---

## Epic 2: Event-Driven Task Orchestration

Users can create tasks, route them to capable agents, and get results back — with all state changes tracked as events.

### Story 2.1: Event Bus & Persistence

As a developer,
I want an in-process event bus that persists all events to SQLite,
So that the system has reliable, ordered event delivery with replay capability.

**Acceptance Criteria:**

**Given** the event bus is initialized on application startup
**When** any component calls `eventBus.Publish(event)`
**Then** the event is persisted to the `events` table before delivery to subscribers
**And** events are delivered to matching subscribers within 200ms p95 (NFR1)
**And** event ordering is strictly maintained via auto-increment ID (NFR7)
**And** subscribers can register for event type prefixes (e.g., `task.*` matches `task.created`)
**And** agents can emit custom events via the adapter protocol (FR16, FR19-FR22)

### Story 2.2: Task State Machine

As the system,
I want tasks with a well-defined state machine,
So that task lifecycle is predictable and debuggable.

**Acceptance Criteria:**

**Given** a task is created
**When** it progresses through its lifecycle
**Then** it follows states: `pending` → `assigned` → `running` → `completed` | `failed`
**And** each state transition emits an event (`task.created`, `task.assigned`, `task.started`, `task.completed`, `task.failed`)
**And** tasks have ULID identifiers
**And** task input/output is stored as JSON in SQLite

### Story 2.3: Capability-Based Task Routing

As a user,
I want tasks automatically routed to the right agent based on capabilities,
So that I don't have to manually assign every task.

**Acceptance Criteria:**

**Given** a task with required capabilities (e.g., `{"requires": ["code-review", "go"]}`)
**When** the task enters the routing engine
**Then** the system matches task requirements against registered agent capabilities
**And** selects the best available agent (healthy, not at capacity)
**And** assigns the task and emits `task.assigned` event
**And** if no capable agent is available, the task remains `pending` with a `task.unroutable` event (FR13)

### Story 2.4: Task Execution & Result Passing

As a user,
I want tasks executed by agents with results passed to downstream tasks,
So that multi-step workflows produce cumulative results.

**Acceptance Criteria:**

**Given** a task is assigned to an agent
**When** the orchestrator invokes the agent's `/invoke` endpoint with the task payload
**Then** the agent processes the task and returns a result
**And** the result is stored in the task's `output` field
**And** downstream tasks receive the upstream result as part of their input
**And** a `task.completed` event is emitted with duration and result summary (FR15)

### Story 2.5: Parallel Task Execution

As a user,
I want independent tasks to execute in parallel,
So that workflows complete as fast as possible.

**Acceptance Criteria:**

**Given** a workflow DAG where tasks A and B have no dependency on each other
**When** both tasks are ready to execute
**Then** the system dispatches both tasks concurrently (separate goroutines)
**And** task C that depends on both A and B starts only after both complete
**And** the system respects a configurable concurrency limit per workflow (FR14)

### Story 2.6: Task Checkpoint & Resume

As a user,
I want long-running tasks to checkpoint their state,
So that work isn't lost if an agent fails or is replaced.

**Acceptance Criteria:**

**Given** a task is running and checkpoint interval is configured (default: 30s)
**When** the checkpoint interval elapses
**Then** the orchestrator calls the agent's `/checkpoint` endpoint
**And** the serialized state is stored in the task's `checkpoint` field in SQLite
**And** if the agent fails, the task can be reassigned to another agent
**And** the new agent's `/resume` endpoint is called with the checkpoint data
**And** the task continues from where it left off without data loss (FR17, FR18, NFR5)

---

## Epic 3: Workflow Definition & Execution

Users can define complex multi-agent workflows in YAML and execute them with event triggers and conditional routing.

### Story 3.1: YAML Workflow Parser

As a user,
I want to define workflows in a `hive.yaml` file,
So that I can declaratively describe how agents collaborate.

**Acceptance Criteria:**

**Given** a `hive.yaml` file with workflow definition
**When** the parser reads the file
**Then** it produces a validated workflow struct with: name, tasks, dependencies, triggers
**And** tasks reference agent capabilities (not agent names) for loose coupling
**And** parser errors include line numbers and clear error messages
**And** the workflow is stored in the `workflows` table (FR8)

### Story 3.2: DAG Dependency Resolution

As the system,
I want workflow task dependencies represented as a DAG,
So that execution order is optimized and deadlocks are impossible.

**Acceptance Criteria:**

**Given** a workflow with task dependencies
**When** the DAG resolver processes the workflow
**Then** it produces a topologically sorted execution plan
**And** identifies tasks that can run in parallel (no mutual dependencies)
**And** detects and rejects circular dependencies with clear error message
**And** calculates the critical path for execution time estimation (FR9)

### Story 3.3: Workflow Execution Engine

As a user,
I want to run a workflow end-to-end with `hive run`,
So that I can see my agents collaborate on a complete task.

**Acceptance Criteria:**

**Given** a valid workflow definition and registered agents
**When** the user runs `hive run` or `hive run --workflow my-workflow`
**Then** the engine creates tasks for each workflow step
**And** routes tasks to agents via the capability router
**And** executes tasks in DAG order with parallel branches
**And** passes results between tasks as defined
**And** emits workflow-level events (`workflow.started`, `workflow.completed`, `workflow.failed`)
**And** displays real-time progress in the terminal (task status updates)

### Story 3.4: Event Triggers

As a user,
I want workflows triggered by events (webhooks, schedules, or manual),
So that my hive reacts automatically to external signals.

**Acceptance Criteria:**

**Given** a workflow with a trigger definition in `hive.yaml`
**When** a matching event occurs (HTTP webhook, cron schedule, or `hive run --trigger manual`)
**Then** the workflow is automatically instantiated and executed
**And** the trigger payload is available as input to the first task
**And** multiple instances of the same workflow can run concurrently (FR10)

### Story 3.5: Conditional Routing

As a user,
I want workflow paths to branch based on task results,
So that my workflows can handle different outcomes intelligently.

**Acceptance Criteria:**

**Given** a workflow with conditional branches defined in YAML
**When** a task completes with a result
**Then** the engine evaluates conditions against the result
**And** routes to the matching branch (e.g., `if result.score > 0.8 then task-A else task-B`)
**And** unmatched conditions fall through to a default branch if defined
**And** missing default branch with unmatched condition produces clear error (FR11)

### Story 3.6: Workflow Validation

As a user,
I want to validate my workflow config before running it,
So that I catch errors early without wasting agent compute.

**Acceptance Criteria:**

**Given** a `hive.yaml` workflow definition
**When** the user runs `hive validate`
**Then** the system checks: YAML syntax, task dependency DAG validity, capability requirements match registered agents, trigger configuration
**And** reports all issues (not just the first one)
**And** exits with code 0 on success, non-zero on failure
**And** output supports `--json` for CI integration (FR12)

---

## Epic 4: Agent Autonomy & Self-Direction

Agents carry behavioral plans and know what to do at each wake-up — observe state, decide, act, or idle — without human task assignment.

### Story 4.1: Agent Identity & Behavioral Plan Parser

As a user,
I want to define agent identity (AGENT.yaml) and behavioral plans (PLAN.yaml),
So that each agent knows who it is and what to do autonomously.

**Acceptance Criteria:**

**Given** an `AGENT.yaml` file with identity, capabilities, constraints, and anti-patterns
**When** the parser loads the file
**Then** the agent's identity is stored and used for all decision-making
**And** constraints are enforced (e.g., "never modify production data")

**Given** a `PLAN.yaml` file with a state machine definition
**When** the parser loads the file
**Then** the plan defines: states, transitions, observation rules, action handlers, idle conditions
**And** the plan can be modified by editing the YAML file (changes take effect at next wake-up)
**And** invalid plans produce clear error messages (FR43, FR49, FR50)

### Story 4.2: Heartbeat Scheduler

As a user,
I want agents to wake up on configurable schedules,
So that they check for work at appropriate intervals.

**Acceptance Criteria:**

**Given** an agent with a configured heartbeat interval (e.g., `heartbeat: 60s`)
**When** the interval elapses
**Then** the scheduler triggers the agent's wake-up cycle
**And** multiple agents can have different heartbeat intervals
**And** heartbeats can also be triggered by events (hybrid scheduling)
**And** the scheduler respects system load (backpressure) (FR44)

### Story 4.3: State Observer

As an agent,
I want to observe the current state of the world at each wake-up,
So that I can make informed decisions about what to do.

**Acceptance Criteria:**

**Given** an agent's wake-up cycle is triggered
**When** the observer runs
**Then** it gathers: pending tasks in shared backlog matching agent capabilities, recent events since last wake-up, current workflow states, other agents' health status
**And** presents this context to the agent's plan evaluator
**And** observation completes within 100ms (FR45)

### Story 4.4: Task Self-Assignment

As an agent,
I want to claim tasks from the shared backlog based on my capabilities,
So that work gets done without a central dispatcher assigning me.

**Acceptance Criteria:**

**Given** the observer found pending tasks matching this agent's capabilities
**When** the plan evaluator decides to take action
**Then** the agent claims a task atomically (SQLite transaction prevents double-claiming)
**And** the task status changes to `assigned` with this agent's ID
**And** a `task.self_assigned` event is emitted
**And** if the task was already claimed by another agent, the agent tries the next one (FR46)

### Story 4.5: Idle Detection & Busywork Prevention

As the system,
I want agents to idle gracefully when there's no work,
So that they don't waste compute generating unnecessary tasks.

**Acceptance Criteria:**

**Given** an agent wakes up and the observer finds no matching work
**When** the plan evaluator runs
**Then** the agent logs "idle — no relevant work" and returns to sleep
**And** the idle decision is recorded as a normal event (`agent.idle`)
**And** agents that create tasks without backlog demand are flagged as "busywork generators"
**And** busywork detection uses a simple heuristic: tasks created without upstream trigger or backlog source (FR47, FR51)

### Story 4.6: Wake-Up Decision Logging

As a user,
I want every agent wake-up decision logged with full reasoning,
So that I can audit and debug autonomous agent behavior.

**Acceptance Criteria:**

**Given** an agent completes a wake-up cycle
**When** the cycle finishes (action taken or idle)
**Then** the system logs: agent ID, timestamp, what was observed (backlog count, events count), what was decided (action/idle), why (plan state + matching rule), duration
**And** logs are queryable via `hive logs --agent <name> --decisions` (FR48)

---

## Epic 5: Resilience & Self-Healing

The system detects agent failures, isolates unhealthy agents, reroutes work, and enables zero-downtime agent replacement.

### Story 5.1: Circuit Breaker

As the system,
I want circuit breakers on all agent invocations,
So that failing agents don't cascade failures across the system.

**Acceptance Criteria:**

**Given** an agent fails 3 consecutive invocations (configurable threshold)
**When** the circuit breaker trips
**Then** subsequent invocations to that agent return immediately with "circuit open" error
**And** a `agent.circuit_open` event is emitted
**And** after 30 seconds (configurable), the circuit enters half-open state
**And** next invocation is a test — success closes circuit, failure reopens it (FR52)

### Story 5.2: Agent Auto-Isolation

As the system,
I want unhealthy agents automatically isolated from task routing,
So that tasks aren't sent to agents that will fail.

**Acceptance Criteria:**

**Given** an agent's health check fails or circuit breaker is open
**When** the isolation threshold is exceeded
**Then** the agent is marked `isolated` in the registry
**And** the task router skips isolated agents
**And** an `agent.isolated` event is emitted with reason
**And** isolation is reversible when health is restored (FR53)

### Story 5.3: Task Failover

As a user,
I want failed tasks automatically rerouted to healthy agents,
So that work completes despite individual agent failures.

**Acceptance Criteria:**

**Given** a task fails due to agent unavailability (not a business logic error)
**When** failover triggers
**Then** the system finds another capable, healthy agent
**And** reassigns the task with its last checkpoint (if available)
**And** the new agent resumes from checkpoint or restarts the task
**And** a `task.failover` event records the original agent, new agent, and reason (FR54)

### Story 5.4: Agent Hot-Swap

As a user,
I want to replace a running agent with zero downtime,
So that I can upgrade or switch agents without losing work.

**Acceptance Criteria:**

**Given** a running agent with in-progress tasks
**When** the user runs `hive agent swap old-agent --to new-agent`
**Then** in-progress tasks are checkpointed
**And** the old agent is gracefully disconnected
**And** the new agent is registered and health-checked
**And** checkpointed tasks are resumed on the new agent
**And** zero tasks are lost in the swap process (FR6, NFR5)

### Story 5.5: Retry Policies & Error Messages

As a user,
I want configurable retry policies and clear error messages,
So that transient failures are handled automatically and I understand persistent failures.

**Acceptance Criteria:**

**Given** a task fails with a retryable error
**When** retry policy is configured (e.g., `retries: 3, backoff: exponential`)
**Then** the system retries with configured backoff (1s, 2s, 4s)
**And** each retry emits a `task.retry` event

**Given** any error in the system
**When** it's surfaced to the user (CLI or logs)
**Then** the error message includes: what went wrong, which agent/task was involved, a suggested remediation action
**And** never includes secrets, tokens, or full stack traces (FR55, FR56, NFR10)

---

## Epic 6: Observability & Monitoring

Users have full visibility into their hive — agent health, task flow, event history, and orchestration decisions.

### Story 6.1: Hive Status Command

As a user,
I want a comprehensive status overview via `hive status`,
So that I can quickly assess my hive's health.

**Acceptance Criteria:**

**Given** a hive with registered agents and tasks
**When** the user runs `hive status`
**Then** output shows: agent count (healthy/degraded/unavailable), active tasks (by status), recent events (last 10), workflow states
**And** data refreshes agent health in real-time
**And** supports `--json` output for scripting
**And** responds within 500ms (NFR2) (FR23, FR29)

### Story 6.2: Log Querying

As a user,
I want to query agent and system logs with filtering,
So that I can debug issues efficiently.

**Acceptance Criteria:**

**Given** the system has logged events and decisions
**When** the user runs `hive logs --agent code-reviewer --since 1h --type error`
**Then** matching log entries are displayed in chronological order
**And** filters support: agent name, time range, event type, log level
**And** output supports `--json` for parsing
**And** `--follow` flag streams new entries in real-time (FR30)

### Story 6.3: Task Execution Timeline

As a user,
I want to see a task execution timeline for workflows,
So that I can understand execution flow and identify bottlenecks.

**Acceptance Criteria:**

**Given** a completed or running workflow
**When** the user runs `hive logs --workflow <id> --timeline`
**Then** output shows each task with: start time, end time, duration, agent, status
**And** parallel tasks are visually indicated
**And** the critical path is highlighted (FR31)

### Story 6.4: Metrics Endpoint

As an ops engineer,
I want a metrics endpoint for external monitoring,
So that I can integrate Hive into my existing observability stack.

**Acceptance Criteria:**

**Given** the Hive server is running
**When** an external system hits `GET /api/v1/metrics`
**Then** it returns: agent count by status, task count by status, event throughput (events/sec), average task duration, circuit breaker states
**And** format is JSON (Prometheus format deferred to v0.2) (FR32)

### Story 6.5: Orchestration Decision Logging

As a user,
I want every orchestration decision logged with reasoning,
So that I can understand why the system made specific choices.

**Acceptance Criteria:**

**Given** the system makes an orchestration decision (routing, failover, isolation, etc.)
**When** the decision is made
**Then** a structured log entry includes: decision type, input context, options considered, choice made, reasoning
**And** decision logs are queryable via `hive logs --decisions`
**And** log format uses slog structured fields (FR33)

---

## Epic 7: CLI & Developer Experience

Users can go from zero to a running multi-agent workflow in under 5 minutes.

### Story 7.1: hive init — Project Scaffolding

As a new user,
I want to scaffold a hive project with one command,
So that I can start orchestrating agents immediately.

**Acceptance Criteria:**

**Given** the user runs `hive init my-project`
**When** scaffolding completes
**Then** a directory `my-project/` is created with: `hive.yaml` (workflow config), `agents/` directory with example agent configs, `README.md` with quickstart instructions
**And** `hive init --template code-review` uses the code review template
**And** `hive init` with no template offers interactive selection (FR34, FR35)

### Story 7.2: Agent Auto-Detection

As a user,
I want `hive add-agent` to auto-detect agent type from project structure,
So that registration is as simple as pointing to a directory.

**Acceptance Criteria:**

**Given** a directory containing a Claude Code skill, MCP server config, or HTTP agent
**When** the user runs `hive add-agent --path ./my-agent`
**Then** the system detects the agent type from project files (e.g., `skill.md` → Claude Code, `mcp.json` → MCP)
**And** auto-configures the adapter
**And** confirms detected type with user before registering (FR2)

### Story 7.3: hive run — Terminal Output

As a user,
I want clear, real-time terminal output when running workflows,
So that I can see what's happening without checking logs.

**Acceptance Criteria:**

**Given** a valid workflow and registered agents
**When** the user runs `hive run`
**Then** terminal shows: workflow start, each task dispatched (agent + capability), task results (success/failure + summary), workflow completion summary with duration
**And** `--quiet` flag suppresses all output except final result
**And** `--json` outputs structured progress events

### Story 7.4: Adapter Template Generator

As an adapter author,
I want to generate adapter boilerplate for my framework,
So that I can implement and contribute a new adapter quickly.

**Acceptance Criteria:**

**Given** the user runs `hive adapter-template my-framework`
**When** generation completes
**Then** a directory `my-framework/` is created with: adapter Go source file with interface stubs, protocol compliance test file, example configuration, README with contribution guide
**And** the generated adapter compiles and passes the compliance test suite (with `TODO` implementations) (FR25, NFR17)

### Story 7.5: Example Template — Code Review Hive

As a new user,
I want a pre-built "code review" hive template,
So that I can see a real orchestration example immediately.

**Acceptance Criteria:**

**Given** the user runs `hive init --template code-review`
**When** initialization completes
**Then** the project includes: workflow that takes a PR URL, routes to a code-review agent, then a summary agent, example agent configs for HTTP-based review and summary agents, documentation explaining the workflow

### Story 7.6: Example Template — Content Pipeline Hive

As a new user,
I want a pre-built "content pipeline" hive template,
So that I can see multi-agent content production in action.

**Acceptance Criteria:**

**Given** the user runs `hive init --template content-pipeline`
**When** initialization completes
**Then** the project includes: workflow with writer → editor → SEO optimizer → publisher stages, example agent configs for each stage, documentation explaining the pipeline

### Story 7.7: Example Template — Research Hive

As a new user,
I want a pre-built "research" hive template,
So that I can see parallel research agent orchestration.

**Acceptance Criteria:**

**Given** the user runs `hive init --template research`
**When** initialization completes
**Then** the project includes: workflow with parallel research agents → aggregator → report generator, example agent configs for research and synthesis, documentation explaining the research pattern

### Story 7.8: Shell Completion

As a power user,
I want shell completion for all hive commands,
So that I can work faster in the terminal.

**Acceptance Criteria:**

**Given** the user runs `hive completion bash` (or zsh/fish)
**When** the output is sourced in the shell
**Then** tab completion works for: all commands and subcommands, `--flags` for each command, agent names for agent-specific commands, workflow names for workflow commands
**And** installation instructions are included in `hive completion --help` (NFR20)

### Story 7.9: CI/CD Pipeline & Cross-Platform Build

As a contributor,
I want automated CI/CD that tests, builds, and releases Hive for all platforms,
So that every merge produces a verified, cross-platform binary.

**Acceptance Criteria:**

**Given** a pull request is opened against the main branch
**When** GitHub Actions CI runs
**Then** it executes `go vet`, `golangci-lint`, and the full test suite
**And** CI fails if any check fails or test coverage drops below 80% on core packages
**And** CI completes in under 5 minutes

**Given** a new tag is pushed (e.g., `v0.1.0`)
**When** the release workflow triggers
**Then** GoReleaser builds binaries for macOS (arm64, x64), Linux (x64, arm64), Windows (x64)
**And** creates a GitHub Release with checksums and changelog
**And** updates the Homebrew tap formula
**And** builds and pushes the Docker image (~15MB scratch-based)
