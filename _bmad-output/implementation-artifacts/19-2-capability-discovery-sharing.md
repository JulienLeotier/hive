# Story 19.2: Capability Discovery & Sharing

Status: done

## Story

As a user,
I want to configure which capabilities my hive shares with federated partners,
so that I control what's exposed.

## Acceptance Criteria

1. **Given** a federation link is established
   **When** the user configures `federation.share: [code-review, summarize]` in `hive.yaml`
   **Then** only the listed capabilities are visible to the partner hive

2. **Given** a federation link with shared capabilities
   **When** the partner hive queries available capabilities
   **Then** only explicitly shared capabilities are returned

3. **Given** a federation share config change
   **When** the next capability refresh occurs
   **Then** the partner receives the updated capability list

4. **Given** no `federation.share` config
   **When** a federation link is established
   **Then** no capabilities are shared by default (secure by default)

## Tasks / Subtasks

- [x] Task 1: Share configuration parsing (AC: #1, #4)
  - [x] Add `federation.share` field to config schema
  - [x] Parse capability share list from `hive.yaml`
  - [x] Default to empty list (share nothing) when not configured
  - [x] Validate that shared capabilities exist in registered agents
- [x] Task 2: Capability filter (AC: #1, #2)
  - [x] Implement `FilterSharedCapabilities(allCapabilities, shareList)` -- returns only shared capabilities
  - [x] Apply filter before sending capabilities to federated peers
  - [x] Include agent metadata (availability, capacity) but not agent identity details
  - [x] Strip sensitive fields from capability metadata before sharing
- [x] Task 3: Capability discovery from peers (AC: #2, #3)
  - [x] Receive and store peer's shared capabilities in federation_links table
  - [x] Index remote capabilities for task routing lookups
  - [x] Handle capability updates on periodic refresh
  - [x] Emit `federation.capabilities.discovered` event when new capabilities appear
- [x] Task 4: Dynamic share updates (AC: #3)
  - [x] Watch for config changes to `federation.share`
  - [x] Push updated capability list to all connected peers on change
  - [x] Peers update their stored remote capabilities accordingly
- [x] Task 5: CLI visibility (AC: #1, #2)
  - [x] Add `--capabilities` flag to `hive federation list` to show shared/remote capabilities
  - [x] Show which capabilities are shared outbound and which are available from peers
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test capability filtering with share list
  - [x] Test default empty share list
  - [x] Test capability update propagation
  - [x] Test sensitive field stripping
  - [x] Test remote capability indexing

## Dev Notes

### Architecture Compliance

- Secure by default: no capabilities shared unless explicitly configured
- Capability filtering happens at the federation protocol layer, before data leaves the hive
- Remote capabilities are indexed in memory for fast task routing lookups
- Uses `slog` for structured logging of capability discovery events

### Key Design Decisions

- Share list is a whitelist, not a blacklist -- explicit opt-in is safer for cross-org sharing
- Agent identity (names, IDs) is never shared; only abstract capability descriptors
- Remote capabilities are cached in the federation_links table and refreshed on the periodic interval set in 19.1
- Config changes trigger immediate capability push rather than waiting for next refresh cycle

### Integration Points

- internal/federation/protocol.go (modified -- capability filtering, share config, discovery)
- internal/federation/protocol_test.go (modified -- capability filtering and discovery tests)
- internal/config/config.go (modified -- federation.share field in Config struct)
- internal/config/config_test.go (modified -- federation share config parsing tests)
- internal/cli/federation.go (modified -- --capabilities flag on list command)
- internal/task/router.go (modified -- lookup remote capabilities for routing decisions)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 19 - Story 19.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR111, FR115]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Capability sharing configured via `federation.share` whitelist in hive.yaml
- Secure by default: empty share list means no capabilities exposed
- Capability filter strips sensitive fields and agent identity before sharing
- Remote capabilities indexed for task routing; refreshed on periodic interval
- Config changes trigger immediate push to connected peers
- CLI shows shared and remote capabilities with --capabilities flag

### Change Log

- 2026-04-16: Story 19.2 implemented -- capability discovery and sharing with whitelist-based exposure control

### File List

- internal/federation/protocol.go (modified -- capability filtering, share list enforcement, remote capability indexing)
- internal/federation/protocol_test.go (modified -- filtering, discovery, and propagation tests)
- internal/config/config.go (modified -- added federation.share config field)
- internal/config/config_test.go (modified -- federation share parsing tests)
- internal/cli/federation.go (modified -- --capabilities flag on list command)
- internal/task/router.go (modified -- remote capability lookup for routing)
