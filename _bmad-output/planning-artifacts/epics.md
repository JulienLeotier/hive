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

---

# v0.2 Epics — Trust & Visibility

## Epic 8: Dashboard UI

Users can monitor their hive in real-time through a web dashboard showing agent health, task flow, events, and costs.

### Story 8.1: Svelte Project Setup & Embedding

As a developer,
I want the Svelte 5 dashboard scaffolded and embedded in the Go binary,
So that the dashboard is served from the single hive binary with no separate deployment.

**Acceptance Criteria:**

**Given** the `web/` directory contains a Svelte 5 project with SvelteKit
**When** `make build` runs
**Then** Svelte builds to `internal/dashboard/dist/` and Go embeds it via `//go:embed`
**And** `hive serve` starts the API server and serves the dashboard at `http://localhost:8233`
**And** the dashboard loads in under 2 seconds (NFR24)

### Story 8.2: Agent Health Dashboard Page

As a user,
I want a dashboard page showing all agents with real-time health status,
So that I can monitor my hive at a glance.

**Acceptance Criteria:**

**Given** the dashboard is loaded in a browser
**When** the agents page is displayed
**Then** it shows a table of all agents with: name, type, health status, trust level, capabilities, last check time
**And** health status updates in real-time via WebSocket without page refresh (FR57, FR61)

### Story 8.3: Task Flow Visualization

As a user,
I want to see active and completed tasks with their status and timing,
So that I can understand workflow execution and spot bottlenecks.

**Acceptance Criteria:**

**Given** workflows have been executed
**When** the tasks page is displayed
**Then** it shows tasks grouped by workflow with: status, agent, duration, result summary
**And** active tasks update in real-time (FR58)

### Story 8.4: Event Timeline

As a user,
I want a real-time event timeline with filtering,
So that I can debug issues and understand system behavior.

**Acceptance Criteria:**

**Given** events are being published
**When** the events page is displayed
**Then** it shows events in reverse chronological order with: type, source, payload preview, timestamp
**And** user can filter by event type prefix and source
**And** new events appear in real-time via WebSocket (FR59, FR61)

### Story 8.5: WebSocket Hub

As a developer,
I want a WebSocket hub that broadcasts events to connected dashboard clients,
So that the dashboard updates in real-time.

**Acceptance Criteria:**

**Given** the API server is running with WebSocket support
**When** a dashboard client connects to `/ws`
**Then** it receives all events as they're published
**And** event delivery to WebSocket is under 100ms from publication (NFR25)
**And** stale connections are detected via ping/pong and cleaned up

### Story 8.6: Cost Tracking Page

As a user,
I want to see cost tracking per agent and per workflow,
So that I can manage my AI spend.

**Acceptance Criteria:**

**Given** agents declare cost_per_run in capabilities
**When** the cost page is displayed
**Then** it shows: cost per agent (total and recent), cost per workflow, cost trend over time (FR60)

---

## Epic 9: Graduated Autonomy Engine

Agents earn increasing trust through demonstrated competence, with configurable thresholds and per-task-type overrides.

### Story 9.1: Trust Level Tracking

As a user,
I want each agent to have a tracked trust level that reflects its performance,
So that I can progressively grant more autonomy to reliable agents.

**Acceptance Criteria:**

**Given** an agent is registered
**When** it completes tasks
**Then** the system tracks: total tasks completed, success rate, error rate, consecutive successes
**And** the trust level is stored in the agents table (FR63)

### Story 9.2: Auto-Promotion Engine

As a user,
I want agents automatically promoted when they meet configured thresholds,
So that trust evolves without manual intervention.

**Acceptance Criteria:**

**Given** an agent has trust thresholds configured (e.g., "Guided after 50 tasks, <5% error")
**When** the agent meets the threshold criteria
**Then** the system promotes the agent to the next trust level
**And** logs the promotion with criteria details in trust_history (FR64, FR66, FR69)

### Story 9.3: Per-Task-Type Overrides

As a user,
I want certain task types to always require specific trust levels,
So that high-risk operations maintain human oversight regardless of agent track record.

**Acceptance Criteria:**

**Given** a trust override is configured (e.g., "financial-transactions: always Supervised")
**When** a task of that type is routed
**Then** the system enforces the override level regardless of the agent's earned trust
**And** the override is logged (FR65, FR68)

### Story 9.4: Manual Trust Management

