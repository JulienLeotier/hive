# summarizer agent

Condenses text via OpenAI's chat completions. Input:

```json
{
  "text": "…long text…",
  "max_words": 80
}
```

Output:

```json
{
  "summary": "…"
}
```

## Run

```bash
export OPENAI_API_KEY=sk-...
go run .                 # listens on :9101
```

## Register

```bash
hive add-agent --name summarizer --type http --url http://localhost:9101
```

## Demo workflow

```bash
hive run examples/agents/summarizer/workflow.yaml
```

## Capabilities

- Task types: `summarize`
- Cost per run: 0.002 (advertised; Hive records this against the cost tracker)
- Model: `gpt-4o-mini` (override in `main.go`)
