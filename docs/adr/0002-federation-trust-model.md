# ADR 0002 — Federation Trust Model

**Status:** Accepted with v1 follow-up
**Date:** 2026-04-17
**Context:** Adversarial review A3, A4 / NFR S5, S6

## Context

Hive federation lets two hive instances share capabilities so a task that
this hive can't satisfy locally is forwarded to a peer. Today trust is
established out-of-band (the operator runs `hive federation add-peer` with
the peer's URL + mTLS material) and then enforced at the TLS transport
layer on every proxy call.

The current model has three gaps:

1. **Capability disclosure** — `GET /api/v1/capabilities` was previously
   unauthenticated (v0 design to simplify peer discovery). Fixed on
   2026-04-17 to require `system:read`.
2. **Response trust** — the response from a peer is authenticated by mTLS
   (we know *who* sent it) but not *bound to our request*. A compromised
   or buggy peer could return the wrong result. Fixed on 2026-04-17 by
   rejecting responses whose `task_id` doesn't echo our request.
3. **Cert rotation** — mTLS certs are stored per-peer. Rotation today
   requires removing and re-adding the peer. No primitive for graceful
   "try the new cert, keep the old one valid for N hours".

## Decision

For v0 we accept: out-of-band trust establishment, mTLS for transport,
task_id binding for response integrity, and `hop <= MaxHops` to prevent
A→B→A loops. Federation TLS material is envelope-encrypted at rest when
`HIVE_MASTER_KEY` is set (see `internal/federation/crypter.go`).

For v1 we plan a `hive federation rotate <peer>` command that:

- Inserts the new cert alongside the old one in a new `federation_cert`
  table (one row per (peer, cert_id, active_from, expires_at)).
- Both certs are tried on outbound calls for the overlap window.
- Peer acknowledges via a `hive.federation.cert_rotated` event.
- Old cert is revoked after the overlap window.

HMAC signatures on response payloads were considered but rejected — mTLS
already authenticates the transport, and the realistic threat isn't
MITM (which mTLS solves) but a malicious peer (which HMAC doesn't solve
because the peer owns the HMAC key).

## Consequences

- Operators must currently rotate certs with downtime. Acceptable for
  v0, where federation is used in controlled enterprise deployments.
- The `federation_cert` migration will be a breaking change to the store
  schema — included in the v1 migration plan.
- `docs/architecture-adversarial-review-2026-04-17.md` A3/A4 are tracked
  in this ADR.