As a user,
I want to manually promote or demote agents via CLI,
So that I can override the automatic system when needed.

**Acceptance Criteria:**

**Given** a registered agent
**When** the user runs `hive agent trust code-reviewer --level autonomous`
**Then** the agent's trust level is updated immediately
**And** a `manual_override` entry is logged in trust_history (FR67)

---

## Epic 10: Shared Knowledge Layer

The hive accumulates operational wisdom — successful approaches and known failures — that new agents inherit.

### Story 10.1: Knowledge Store & CRUD

As a developer,
I want a knowledge store backed by SQLite,
So that learned patterns persist across restarts.

**Acceptance Criteria:**

**Given** the v0.2 migration has run (002_v02.sql)
**When** a task completes (success or failure)
**Then** the approach and outcome are stored in the knowledge table
**And** entries include: task_type, approach description, outcome, context JSON (FR70, FR71)

### Story 10.2: Vector Similarity Search

As an agent,
I want to search for similar prior approaches before starting a task,
So that I can learn from the colony's experience.

**Acceptance Criteria:**

**Given** knowledge entries exist with embeddings
**When** an agent queries for approaches similar to its current task
**Then** the system returns the top-5 most similar entries ranked by cosine similarity
**And** results include both successful and failed approaches (FR73)

### Story 10.3: Knowledge Decay & Lifecycle

As the system,
I want knowledge entries to decay over time,
So that stale patterns don't override recent learnings.

**Acceptance Criteria:**

**Given** knowledge entries with varying ages
**When** similarity search runs
**Then** results are weighted by recency (newer entries rank higher at equal similarity)
**And** entries older than configurable threshold (default 90 days) are excluded (FR74)

### Story 10.4: Knowledge CLI

As a user,
I want to view and manage knowledge entries via CLI,
So that I can audit what my hive has learned.

**Acceptance Criteria:**

**Given** knowledge entries exist
**When** the user runs `hive knowledge list --type code-review`
**Then** entries are displayed with: task type, approach summary, outcome, age
**And** `hive knowledge search "how to handle timeouts"` returns semantically similar entries (FR75)

---

## Epic 11: Agent Collaboration & Webhooks

Agents can collaborate through dialog threads, and the system sends notifications via webhooks.

### Story 11.1: Dialog Thread Management

As an agent,
I want to start a conversation with another agent,
So that we can collaboratively solve complex problems.

**Acceptance Criteria:**

**Given** two registered agents
**When** agent A initiates a dialog with agent B on a topic
**Then** a dialog thread is created with: initiator, participant, topic, status
**And** messages are stored in dialog_messages table (FR76, FR77)

### Story 11.2: Dialog Thread API & CLI

As a user,
I want to view dialog threads and their messages,
So that I can understand how agents collaborate.

**Acceptance Criteria:**

**Given** dialog threads exist
**When** the user runs `hive dialogs list`
**Then** active and recent threads are displayed
**And** `hive dialogs show <thread-id>` displays the full conversation (FR78, FR79)

### Story 11.3: Webhook Configuration

As a user,
I want to configure webhooks for event notifications,
So that I'm notified when important things happen in my hive.

**Acceptance Criteria:**

**Given** the user runs `hive webhook add --name slack-alerts --url https://hooks.slack.com/... --type slack --events task.failed,agent.isolated`
**When** a matching event occurs
**Then** the system sends a formatted notification to the webhook URL
**And** webhook delivery retries 3 times with exponential backoff on failure (FR80, FR81, FR83)

### Story 11.4: GitHub & Generic Webhook Formats

As a user,
I want webhook notifications in GitHub and generic formats,
So that I can integrate with my existing tools.

**Acceptance Criteria:**

**Given** a webhook configured with `--type github` or `--type generic`
**When** a matching event occurs
**Then** the notification is formatted for the specified platform
**And** GitHub format includes PR/issue context when relevant (FR82)

---

## Epic 12: v0.2 Integration & Polish

End-to-end integration of all v0.2 features with comprehensive testing.

### Story 12.1: v0.2 Migration & Schema Update

As a developer,
I want the v0.2 database schema migration,
So that all new tables are created on upgrade.

**Acceptance Criteria:**

**Given** an existing v0.1 database
**When** the v0.2 binary starts
**Then** migration 002 runs automatically creating: knowledge, trust_history, dialog_threads, dialog_messages, webhooks tables
**And** existing data is preserved
**And** migration is idempotent

