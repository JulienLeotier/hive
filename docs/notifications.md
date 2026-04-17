# Notifications

Ops-shaped events are forwarded to email and Slack channels dedicated
to on-call. Different from the generic outbound webhook system — those
are for user-configured integrations; notifications are for paging.

## Triggers

The notifier subscribes to three event types:

- **`task.failed`** — a task run ended with status=failed
- **`cost.alert`** — a budget limit was crossed
- **`agent.isolated`** — a circuit breaker opened, removing an agent
  from the routable pool

Each event type is debounced per channel: once a message fires, the
next 60 seconds of identical events are suppressed. Prevents an
incident from turning into hundreds of duplicate pages. The window is
overridable in code (`notifier.WithDebounce(d time.Duration)`).

## Email (SMTP)

```yaml
notifications:
  email:
    host: smtp.sendgrid.net
    port: 587
    starttls: true           # required for 587
    # smtps_only: true       # set for port 465 implicit TLS
    from: Hive <alerts@yourdomain.io>
    to:
      - oncall@yourdomain.io
      - secondary@yourdomain.io
    username: apikey
    password_env: SMTP_PASSWORD   # read from env, never from YAML
    timeout_secs: 10
```

The password is read from the named env var at boot so credentials
don't live in version control. Dev setups can run against Mailpit or
MailHog with no auth — leave `username`/`password_env` unset.

## Slack (Incoming Webhook)

```yaml
notifications:
  slack:
    webhook_url: https://hooks.slack.com/services/T000/B000/XXXXXXXX
    timeout_secs: 10
```

Messages post as `:warning: *<event_type>* from `<source>` — <payload>`.
Channel routing is encoded in the webhook URL on Slack's side. To
send to multiple channels, configure additional outbound webhooks on
the `/webhooks` dashboard page (those give you per-filter control but
skip the debounce).

## Boot behaviour

Both channels log their state at startup:

```
notify: email channel armed types=[task.failed cost.alert agent.isolated] recipients=2
notify: slack channel disabled — no webhook URL
```

Missing or incomplete config = silent no-op. Hive keeps booting.
