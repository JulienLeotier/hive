# UX Design Specification -- Hive Dashboard

**Version:** 0.2  
**Date:** 2026-04-16  
**Covers:** FR57-FR62, NFR24-NFR27

## 1. Design Principles

- **Minimal**: No chrome beyond what conveys information. Every pixel earns its place.
- **Fast**: Initial load < 2s (NFR24). No spinners for cached data; skeleton placeholders only on first paint.
- **Developer-focused**: Monospace for IDs/payloads, keyboard-navigable tables, NO_COLOR/prefers-reduced-motion respected.
- **Real-time**: Data arrives via WebSocket; the UI never requires manual refresh under normal operation.

## 2. Information Architecture

```
/ (Home)
  +-- /agents        Agent registry, health, capabilities
  +-- /tasks         Task list with status, duration, assignment
  +-- /events        Real-time event timeline with filters
```

**Navigation**: Persistent top bar with logo + page links (Home, Agents, Tasks, Events). Active page highlighted. Back-link breadcrumbs removed in favor of global nav.

## 3. Page Specifications

### 3.1 Home (`/`)

**Layout**: Header + 4 stat cards in a responsive grid + quick-nav links.

| Card | Data Source | Format |
|---|---|---|
| Agents (total / healthy) | `/api/v1/metrics` | `12 / 14` with health ratio bar |
| Active Tasks | `/api/v1/metrics` | Count, colored by any failures |
| Events (last hour) | `/api/v1/metrics` | Count |
| System Uptime | `/api/v1/metrics` | Duration string |

**Interactions**: Cards are clickable links to their detail page. Metrics poll every 5s (existing behavior).  
**Empty state**: Cards show `--` with "No data yet" subtext.  
**Error state**: Red outline on card, tooltip "API unreachable".

### 3.2 Agents (`/agents`)

**Layout**: Data table with sortable columns.

| Column | Content | Notes |
|---|---|---|
| Name | Agent name (bold) | Links to future detail view |
| Type | Framework type | Badge style |
| Health | Status badge | Color-coded (see section 8) |
| Trust | Trust level | Text with level indicator |
| Capabilities | Comma-separated list | Truncated with tooltip |

**Interactions**: Column header click sorts asc/desc. Row hover highlights.  
**Empty state**: Illustration-free message: "No agents registered. Run `hive add-agent` to get started."  
**Error state**: Inline banner above table: "Failed to load agents. Retrying..."

### 3.3 Tasks (`/tasks`)

**Layout**: Filterable data table.

| Column | Content | Notes |
|---|---|---|
| ID | Short hash | Monospace |
| Type | Task type | |
| Status | Status badge | Color-coded (pending/assigned/running/completed/failed) |
| Agent | Assigned agent name | |
| Duration | Elapsed or total time | Human-readable |
| Created | Timestamp | Relative ("2m ago") with absolute tooltip |

**Interactions**: Status filter dropdown (All / Pending / Running / Completed / Failed). Click row to expand inline detail (payload, result preview).  
**Empty state**: "No tasks yet."  
**Error state**: Same inline banner pattern as Agents.

### 3.4 Events (`/events`)

**Layout**: Filter bar + vertical timeline list. Most recent first.

**Filter bar**: Text input for type filter + optional source filter + time range selector (Last 1h / 6h / 24h / All).

**Event item**: Left border colored by event type. Shows: timestamp, type badge, source, truncated payload (expandable on click).

**Interactions**: Type-ahead filter (existing). Click event to expand full payload in formatted JSON. WebSocket indicator in top-right of page (green dot = connected, amber dot = reconnecting).  
**Empty state**: "Waiting for events..." with subtle pulse animation.  
**Error state**: "WebSocket disconnected. Reconnecting..." banner.

## 4. Responsive Design

| Breakpoint | Behavior |
|---|---|
| >= 1024px | Full layout, side-by-side stat cards, full table columns |
| 768-1023px | Stat cards 2x2 grid, table hides Capabilities column |
| < 768px | Stat cards stack vertically, tables become card-list layout, nav collapses to hamburger |

Mobile: Touch targets minimum 44x44px. Tables convert to stacked card format below 768px.

## 5. Accessibility

- All interactive elements keyboard-focusable with visible focus ring.
- Tables use proper `<th scope="col">` and `aria-sort` attributes.
- Status badges include `aria-label` (not color-only semantics).
- WebSocket connection indicator has `role="status"` and `aria-live="polite"`.
- Respect `prefers-reduced-motion`: disable pulse animations, transitions.
- Respect `prefers-color-scheme`: support light/dark modes via CSS custom properties.
- Respect `NO_COLOR`: not directly applicable to browser UI, but embedded CSS avoids reliance on color alone for meaning (shapes/icons accompany color).
- Minimum contrast ratio: 4.5:1 for text, 3:1 for large text/UI components (WCAG AA).

