# translator agent

Translates text between languages via OpenAI's chat completions. Input:

```json
{
  "text": "…",
  "target_lang": "French",
  "source_lang": "English"
}
```

`source_lang` is optional — the model auto-detects when omitted.

## Run

```bash
export OPENAI_API_KEY=sk-...
go run .                 # listens on :9102
```

## Register

```bash
hive add-agent --name translator --type http --url http://localhost:9102
```

## Demo workflow

```bash
hive run examples/agents/translator/workflow.yaml
```

Fires two parallel translations (French + Japanese) of the same source
sentence. Open `/workflows/<id>` in the dashboard to see both tasks
complete side-by-side.

## Capabilities

- Task types: `translate`
- Cost per run: 0.002
- Model: `gpt-4o-mini`, temperature 0 for deterministic output
