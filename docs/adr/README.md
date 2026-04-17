# Architecture Decision Records

Short, dated records of design decisions that aren't obvious from reading
the code. Each ADR captures the context, the decision, and the
consequences.

## Index

- [0001 — Event Bus Consistency Model](0001-event-bus-consistency-model.md)
- [0002 — Federation Trust Model](0002-federation-trust-model.md)
- [0003 — Tenant Isolation Contract](0003-tenant-isolation-contract.md)
- [0004 — SQLite Is Dev-Only in Production Load](0004-sqlite-is-dev-only.md)

## Writing a new ADR

1. Copy `0001-event-bus-consistency-model.md` as a template.
2. Number sequentially.
3. Status: Proposed → Accepted → Superseded (link to the superseding ADR).
4. Keep it short. If you need more than a page, the decision isn't
   settled yet.
