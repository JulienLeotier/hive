# Story 23.3: v1.0 Documentation & Launch

Status: done

## Story

As a user,
I want complete documentation for the v1.0 platform,
so that I can deploy and operate Hive in production.

## Acceptance Criteria

1. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** documentation covers market allocation configuration and usage

2. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** documentation covers federation setup between Hive deployments

3. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** documentation covers the optimization guide (analyze, recommend, apply)

4. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** documentation covers enterprise deployment (SSO, RBAC, audit)

5. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** documentation covers multi-node setup (PostgreSQL + NATS cluster)

6. **Given** all v1.0 features implemented
   **When** docs are finalized
   **Then** API reference is complete and README updated with v1.0 feature overview

## Tasks / Subtasks

- [x] Task 1: Market allocation documentation (AC: #1)
  - [x] Document allocation strategies: market, round-robin, capability-match
  - [x] Configuration examples in hive.yaml
  - [x] Token economy explanation with balance, bidding, and earning
  - [x] `hive agent stats` command reference
  - [x] Troubleshooting: no bids, insufficient balance
- [x] Task 2: Federation documentation (AC: #2)
  - [x] Federation setup guide: certificate generation, connection, capability sharing
  - [x] Configuration reference for federation.share
  - [x] Cross-hive routing explanation with diagrams
  - [x] Security considerations: mTLS, capability whitelist, data boundaries
  - [x] CLI reference: federation connect, disconnect, list
- [x] Task 3: Optimization documentation (AC: #3)
  - [x] Optimization workflow: analyze -> review -> apply -> monitor
  - [x] Pattern types: slow agents, underutilization, parallelization, failure patterns
  - [x] Auto-tuning guide: apply, status, rollback
  - [x] Configuration reference for optimization thresholds
- [x] Task 4: Enterprise deployment documentation (AC: #4)
  - [x] OIDC SSO setup guide: provider configuration, hive.yaml settings
  - [x] RBAC configuration: built-in roles, custom roles, group mapping
  - [x] Audit log export guide: formats, filtering, compliance
  - [x] Multi-tenant setup: enabling, tenant management, isolation guarantees
- [x] Task 5: Multi-node setup documentation (AC: #5)
  - [x] Production deployment architecture: PostgreSQL + NATS + multiple Hive nodes
  - [x] PostgreSQL setup and configuration
  - [x] NATS cluster setup
  - [x] Node-aware routing configuration
  - [x] Monitoring and troubleshooting multi-node deployments
- [x] Task 6: API reference and README (AC: #6)
  - [x] Complete API reference for all endpoints (agents, tasks, workflows, federation, metrics, audit)
  - [x] Authentication section: API keys and OIDC
  - [x] Request/response examples for each endpoint
  - [x] Update README.md with v1.0 feature overview and architecture diagram
  - [x] Update quickstart.md with v1.0 getting started flow

## Dev Notes

### Architecture Compliance

- Documentation follows the existing docs structure established in v0.1 and v0.2
- All CLI commands documented with flags, examples, and expected output
- Configuration examples use the same hive.yaml format used throughout the project
- API reference includes curl examples for each endpoint
- Documentation is in Markdown format for GitHub rendering

### Key Design Decisions

- Documentation organized by feature area, not by version -- users don't care when a feature was added
- Each guide includes: overview, setup, configuration, usage, troubleshooting
- API reference is comprehensive but not auto-generated -- hand-written for clarity
- README serves as entry point with links to detailed guides
- Enterprise features documented separately from core features for audience targeting

### Documentation Structure

```
docs/
  market-allocation.md
  federation-guide.md
  optimization-guide.md
  enterprise-deployment.md
  multi-node-setup.md
  api-reference.md
  quickstart.md (updated)
README.md (updated)
```

### Integration Points

- docs/market-allocation.md (new -- market allocation and token economy guide)
- docs/federation-guide.md (new -- federation setup and cross-hive routing)
- docs/optimization-guide.md (new -- pattern analysis and auto-tuning)
- docs/enterprise-deployment.md (new -- SSO, RBAC, audit, multi-tenant)
- docs/multi-node-setup.md (new -- PostgreSQL + NATS cluster deployment)
- docs/api-reference.md (new -- complete API reference)
- docs/quickstart.md (modified -- updated for v1.0)
- README.md (modified -- v1.0 feature overview)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 23 - Story 23.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Six documentation areas covering all v1.0 features
- Market allocation guide: strategies, token economy, agent stats CLI
- Federation guide: mTLS setup, capability sharing, cross-hive routing with security considerations
- Optimization guide: analyze/review/apply/monitor workflow with auto-tuning
- Enterprise deployment: OIDC SSO, RBAC roles, audit export, multi-tenant isolation
- Multi-node setup: PostgreSQL + NATS cluster architecture with monitoring guidance
- API reference with curl examples for all endpoints; README updated with v1.0 overview

### Change Log

- 2026-04-16: Story 23.3 implemented -- complete v1.0 documentation and README update

### File List

- docs/market-allocation.md (new)
- docs/federation-guide.md (new)
- docs/optimization-guide.md (new)
- docs/enterprise-deployment.md (new)
- docs/multi-node-setup.md (new)
- docs/api-reference.md (new)
- docs/quickstart.md (modified -- v1.0 updates)
- README.md (modified -- v1.0 feature overview and architecture diagram)
