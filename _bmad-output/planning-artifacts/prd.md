---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-02b-vision', 'step-02c-executive-summary', 'step-03-success', 'step-04-journeys', 'step-05-domain', 'step-06-innovation', 'step-07-project-type', 'step-08-scoping', 'step-09-functional', 'step-10-nonfunctional', 'step-11-polish', 'step-12-complete']
inputDocuments:
  - '_bmad-output/planning-artifacts/product-brief-hive.md'
  - '_bmad-output/planning-artifacts/product-brief-hive-distillate.md'
  - '_bmad-output/brainstorming/brainstorming-session-2026-04-16-1630.md'
workflowType: 'prd'
documentCounts:
  briefs: 2
  research: 0
  brainstorming: 1
  projectDocs: 0
classification:
  projectType: 'developer_tool'
  domain: 'general_ai_orchestration'
  complexity: 'medium'
  projectContext: 'greenfield'
---

# Product Requirements Document — Hive

**Author:** Julienvadic
**Date:** 2026-04-16

## Executive Summary

Hive is an open-source AI agent orchestration platform that coordinates agents from any framework through a standardized open protocol. It solves the agent interoperability problem created by the 2024-2026 framework explosion: teams build agents with Claude Code, CrewAI, LangChain, AutoGen, MCP servers, and custom scripts — but no standard exists to make them work together.

Hive defines the **Agent Adapter Protocol**, an open interoperability standard (analogous to OCI for containers). Any agent that implements this minimal interface — under 20 lines for basic HTTP agents — can participate in orchestrated multi-agent workflows. Pre-built adapters ship for Claude Code, BMAD agents, MCP servers, and generic HTTP/CLI.

Beyond interoperability, Hive delivers **fully autonomous agent operations**. Each agent carries its own behavioral plan — a state machine defining what it does at each wake-up cycle. Agents don't wait for task assignments: they observe the current state of the world, decide what needs doing, and execute. The orchestrator provides the heartbeat schedule and event triggers, but agents are self-directed. Combined with event-driven execution, capability-based routing, graduated trust, and a shared knowledge layer, this creates a system that runs business operations autonomously with minimal human intervention.

### What Makes This Special

- **Universal agent import**: No framework lock-in. Orchestrate agents from any source through one protocol. Framework authors are allies, not competitors
- **Open standard positioning**: The Agent Adapter Protocol is designed as a community-owned interop standard, not a proprietary integration layer
- **Event-driven architecture**: Agents react to events in real-time — no polling waste, sub-second response times
- **Graduated autonomy**: Trust earned through track record, not binary approve/deny gates
- **Collective learning**: Shared knowledge layer that makes the entire system smarter with every completed task
- **True autonomy**: Each agent has a behavioral plan (heartbeat state machine) — it knows what to do at every wake-up without human direction. Idle when there's nothing to do, active when there is

## Project Classification

- **Type:** Developer tool (CLI + open protocol + orchestration runtime)
- **Domain:** AI agent orchestration
- **Complexity:** Medium (technically ambitious, no regulatory constraints)
- **Context:** Greenfield (new project, no existing codebase)

## Success Criteria

### User Success

- New user completes first multi-agent workflow within 5 minutes of installation (`npx create-hive` → orchestrated result)
- Developer writes a custom adapter for their framework in under 30 minutes using adapter guide
- User successfully orchestrates agents from 2+ different frameworks in a single workflow without modifying agent source code
- User reports the system "just works" for common orchestration patterns without reading documentation beyond quickstart

### Business Success

- 100+ unique deployers running real workloads within 6 months (measured by opt-in telemetry)
- 10+ community-contributed adapters within 6 months
- 3+ blog posts or conference talks from external users within 9 months
- Active Discord community with 500+ members within 6 months

### Technical Success

- Event-to-agent-invocation latency under 200ms p95
- Zero-downtime agent replacement via hot-swap protocol
- Single binary install with zero external dependencies passes on macOS, Linux, and Windows
- All adapter protocol tests pass for every supported framework adapter
- CI pipeline achieves 80%+ code coverage on core orchestration engine

### Measurable Outcomes