### Story 12.2: Integration Test — Full v0.2 Flow

As a developer,
I want an end-to-end test exercising all v0.2 features,
So that I'm confident the system works as a whole.

**Acceptance Criteria:**

**Given** the full v0.2 system is running
**When** the integration test runs
**Then** it exercises: register agent → run workflow → trust promotes → knowledge stored → dashboard shows updates → webhook fires
**And** all assertions pass

### Story 12.3: v0.2 Documentation Update

As a user,
I want updated documentation for all v0.2 features,
So that I can use the new capabilities.

**Acceptance Criteria:**

**Given** all v0.2 features are implemented
**When** the documentation is updated
**Then** quickstart.md covers dashboard access, trust configuration, and knowledge CLI
**And** new docs: dashboard-guide.md, trust-configuration.md, knowledge-layer.md, webhooks.md

---

# v0.3 Epics — Ecosystem & Scale

## Epic 13: Framework Adapters

Users can orchestrate agents from CrewAI, LangChain, AutoGen, and OpenAI Assistants alongside existing agents.

### Story 13.1: CrewAI Adapter

As a user,
I want to register CrewAI agents with Hive,
So that I can orchestrate my CrewAI crews alongside other frameworks.

**Acceptance Criteria:**

**Given** a CrewAI project
**When** the user runs `hive add-agent --type crewai --path ./my-crew`
**Then** the adapter detects CrewAI crew configuration and maps crew capabilities to Hive protocol
**And** tasks are invoked by running the CrewAI crew via subprocess (FR84, FR88)

### Story 13.2: LangChain/LangGraph Adapter

As a user,
I want to register LangChain and LangGraph agents with Hive,
So that I can orchestrate my LangChain chains and graphs.

**Acceptance Criteria:**

**Given** a LangChain agent exposed via HTTP (LangServe)
**When** the user runs `hive add-agent --type langchain --url http://localhost:8000`
**Then** the adapter connects to the LangServe endpoint and maps available chains to Hive capabilities
**And** tasks invoke specific chains via the LangServe API (FR85, FR88)

### Story 13.3: AutoGen Adapter

As a user,
I want to register Microsoft AutoGen agents with Hive,
So that I can include AutoGen conversations in multi-framework workflows.

**Acceptance Criteria:**

**Given** an AutoGen agent exposed via HTTP
**When** the user runs `hive add-agent --type autogen --url http://localhost:8001`
**Then** the adapter connects and maps AutoGen agent capabilities
**And** tasks invoke AutoGen conversations via HTTP (FR86, FR88)

### Story 13.4: OpenAI Assistants Adapter

As a user,
I want to register OpenAI Assistants with Hive,
So that I can orchestrate GPT-based assistants alongside local agents.

**Acceptance Criteria:**

**Given** an OpenAI API key and Assistant ID
**When** the user runs `hive add-agent --type openai --assistant-id asst_xxx --api-key $OPENAI_API_KEY`
**Then** the adapter creates threads and runs via the OpenAI Assistants API
**And** tasks create a run, poll for completion, and return the result (FR87, FR88)

---

## Epic 14: HiveHub Template Registry

Users can publish, discover, and install pre-built hive configurations from a community registry.

### Story 14.1: Template Packaging & Publishing

As a user,
I want to publish my hive configuration as a reusable template,
So that other users can benefit from my orchestration patterns.

**Acceptance Criteria:**

**Given** a working hive project with `hive.yaml` and agent configs
**When** the user runs `hive publish --name my-template --description "..."`
**Then** the system packages: hive.yaml, agents/ directory, README.md, metadata.json
**And** pushes the package to the HiveHub Git registry (FR89, FR92, FR93)

### Story 14.2: Template Search

As a user,
I want to search for templates by keyword or category,
So that I can find relevant orchestration patterns quickly.

**Acceptance Criteria:**

**Given** the HiveHub registry contains templates
**When** the user runs `hive search code-review`
**Then** matching templates are displayed with: name, description, author, download count
**And** results are fetched from the HiveHub Git registry index (FR90)

### Story 14.3: Template Installation

As a user,
I want to install a HiveHub template into my project,
So that I can start with a proven orchestration pattern.

**Acceptance Criteria:**

**Given** a template exists in HiveHub
**When** the user runs `hive install content-pipeline`
**Then** the template files are downloaded and merged into the current project
**And** existing files are not overwritten without confirmation (FR91)

