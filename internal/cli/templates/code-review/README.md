# Code Review Hive

A two-stage PR review pipeline.

## Flow

```
PR diff ──► reviewer ──► summarizer ──► markdown summary
```

## Setup

```bash
hive add-agent --name reviewer  --type http --url http://localhost:8080
hive add-agent --name summarizer --type http --url http://localhost:8081
hive run --workflow hive.yaml
```

Each agent must expose `/declare`, `/task`, `/health`, `/checkpoint`.

## Files

- `hive.yaml` — workflow definition (review → summarize)
- `agents/reviewer.yaml` — reviewer persona
- `agents/summarizer.yaml` — summarizer persona

## Customisation

- Swap `type: http` for `type: claude-code` to register local Claude Code projects
- Add a `condition` to `summarize` to skip the summary when the review has no findings:

  ```yaml
  - name: summarize
    condition: 'upstream.review.finding_count > 0'
  ```