| Metric | Target | Measurement |
|---|---|---|
| Time to first orchestration | < 5 minutes | User testing sessions |
| Adapter authoring time | < 30 minutes | Contributor reports |
| Event latency (p95) | < 200ms | Built-in metrics endpoint |
| Agent hot-swap success rate | 99.9% | Integration test suite |
| Community adapters (6 months) | 10+ | GitHub repository count |

## Product Scope

### MVP — Minimum Viable Product

Proves one thing: **agents from different frameworks can be orchestrated together through a simple open protocol.**

- Agent Adapter Protocol v1 specification (open standard document)
- Core event bus (lightweight, embedded, zero external dependencies)
- Adapters: Claude Code, MCP servers, generic HTTP/CLI
- Capability-based task routing (agents declare capabilities, tasks declare requirements, system matches)
- Task state externalization (checkpoint/resume across agent replacements)
- CLI: `hive init`, `hive add-agent`, `hive run`, `hive status`, `hive logs`
- 3 example hive templates: code review, content pipeline, research
- Documentation site with "Hello Hive" quickstart tutorial
- Single binary, single node, embedded SQLite, zero external dependencies

### v0.2 Scope — Trust & Visibility

**Goal:** Add visual dashboard, earned autonomy, and institutional memory.

- Dashboard UI: real-time agent health, task flow, event timeline, cost tracking (Svelte 5, embedded in Go binary)
- Graduated autonomy engine: 4-level trust spectrum (Supervised → Guided → Autonomous → Trusted) with configurable thresholds per agent and per task type
- Shared knowledge layer: append-only pattern store with vector similarity search, new agents inherit colony knowledge on boot
- Additional adapters: CrewAI, LangChain/LangGraph, AutoGen, OpenAI Assistants
- Agent-to-agent dialog threads: multi-turn conversations between agents for collaborative problem-solving
- Webhook integrations: Slack, GitHub, email notifications for key events
- WebSocket support for real-time dashboard updates

### v0.3 Scope — Ecosystem & Scale

**Goal:** Expand the adapter ecosystem, add template marketplace, and prepare for multi-node.

- Additional adapters: CrewAI, LangChain/LangGraph, AutoGen, OpenAI Assistants
- HiveHub: template registry where users publish and discover pre-built hive configurations
- NATS event bus: pluggable replacement for in-process bus, enabling multi-node deployment
- Vector embeddings for knowledge search (replace keyword matching with semantic search)
- Agent cost tracking: per-agent and per-workflow cost aggregation with budget alerts
- `hive publish` and `hive search` CLI commands for HiveHub

### v1.0 Scope — Full Platform

**Goal:** Complete the platform with market allocation, cross-hive networking, self-optimization, and multi-node production readiness.

- Market-based task allocation: agents bid on tasks, internal token economy drives optimal allocation
- Cross-hive networking: secure federation between independent Hive deployments for inter-org collaboration
- Self-optimizing orchestration: AI analyzes execution patterns and auto-tunes routing, scheduling, agent selection
- Multi-node deployment: horizontal scaling with NATS cluster, shared SQLite via Litestream or PostgreSQL option
- Enterprise features: SSO (OIDC), RBAC, audit log export, compliance dashboards
- Hive Cloud: managed SaaS deployment (API-first, multi-tenant)

## User Journeys

### Journey 1: Alex, Senior Engineer — First Orchestration

**Persona:** Alex, senior engineer at a startup. Has 3 Claude Code agents for code review, 2 Python scripts calling GPT for data analysis, and a LangChain pipeline for document processing. Currently coordinates them manually via cron jobs and shell scripts.

**Opening Scene:** Alex discovers Hive on Hacker News. Skeptical but intrigued by "orchestrate any agent" claim. Runs `npx create-hive` in a fresh directory. CLI scaffolds a project in 15 seconds.

**Rising Action:** Alex runs `hive add-agent --type claude-code --path ./review-agent` and the CLI auto-detects the Claude Code agent, generates an adapter config. Repeats for the HTTP-based Python script: `hive add-agent --type http --url localhost:8080`. Creates a simple workflow YAML: "When a PR is created, run code-review agent, then data-analysis agent on the results."

**Climax:** Alex runs `hive run` and watches in the terminal as events flow: PR webhook received → code-review agent invoked → result passed to data-analysis agent → final report generated. Two agents from different frameworks, working together, in under 5 minutes.

