# Federated marketplace

Hive's marketplace is **peer-to-peer**: each hive publishes its own
agent catalog, and operators on any federated hive can browse the
aggregated view. No central marketplace server, no account to create,
no brokered payment — just the federation mesh you already set up for
task routing.

## How it works

1. You connect to a peer with `hive federation add <name> <url>` (or
   the `/federation` dashboard page). mTLS certificates are exchanged
   and stored encrypted at rest when `HIVE_MASTER_KEY` is set.
2. On your hive, you flip the *publish* checkbox on any agent you're
   willing to share. This sets `agents.publishable = 1` in the DB.
3. Peers hit `GET /api/v1/federation/catalog` on your hive and see
   only the publishable + healthy agents, with a narrow shape
   (`name`, `type`, `version`, `task_types`, `cost_per_run`). Adapter
   config stays private — no leaking internal URLs or API keys.
4. The `/marketplace` dashboard page on a peer calls
   `GET /api/v1/marketplace`, which fans out to every active
   federation link in parallel and aggregates the results.

## Consuming a peer's agent

The marketplace is currently **discovery-only** — the UI tells you
*what* is out there. Routing to a peer agent happens through the
existing `task.Router.WithFederation` mechanism: declare a task type
that no local agent handles, and the federation resolver will proxy
to the first peer that advertises it.

For explicit cross-hive invocation, use the existing federation proxy
endpoints (see [federation.md](federation.md)).

## Status codes

Each peer slice in the marketplace response carries one of:

- `ok` — catalog fetched and parsed successfully
- `unreachable` — transport failure (DNS, TCP, TLS); peer is down or
  the link is misconfigured
- `error` — peer answered but returned a non-2xx or unparseable body

Unreachable peers stay visible in the dashboard rather than silently
disappearing so you notice a degraded federation.

## Opt-out

Remove `publishable` or flip `enabled=0` to hide an agent from peers
without deleting it. The agent keeps serving local workflows
normally.
