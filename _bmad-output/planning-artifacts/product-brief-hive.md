---
title: "Product Brief: Hive"
status: "complete"
created: "2026-04-16"
updated: "2026-04-16"
inputs:
  - "_bmad-output/brainstorming/brainstorming-session-2026-04-16-1630.md"
reviews:
  - "skeptic-review"
  - "opportunity-review"
  - "dx-adoption-review"
---

# Product Brief: Hive

## Executive Summary

The AI agent landscape has fragmented. Teams build agents with Claude Code, CrewAI, LangChain, AutoGen, MCP servers, custom scripts — often multiple frameworks in the same organization. But there is no universal way to make these agents work together. Each framework is its own silo.

Hive is an open-source **agent orchestration platform** that coordinates any AI agent, from any framework, through a standardized protocol. Think of it as the **OCI (Open Container Initiative) for AI agents** — an open interoperability standard backed by an event-driven orchestration runtime. Import your Claude Code skills, your BMAD workflows, your CrewAI crews, your bare HTTP endpoints — write a 15-line adapter and Hive orchestrates them all.

Beyond interoperability, Hive replaces rigid scheduling and static hierarchies with **adaptive orchestration**: event-driven execution, dynamic task allocation, graduated trust, and a shared knowledge layer that makes the system smarter over time.

## The Problem

Teams deploying multiple AI agents face compounding coordination problems:

- **Framework lock-in**: Agents built in CrewAI can't talk to agents built in LangChain. Switching orchestrators means rebuilding agents from scratch. There is no interop standard
- **Rigid scheduling**: Heartbeat-based systems (e.g., Paperclip) poll on fixed intervals — wasting compute when idle, missing urgent events between beats
- **Static hierarchies**: Fixed org charts can't adapt when a content task suddenly needs a developer's input, or when the assigned "manager" agent is wrong for the decision
- **No institutional memory**: Every agent operates in isolation. A lesson learned by one agent is invisible to others. The same mistakes repeat across the system
- **Binary governance**: Either full human approval on every action (bottleneck) or full autonomy (risk). No middle ground that grows with demonstrated competence

These aren't edge cases — they're the daily reality for any team running more than a handful of agents.

## The Solution

Hive is built on three pillars:

### 1. Universal Agent Protocol (the moat)

Hive defines an **open Agent Adapter Protocol** — a minimal interface any agent can implement to participate in orchestrated workflows. The protocol covers:
- **Capability declaration**: Agent describes what it can do (structured format)
- **Task interface**: Receive work, return results (request/response + streaming)
- **State protocol**: Checkpoint, resume, report health
- **Event hooks**: Subscribe to and emit events

Writing an adapter is trivial — under 20 lines for a basic HTTP agent. Pre-built adapters ship for Claude Code, BMAD agents, MCP servers, and generic HTTP/CLI. Community contributes the rest.

**This is the real defensibility**: Hive doesn't compete with agent frameworks — it connects them. Framework authors are allies, not competitors.

### 2. Adaptive Orchestration

Replace static coordination with dynamic, event-driven patterns:
- **Event bus**: Agents react to events in real-time (task created, result available, error occurred) — no polling, no wasted cycles
- **Capability-based routing**: Tasks routed to agents based on declared capabilities and current availability — not fixed assignment
- **Dynamic task graphs**: Dependencies expressed as DAGs, automatically parallelized where possible, with critical path optimization
- **Circuit breakers**: Failing agents auto-isolated, work rerouted to healthy alternatives. Graceful degradation, not cascade failure

These are proven distributed systems patterns (event sourcing, service mesh, circuit breakers) applied to agent coordination — not speculative research.

### 3. Autonomous Self-Directed Agents

Each agent carries its own behavioral plan — a YAML state machine defining what it does at every wake-up. Agents don't wait for task assignments: they observe the shared state, decide what needs doing, and execute. The orchestrator provides heartbeat schedules and event triggers, but agents are self-directed. "Idle is success" — agents with empty backlogs don't generate busywork.

### 4. Graduated Trust & Shared Learning

**Graduated Autonomy** — a trust spectrum, not binary gates:

| Trust Level | Behavior | Earned After |
|---|---|---|
| **Supervised** | Every action requires human approval | Default for new agents |
| **Guided** | Human notified, can intervene within window | 50+ successful tasks |
| **Autonomous** | Agent acts freely, human reviews async | 200+ tasks, <5% error rate |
| **Trusted** | Full autonomy, exception-only alerts | 500+ tasks, <2% error rate |

Trust levels are configurable and can be overridden per task type (e.g., "always supervised for financial transactions").