**Resolution:** Alex replaces 200 lines of bash glue code with a 15-line hive workflow config. Adds the document processing pipeline as a third agent the next day. Tells the team: "You need to see this."

### Journey 2: Sarah, Platform Engineering Lead — Team Rollout

**Persona:** Sarah, platform engineering lead at a mid-size company (200 engineers). Tasked with standardizing AI agent usage across 8 teams, each using different frameworks.

**Opening Scene:** Sarah evaluates Hive after her team spends 3 weeks debugging agent coordination failures across teams. She needs governance, visibility, and a standard way for teams to register and orchestrate agents.

**Rising Action:** Sarah deploys Hive on the company's internal infrastructure. Creates adapter templates for each team's framework. Establishes governance policies: new agents start in Supervised mode, code-affecting agents require review-within-1h approval. Sets up cost tracking per team with budget alerts.

**Climax:** During a production incident, the SRE hive auto-detects the anomaly (monitoring agent), pages the on-call (notification agent), generates an incident report (analysis agent), and proposes a rollback (deployment agent) — all within 90 seconds, across 4 different agent frameworks.

**Resolution:** Sarah presents quarterly results: 60% reduction in agent coordination overhead, zero cross-framework integration incidents, and every team onboarded in under a day. Engineering leadership approves expanding to all teams.

### Journey 3: Marcus, Framework Maintainer — Adapter Contribution

**Persona:** Marcus maintains a popular open-source agent framework (2K stars). Users keep asking "Can I use this with Hive?"

**Opening Scene:** Marcus reads the Adapter Protocol spec on the Hive docs site. It's 2 pages. The interface has 5 methods: `declare()`, `invoke()`, `health()`, `checkpoint()`, `resume()`.

**Rising Action:** Marcus clones the adapter template (`hive adapter-template my-framework`), implements the 5 methods mapping to his framework's API. Total: 45 lines of TypeScript. Runs the adapter test suite — all pass on first try.

**Climax:** Marcus submits a PR to the hive-adapters community repo. It's reviewed and merged within 24 hours. His framework now appears in `hive add-agent --type my-framework`.

**Resolution:** Marcus's framework gains 300 new users in the first month from Hive cross-pollination. He adds a "Works with Hive" badge to his README. Starts building more sophisticated adapter features (streaming support, checkpoint optimization).

### Journey 4: Admin — Monitoring & Troubleshooting

**Persona:** DevOps engineer responsible for the Hive deployment.

**Opening Scene:** Alert fires: task completion rate dropped 15% in the last hour.

**Rising Action:** Opens `hive status` — sees one agent reporting degraded health (API rate limit hit). Checks `hive logs --agent data-processor` — confirms rate limit errors started 47 minutes ago.

**Climax:** Runs `hive agent swap data-processor --to backup-processor` — zero-downtime replacement. Tasks that were queued for the degraded agent automatically route to the backup. No in-progress work is lost (checkpoint/resume).

**Resolution:** Sets up a permanent health-based routing rule: "If agent health degrades below 80%, auto-failover to backup." Adds a rate-limit monitoring alert to prevent recurrence.

### Journey Requirements Summary

| Journey | Key Capabilities Revealed |
|---|---|
| Alex (first use) | CLI scaffolding, auto-detect adapters, workflow YAML, event streaming, terminal output |
| Sarah (team rollout) | Multi-team deployment, governance policies, cost tracking, budget alerts, incident orchestration |
| Marcus (adapter author) | Adapter template generator, test suite, community contribution flow, adapter registry |
| Admin (troubleshooting) | Health monitoring, log querying, agent swap, auto-failover rules, alerting |

## Innovation & Novel Patterns

### Detected Innovation Areas

**1. Open Agent Interoperability Standard**
No existing platform defines an open, framework-agnostic agent protocol. CrewAI, AutoGen, LangGraph, and Paperclip each require agents built in their own paradigm. Hive's Agent Adapter Protocol is the first attempt at a universal agent interface standard — positioning analogous to OCI's role in the container ecosystem.

**2. Event-Driven Agent Orchestration**
Current agent orchestration is predominantly sequential (CrewAI), state-machine-based (LangGraph), or heartbeat-polled (Paperclip). Event-driven reactive orchestration applied to AI agents is a novel combination of proven distributed systems patterns with the emerging agent coordination problem.

