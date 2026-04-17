# ADR 0004 — SQLite is Dev-Only in Production Load

**Status:** Accepted
**Date:** 2026-04-17
**Context:** Adversarial review A7 / NFR P1

## Context

Hive ships with two storage backends behind the `storage.Open2` dispatch:
SQLite (default, file-backed) and Postgres. SQLite is dramatically
easier to operate for single-user dev, for the quickstart, and for small
single-node deployments. It's also a real database that handles
migrations, WAL, foreign keys, etc.

SQLite serializes writers. Under a realistic production workload (50+
concurrent HTTP requests + a checkpoint supervisor + a scheduler + an
event bus all writing), contention manifests as `SQLITE_BUSY` errors and
spiking latency. The lock is database-wide, not per-table.

## Decision

Keep SQLite as the zero-config default for dev and small deployments.
Document it as inadequate for production multi-writer workloads. At
startup, if `HIVE_ENV=prod` or `=production` AND `storage=sqlite`, emit
a loud `slog.Warn` line ("SQLite storage in production — concurrent
writers will block on SQLITE_BUSY. Set storage=postgres for multi-writer
workloads.").

## Consequences

- Dev / demo / quickstart remain friction-free. `hive serve` works out
  of the box.
- Production-targeted operators see the warning in their first log
  flush and have a clear instruction. No config change is silent.
- We do NOT refuse to start with SQLite in prod — some users legitimately
  run small single-instance deployments where SQLite is plenty, and
  breaking their startup would be aggressive.
- All code continues to support both backends (parameterized SQL, no
  SQLite-only builtins in query strings except where migration
  branching covers it).