**Shared Knowledge Layer** — a key-value store of operational patterns:
- Successful task approaches stored as reusable entries
- Failed approaches flagged to prevent repetition
- New agents inherit the colony's accumulated knowledge on first boot
- Implementation: straightforward append-only log with vector search — not magic, just useful persistence

## What Makes This Different

| Dimension | Paperclip | CrewAI | LangGraph | Hive |
|---|---|---|---|---|
| Agent source | Own runtime | Own framework | LangChain only | **Any framework (open protocol)** |
| Scheduling | Fixed heartbeat | Sequential/parallel | Graph steps | **Event-driven reactive** |
| Coordination | Org chart | Role assignment | State machine | **Capability-based dynamic routing** |
| Learning | None | None | None | **Shared knowledge layer** |
| Governance | Binary gates | Minimal | Checkpoints | **Graduated autonomy spectrum** |
| Resilience | Manual | Retry | Retry | **Circuit breakers + failover** |

**Why now**: The 2024-2026 agent framework explosion created massive fragmentation. The market needs an interop layer — the same way Docker fragmentation created the need for OCI, and container sprawl created the need for Kubernetes.

## Who This Serves

**Primary: Engineering teams running 2+ agent frameworks**
They've built agents in Claude Code for dev tasks, LangChain for data pipelines, custom scripts for ops. They need these agents coordinating, not siloed. Hive is the connective tissue.

**Secondary: Platform engineering teams**
Building internal developer platforms for AI adoption. They need a standardized orchestration layer to offer their org — with governance, observability, and cost controls built in.

**Tertiary: Agent framework authors**
Want their agents composable with others. Implementing the Hive adapter protocol gives their framework instant access to the entire Hive ecosystem.

**Adjacent opportunity: RPA migration**
Enterprises with legacy UiPath/Automation Anywhere workflows seeking AI-native replacements. Hive + adapters for RPA bots = direct migration path.

## Success Criteria

- **Developer adoption**: 100+ unique deployers running real workloads within 6 months (measured by telemetry opt-in, not GitHub stars)
- **Ecosystem growth**: 10+ community-contributed adapters within 6 months
- **Time-to-first-orchestration**: New user goes from `npx create-hive` to first multi-agent workflow in under 5 minutes
- **Latency**: Event-to-agent-invocation under 200ms p95
- **Reliability**: Zero-downtime agent replacement via hot-swap protocol validated in CI

## Scope

### MVP (v0.1) — Laser Focus

The MVP proves ONE thing: **agents from different frameworks can be orchestrated together through a simple protocol.**

- Agent Adapter Protocol specification (open standard)
- Core event bus (lightweight, embedded — zero external dependencies)
- Adapters: Claude Code, MCP servers, generic HTTP
- Capability-based task routing (simple matching, not market-based)
- CLI: `hive init`, `hive add-agent`, `hive run`, `hive status`
- 3 example hive templates (code review, content pipeline, research)
- Documentation site with "Hello Hive" tutorial
- Single binary, single node, zero dependencies (SQLite embedded)

### Deferred to v0.2+
- Dashboard UI
- Graduated autonomy engine (v0.2)
- Shared knowledge layer (v0.2)
- Market-based task allocation (v0.3)
- Multi-node deployment (v0.4)
- HiveHub marketplace (v1.0)

### Explicitly Out of Scope
- Visual orchestration canvas
- Cross-hive networking
- Orchestration DSL
- Mobile interface

## Business Model

Open-source core (MIT). Revenue through:
- **Hive Cloud** (managed orchestration-as-a-service) for teams that don't want to self-host
- **Enterprise features** (open-core): SSO, RBAC, advanced audit logs, compliance dashboards
- **HiveHub transactions** (future): cut on marketplace template deployments
- **Certification program** (future): "Hive-compatible" adapter certification for framework authors

## Vision

Year 1: Open-source platform with proven interop across major frameworks. Community-driven adapter ecosystem. Adopted by early-stage teams and indie hackers running multi-agent workflows.

Year 2: **HiveHub** — a marketplace of pre-built hive templates. "Deploy a content marketing hive" in one click. Graduated autonomy and shared learning ship as mature features. First enterprise customers on Hive Cloud.

Year 3: **Hive Protocol** becomes the de facto agent interop standard. Cross-organization agent collaboration. The platform layer for the emerging agent economy.

The endgame: Hive is to AI agents what Kubernetes is to containers — the orchestration layer everyone runs on, not because of lock-in, but because it's the connective tissue the ecosystem needs.
