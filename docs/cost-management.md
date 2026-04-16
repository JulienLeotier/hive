# Cost Management

Hive tracks per-agent spend and raises alerts when daily budgets are breached.

## Recording spend

Agents declare `cost_per_run` in their capabilities. The orchestrator calls
`cost.Tracker.Record(ctx, agentID, agentName, workflowID, taskID, cost)` after
each completed task, writing a row to the `costs` table.

## Inspecting costs

```bash
hive status --costs         # per-agent spend + any active breaches
hive budget list            # configured daily limits with utilisation
```

The `/api/v1/costs` endpoint exposes the same rollup as JSON, and the `/costs`
dashboard route renders stat cards + breach highlights.

## Budgets

```bash
hive budget set reviewer 5.00      # $5/day cap
hive budget list                   # show today's spend vs limit
hive budget remove reviewer        # clear the cap
```

`cost.Tracker.EvaluateAlerts(ctx)` returns every configured budget with the
current day's spend; when an event bus is attached (`.WithBus(bus.PublishErr)`)
a breached budget also emits `cost.alert` so webhooks and the event timeline
can surface it in real time.

## Dashboard

The `/costs` page shows:

- Total spend across all agents
- Agents tracked + breach count stat cards
- Per-agent utilisation bar (green → amber → red)
- All-time spend + average cost per task
