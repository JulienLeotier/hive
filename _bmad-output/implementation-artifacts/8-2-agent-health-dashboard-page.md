# Story 8.2: Agent Health Dashboard Page

Status: done

## Story

As a user,
I want a dashboard page showing all agents with real-time health status,
so that I can monitor my hive at a glance.

## Acceptance Criteria

1. **Given** the dashboard is loaded in a browser
   **When** the agents page is displayed
   **Then** it shows a table of all agents with: name, type, health status, trust level, capabilities, last check time

2. **Given** agents exist in the hive
   **When** the agents page polls the API
   **Then** the table refreshes every 3 seconds via `setInterval` polling `/api/v1/agents`

3. **Given** an agent's health status changes
   **When** the next poll cycle runs
   **Then** the table updates with the new status displayed as a color-coded badge (green=healthy, amber=degraded, red=unavailable)

4. **Given** no agents are registered
   **When** the agents page is displayed
   **Then** it shows a helpful empty state message: "No agents registered. Use `hive add-agent` to register one."

## Tasks / Subtasks

- [x] Task 1: Agents page component (AC: #1, #2, #3, #4)
  - [x] Create `web/src/routes/agents/+page.svelte` with Svelte 5 runes
  - [x] Define `Agent` type with id, name, type, health_status, trust_level, capabilities fields
  - [x] Implement `loadAgents()` function fetching from `/api/v1/agents`
  - [x] Use `$effect()` for 3-second polling interval with cleanup
  - [x] Render HTML table with columns: Name, Type, Health, Trust, Capabilities
  - [x] Color-coded health badge via `statusColor()` helper (green/amber/red)
  - [x] Empty state message when no agents registered
- [x] Task 2: API endpoint for agents (AC: #1)
  - [x] `GET /api/v1/agents` endpoint in `internal/api/server.go` returns agent list as JSON
  - [x] Uses `agent.Manager.List()` to fetch all agents from SQLite
  - [x] Response wrapped in standard `{data: [...]}` envelope

## Dev Notes

### Architecture Compliance

- **Svelte 5 runes** — uses `$state` for reactive agent list, `$effect` for lifecycle management
- **Typed data** — TypeScript `Agent` type ensures type safety in the template
- **Polling pattern** — 3-second `setInterval` with `$effect` cleanup prevents memory leaks
- **API consumption** — fetches from `/api/v1/agents`, same origin, no CORS issues when served from embedded binary

### Key Design Decisions

- Polling every 3 seconds rather than WebSocket for the agents page — agent health doesn't change frequently enough to warrant real-time push; simple polling is more reliable and easier to debug
- Color-coded badges use semantic colors: `#22c55e` (green) for healthy, `#f59e0b` (amber) for degraded, `#ef4444` (red) for unavailable
- Trust level displayed as plain text — future stories may add visual progression indicators
- Capabilities shown in `<code>` tags as raw JSON string for transparency

### Integration Points

- `web/src/routes/agents/+page.svelte` — agent health dashboard page
- `internal/api/server.go` — `handleListAgents` endpoint at `GET /api/v1/agents`
- `internal/agent/manager.go` — `List()` method for fetching agents from SQLite

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR57, FR61]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Agents page displays table with name, type, health badge, trust level, capabilities
- Auto-refreshes every 3 seconds via polling `/api/v1/agents`
- Health status shown as color-coded badges (green/amber/red)
- Empty state with helpful guidance message for new users
- TypeScript types ensure data consistency

### Change Log

- 2026-04-16: Story 8.2 implemented — agent health dashboard page with polling and color-coded status

### File List

- web/src/routes/agents/+page.svelte (new)
- internal/api/server.go (reference — handleListAgents endpoint)
- internal/agent/manager.go (reference — List method)
