#!/usr/bin/env bash
# claude-e2e.sh — drive one full BMAD build against the real `claude`
# CLI end-to-end.
#
# What this script proves:
#   - Creating a project through the API works
#   - The PM intake conversation completes (scripted agent, fast)
#   - The Architect produces an epic/story/AC tree (scripted, fast)
#   - The Dev loop invokes the real `claude` CLI and Claude Code writes
#     real files in the workdir, then the Reviewer evaluates the ACs
#   - Stories eventually flip to done and the project flips to shipped
#
# What this script does NOT do:
#   - Validate conversation quality or PRD quality (that's human judgment)
#   - Test parallel projects (BMAD is single-project-at-a-time)
#
# Env:
#   HIVE_E2E_TIMEOUT  — max seconds to wait for the build (default 1200)
#   HIVE_E2E_PORT     — port to bind hive to (default 18233, random-ish)
#   HIVE_E2E_KEEP     — 1 to keep the temp hive binary + workdir after exit
#
# Exit codes:
#   0 — project shipped
#   1 — build failed or timed out
#   2 — env problem (no claude, no go, etc.)

set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${HIVE_E2E_PORT:-18233}"
TIMEOUT="${HIVE_E2E_TIMEOUT:-1200}"
TMP="$(mktemp -d -t hive-e2e-XXXXXX)"
WORKDIR="$TMP/workdir"
STATE="$TMP/state"
BIN="$TMP/hive"
LOG="$TMP/hive.log"

mkdir -p "$WORKDIR" "$STATE"

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

echo "→ building hive in $BIN"
(cd "$HERE" && go build -o "$BIN" ./cmd/hive)

echo "→ starting hive on :$PORT with real claude dev agent"
HIVE_DEV_AGENT=claude-code \
HIVE_INTAKE_AGENT=scripted \
HIVE_ARCHITECT=scripted \
HIVE_DEVLOOP_INTERVAL=3s \
HIVE_DATA_DIR="$STATE" \
HIVE_PORT="$PORT" \
"$BIN" serve >"$LOG" 2>&1 &
HIVE_PID=$!

# Wait for the server to accept connections.
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:$PORT/api/v1/setup/status" >/dev/null 2>&1; then
		break
	fi
	sleep 0.5
done
if ! curl -fsS "http://127.0.0.1:$PORT/api/v1/setup/status" >/dev/null 2>&1; then
	echo "✗ hive did not come up — tail of log:" >&2
	tail -40 "$LOG" >&2
	exit 1
fi

API="http://127.0.0.1:$PORT/api/v1"

echo "→ bootstrapping admin"
BOOTSTRAP=$(curl -fsS -X POST "$API/setup/bootstrap" \
	-H 'content-type: application/json' \
	-d '{"subject":"e2e@hive.local","tenant_id":"default"}')
KEY=$(echo "$BOOTSTRAP" | jq -r '.data.api_key')
[ "$KEY" != "null" ] && [ -n "$KEY" ] || {
	echo "✗ bootstrap failed: $BOOTSTRAP" >&2
	exit 1
}
AUTH=(-H "x-api-key: $KEY")

echo "→ creating project (workdir=$WORKDIR)"
IDEA='a tiny cli that prints a random compliment on each run, in Go'
PROJECT=$(curl -fsS -X POST "$API/projects" "${AUTH[@]}" \
	-H 'content-type: application/json' \
	-d "$(jq -nc --arg idea "$IDEA" --arg wd "$WORKDIR" \
		'{name:"e2e-compliment", idea:$idea, workdir:$wd}')")
PID=$(echo "$PROJECT" | jq -r '.data.id')
echo "  project id: $PID"

echo "→ driving the scripted PM intake to completion"
# Scripted PM asks up to 5 questions. Feed short canned answers until done=true.
DONE=false
for i in 1 2 3 4 5 6; do
	RESP=$(curl -fsS -X POST "$API/projects/$PID/intake/messages" "${AUTH[@]}" \
		-H 'content-type: application/json' \
		-d "$(jq -nc --arg c "OK, answer #$i" '{content:$c}')")
	D=$(echo "$RESP" | jq -r '.data.done')
	if [ "$D" = "true" ]; then
		DONE=true
		break
	fi
done
[ "$DONE" = "true" ] || {
	echo "✗ intake never reached done=true" >&2
	exit 1
}

echo "→ finalising PRD (async architect kicks in)"
curl -fsS -X POST "$API/projects/$PID/intake/finalize" "${AUTH[@]}" \
	-H 'content-type: application/json' -d '{}' >/dev/null

echo "→ waiting up to ${TIMEOUT}s for project to ship"
DEADLINE=$(( $(date +%s) + TIMEOUT ))
LAST_STATUS=""
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
	SUMMARY=$(curl -fsS "$API/projects/$PID" "${AUTH[@]}")
	STATUS=$(echo "$SUMMARY" | jq -r '.data.status')
	if [ "$STATUS" != "$LAST_STATUS" ]; then
		echo "  [$(date +%H:%M:%S)] project.status = $STATUS"
		LAST_STATUS="$STATUS"
	fi
	case "$STATUS" in
		shipped)
			echo "✓ project shipped"
			echo "→ files produced in $WORKDIR:"
			find "$WORKDIR" -type f ! -path '*/.git/*' | head -20
			exit 0 ;;
		failed)
			echo "✗ project failed — tail of log:"
			tail -60 "$LOG"
			exit 1 ;;
	esac
	# Print blocked stories as they happen so we notice a wedged loop.
	BLOCKED=$(echo "$SUMMARY" | jq -r '.data.epics[].stories[]? | select(.status=="blocked") | .title' | head -5)
	if [ -n "$BLOCKED" ]; then
		echo "  blocked stories: $BLOCKED"
	fi
	sleep 5
done

echo "✗ timeout after ${TIMEOUT}s — last status=$LAST_STATUS"
echo "--- tail of hive log ---"
tail -40 "$LOG"
exit 1