---

## Epic 15: NATS Distributed Event Bus

The system supports NATS as a pluggable event bus backend for multi-node deployments.

### Story 15.1: EventBus Interface Extraction

As a developer,
I want the event bus behind a pluggable interface,
So that I can swap between embedded and NATS backends.

**Acceptance Criteria:**

**Given** the existing in-process event bus
**When** the EventBus interface is extracted
**Then** both embedded and NATS backends implement the same interface
**And** all existing tests pass with no changes (FR94, FR97)

### Story 15.2: NATS Backend Implementation

As a user,
I want to configure NATS as my event bus backend,
So that multiple Hive nodes can share the same event stream.

**Acceptance Criteria:**

**Given** a NATS server is running
**When** the user sets `event_bus: nats` and `nats_url: nats://localhost:4222` in `hive.yaml`
**Then** events are published to and subscribed from NATS subjects
**And** event ordering is maintained per-subject (FR95, FR96)

### Story 15.3: NATS Connection Management

As the system,
I want robust NATS connection handling,
So that the event bus recovers from network issues.

**Acceptance Criteria:**

**Given** a NATS connection is established
**When** the connection drops
**Then** the system automatically reconnects with exponential backoff
**And** queued events are delivered after reconnection
**And** connection state is reported in `hive status`

---

## Epic 16: Enhanced Knowledge & Cost Management

The knowledge layer gets semantic search, and the system tracks costs with budget alerts.

### Story 16.1: Vector Embedding for Knowledge Search

As a user,
I want knowledge search to understand meaning, not just keywords,
So that I find relevant approaches even when using different terminology.

**Acceptance Criteria:**

**Given** knowledge entries exist
**When** the user searches "how to handle API timeouts"
**Then** entries about "retry on connection failure" or "backoff strategy" are returned
**And** embeddings are generated locally with a lightweight model (FR98, FR99)

### Story 16.2: External Embedding API Support

As a user,
I want to optionally use OpenAI or Anthropic embeddings for higher quality search,
So that knowledge retrieval is more accurate for complex domains.

**Acceptance Criteria:**

**Given** the user configures `embedding_api: openai` in `hive.yaml`
**When** knowledge entries are created or searched
**Then** embeddings are generated via the configured API
**And** the system falls back to local embeddings if the API is unavailable (FR100)

### Story 16.3: Cost Tracker

As a user,
I want to see how much each agent and workflow costs,
So that I can manage my AI spend effectively.

**Acceptance Criteria:**

**Given** agents declare `cost_per_run` in their capabilities
**When** tasks complete
**Then** the system accumulates cost per agent and per workflow in the costs table
**And** `hive status --costs` shows a cost breakdown (FR101, FR102)

### Story 16.4: Budget Alerts

As a user,
I want to set budget alerts so I'm notified when spending exceeds thresholds,
So that I avoid runaway AI costs.

**Acceptance Criteria:**

**Given** a budget alert configured via `hive budget set --agent code-reviewer --daily-limit 10`
**When** the agent's daily cost exceeds $10
**Then** a `cost.alert` event is emitted and webhook notifications fire
**And** the alert is shown in `hive status` (FR103, FR104)

---

## Epic 17: v0.3 Integration & Polish

### Story 17.1: v0.3 Migration

As a developer,
I want the v0.3 database migration adding costs and budget tables,
So that cost tracking persists.

**Acceptance Criteria:**

**Given** an existing v0.2 database
**When** the v0.3 binary starts
**Then** migration 003 runs creating: costs, budget_alerts tables
**And** existing data preserved, migration idempotent

### Story 17.2: v0.3 Documentation

As a user,
I want documentation for all v0.3 features,
So that I can use adapters, HiveHub, NATS, and cost management.

**Acceptance Criteria:**

**Given** all v0.3 features implemented
**When** docs are updated
**Then** new docs: adapters-guide.md (CrewAI, LangChain, AutoGen, OpenAI), hivehub-guide.md, nats-setup.md, cost-management.md

---

# v1.0 Epics — Full Platform

## Epic 18: Market-Based Task Allocation

Agents bid on tasks through an internal auction, optimizing allocation via price signals.

### Story 18.1: Auction Engine

As a user,
I want tasks allocated through an auction where agents bid,
So that the best agent for each task is selected automatically.

