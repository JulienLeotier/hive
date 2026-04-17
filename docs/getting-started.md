# Getting started with Hive

This walks you from zero to "my first workflow fired" in about five
minutes. You need Docker and a terminal; everything else is provided by
the compose stack.

## 1. Clone and boot

```bash
git clone https://github.com/JulienLeotier/hive.git
cd hive
docker compose up -d
```

That's it. Compose builds the binary, opens port `8233`, and puts an
SQLite volume on disk so your data survives restarts. Check the logs:

```bash
docker compose logs -f hive
```

You should see the 14 migrations applying, then
`hive server started addr=:8233`.

## 2. Run the setup wizard

Open **<http://localhost:8233>**. The layout redirects you to `/setup`
because there's no admin yet. Pick:

- **Admin identity** — any string. An email is idiomatic. It becomes
  the owner of the first API key.
- **Tenant** — `default` is fine.

Click *Create admin and bootstrap API key*. You get a one-time
`hive_...` key. Copy it into your password manager — the dashboard
auto-persists it in `localStorage`, but if you ever clear browser data
you'll need to paste it on `/login`.

## 3. Register an agent

Go to **Agents**. The built-in Claude Code adapter works without any
external service: use `type=claude-code` and `url=local` (the type
dispatch handles the rest). For a remote HTTP adapter:

- `type=http`
- `url=https://your-agent.example.com` — the adapter must answer
  `GET /health` and `GET /declare` per the Adapter Protocol.

If you leave **cap** at `0`, the agent uses the global concurrency
limit. Set a per-agent cap to throttle a flaky adapter without
starving the rest of the fleet.

## 4. Fire a workflow

Go to **Workflows** → *+ New workflow*. The editor pre-fills a starter
YAML. The minimal shape:

```yaml
name: my-first
tasks:
  - name: review
    type: code-review  # whatever task type your agent declares
```

Click *Create workflow*, then the *▶* icon on the row. The workflow
appears in **Tasks** almost immediately; click the task row to see
input/output/timing. For scheduled or webhook-triggered workflows,
add a `trigger:` block — see the reference below.

## 5. Wire alerts (optional)

Edit `hive.yaml` (in your mounted volume) to add email or Slack
notifications:

```yaml
notifications:
  email:
    host: smtp.sendgrid.net
    port: 587
    starttls: true
    from: Hive <alerts@yourdomain.io>
    to: ["oncall@yourdomain.io"]
    username: apikey
    password_env: SMTP_PASSWORD     # read from env, not the YAML
  slack:
    webhook_url: https://hooks.slack.com/services/...
```

Restart: `docker compose restart hive`. The boot log tells you whether
each channel armed. Triggers: `task.failed`, `cost.alert`,
`agent.isolated` (circuit breaker opens). Per-type debounce of 60s
prevents storms.

## 6. Backups

SQLite deployments:

```bash
docker compose exec hive /usr/local/bin/hive backup /data/backup.tar.gz
docker cp hive-hive-1:/data/backup.tar.gz ./hive-backup.tar.gz
```

Postgres deployments: use `pg_dump` on the `postgres` container — the
native tooling is richer (PITR, WAL shipping) than anything Hive can
wrap.

## Reference

- **API** — `docs/api-reference.md`
- **Workflow YAML grammar** — `docs/architecture-overview.md`
- **Adapter protocol** — `docs/adapter-guide.md`
- **Multi-node** — `docs/multi-node.md` (Postgres + NATS)
- **Federation** — `docs/federation.md` (mTLS between hives)

## Going to production

- Set `HIVE_MASTER_KEY` so sensitive writes (federation certs, webhook
  URLs) get encrypted at rest.
- Enable the `postgres` compose profile for multi-writer workloads.
- Add the `nats` profile if you're running >1 node.
- Put TLS in front of port 8233 (your ingress, or the built-in TLS
  block in `hive.yaml`).
- Mint a bootstrap API key with `hive api-key create <name>` from the
  container, then revoke the setup-wizard key.
