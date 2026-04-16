# Trust Configuration -- Graduated Autonomy

Hive implements graduated autonomy where agents earn increasing trust through demonstrated competence. The trust engine is in `internal/trust/engine.go`.

## Trust Levels

Agents progress through 4 trust levels in ascending order:

| Level | Constant | Rank | Behavior |
|---|---|---|---|
| supervised | `LevelSupervised` | 0 | Agent requires approval for all actions |
| guided | `LevelGuided` | 1 | Agent acts with oversight, some autonomy |
| autonomous | `LevelAutonomous` | 2 | Agent acts independently for routine tasks |
| trusted | `LevelTrusted` | 3 | Agent has full autonomy within constraints |

New agents start at trust level `scripted` (set during registration in `agent.Manager.Register()`).

## Promotion Thresholds

Promotion is based on two metrics: **total completed tasks** and **error rate** (failures / total). Default thresholds from `trust.DefaultThresholds()`:

```go
Thresholds{
    GuidedAfterTasks:     50,   // promote to guided after 50 tasks
    GuidedMaxErrorRate:   0.10, // with error rate <= 10%

    AutonomousAfterTasks: 200,  // promote to autonomous after 200 tasks
    AutonomousMaxError:   0.05, // with error rate <= 5%

    TrustedAfterTasks:    500,  // promote to trusted after 500 tasks
    TrustedMaxError:      0.02, // with error rate <= 2%
}
```

### Evaluation Logic

The `Engine.Evaluate()` method:

1. Queries agent stats: total tasks, successes, failures, error rate
2. Calls `calculateTargetLevel()` to determine the highest level the agent qualifies for
3. Compares target level rank against current level rank
4. **Only promotes, never auto-demotes** -- if the target level rank is lower or equal, no change occurs
5. Updates the `agents` table and records the change in `trust_history`

```go
func (e *Engine) calculateTargetLevel(stats AgentStats) string {
    if stats.TotalTasks >= t.TrustedAfterTasks && stats.ErrorRate <= t.TrustedMaxError {
        return LevelTrusted
    }
    if stats.TotalTasks >= t.AutonomousAfterTasks && stats.ErrorRate <= t.AutonomousMaxError {
        return LevelAutonomous
    }
    if stats.TotalTasks >= t.GuidedAfterTasks && stats.ErrorRate <= t.GuidedMaxErrorRate {
        return LevelGuided
    }
    return LevelSupervised
}
```

Evaluation checks the highest level first (trusted), then falls through. This means an agent can skip levels if it qualifies directly.

## Manual Override

Administrators can manually set any trust level:

```go
engine.SetManual(ctx, agentID, "autonomous")
```

This records a `manual_override` reason in the trust history. Manual overrides can promote or demote.

## Trust History

Every level change is recorded in the `trust_history` table:

| Column | Description |
|---|---|
| `id` | ULID |
| `agent_id` | Agent being changed |
| `old_level` | Previous trust level |
| `new_level` | New trust level |
| `reason` | `auto_promotion` or `manual_override` |
| `criteria` | Human-readable metrics (e.g., `tasks=200, error_rate=3.50%`) |
| `created_at` | Timestamp |

## Custom Thresholds

Create an `Engine` with custom thresholds:

```go
engine := trust.NewEngine(db, trust.Thresholds{
    GuidedAfterTasks:     20,
    GuidedMaxErrorRate:   0.15,
    AutonomousAfterTasks: 100,
    AutonomousMaxError:   0.08,
    TrustedAfterTasks:    300,
    TrustedMaxError:      0.03,
})
```

Threshold fields use `yaml` struct tags (`guided_after_tasks`, `guided_max_error_rate`, etc.) for YAML configuration.

## Agent Stats

The `Engine.GetStats()` method returns:

```go
type AgentStats struct {
    TotalTasks   int
    Successes    int
    Failures     int
    ErrorRate    float64
    CurrentLevel string
}
```

Stats are computed from the `tasks` table by counting rows where `agent_id` matches and `status` is `completed` or `failed`.

## Integration with Task Routing

Trust levels can inform task routing decisions. Higher-trust agents can be preferred for sensitive task types, while supervised agents may require human approval gates.

## Dashboard Visibility

The Agents page in the dashboard (`/agents`) displays each agent's current trust level in the Trust column. Trust transitions emit events visible in the Events timeline.

## Design Principles

1. **Trust is earned** -- agents start low and prove reliability through track record
2. **No auto-demotion** -- a spike in errors does not automatically downgrade; this prevents oscillation. Manual demotion is available for administrators
3. **Transparent history** -- every promotion and override is auditable in `trust_history`
4. **Gradual progression** -- each level requires significantly more completed tasks than the previous