## 6. Component Library

### StatCard
Props: `label`, `value`, `subvalue?`, `href`, `status?` (ok | warn | error).  
Renders: Clickable card with large value, label below, optional sub-value and status-colored accent bar.

### DataTable
Props: `columns[]`, `rows[]`, `sortable?`, `emptyMessage`.  
Renders: Sortable table with hover rows, responsive card fallback on mobile.

### EventItem
Props: `timestamp`, `type`, `source`, `payload`, `expanded?`.  
Renders: Timeline entry with colored left border, expandable payload.

### StatusBadge
Props: `status`, `label?`.  
Renders: Pill with background color + text. Includes `aria-label`.

### FilterBar
Props: `filters[]`, `onchange`.  
Renders: Horizontal bar with text inputs, dropdowns, and apply button.

### ConnectionIndicator
Props: `connected`.  
Renders: Small dot (green/amber) with tooltip text and `aria-live` region.

## 7. Real-Time Behavior

- **WebSocket endpoint**: `ws(s)://{host}/ws`
- **Connection**: Established on Events page mount. Reconnect with exponential backoff (3s, 6s, 12s, max 30s).
- **Message format**: JSON event objects appended to head of list.
- **Buffer**: Client keeps max 100 events in memory (existing behavior, appropriate).
- **Visual indicator**: `ConnectionIndicator` component shown on Events page. Green = connected, amber = reconnecting, red = failed after 5 retries.
- **Home page metrics**: Continue polling via REST (5s interval). WebSocket not required for stat cards -- simplicity over consistency.

## 8. Color System

CSS custom properties on `:root`, with `prefers-color-scheme: dark` overrides.

| Token | Light | Dark | Usage |
|---|---|---|---|
| `--color-healthy` | `#16a34a` | `#22c55e` | Healthy agents, completed tasks |
| `--color-degraded` | `#d97706` | `#f59e0b` | Degraded health, running tasks |
| `--color-unavailable` | `#dc2626` | `#ef4444` | Unavailable agents, failed tasks |
| `--color-neutral` | `#6b7280` | `#9ca3af` | Pending, unknown, inactive |
| `--color-info` | `#2563eb` | `#3b82f6` | Assigned tasks, info events |
| `--color-surface` | `#ffffff` | `#1a1a1a` | Page background |
| `--color-card` | `#f5f5f5` | `#262626` | Card/row background |
| `--color-text` | `#1a1a1a` | `#e5e5e5` | Primary text |
| `--color-text-muted` | `#6b7280` | `#9ca3af` | Secondary text, labels |
| `--color-border` | `#e5e7eb` | `#374151` | Table borders, dividers |

## 9. Typography

```css
--font-sans: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
--font-mono: 'SF Mono', 'Cascadia Code', 'Fira Code', Consolas, monospace;

--text-xs: 0.75rem;   /* timestamps, payload previews */
--text-sm: 0.875rem;  /* table cells, labels, badges */
--text-base: 1rem;    /* body text */
--text-lg: 1.25rem;   /* page titles */
--text-xl: 2rem;      /* stat card values */

--font-normal: 400;
--font-medium: 500;   /* table headers, nav links */
--font-bold: 700;     /* stat values, agent names */
```

## 10. Missing UX Gaps (Current vs. FRs)

| Gap | FR | Current State | Needed |
|---|---|---|---|
| No shared layout/nav | -- | Each page has inline back-link; no global nav bar | Add `+layout.svelte` with persistent top nav |
| Task/Event count not populated on Home | FR57 | `taskCount` and `eventCount` always 0 | Wire to `/api/v1/metrics` response fields |
| No task flow visualization | FR58 | Task page shows flat event list, not task objects | Dedicated task table with status, duration, agent columns |
| No time-range filter on Events | FR59 | Type filter only | Add source filter and time-range selector |
| No cost tracking view | FR60 | Not implemented | Add cost column to Agents table; cost summary card on Home |
| No dark mode | -- | Hardcoded light colors | CSS custom properties with `prefers-color-scheme` |
| No loading/error states | -- | Silent catch, no UI feedback | Skeleton loaders, error banners, retry indicators |
| No WebSocket indicator | FR61 | WebSocket connects silently | `ConnectionIndicator` component on Events page |
| No sortable tables | -- | Static table rendering | Sortable column headers on Agents and Tasks |
| No mobile responsiveness | -- | Fixed-width layout | Responsive breakpoints per section 4 |
| No keyboard navigation | FR42 | No focus management | Focus rings, `aria-sort`, keyboard-operable filters |
| Tasks page fetches events, not tasks | FR58 | Calls `/api/v1/events?type=task` | Should call a dedicated tasks endpoint with proper fields |
