#!/usr/bin/env bash
# claude-e2e-brownfield.sh — drive one full BMAD brownfield iteration
# against a real existing GitHub repo.
#
# Proves:
#   - Creating a project with clone_repo works
#   - is_existing flag flips to true automatically
#   - IterationPipeline runs (/bmad-document-project,
#     /bmad-generate-project-context, /bmad-edit-prd, ...)
#   - Devloop picks up the new stories and ships at least one
#
# Env:
#   HIVE_BROWNFIELD_REPO  — repo to clone (default : a tiny throwaway repo)
#   HIVE_E2E_TIMEOUT      — seconds (default 2400, BMAD brownfield is slow)
#   HIVE_E2E_PORT         — HTTP port (default 19888)
#   HIVE_E2E_KEEP         — 1 to keep the temp dir after exit
#
# Exit:
#   0 — project shipped
#   1 — failed or timed out
#   2 — env problem

set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${HIVE_E2E_PORT:-19888}"
TIMEOUT="${HIVE_E2E_TIMEOUT:-2400}"
REPO="${HIVE_BROWNFIELD_REPO:-}"
TMP="$(mktemp -d -t hive-bf-XXXXXX)"
WORKDIR="$TMP/workdir"
STATE="$TMP/state"
BIN="$TMP/hive"
LOG="$TMP/hive.log"

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "✗ missing dep: $1" >&2
		exit 2
	}
}
need go
need claude
need curl
need jq
need gh

# Pick a repo if the user didn't specify one. We default to their own
# first available repo under 5 stars (small, low-stakes). Fallback to a
# well-known tiny demo repo.
if [ -z "$REPO" ]; then
	REPO="$(gh repo list --limit 1 --json nameWithOwner --jq '.[0].nameWithOwner' 2>/dev/null || true)"
fi
if [ -z "$REPO" ]; then
	echo "✗ no brownfield repo available — set HIVE_BROWNFIELD_REPO=owner/name" >&2
	exit 2
fi

cleanup() {
	set +e
	if [ -n "${HIVE_PID:-}" ]; then
		kill "$HIVE_PID" 2>/dev/null
		wait "$HIVE_PID" 2>/dev/null
	fi
	if [ "${HIVE_E2E_KEEP:-0}" != "1" ]; then
		rm -rf "$TMP"
	else
		echo "--- kept: $TMP" >&2
	fi
}
trap cleanup EXIT

echo "→ building hive in $BIN"
(cd "$HERE" && go build -o "$BIN" ./cmd/hive)

echo "→ starting hive on :$PORT (brownfield e2e, clone $REPO)"
HIVE_DEV_AGENT=claude-code \
HIVE_DEVLOOP_INTERVAL=3s \
HIVE_DATA_DIR="$STATE" \
HIVE_PORT="$PORT" \
"$BIN" serve >"$LOG" 2>&1 &
HIVE_PID=$!

# Wait for readiness.
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
		break
	fi
	sleep 0.5
done
if ! curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; then
	echo "✗ hive did not come up — tail of log:" >&2
	tail -40 "$LOG" >&2
	exit 1
fi

API="http://127.0.0.1:$PORT/api/v1"

echo "→ creating brownfield project (clone $REPO into $WORKDIR)"
IDEA='Ajouter une commande CLI `--version` qui imprime la version courante.'
PROJECT=$(curl -fsS -X POST "$API/projects" \
	-H 'content-type: application/json' \
	-d "$(jq -nc --arg idea "$IDEA" --arg wd "$WORKDIR" --arg repo "$REPO" \
		'{name:"e2e-brownfield", idea:$idea, workdir:$wd, clone_repo:$repo}')")
PID=$(echo "$PROJECT" | jq -r '.data.id')
IS_EX=$(echo "$PROJECT" | jq -r '.data.is_existing')
echo "  project id: $PID"
echo "  is_existing: $IS_EX"

if [ "$IS_EX" != "true" ]; then
	echo "✗ clone_repo didn't flip is_existing — brownfield detection broken" >&2
	exit 1
fi

# Verify the repo was actually cloned.
if [ ! -d "$WORKDIR/.git" ]; then
	echo "✗ workdir doesn't look cloned — no .git" >&2
	tail -20 "$LOG" >&2
	exit 1
fi
echo "  ✓ workdir cloned ($(wc -l < <(ls "$WORKDIR")) top-level entries)"

# Intake : driven by scripted PM or user answers. For brownfield we still
# go through the conversation to capture the iteration brief.
echo "→ driving scripted PM intake"
for i in 1 2 3 4 5 6; do
	RESP=$(curl -fsS -X POST "$API/projects/$PID/intake/messages" \
		-H 'content-type: application/json' \
		-d "$(jq -nc --arg c "OK, answer #$i for brownfield iteration" '{content:$c}')")
	D=$(echo "$RESP" | jq -r '.data.done')
	if [ "$D" = "true" ]; then
		break
	fi
done

echo "→ finalising (launches IterationPipeline — bmad-document-project → ...)"
curl -fsS -X POST "$API/projects/$PID/intake/finalize" \
	-H 'content-type: application/json' -d '{}' >/dev/null

echo "→ waiting up to ${TIMEOUT}s for brownfield iteration to ship"
DEADLINE=$(( $(date +%s) + TIMEOUT ))
LAST_STATUS=""
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
	SUMMARY=$(curl -fsS "$API/projects/$PID")
	STATUS=$(echo "$SUMMARY" | jq -r '.data.status')
	COST=$(echo "$SUMMARY" | jq -r '.data.total_cost_usd // 0')
	if [ "$STATUS" != "$LAST_STATUS" ]; then
		echo "  [$(date +%H:%M:%S)] status=$STATUS cost=\$$COST"
		LAST_STATUS="$STATUS"
	fi
	case "$STATUS" in
		shipped)
			echo "✓ brownfield iteration shipped"
			echo "→ files changed vs. origin:"
			(cd "$WORKDIR" && git log --oneline origin/HEAD..HEAD | head -10) || true
			exit 0 ;;
		failed)
			STAGE=$(echo "$SUMMARY" | jq -r '.data.failure_stage // ""')
			ERR=$(echo "$SUMMARY" | jq -r '.data.failure_error // ""')
			echo "✗ brownfield failed at stage=$STAGE"
			echo "  error: $ERR"
			tail -40 "$LOG"
			exit 1 ;;
	esac
	sleep 10
done

echo "✗ timeout after ${TIMEOUT}s — last status=$LAST_STATUS"
tail -40 "$LOG"
exit 1