**3. Graduated Autonomy Spectrum**
Binary human-in-the-loop (approve/deny) is standard across all platforms. A progressive trust model where agents earn increasing autonomy through demonstrated competence is unexplored in production agent platforms.

### Validation Approach

- **Protocol validation**: Ship 3 adapters (Claude Code, MCP, HTTP) and measure adapter authoring time for external contributors. Target: < 30 minutes for experienced developers
- **Event-driven validation**: Benchmark event-to-invocation latency vs. heartbeat-based systems. Target: < 200ms vs. minutes
- **Graduated autonomy validation**: Pilot with 2-3 teams; measure human intervention rate decrease over 30-day period

### Risk Mitigation

- **Protocol too complex**: Continuously simplify based on adapter author feedback. If 20 lines isn't achievable, redesign
- **Event bus bottleneck**: Start with embedded bus; architecture allows swap to NATS/Redis if scale demands
- **Graduated autonomy gaming**: Trust metrics based on multiple signals (success rate, error rate, human override rate), not single metric

## Agent Autonomy Model

### Core Concept: Self-Directed Agents

Each agent in Hive operates autonomously. The orchestrator doesn't tell agents what to do — it provides the schedule, the context, and the shared state. The agent decides.

**Agent Behavioral Plan (HEARTBEAT.md equivalent):**
Every registered agent has a behavioral plan — a state machine that defines:
- **What to check** on each wake-up (backlog, events, shared state, external triggers)
- **What to do** based on current state (execute task, delegate, report, idle)
- **When to escalate** (confidence too low, error threshold hit, scope exceeded)
- **When to idle** (nothing to do = success, not busywork generation)

**Wake-Up Cycle:**
```
Wake → Observe (check state/backlog/events)
     → Orient (assess priorities, match capabilities)
     → Decide (select action or idle)
     → Act (execute, delegate, or report)
     → Record (log decision + outcome to shared knowledge)
     → Sleep (until next heartbeat or event trigger)
```

This is an OODA loop applied to each agent's lifecycle.

### Autonomy Levels

| Level | Behavior | Agent Self-Direction |
|---|---|---|
| **Scripted** | Agent follows exact task list | Low — executes assigned tasks only |
| **Reactive** | Agent responds to events | Medium — chooses how to respond |
| **Proactive** | Agent scans for work and self-assigns | High — discovers and executes work |
| **Strategic** | Agent plans multi-step approaches | Full — sets own goals within constraints |

MVP supports Scripted and Reactive. Growth adds Proactive. Vision adds Strategic.

### "Idle is Success" Principle

Agents must not generate busywork when there is nothing to do. An idle agent with an empty backlog is a healthy agent. The system explicitly monitors and rewards appropriate idling — agents that invent unnecessary work are flagged.

### Agent Context Files

