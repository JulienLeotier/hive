# Research Hive

Parallel multi-source research → synthesis → structured report.

## Flow

```
              ┌── search-academic ──┐
query ──► split                    ├── aggregate ──► report
              └── search-web ──────┘
```

## Setup

```bash
hive add-agent --name academic-searcher --type http --url http://localhost:8080
hive add-agent --name web-searcher      --type http --url http://localhost:8081
hive add-agent --name aggregator        --type http --url http://localhost:8082
hive add-agent --name report-writer     --type http --url http://localhost:8083
hive run --workflow hive.yaml --input query="multi-agent orchestration"
```

## Files

- `hive.yaml` — workflow with parallel searchers → aggregator → report
- `agents/academic-searcher.yaml` — peer-reviewed sources
- `agents/web-searcher.yaml` — web + primary docs
- `agents/aggregator.yaml` — cross-source synthesiser
- `agents/report-writer.yaml` — final report producer

## Pattern

The first two tasks have no `depends_on`, so Hive executes them in parallel. The aggregator
waits on both via `depends_on: [search-academic, search-web]`.
