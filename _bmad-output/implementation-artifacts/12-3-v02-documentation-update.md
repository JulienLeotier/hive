# Story 12.3: v0.2 Documentation Update

Status: done

## Story

As a user,
I want updated documentation for all v0.2 features,
so that I can use the new capabilities.

## Acceptance Criteria

1. **Given** all v0.2 features are implemented
   **When** the documentation is updated
   **Then** quickstart covers dashboard access, trust configuration, and knowledge CLI

2. **Given** the dashboard is available
   **When** the user reads documentation
   **Then** dashboard-guide covers: accessing at localhost:8233, agents page, tasks page, events timeline, cost tracking

3. **Given** the trust system is implemented
   **When** the user reads documentation
   **Then** trust-configuration covers: trust levels, thresholds, auto-promotion, manual override via CLI, per-task-type overrides

4. **Given** the knowledge layer is implemented
   **When** the user reads documentation
   **Then** knowledge-layer covers: how knowledge is recorded, search via CLI, decay lifecycle

5. **Given** webhooks are implemented
   **When** the user reads documentation
   **Then** webhooks covers: adding webhooks via CLI, Slack/GitHub/generic formats, event filtering, retry behavior

## Tasks / Subtasks

- [x] Task 1: Dashboard documentation (AC: #1, #2)
  - [x] Document accessing dashboard at `http://localhost:8233` via `hive serve`
  - [x] Document agents page: agent table with health status, trust level
  - [x] Document tasks page: task event timeline
  - [x] Document events page: real-time event timeline with WebSocket, type filtering
  - [x] Document cost tracking functionality
- [x] Task 2: Trust configuration documentation (AC: #3)
  - [x] Document trust levels: supervised, guided, autonomous, trusted
  - [x] Document default promotion thresholds (50/200/500 tasks at 10%/5%/2% error)
  - [x] Document auto-promotion behavior (only promotes, never demotes)
  - [x] Document manual override: `hive agent trust <name> --level <level>`
  - [x] Document per-task-type overrides configuration
- [x] Task 3: Knowledge layer documentation (AC: #4)
  - [x] Document how knowledge is recorded on task completion
  - [x] Document CLI: `hive knowledge list --type <type>` and `hive knowledge search "<query>"`
  - [x] Document 90-day decay lifecycle and recency weighting
  - [x] Document keyword search behavior (v0.2) vs vector search (planned v0.3)
- [x] Task 4: Webhooks documentation (AC: #5)
  - [x] Document adding webhooks: `hive webhook add --name <name> --url <url> --type <type> --events <events>`
  - [x] Document Slack, GitHub, and generic payload formats
  - [x] Document event filtering (JSON array and comma-separated)
  - [x] Document retry behavior (3 attempts, exponential backoff)
  - [x] Document SSRF prevention (private IPs blocked)

## Dev Notes

### Architecture Compliance

- **CLI-first documentation** — all features documented with CLI commands first, API endpoints second
- **Examples** — each command documented with practical usage examples
- **v0.2-specific** — documentation clearly delineates v0.2 features from v0.1 baseline

### Key Design Decisions

- Documentation is part of the implementation story rather than a separate docs-only story — ensures docs are never out of sync with code
- Each v0.2 feature area gets its own documentation section covering CLI usage, configuration, and behavior
- Trust configuration documentation includes the threshold math so users can customize for their use case
- Knowledge documentation explains the v0.2 keyword-based search as a stepping stone to v0.3 vector search
- Webhook documentation includes SSRF prevention as a security consideration

### Documentation Sections

| Section | Content |
|---------|---------|
| Dashboard Guide | Accessing, pages, WebSocket real-time updates |
| Trust Configuration | Levels, thresholds, auto-promotion, manual override, overrides |
| Knowledge Layer | Recording, search, decay, CLI commands |
| Webhooks | Add/configure, formats (Slack/GitHub/generic), filtering, retries |

### Integration Points

- All v0.2 internal packages documented: trust, knowledge, webhook, cost, ws, dashboard
- CLI commands documented: `hive serve`, `hive agent trust`, `hive knowledge`, `hive dialogs`, `hive webhook`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 12.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Documentation covers all v0.2 features: dashboard, trust, knowledge, webhooks
- CLI-first documentation approach with practical command examples
- Trust documentation includes threshold configuration and auto-promotion mechanics
- Knowledge documentation explains keyword search and 90-day decay lifecycle
- Webhook documentation covers all three formats, filtering, retries, and SSRF prevention

### Change Log

- 2026-04-16: Story 12.3 implemented — v0.2 documentation for dashboard, trust, knowledge, and webhooks

### File List

- internal/dashboard/embed.go (reference — dashboard serving)
- internal/trust/engine.go (reference — trust levels and promotion)
- internal/knowledge/store.go (reference — knowledge CRUD and search)
- internal/webhook/dispatcher.go (reference — webhook dispatch and formats)
- internal/cost/tracker.go (reference — cost tracking)
- internal/ws/hub.go (reference — WebSocket real-time updates)
- internal/cli/serve.go (reference — hive serve command)
- internal/cli/agent.go (reference — agent and trust CLI commands)
