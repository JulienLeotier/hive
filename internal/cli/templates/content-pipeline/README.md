# Content Pipeline Hive

A four-stage content production pipeline: write → edit → SEO → publish.

## Flow

```
topic brief ──► writer ──► editor ──► seo ──► publisher
```

## Setup

```bash
hive add-agent --name writer    --type http --url http://localhost:8080
hive add-agent --name editor    --type http --url http://localhost:8081
hive add-agent --name seo       --type http --url http://localhost:8082
hive add-agent --name publisher --type http --url http://localhost:8083
hive run --workflow hive.yaml --input topic="why Hive"
```

## Files

- `hive.yaml` — workflow definition (write → edit → seo → publish)
- `agents/writer.yaml` — drafter persona
- `agents/editor.yaml` — line-editor persona
- `agents/seo.yaml` — SEO optimiser persona
- `agents/publisher.yaml` — CMS publisher persona

## Input variables

- `topic` — headline / concept to write about
- Substituted wherever `{{topic}}` appears in `hive.yaml`
