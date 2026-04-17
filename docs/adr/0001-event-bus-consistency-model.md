# ADR 0001 — Event Bus Consistency Model

**Status:** Accepted with follow-up (see Consequences)
**Date:** 2026-04-17
**Context:** Adversarial review A1 / NFR R1

## Context

Hive runs an event bus that persists every event to local storage and, in
multi-node deployments, mirrors events to NATS so other nodes observe them.
Today the persistence path and the NATS fan-out are independent:

1. `Bus.Publish` writes to the local `events` table (authoritative for this
   node).
2. `NATSBus.Publish` publishes to NATS (best-effort fan-out).

If NATS is unreachable, the local write succeeds and the event is stored,
but no remote node sees it. When the broker comes back, nothing replays the
missed window. There is no global causal ordering: node A can publish
`task.started` locally, node B can publish `task.completed` on a different
shard, and a third node may see them in either order depending on NATS
delivery.

Additionally, the checkpoint supervisor runs on every node and scans for
stale tasks. Two nodes can independently decide the same orphaned task is
stale and re-assign it to different agents.

## Decision

We accept this as a known limitation of the v0 event bus. It is adequate
for single-node deployments (the dominant shape today) and tolerable for
small multi-node clusters where a minute of missed events isn't
catastrophic.

For v1 we will adopt one of:

- **NATS JetStream streams** with durable consumers per node. Provides
  replay on reconnect and per-stream ordering. Straightforward migration;
  the bus stays the same shape.
- **Event log in Postgres + CDC stream** to NATS. Gives one source of
  truth (Postgres) and one fan-out channel. Higher operational cost.

Lamport clocks or vector versions are explicitly *not* on the table —
operator cost outweighs the value at our scale.

## Consequences

- Single-node deployments: no behavioural change, fully consistent.
- Multi-node deployments: documented that event order across nodes is
  "eventually consistent, not causally ordered". Workflows must not
  assume global order; they use per-workflow state in the DB which is
  strongly consistent.
- Checkpoint supervisor can double-reassign during partitions. We accept
  this for v0; the second re-assignment is idempotent (the original
  agent's result overwrites the second one's when it returns), so the
  worst case is wasted compute.
- Task tracking: `docs/nfr-assessment-2026-04-17.md` R1, R3 (flaky NATS
  test to investigate).
