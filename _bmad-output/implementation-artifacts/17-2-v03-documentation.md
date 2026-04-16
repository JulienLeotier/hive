# Story 17.2: v0.3 Documentation

Status: done

## Story

As a user,
I want documentation for all v0.3 features,
so that I can use adapters, HiveHub, NATS, and cost management.

## Acceptance Criteria

1. **Given** all v0.3 features are implemented
   **When** docs are updated
   **Then** new docs are created: `adapters-guide.md` covering CrewAI, LangChain, AutoGen, OpenAI adapters

2. **Given** HiveHub is implemented
   **When** docs are updated
   **Then** `hivehub-guide.md` covers publishing, searching, and installing templates

3. **Given** NATS support is implemented
   **When** docs are updated
   **Then** `nats-setup.md` covers NATS configuration, connection management, and multi-node setup

4. **Given** cost management is implemented
   **When** docs are updated
   **Then** `cost-management.md` covers cost tracking, budget alerts, and cost reporting

5. **Given** all documentation is written
   **When** a user reads the guides
   **Then** each guide includes: overview, prerequisites, step-by-step setup, configuration reference, examples, troubleshooting

## Tasks / Subtasks

- [x] Task 1: Adapters guide (AC: #1)
  - [x] Create `docs/adapters-guide.md`
  - [x] Document CrewAI adapter: prerequisites (Python), registration command, how tasks are invoked
  - [x] Document LangChain adapter: LangServe setup, registration command, chain invocation
  - [x] Document AutoGen adapter: HTTP endpoint setup, registration command
  - [x] Document OpenAI Assistants adapter: API key setup, assistant ID, thread/run lifecycle
  - [x] Include comparison table of adapter capabilities and limitations
- [x] Task 2: HiveHub guide (AC: #2)
  - [x] Create `docs/hivehub-guide.md`
  - [x] Document `hive search` command with example output
  - [x] Document `hive install` command with conflict resolution workflow
  - [x] Document `hive publish` command with packaging requirements
  - [x] Include template structure specification
- [x] Task 3: NATS setup guide (AC: #3)
  - [x] Create `docs/nats-setup.md`
  - [x] Document NATS server installation and startup
  - [x] Document Hive configuration: `event_bus: nats`, `nats_url`
  - [x] Document connection management: reconnection behavior, buffering, status monitoring
  - [x] Include multi-node deployment architecture diagram (text-based)
- [x] Task 4: Cost management guide (AC: #4)
  - [x] Create `docs/cost-management.md`
  - [x] Document cost tracking: how costs are recorded, `hive status --costs`
  - [x] Document budget alerts: `hive budget set`, `hive budget list`, `hive budget remove`
  - [x] Document webhook integration for cost alerts
  - [x] Include examples with realistic cost scenarios
- [x] Task 5: README and quickstart updates (AC: #5)
  - [x] Update main README with v0.3 feature overview
  - [x] Add v0.3 features to feature comparison table
  - [x] Update quickstart with links to new guides

## Dev Notes

### Architecture Compliance

- Documentation follows existing docs/ directory structure
- Markdown format consistent with existing documentation style
- Each guide is self-contained with prerequisites, setup, configuration, examples
- Code examples use actual CLI commands and configuration syntax from the implementation
- Troubleshooting sections address common failure modes identified during implementation

### Key Design Decisions

- Each v0.3 feature area gets its own standalone guide rather than a single monolithic document -- easier to find and maintain
- The adapters guide includes a comparison table so users can quickly assess which adapter fits their framework
- NATS setup guide includes a text-based architecture diagram since the project is CLI-first
- Cost management guide includes realistic cost scenarios to help users set appropriate budget limits
- All guides reference the relevant `hive.yaml` configuration fields with their default values

### Documentation Structure

```
docs/
  adapters-guide.md    -- CrewAI, LangChain, AutoGen, OpenAI Assistants
  hivehub-guide.md     -- Template publishing, search, installation
  nats-setup.md        -- NATS configuration and multi-node deployment
  cost-management.md   -- Cost tracking, budget alerts, reporting
```

### Integration Points

- `docs/adapters-guide.md` -- references `internal/adapter/` implementations
- `docs/hivehub-guide.md` -- references `internal/hivehub/registry.go`
- `docs/nats-setup.md` -- references `internal/event/nats_bus.go` and config fields
- `docs/cost-management.md` -- references `internal/cost/tracker.go` and CLI commands
- `README.md` -- updated with v0.3 feature overview

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 17.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- adapters-guide.md: CrewAI (subprocess), LangChain (LangServe HTTP), AutoGen (HTTP), OpenAI (Assistants API v2) with comparison table
- hivehub-guide.md: search, install, publish workflows with template structure spec
- nats-setup.md: server setup, Hive config, connection management, multi-node architecture
- cost-management.md: cost tracking, budget alerts, webhook integration, realistic scenarios
- README updated with v0.3 feature overview and links to new guides

### Change Log

- 2026-04-16: Story 17.2 implemented -- v0.3 documentation for adapters, HiveHub, NATS, and cost management

### File List

- docs/adapters-guide.md (new)
- docs/hivehub-guide.md (new)
- docs/nats-setup.md (new)
- docs/cost-management.md (new)
- README.md (modified -- v0.3 feature overview)