**Acceptance Criteria:**

**Given** a task is created with allocation_strategy: "market"
**When** capable agents are notified
**Then** each agent submits a bid (price + estimated duration)
**And** the system selects the winner based on configured strategy (lowest cost, fastest, best reputation)
**And** a `task.auction.won` event is emitted with bid details (FR105, FR106)

### Story 18.2: Allocation Strategies

As a user,
I want to configure different allocation strategies per workflow,
So that I can optimize for cost, speed, or quality depending on the use case.

**Acceptance Criteria:**

**Given** a workflow with `allocation: market` or `allocation: round-robin` or `allocation: capability-match`
**When** tasks are created
**Then** the system uses the configured strategy for agent selection
**And** `hive validate` checks that the strategy is valid (FR107)

### Story 18.3: Token Economy

As the system,
I want agents to accumulate internal tokens based on task completions,
So that the market has price signals for optimal allocation.

**Acceptance Criteria:**

**Given** an agent completes a task
**When** the result is accepted
**Then** the agent earns tokens proportional to task value
**And** token balance is tracked in the agents table
**And** bid history and win rates are queryable via `hive agent stats <name>` (FR108, FR109)

---

## Epic 19: Cross-Hive Federation

Independent Hive deployments can securely share agent capabilities across organizational boundaries.

### Story 19.1: Federation Protocol

As a user,
I want to connect my Hive to another organization's Hive,
So that we can share agent capabilities for collaboration.

**Acceptance Criteria:**

**Given** two Hive deployments with mTLS certificates
**When** the user runs `hive federation connect --url hive.partner.com --cert ./partner.pem`
**Then** a secure federation link is established
**And** capability metadata is exchanged (not task data)
**And** connection health is monitored (FR110, FR113, FR114)

### Story 19.2: Capability Discovery & Sharing

As a user,
I want to configure which capabilities my hive shares with federated partners,
So that I control what's exposed.

**Acceptance Criteria:**

**Given** a federation link is established
**When** the user configures `federation.share: [code-review, summarize]` in `hive.yaml`
**Then** only the listed capabilities are visible to the partner hive
**And** the partner can route tasks requiring those capabilities to our agents (FR111, FR115)

### Story 19.3: Cross-Hive Task Routing

As the system,
I want to route tasks to federated agents when local agents can't handle them,
So that the network effect increases available capabilities.

**Acceptance Criteria:**

**Given** a task requires capability "data-analysis" and no local agent has it
**When** a federated hive has an agent with that capability
**Then** the task is proxied to the federated hive via the federation protocol
**And** results are returned to the originating hive
**And** a `task.federated` event records the cross-hive routing (FR112)

---

## Epic 20: Self-Optimizing Orchestration

The system analyzes its own execution patterns and suggests or applies optimizations.

### Story 20.1: Pattern Analyzer

As the system,
I want to analyze historical execution data for optimization patterns,
So that I can identify bottlenecks and inefficiencies.

**Acceptance Criteria:**

**Given** the system has executed multiple workflows
**When** the analyzer runs (triggered by `hive optimize` or on schedule)
**Then** it identifies: slow agents (p95 duration), underutilized agents, sequential tasks that could parallelize, frequently failing task types
**And** findings are stored for recommendation generation (FR116)

### Story 20.2: Optimization Recommendations

As a user,
I want to see actionable optimization recommendations,
So that I can improve my hive's performance.

**Acceptance Criteria:**

**Given** the analyzer has identified patterns
**When** the user runs `hive optimize`
**Then** recommendations are displayed: "Agent X is 3x slower than Agent Y for code-review tasks", "Tasks A and B in workflow W could run in parallel", "Agent Z has 40% idle rate — consider reducing heartbeat interval"
**And** each recommendation includes estimated impact (FR118, FR119)

### Story 20.3: Auto-Tuning

As the system,
I want to automatically apply approved optimizations,
So that performance improves without manual intervention.

**Acceptance Criteria:**

**Given** the user approves an optimization via `hive optimize --apply`
**When** the next workflow run executes
**Then** the approved optimizations are applied (e.g., prefer faster agent, parallelize tasks)
**And** results are compared to pre-optimization baseline
**And** a `system.optimization.applied` event is logged (FR117, FR120)

---

## Epic 21: Enterprise Features

SSO, RBAC, audit logging, and multi-tenant support for enterprise deployments.

