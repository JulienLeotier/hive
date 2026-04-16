---
title: "Product Brief Distillate: Hive"
type: llm-distillate
source: "product-brief-hive.md"
created: "2026-04-16"
purpose: "Token-efficient context for downstream PRD creation"
---

# Product Brief Distillate: Hive

## Requirements Hints

- Agent Adapter Protocol must be under 20 lines to implement for basic HTTP agents
- `npx create-hive` or equivalent zero-config bootstrapping required
- Single binary distribution, zero external dependencies (embedded SQLite, embedded event bus)
- Must support both request/response and streaming task interfaces
- Adapters needed for MVP: Claude Code, MCP servers, generic HTTP/CLI
- Event bus must deliver events to agents within 200ms p95
- Hot-swap: replace an agent mid-workflow without losing task state
- CLI commands: `hive init`, `hive add-agent`, `hive run`, `hive status`, `hive logs`
- 3 example templates must ship with MVP: code review hive, content pipeline hive, research hive
- Capability-based routing: agents declare capabilities, tasks declare requirements, system matches
- Task state must be externalizable (checkpoint/resume across agent replacements)
- Event sourcing for all state changes (enables time-travel debugging in future versions)

## Technical Context & Preferences

- User wants to import existing agents (Claude Code skills, BMAD agents) — not rebuild them
- MCP (Model Context Protocol) ecosystem is a strategic integration target
- Architecture should mirror proven distributed systems patterns: event sourcing, service mesh, circuit breakers
- Bio-inspired metaphors are marketing language — implementation uses concrete distributed systems primitives
- The "swarm intelligence" from brainstorming translates to: event bus + capability routing + shared state + circuit breakers
- Consider TypeScript as primary language (aligns with Claude Code, MCP, and Paperclip ecosystems)
- SQLite for persistence (zero-dependency, embeddable, proven)
- Consider NATS or custom lightweight event bus for internal messaging

## Detailed Agent Adapter Protocol Spec Hints

- `declare()` — returns JSON describing agent capabilities, supported task types, cost estimates
- `invoke(task)` — receives a task payload, returns result (sync or streaming)
- `health()` — returns agent status (healthy, degraded, unavailable)
- `checkpoint()` — returns serializable state for resume capability
- `resume(state)` — restores from checkpoint
- Adapters are thin wrappers: translate between Hive protocol and framework-specific agent interfaces
- Protocol should be versioned from day 1 (semver, with backwards compatibility commitment)

## Graduated Autonomy System (v0.2 feature, design now)

- Trust levels: Supervised → Guided → Autonomous → Trusted
- Trust earned by track record: successful task count, error rate, task type complexity
- Trust configurable per agent AND per task type (matrix)
- Trust can be manually overridden (promote/demote)
- Financial/legal/security tasks default to Supervised regardless of agent trust level
- Logging: every autonomy decision logged with reasoning for audit trail

## Shared Knowledge Layer (v0.2 feature, design now)

- Append-only log of operational patterns (successes and failures)
- Vector search for similarity-based retrieval
- Entries have: task type, approach description, outcome, embedding
- New agents query knowledge layer before starting a task type for the first time
- Knowledge entries decay over time (weighted by recency and success rate)
- NOT a magic AI system — it's a structured experience database with search

## Competitive Intelligence

- **Paperclip** (~54K stars): Manages agents as company employees. Org charts, heartbeats, budgets. Strength: governance/audit. Weakness: framework lock-in, rigid heartbeats, no learning, no interop
- **CrewAI**: Role-based multi-agent framework. Popular for simplicity. Weakness: agents must be CrewAI agents, limited orchestration patterns
- **AutoGen (Microsoft)**: Conversation-driven. Strong in research/prototyping. Weakness: complex enterprise deployment, major v0.4 rewrite created instability
- **LangGraph (LangChain)**: Graph-based state machines. Powerful but steep learning curve, tightly coupled to LangChain
- **OpenAI Swarm**: Experimental/educational. Not production-grade. Signals OpenAI interest
- **Semantic Kernel (Microsoft)**: Enterprise-oriented but not primarily an orchestrator
- None of the above solve the interop problem — every framework requires agents built in its own paradigm
- The "Kubernetes of agents" positioning is used by multiple competitors — differentiate on "OCI for agents" (protocol/standard) instead

## Rejected Ideas & Why

- "Market-based task allocation with internal token economy" — deferred from MVP, too complex for v0.1. Simple capability matching is sufficient to prove value
- "Evolutionary agent selection" — speculative research, not production feature. Deferred indefinitely
- "Neural pathway strengthening" — marketing metaphor. Implementation is simpler: track which agent pairs succeed together, prefer proven pairs. No neural network involved
- "Stigmergic communication" — interesting concept but indirect communication adds complexity without clear MVP benefit. Direct event bus is sufficient
- "Orchestration DSL (HiveQL)" — premature abstraction. CLI + YAML config for v0.x. DSL only if community demands it
- "Visual orchestration canvas" — high effort, low MVP value. Dashboard with read-only visualization first
- "Conductor-less orchestra" — requires mature agents. Initial version needs explicit task routing
- Mobile interface — unnecessary for developer-focused MVP

## Scope Signals

- **Hard MVP boundary**: Prove multi-framework agent interop through simple protocol. Everything else is v0.2+
- **MVP must feel like**: `npx create-hive` → add 2 agents from different frameworks → run a workflow → see results. Under 5 minutes
- **v0.2 is about trust**: Graduated autonomy + shared knowledge layer
- **v0.3 is about efficiency**: Market-based allocation, cost optimization
- **v1.0 is about ecosystem**: HiveHub marketplace, enterprise features, Hive Cloud

## Target Personas (Detailed)

- **Alex, Senior Engineer**: Has 4 Claude Code agents, 2 Python scripts that call GPT, and a LangChain data pipeline. Wants them working together without rewriting anything. Will evaluate Hive on: "How fast can I get my existing agents orchestrated?"
- **Sarah, Platform Engineering Lead**: Building an AI platform for 200 engineers. Needs governance, observability, cost tracking. Will evaluate Hive on: "Can I standardize our agent orchestration across teams?"
- **Marcus, CrewAI Framework Maintainer**: Wants his agents usable in multi-framework deployments. Will evaluate Hive on: "How easy is it to write an adapter for my framework?"

## Community & Ecosystem Strategy

- Adapter bounty program to seed first 20 adapters
- Ship example hive templates from day 1 (not marketplace — just YAML configs in repo)
- Discord-first community (developer audience lives there)
- Target launch venues: Hacker News, Reddit r/LocalLLaMA, AI Twitter/X
- Contributor-friendly: clear adapter contribution guide, adapter template generator CLI tool
- Partnership opportunity: Position as native orchestration layer for MCP ecosystem (Anthropic alignment)

## Open Questions

- TypeScript vs Rust vs Go for core runtime? (TS aligns with ecosystem, Go/Rust for performance)
- Should the Agent Adapter Protocol be submitted as a formal open standard (like OCI)?
- How to handle agent-to-agent direct communication vs everything-through-event-bus?
- Pricing model for Hive Cloud: per-agent, per-event, per-task, or flat tier?
- Should graduated autonomy be opt-in or opt-out (default supervised vs default autonomous)?