Each agent carries context files (inspired by Paperclip's SOUL.md / HEARTBEAT.md):
- **AGENT.yaml**: Identity, capabilities, constraints, personality, anti-patterns
- **PLAN.yaml**: Behavioral state machine (what to do on each wake-up)
- **KNOWLEDGE.md**: Agent-specific learned patterns and institutional context

These files are version-controlled and human-readable — operators can inspect and modify agent behavior by editing YAML.

## Developer Tool Specific Requirements

### CLI Architecture

**Command structure:**

| Command | Purpose |
|---|---|
| `hive init` | Scaffold new hive project with config and example |
| `hive add-agent` | Register an agent with auto-detection or manual config |
| `hive remove-agent` | Unregister an agent |
| `hive run` | Execute a workflow |
| `hive status` | Show hive health, agent states, active tasks |
| `hive logs` | Query agent and system logs |
| `hive agent swap` | Hot-swap an agent (zero-downtime replacement) |
| `hive adapter-template` | Generate adapter boilerplate for a new framework |
| `hive validate` | Validate workflow config and agent connectivity |

**Output formats:** Human-readable (default), JSON (`--json`), quiet (`--quiet` for scripting).

### Agent Adapter Protocol Specification

```
interface HiveAdapter {
  declare(): AgentCapabilities    // What can this agent do?
  invoke(task: Task): TaskResult  // Execute a task
  health(): HealthStatus          // Current agent health
  checkpoint(): State             // Serialize current state
  resume(state: State): void      // Restore from checkpoint
}
```

- Protocol versioned from v1 with backwards compatibility commitment
- Transport: HTTP/JSON (default), WebSocket (streaming), stdio (CLI agents)
- Authentication: API key or mTLS for production deployments

### Configuration

- Workflow definition: YAML files (`hive.yaml`)
- Agent registration: YAML or auto-detected from project structure
- Environment-based overrides for secrets and deployment-specific config
- Zero-config defaults for local development

### Documentation Requirements

- "Hello Hive" tutorial: 0 to first orchestrated workflow in 5 minutes
- Adapter authoring guide with step-by-step walkthrough
- API reference auto-generated from TypeScript types
- 3 example hive templates with full documentation
- Contributing guide for community adapter submissions

### Technology Constraints

- **Server runtime:** Go or Rust — single binary, ultra-lightweight, no runtime dependencies
- **Embedded storage:** SQLite (via embedded driver — no external DB process)
- **Embedded event bus:** In-process (no external message broker for single-node)
- **Frontend (dashboard):** Lightweight framework (Svelte, Preact, or similar) — no heavy frameworks. Served embedded in the single binary (embedded static assets)
- **Protocol:** HTTP/JSON for adapter communication, WebSocket for streaming/events

### Installation & Distribution

- Single binary via Homebrew, curl script, GitHub releases
- Zero external dependencies (everything embedded in the binary)
- Cross-platform: macOS (arm64, x64), Linux (x64, arm64), Windows (x64)
- Docker image for containerized deployments
- Optional: npm wrapper (`npx create-hive`) that downloads and runs the Go/Rust binary

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-solving MVP — prove that multi-framework agent interop works and is useful.

**Resource Requirements:** 1-2 developers, 3-4 months. Core competencies: TypeScript, distributed systems, CLI tooling.

### MVP Feature Set (Phase 1)

**Core User Journeys Supported:** Alex (first orchestration), Marcus (adapter contribution)

**Must-Have Capabilities:**
- Agent Adapter Protocol v1 spec + reference implementation
- Event bus (embedded, lightweight)
- 3 adapters (Claude Code, MCP, HTTP)
- Capability-based task routing
- Workflow YAML definition
- CLI (init, add-agent, run, status, logs)
- Checkpoint/resume for task state
- 3 example templates
- Documentation site + Hello Hive tutorial
- Single binary, zero dependencies

### Post-MVP Features

**Phase 2 — Trust & Visibility (v0.2-v0.3):**
- Dashboard UI
- Graduated autonomy engine
- Shared knowledge layer
- 7+ additional framework adapters
- Agent-to-agent dialog threads
- Cost tracking and budget alerts

**Phase 3 — Ecosystem (v0.4-v1.0):**
- HiveHub template marketplace
- Market-based task allocation
- Multi-node deployment
- Cross-hive networking
- Enterprise features (SSO, RBAC, audit)
- Hive Cloud (managed service)

### Risk Mitigation Strategy

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Protocol too complex for adapter authors | Medium | High | User testing with 5 adapter authors during design; simplify until < 30 min authoring time |
| Event bus becomes bottleneck at scale | Low | Medium | Architecture allows swap to NATS/Redis; start simple, scale when needed |
| Low community adoption | Medium | High | Adapter bounty program; Discord community; launch on HN/Reddit; ship useful templates |
| Competing standard emerges from major player | Low | High | Move fast on adoption; open governance; attract framework maintainers early |
| Scope creep into agent-building (competing with frameworks) | Medium | Medium | Strict scope discipline: Hive orchestrates, never builds agents |

## Functional Requirements

### Agent Management

- FR1: User can register an agent from any supported framework via CLI command
- FR2: User can auto-detect agent type and capabilities from project structure (supports detection of: skill.md for Claude Code, mcp.json for MCP, pyproject.toml with crewai for CrewAI, HTTP health endpoint for generic HTTP agents)
- FR3: User can manually configure agent capabilities via YAML
- FR4: User can list all registered agents with their health status and capabilities
- FR5: User can remove a registered agent from the hive
- FR6: User can hot-swap a running agent with a replacement without losing in-progress task state
- FR7: System validates agent connectivity and protocol compliance on registration

### Workflow Definition

- FR8: User can define multi-agent workflows in YAML format
- FR9: User can specify task dependencies as a directed acyclic graph (DAG)
- FR10: User can define event triggers that initiate workflows (webhook, schedule, manual)
- FR11: User can define conditional routing based on task results
- FR12: System validates workflow configuration before execution

### Task Orchestration

- FR13: System routes tasks to capable agents based on declared capabilities and availability
- FR14: System executes independent tasks in parallel when DAG allows
- FR15: System passes task results between agents as defined in workflow
- FR16: System emits events for all state changes (task created, started, completed, failed)
- FR17: System checkpoints in-progress task state at configurable intervals
- FR18: System resumes tasks from checkpoint after agent replacement or failure

### Event System

- FR19: System delivers events to subscribed agents within 200ms p95
- FR20: Agents can subscribe to specific event types
- FR21: Agents can emit custom events
- FR22: System maintains ordered event log for replay and debugging
- FR23: User can query event history via CLI

### Agent Adapter Protocol

- FR24: Adapter author can implement the protocol interface in under 20 lines of configuration for an agent that exposes /health, /declare, and /invoke HTTP endpoints
- FR25: Adapter author can generate boilerplate via CLI template command
- FR26: Adapter author can run protocol compliance test suite against their adapter
- FR27: System supports HTTP/JSON, WebSocket (streaming), and stdio transport
- FR28: Adapters declare agent capabilities in structured format (task types, cost estimates, constraints)

### Observability

- FR29: User can view real-time hive status (agent health, active tasks, event flow)
- FR30: User can query agent-specific logs with filtering
- FR31: User can view task execution timeline with durations and results
- FR32: System exposes metrics endpoint for external monitoring integration
- FR33: System logs all orchestration decisions with reasoning context

### Configuration & Templates

- FR34: User can scaffold a new hive project with example config via `hive init`
- FR35: User can select from pre-built hive templates during initialization
- FR36: System supports environment-based configuration overrides
- FR37: System stores all state in an embedded database requiring zero external dependencies

### Data Lifecycle & Migration

- FR38: User can export all hive data (agents, tasks, events, knowledge) in a standard format for backup
- FR39: User can import previously exported data into a new hive deployment
- FR40: System provides upgrade migration path between Hive versions automatically on startup
- FR41: User can request deletion of specific knowledge entries or agent data (data retention compliance)
- FR42: CLI output supports screen reader compatibility and respects NO_COLOR environment variable

### Agent Autonomy

- FR43: User can define an agent's behavioral plan as a YAML state machine (PLAN.yaml)
- FR44: System executes agent wake-up cycles on configurable schedules (heartbeat intervals)
- FR45: Agent observes shared state, event history, and its backlog at each wake-up to decide its action
- FR46: Agent can self-assign tasks from a shared backlog based on capability match
- FR47: Agent can choose to idle when no relevant work exists (idle is a valid action)
- FR48: System logs each agent's wake-up decision (what it observed, what it decided, why)
- FR49: User can define agent identity and constraints via AGENT.yaml (capabilities, anti-patterns, personality)
- FR50: User can inspect and modify agent behavior by editing YAML context files
- FR51: System flags agents that create tasks without upstream trigger or backlog demand as busywork generators (threshold: 3+ unprompted tasks per wake-up cycle)

### Error Handling & Resilience

- FR52: System implements circuit breaker pattern for failing agents
- FR53: System auto-isolates agents that exceed configurable failure thresholds
- FR54: System reroutes queued tasks from isolated agents to healthy alternatives
- FR55: System provides error messages that include: what failed, which agent/task was involved, and a specific remediation suggestion
- FR56: User can configure retry policies per agent or per task type

### Dashboard (v0.2)

- FR57: User can view real-time agent health, status, and capabilities in a web dashboard
- FR58: User can view task flow visualization with status, duration, and agent assignment
- FR59: User can view event timeline with filtering by type, source, and time range
- FR60: User can view cost tracking per agent, per workflow, and per time period
- FR61: Dashboard updates in real-time via WebSocket without page refresh
- FR62: Dashboard is served embedded in the server binary (no separate frontend deployment)

### Graduated Autonomy (v0.2)

- FR63: System tracks agent trust level (Supervised, Guided, Autonomous, Trusted) based on performance history
- FR64: User can configure trust thresholds per agent (e.g., "promote to Guided after 50 successful tasks with <5% error rate")
- FR65: User can configure trust overrides per task type (e.g., "always Supervised for financial transactions")
- FR66: System automatically promotes agents when they meet configured thresholds
- FR67: User can manually promote or demote an agent's trust level
- FR68: System enforces approval gates based on trust level (Supervised requires approval, Trusted acts freely)
- FR69: System logs all trust level changes with reasoning (promotion criteria met, manual override, demotion trigger)

### Shared Knowledge Layer (v0.2)

- FR70: System stores successful task approaches as reusable knowledge entries (task type, approach, outcome, context)
- FR71: System stores failed approaches as negative knowledge to prevent repetition
- FR72: New agents query the knowledge layer before starting a task type for the first time
- FR73: Knowledge entries support vector similarity search for finding relevant prior approaches
- FR74: Knowledge entries decay over time (weighted by recency and success rate)
- FR75: User can view, search, and manage knowledge entries via CLI (`hive knowledge list`, `hive knowledge search`)

### Agent Collaboration (v0.2)

- FR76: Agents can initiate multi-turn dialog threads with other agents for collaborative problem-solving
- FR77: Dialog threads maintain conversation history and context
- FR78: Dialog results are logged as events and available in the event timeline
- FR79: User can view active and completed dialog threads

### Webhook Integrations (v0.2)

- FR80: User can configure webhook notifications for key events (task completed, agent failed, workflow finished)
- FR81: System supports Slack webhook format for channel notifications
- FR82: System supports GitHub webhook format for PR/issue integration
- FR83: User can configure notification rules with filters (e.g., "only notify on failed tasks")

### Additional Adapters (v0.3)

- FR84: User can register CrewAI agents via `hive add-agent --type crewai`
- FR85: User can register LangChain/LangGraph agents via `hive add-agent --type langchain`
- FR86: User can register AutoGen agents via `hive add-agent --type autogen`
- FR87: User can register OpenAI Assistants via `hive add-agent --type openai`
- FR88: Each adapter auto-detects framework capabilities and maps to Hive protocol

### HiveHub Template Registry (v0.3)

- FR89: User can publish a hive configuration as a template to HiveHub via `hive publish`
- FR90: User can search HiveHub for templates by keyword, category, or capability via `hive search`
- FR91: User can install a HiveHub template into a local project via `hive install <template-name>`
- FR92: Published templates include: hive.yaml, agent configs, README, metadata (author, version, description)
- FR93: HiveHub stores templates in a version-controlled registry

### Distributed Event Bus (v0.3)

- FR94: System supports a distributed message broker as a pluggable event bus backend (alternative to in-process)
- FR95: User can configure event bus backend via `hive.yaml` (`event_bus: distributed` or `event_bus: embedded`)
- FR96: Distributed backend enables multi-node Hive deployments sharing the same event stream
- FR97: All existing event bus features (pub/sub, query, replay) work identically on both backends

### Enhanced Knowledge (v0.3)

- FR98: Knowledge search uses vector embeddings for semantic similarity (not just keyword matching)
- FR99: System generates embeddings locally without requiring external API calls for basic usage
- FR100: User can optionally configure external embedding API (OpenAI, Anthropic) for higher quality

### Cost Management (v0.3)

- FR101: System tracks cumulative cost per agent based on declared cost_per_run
- FR102: System tracks cumulative cost per workflow execution
- FR103: User can set budget alerts per agent or per workflow (e.g., "alert when agent exceeds $10/day")
- FR104: Budget alerts trigger webhook notifications when thresholds are exceeded

### Market-Based Task Allocation (v1.0)

- FR105: Agents can bid on tasks with price and estimated duration
- FR106: System selects winning bid based on configurable strategy (lowest cost, fastest, best reputation)
- FR107: User can configure allocation strategy per workflow (auction, round-robin, capability-match, market)
- FR108: System tracks bid history and win rates per agent
- FR109: Agents accumulate internal tokens based on task completions (token economy)

### Cross-Hive Networking (v1.0)

- FR110: User can connect two Hive deployments via secure federation protocol
- FR111: Federated hives can share agent capabilities across organizational boundaries
- FR112: Tasks can be routed to agents in federated hives when local agents lack capability
- FR113: Federation uses mTLS for secure inter-hive communication
- FR114: Each hive maintains data isolation — only capability metadata is shared, not task data
- FR115: User can configure federation rules (which capabilities to share, which hives to trust)

### Self-Optimizing Orchestration (v1.0)

- FR116: System analyzes historical execution patterns to identify optimization opportunities
- FR117: System auto-tunes task routing based on agent performance history (prefer faster/cheaper/more reliable agents)
- FR118: System suggests workflow optimizations (parallelize sequential tasks, eliminate bottlenecks)
- FR119: User can view optimization recommendations via `hive optimize` command
- FR120: System applies approved optimizations automatically on next workflow run

### Enterprise Features (v1.0)

- FR121: System supports SSO via OIDC (OpenID Connect) for user authentication
- FR122: User can define RBAC roles (admin, operator, viewer) with configurable permissions
- FR123: System exports audit logs in standard format (JSON, CSV) for compliance
- FR124: Dashboard includes compliance view showing audit trail, access history, and policy violations
- FR125: System supports multi-tenant deployment with data isolation per tenant

### Multi-Node Deployment (v1.0)

- FR126: System supports horizontal scaling with multiple Hive nodes sharing NATS event bus
- FR127: Agent registration is replicated across nodes via NATS
- FR128: Task routing considers agent location (prefer local node, fallback to remote)
- FR129: System supports a relational database as storage backend for multi-node deployments
- FR130: User can configure storage backend via `hive.yaml` (`storage: embedded` or `storage: external`)

## Non-Functional Requirements

### Performance

- NFR1: Event-to-agent-invocation latency under 200ms at p95 under normal load (< 50 concurrent tasks)
- NFR2: CLI commands (`status`, `logs`) respond within 500ms
- NFR3: Workflow validation completes within 1 second for configs with up to 20 agents and 100 tasks
- NFR4: Embedded event bus handles 1,000 events/second on single node

### Reliability

- NFR5: Agent hot-swap completes with zero task data loss in 99.9% of swap operations
- NFR6: System recovers from crash and resumes in-progress tasks from last checkpoint within 10 seconds
- NFR7: Event log maintains strict ordering guarantees (no event reordering)
- NFR8: Embedded SQLite database maintains ACID compliance for all state mutations

### Security

- NFR9: Agent-to-orchestrator communication supports API key and mTLS authentication
- NFR10: Secrets (API keys, tokens) never logged or exposed in CLI output
- NFR11: Adapter protocol validates all incoming payloads against schema before processing
- NFR12: File system permissions restrict hive config and state to owner-only access

### Portability

- NFR13: Single binary runs on macOS (arm64, x64), Linux (x64, arm64), Windows (x64) without additional dependencies
- NFR14: Zero external services required for local development (no Postgres, no Redis, no message broker)
- NFR15: Docker image available for containerized deployment with identical behavior

### Developer Experience

- NFR16: New user achieves first successful multi-agent workflow within 5 minutes following quickstart guide
- NFR17: Adapter author completes basic adapter implementation within 30 minutes following adapter guide
- NFR18: All CLI commands provide `--help` with examples
- NFR19: Error messages include actionable remediation suggestions (not just error codes)
- NFR20: CLI supports shell completion for bash, zsh, and fish

### Dashboard Performance (v0.2)

- NFR24: Dashboard initial page load under 2 seconds
- NFR25: WebSocket event delivery to dashboard under 100ms from event publication
- NFR26: Dashboard serves from embedded assets in Go binary (no separate CDN or static file server)
- NFR27: Dashboard supports 10 concurrent browser sessions without degradation

### Scalability (Design Constraints for Future)

- NFR21: Architecture allows event bus replacement without changing adapter protocol (pluggable transport)
- NFR22: State storage abstracted behind interface to allow future migration from SQLite to PostgreSQL
- NFR23: Agent routing logic isolated in pluggable module to enable future market-based allocation
