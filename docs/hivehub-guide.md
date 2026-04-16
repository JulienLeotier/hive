# HiveHub Guide

HiveHub is a community registry of ready-made hive configurations (workflow
YAML + agent personas + README). Discover, install, and publish templates
directly from the CLI.

## Search

```bash
hive search                 # list everything
hive search code-review     # keyword search across name/description/category
hive search --registry https://example.org/hivehub.json review
```

## Install

```bash
hive install code-review-hive
hive install code-review-hive --dest my-review   # override target directory
```

Templates are published as JSON manifests (`[{ path, content }, …]`).
The installer materialises every entry and refuses paths containing `..`.

## Publish

```bash
cd my-hive
hive publish . \
  --name code-review-hive \
  --description "Two-stage PR review pipeline" \
  --version 0.1.0 \
  --author julien \
  --category review
# → writes code-review-hive-0.1.0.json
```

Submit the resulting JSON file as a PR against the HiveHub index repository
(default: `github.com/JulienLeotier/hivehub`).

## Template authoring tips

- Put a `README.md` at the root — the installer copies it too.
- Skip `.git/`, `node_modules/`, `dist/`, `.svelte-kit/`, `.hive/`, and
  SQLite `hive.db*` files automatically (see `internal/hivehub/fs.go`).
- Keep agent personas under `agents/` for consistency with `hive init`.
