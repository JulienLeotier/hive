# Dashboard Guide

The Hive web dashboard is a Svelte 5 single-page application embedded in the Go binary. It provides real-time visibility into agent health, tasks, and events.

## Accessing the Dashboard

Start the server and open your browser:

```bash
hive serve
# Dashboard: http://localhost:8233
```

The dashboard is served at the root path `/`. API endpoints are at `/api/v1/*`. The SPA router in `internal/dashboard/embed.go` serves `index.html` for any path that does not match a static file.

## Pages

### Home (`/`)

The main dashboard page shows:
- **Agent count** -- total registered agents
- **Task count** -- current task activity
- **Event count** -- recent event volume

Stats refresh every 5 seconds via polling `GET /api/v1/metrics`. Navigation links to Agents, Tasks, and Events pages.

**Source:** `web/src/routes/+page.svelte`

### Agents (`/agents`)

Displays a table of all registered agents with columns:
- **Name** -- agent identifier
- **Type** -- adapter type (http, claude-code, mcp, etc.)
- **Health** -- color-coded badge: green (healthy), amber (degraded), red (unavailable)
- **Trust** -- current trust level (scripted, supervised, guided, autonomous, trusted)
- **Capabilities** -- JSON capability declaration

If no agents are registered, shows a prompt to use `hive add-agent`.

Data refreshes every 3 seconds via `GET /api/v1/agents`.

**Source:** `web/src/routes/agents/+page.svelte`

### Tasks (`/tasks`)

Shows task-related events in a table with columns: ID, Type, Source, Time. Data comes from `GET /api/v1/events?type=task` and refreshes every 3 seconds.

**Source:** `web/src/routes/tasks/+page.svelte`

### Events (`/events`)

Real-time event timeline with:
- **Type filter** -- text input to filter events by type prefix (e.g., `task`, `agent.health`)
- **Timeline view** -- each event shows timestamp, type, source, and payload

Events load initially via `GET /api/v1/events` and update in real-time via WebSocket.

**Source:** `web/src/routes/events/+page.svelte`

## WebSocket Real-Time Updates

The Events page establishes a WebSocket connection to `/ws` for live event streaming.

```javascript
const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
ws = new WebSocket(`${proto}//${location.host}/ws`);
ws.onmessage = (msg) => {
    const evt = JSON.parse(msg.data);
    events = [evt, ...events].slice(0, 100);
};
```

The WebSocket hub (`internal/ws/hub.go`) broadcasts every event to all connected clients. Key behaviors:
- Auto-reconnect on disconnect (3-second delay)
- Events prepended to list, capped at 100
- Each event message is JSON: `{id, type, source, payload, created_at}`

### Origin Policy

WebSocket connections from `localhost` and `127.0.0.1` are always allowed. Additional origins can be configured via `ws.AllowedOrigins` in `internal/ws/hub.go`.

## API Endpoints Used by Dashboard

| Endpoint | Method | Used By | Params |
|---|---|---|---|
| `/api/v1/metrics` | GET | Home page | -- |
| `/api/v1/agents` | GET | Agents page | -- |
| `/api/v1/events` | GET | Events, Tasks pages | `type`, `source`, `since` |
| `/ws` | WebSocket | Events page | -- |

All API responses use the envelope format: `{"data": ..., "error": null}`.

## Authentication

If API keys have been configured, API requests require a `Bearer` token in the `Authorization` header. The dashboard's static pages are served without authentication. In dev mode (no API keys), all API requests are allowed.

## Technical Details

### Build Pipeline

1. `make dashboard` runs `cd web && npm run build` (Svelte/Vite)
2. Build output goes to `internal/dashboard/dist/`
3. `//go:embed dist/*` in `internal/dashboard/embed.go` bundles assets into the Go binary
4. `make build` compiles the final single binary

### SvelteKit Configuration

- **SSR disabled**: `export const ssr = false` in `+layout.ts` -- pure client-side SPA
- **Prerender enabled**: `export const prerender = true`
- **Framework**: Svelte 5 with `$state` and `$effect` runes

### Development Workflow

For frontend development with hot reload:

```bash
# Terminal 1: Start the Go API server
hive serve

# Terminal 2: Start Vite dev server with proxy
cd web && npm run dev
```

The Vite dev server proxies API requests to the Go backend.
