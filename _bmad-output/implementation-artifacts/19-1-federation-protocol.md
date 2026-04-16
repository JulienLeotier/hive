# Story 19.1: Federation Protocol

Status: done

## Story

As a user,
I want to connect my Hive to another organization's Hive,
so that we can share agent capabilities for collaboration.

## Acceptance Criteria

1. **Given** two Hive deployments with mTLS certificates
   **When** the user runs `hive federation connect --url hive.partner.com --cert ./partner.pem`
   **Then** a secure federation link is established

2. **Given** a federation link
   **When** the connection is established
   **Then** capability metadata is exchanged (not task data)

3. **Given** a federation link
   **When** connection health degrades
   **Then** the system monitors and reports connection status

4. **Given** a federation link
   **When** the user runs `hive federation list`
   **Then** all federation links are displayed with connection status

## Tasks / Subtasks

- [x] Task 1: Federation data model (AC: #1)
  - [x] Define `FederationLink` struct with peer URL, certificate path, status, capabilities, last seen
  - [x] Create `federation_links` table in v1.0 migration
  - [x] Define federation states: connecting, active, degraded, disconnected
- [x] Task 2: FederationProtocol core (AC: #1, #2)
  - [x] Create `FederationProtocol` struct in `internal/federation/protocol.go`
  - [x] Implement `Connect(peerURL, certPath)` -- establish mTLS connection
  - [x] Implement `ExchangeCapabilities()` -- send/receive capability metadata
  - [x] Implement `Disconnect(peerURL)` -- gracefully close federation link
  - [x] Store federation link in SQLite
- [x] Task 3: mTLS connection management (AC: #1, #3)
  - [x] Load client certificate and CA from file paths
  - [x] Configure `tls.Config` with mutual TLS verification
  - [x] Implement connection health check via periodic ping
  - [x] Auto-reconnect with exponential backoff on connection loss
  - [x] Transition link status based on health check results
- [x] Task 4: Capability exchange protocol (AC: #2)
  - [x] Define capability metadata message format (JSON over mTLS HTTP)
  - [x] Send local shared capabilities on connection
  - [x] Receive and store remote capabilities
  - [x] Refresh capabilities on configurable interval (default 60s)
  - [x] Emit `federation.capabilities.updated` event on change
- [x] Task 5: CLI commands (AC: #1, #4)
  - [x] Implement `hive federation connect --url <url> --cert <path>` command
  - [x] Implement `hive federation disconnect <peer>` command
  - [x] Implement `hive federation list` -- table of links with status, capabilities, last seen
  - [x] Support `--json` output flag
- [x] Task 6: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test federation link lifecycle (connect, active, disconnect)
  - [x] Test capability exchange serialization/deserialization
  - [x] Test health check state transitions
  - [x] Test reconnection with backoff
  - [x] Test CLI output formatting

## Dev Notes

### Architecture Compliance

- `internal/federation/protocol.go` contains all federation logic
- mTLS is mandatory for federation -- no plaintext federation links allowed
- Only capability metadata crosses federation boundaries; task data stays local until explicitly routed (Story 19.3)
- Uses `slog` for structured logging of federation events
- Federation links persisted to SQLite for recovery after restart

### Key Design Decisions

- Federation is peer-to-peer, not hub-and-spoke -- each hive connects directly to partners
- Capability exchange happens at connection time and periodically thereafter
- Health monitoring uses periodic ping (default 15s) with degraded status after 3 missed pings
- mTLS certificates are loaded from filesystem paths, not embedded in config -- allows rotation without config changes
- Federation protocol uses JSON over HTTPS (not gRPC) for simplicity and compatibility

### Integration Points

- internal/federation/protocol.go -- FederationProtocol, FederationLink, capability exchange
- internal/federation/protocol_test.go -- federation lifecycle and protocol tests
- internal/cli/federation.go (new) -- federation connect/disconnect/list commands
- internal/event/types.go -- federation event constants
- internal/storage/migrations/004_v10.sql -- federation_links table
- internal/api/server.go -- federation endpoints for peer-to-peer communication

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 19 - Story 19.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR110, FR113, FR114]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- FederationProtocol implements secure peer-to-peer mTLS connections between Hive deployments
- Capability metadata exchanged on connect and refreshed periodically (60s default)
- Health monitoring with periodic ping, automatic status transitions, and exponential backoff reconnection
- CLI commands for connect, disconnect, and list with --json support
- Federation links persisted to SQLite for recovery; states: connecting, active, degraded, disconnected

### Change Log

- 2026-04-16: Story 19.1 implemented -- federation protocol with mTLS, capability exchange, and health monitoring

### File List

- internal/federation/protocol.go (modified -- FederationProtocol, FederationLink, mTLS, capability exchange, health monitoring)
- internal/federation/protocol_test.go (new -- federation lifecycle, capability exchange, health check tests)
- internal/cli/federation.go (new -- federation connect/disconnect/list commands)
- internal/event/types.go (modified -- federation event constants)
- internal/api/server.go (modified -- federation peer endpoints)
- internal/storage/migrations/004_v10.sql (reference -- federation_links table)
