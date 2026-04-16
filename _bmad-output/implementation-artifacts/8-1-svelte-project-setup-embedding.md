# Story 8.1: Svelte Project Setup & Embedding

Status: done

## Story

As a developer,
I want the Svelte 5 dashboard scaffolded and embedded in the Go binary,
so that the dashboard is served from the single hive binary with no separate deployment.

## Acceptance Criteria

1. **Given** the `web/` directory contains a Svelte 5 project with SvelteKit
   **When** `make build` runs
   **Then** Svelte builds to `internal/dashboard/dist/` and Go embeds it via `//go:embed`

2. **Given** the embedded dashboard assets exist
   **When** `hive serve` starts the API server
   **Then** the dashboard is served at `http://localhost:8233`
   **And** API routes at `/api/` are routed to the API server, not the dashboard

3. **Given** a browser navigates to a dashboard route (e.g., `/agents`)
   **When** the file doesn't exist in the embedded assets
   **Then** the SPA fallback serves `index.html` so client-side routing works

4. **Given** the dashboard is loaded
   **When** it renders in the browser
   **Then** it loads in under 2 seconds (NFR24)

## Tasks / Subtasks

- [x] Task 1: Svelte 5 project scaffolding (AC: #1)
  - [x] Initialize SvelteKit project in `web/` with Svelte 5 and runes mode
  - [x] Configure `@sveltejs/adapter-static` to output to `../internal/dashboard/dist`
  - [x] Set `prerender: true` and `ssr: false` in `+layout.ts` for static SPA build
  - [x] Add TypeScript support via `tsconfig.json`
  - [x] Configure Vite build with `vite.config.ts`
- [x] Task 2: Go embed integration (AC: #1, #2)
  - [x] Create `internal/dashboard/embed.go` with `//go:embed dist/*`
  - [x] Implement `Handler()` returning `http.Handler` for serving embedded assets
  - [x] SPA fallback: serve `index.html` for unmatched routes (client-side routing)
  - [x] Skip `/api/` and `/ws` paths so they reach the API server
- [x] Task 3: Serve command integration (AC: #2)
  - [x] Update `internal/cli/serve.go` to mount dashboard handler at `/`
  - [x] Mount API routes at `/api/` with auth middleware
  - [x] Log dashboard URL on startup (`http://localhost:8233`)
- [x] Task 4: Makefile build pipeline (AC: #1)
  - [x] Add `dashboard` target: `cd web && npm run build`
  - [x] Update `build` target to depend on `dashboard`
  - [x] Add `serve` target: build and run `./hive serve`

## Dev Notes

### Architecture Compliance

- **Svelte 5** with runes mode enabled via `compilerOptions.runes` in `svelte.config.js`
- **SvelteKit** with `adapter-static` outputs pre-rendered HTML/CSS/JS to `internal/dashboard/dist/`
- **Go embed** via `//go:embed dist/*` in `internal/dashboard/embed.go` — zero external file dependencies at runtime
- **Single binary** — dashboard assets are compiled into the Go binary, no separate static file server needed
- **SPA routing** — `Handler()` falls back to `index.html` for unknown paths, enabling client-side routing with SvelteKit

### Key Design Decisions

- `adapter-static` with `fallback: 'index.html'` ensures all SvelteKit routes have pre-rendered HTML entry points
- Dashboard handler checks if file exists in embedded FS before falling back to index — this ensures CSS/JS/images are served correctly
- API and WebSocket paths (`/api/`, `/ws`) are explicitly excluded from dashboard handler so they reach their respective handlers
- Port 8233 is the default — configurable via `hive.yaml` or `HIVE_PORT` env var

### Integration Points

- `internal/dashboard/embed.go` — `Handler()` serves static assets and SPA fallback
- `internal/cli/serve.go` — mounts dashboard at `/`, API at `/api/`
- `web/svelte.config.js` — configures static adapter output path
- `Makefile` — `dashboard` and `build` targets

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Technology Stack]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Svelte 5 project scaffolded with SvelteKit, adapter-static, runes mode, TypeScript
- Static build outputs to `internal/dashboard/dist/` — Go embeds via `//go:embed dist/*`
- SPA fallback handler serves index.html for client-side routing while preserving static asset serving
- Serve command mounts dashboard at root, API at `/api/`, logs dashboard URL on startup
- Makefile `dashboard` target runs `npm run build`, `build` depends on it

### Change Log

- 2026-04-16: Story 8.1 implemented — Svelte 5 dashboard scaffolded, embedded in Go binary, served via `hive serve`

### File List

- web/package.json (new)
- web/svelte.config.js (new)
- web/vite.config.ts (new)
- web/tsconfig.json (new)
- web/src/app.html (new)
- web/src/app.d.ts (new)
- web/src/lib/index.ts (new)
- web/src/lib/assets/favicon.svg (new)
- web/src/routes/+layout.svelte (new)
- web/src/routes/+layout.ts (new)
- web/src/routes/+page.svelte (new)
- internal/dashboard/embed.go (new)
- internal/dashboard/dist/ (generated — built Svelte assets)
- internal/cli/serve.go (modified — mounts dashboard handler)
- Makefile (modified — added dashboard and serve targets)
