# NATS Setup

For multi-node deployments, swap the default SQLite-backed event bus for a
NATS-backed one so events flow across processes.

## Requirements

- NATS server reachable by every Hive node (NATS 2.10+ recommended)
- Same topic subject prefix across all nodes (default `hive.events`)

## Quickstart

```bash
docker run --rm -p 4222:4222 nats:2.10
```

Then in `hive.yaml`:

```yaml
event_bus: nats
nats_url: nats://localhost:4222
```

Under the hood, `event.NewNATSBus(conn, cfg)` wraps a `NATSConn` implementation
(`*nats.Conn` in production, a fake in tests). Events are published to
`hive.events.<eventType>` and fanned out to local subscribers.

## History

NATS itself is not a durable log — `NATSBus` keeps a bounded in-memory ring
(default 1000 events) so `Query()` still works for the dashboard timeline
without requiring JetStream. If you need durable multi-day history, enable
JetStream on the NATS cluster and back `Query()` with a stream consumer.

## Subscription wildcards

The bus subscribes to `hive.events.>`, so any new event type is delivered
automatically to every process.

## Troubleshooting

- **Events stop arriving**: check the NATS cluster's slow-consumer warnings
  and make sure max-payload matches the largest event you publish.
- **Ordering**: NATS guarantees per-subject ordering — for strict ordering
  across types, subscribe per subject rather than the `>` wildcard.
