# Optimization

The optimiser analyses historical execution data and either surfaces
recommendations or records approved tunings as events.

## CLI

```bash
hive optimize                 # ad-hoc recommendations
hive optimize --trend         # 7-day window stats (current vs previous)
hive optimize --auto-tune     # suggested settings when trends regress
hive optimize --apply         # emit system.optimization.applied events
hive optimize --json          # machine-readable output
hive optimize --window 14 --trend
```

## What it looks at

- **Slow agents**: p95 duration vs median for the same task type.
- **Idle agents**: < 3 tasks claimed in the last 7 days.
- **Parallel opportunities**: same workflow, many tasks with no `depends_on`.
- **Trend regressions** (`--trend`):
  - failure rate delta
  - average task duration delta

## Auto-tune heuristics

`optimizer.AutoTune` currently proposes:

| Condition                                            | Setting                            | Change      |
|------------------------------------------------------|------------------------------------|-------------|
| Failure rate rose by ≥ 5 percentage points           | `resilience.breaker.threshold`     | lower to 2  |
| Average duration rose by ≥ 50%                       | `resilience.retry.max_wait_seconds`| raise to 5  |

Each `Tuning` includes a human-readable rationale.

## `--apply`

`--apply` emits one `system.optimization.applied` event per suggested tuning
with `{setting, old_value, new_value, rationale}` in the payload. Config-file
rewriting is intentionally left to the caller — a follow-up workflow can
subscribe to those events and patch `hive.yaml` in CI.

## Scheduling

Run the optimiser nightly via cron or a `schedule`-triggered workflow:

```yaml
name: nightly-optimise
trigger:
  type: schedule
  schedule: "*/60 * * * *"
tasks:
  - name: analyse
    type: optimize
```