### Story 21.1: OIDC SSO Authentication

As an enterprise admin,
I want users to authenticate via SSO (OpenID Connect),
So that access is managed through our identity provider.

**Acceptance Criteria:**

**Given** OIDC is configured in `hive.yaml` (issuer URL, client ID, client secret)
**When** a user accesses the dashboard or API
**Then** they are redirected to the OIDC provider for authentication
**And** JWT tokens are validated on each request (FR121)

### Story 21.2: RBAC Roles & Permissions

As an admin,
I want to define roles with specific permissions,
So that users only access what they're authorized to.

**Acceptance Criteria:**

**Given** roles defined: admin (full access), operator (manage agents/workflows), viewer (read-only)
**When** a user with "viewer" role tries to register an agent
**Then** the request is rejected with 403 Forbidden
**And** roles are configurable via `hive.yaml` or API (FR122)

### Story 21.3: Audit Log Export

As a compliance officer,
I want audit logs exported in standard formats,
So that I can meet regulatory requirements.

**Acceptance Criteria:**

**Given** the system has been running with events
**When** the user runs `hive audit export --format json --since 30d --output audit.json`
**Then** all system events, auth events, and agent actions are exported
**And** CSV format is also supported (FR123)

### Story 21.4: Multi-Tenant Support

As a platform operator,
I want to run multiple tenants on a single Hive deployment,
So that I can offer Hive as a service.

**Acceptance Criteria:**

**Given** multi-tenant mode is enabled in `hive.yaml`
**When** tenants are created via `hive tenant create <name>`
**Then** each tenant has isolated: agents, workflows, tasks, events, knowledge
**And** tenant data never leaks across boundaries (FR125)

---

## Epic 22: Multi-Node & PostgreSQL

Horizontal scaling with PostgreSQL storage and NATS cluster for production deployments.

### Story 22.1: PostgreSQL Storage Backend

As a user,
I want to use PostgreSQL instead of SQLite for production deployments,
So that my hive can handle higher concurrency and larger datasets.

**Acceptance Criteria:**

**Given** `storage: postgres` and `postgres_url: postgres://...` in `hive.yaml`
**When** the system starts
**Then** it connects to PostgreSQL and runs migrations
**And** all features work identically to SQLite mode (FR129, FR130)

### Story 22.2: Multi-Node Clustering

As a user,
I want to run multiple Hive nodes for high availability,
So that my hive survives node failures.

**Acceptance Criteria:**

**Given** multiple Hive nodes connected via NATS cluster
**When** an agent registers on node A
**Then** the registration is replicated to node B via NATS events
**And** tasks can be routed to agents on any node (FR126, FR127, FR128)

### Story 22.3: Node-Aware Routing

As the system,
I want task routing to prefer local agents over remote ones,
So that latency is minimized.

**Acceptance Criteria:**

**Given** agents exist on multiple nodes
**When** a task is routed
**Then** the router prefers agents on the same node
**And** falls back to remote agents if no local agent has the capability
**And** routing preference is configurable: `routing: local-first` or `routing: best-fit` (FR128)

---

## Epic 23: v1.0 Integration, Polish & Launch

### Story 23.1: v1.0 Migration

As a developer,
I want the v1.0 database migration,
So that all new tables are created on upgrade.

**Acceptance Criteria:**

**Given** an existing v0.3 database
**When** the v1.0 binary starts
**Then** migration 004 runs creating: bids, federation_links, optimizations, tenants, roles tables
**And** existing data preserved, migration idempotent

### Story 23.2: v1.0 End-to-End Test

As a developer,
I want a comprehensive E2E test covering all v1.0 features,
So that I'm confident the full platform works.

**Acceptance Criteria:**

**Given** the full v1.0 system
**When** the E2E test runs
**Then** it exercises: market allocation, federation (mock), optimization analysis, RBAC enforcement, multi-tenant isolation
**And** all assertions pass

### Story 23.3: v1.0 Documentation & Launch

As a user,
I want complete documentation for the v1.0 platform,
So that I can deploy and operate Hive in production.

**Acceptance Criteria:**

**Given** all v1.0 features implemented
**When** docs are finalized
**Then** docs cover: market allocation, federation setup, optimization guide, enterprise deployment (SSO, RBAC, audit), multi-node setup (PostgreSQL + NATS cluster), API reference
**And** README updated with v1.0 feature overview
