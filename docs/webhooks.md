# Webhooks

Hive can forward events to external HTTP endpoints (Slack, GitHub, or a generic
receiver). Every dispatch retries up to 3 times with exponential backoff; a
failing webhook never blocks event delivery to other subscribers.

## Registering a webhook

```bash
hive webhook add \
  --name slack-alerts \
  --url https://hooks.slack.com/services/… \
  --type slack \
  --events task.failed,agent.isolated
hive webhook list
hive webhook remove slack-alerts
```

`--events` is a comma-separated list of event-type prefixes. Use `*` to match
everything.

## Formats

### `--type slack`

```json
{"text": "[Hive] task.failed from system: {...}"}
```

### `--type github`

Produces a `repository_dispatch`-compatible body:

```json
{
  "event_type": "task.failed",
  "client_payload": {
    "source": "system",
    "payload": "…raw event payload…",
    "pr_number": 42,
    "repository": "acme/widgets"
  }
}
```

`pr_number`, `issue_number`, `repository`, and `commit_sha` are auto-extracted
from the event payload (Story 11.4). Recognised JSON keys: `pr_number`,
`pull_request_number`, `pr`, `issue_number`, `issue`, `repository`, `repo`,
`sha`, `commit_sha`, `commit`. GitHub URLs of the form `…/pull/123` or
`…/issues/456` are also parsed.

### `--type generic` (default)

```json
{
  "id": 123,
  "type": "task.failed",
  "source": "system",
  "payload": "…",
  "created_at": "2026-04-16T20:10:00Z"
}
```

## Delivery guarantees

- Dispatch is persisted as an event subscription — restarts don't drop pending retries.
- Failure response bodies are logged (truncated to 2 KB) for debugging.
- A webhook that returns non-2xx three times in a row is logged but stays enabled.
