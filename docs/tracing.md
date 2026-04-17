# Tracing / observability

Hive ships with OpenTelemetry instrumentation wired through the HTTP
server, adapter invocations, and workflow execution. Point it at any
OTLP-compatible backend and you get end-to-end traces for every
request.

## Quick wire-up

### Env var (zero config)

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
./hive serve
```

That's it. The log line `tracing: enabled endpoint=localhost:4317
protocol=grpc sample_ratio=1` confirms the provider is live.

### YAML (`hive.yaml`)

```yaml
observability:
  traces:
    enabled: true
    endpoint: otel-collector.local:4317
    protocol: grpc        # or http
    sample_ratio: 0.1     # 10% — tune for high-volume deployments
    service_version: v0.4.2
```

YAML takes precedence when both are set. Leaving both blank installs
a no-op provider so dev deployments pay nothing.

## What's traced

| Span name | Package | Attributes |
|---|---|---|
| `METHOD /path` | HTTP middleware | http.route, http.status_code, http.method |
| `workflow.run` | `internal/workflow` | workflow.name, workflow.id, workflow.tasks, workflow.allocation |
| `adapter.invoke` | `internal/adapter` | task.id, task.type, task.status |

Outgoing HTTP calls from `adapter.HTTPAdapter` also ride through
`otelhttp.NewTransport`, so the trace context propagates to remote
agents that support W3C trace headers.

## Compatible backends

- **Jaeger** — `docker run -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one`
- **Grafana Tempo** — part of the Grafana stack; point at its
  OTLP receiver
- **Honeycomb** — set `OTEL_EXPORTER_OTLP_ENDPOINT=api.honeycomb.io:443`
  and `OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY`
- **Any OTLP-aware collector** — the OpenTelemetry Collector sits in
  front and routes to multiple backends simultaneously.

## Metrics + logs

Metrics live under `/metrics` (Prometheus scrape format) — unchanged
from before tracing was added. Structured logs use Go's slog with JSON
handler for easy ingestion into Loki / Elasticsearch / Datadog.
