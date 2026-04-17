#!/usr/bin/env bash
# fake-claude.sh — un stub de la CLI `claude` pour la CI + les smoke
# tests locaux. Il répond dans le même format que `claude --print
# --output-format json` (envelope {result, is_error, usage,
# total_cost_usd}) et se contente d'écrire un artefact plausible
# quand on lui passe une slash-command BMAD.
#
# Ajoute ce fichier au PATH devant `claude` pour simuler BMAD sans
# consommer de tokens :
#   export PATH="$(dirname "$PWD/scripts/fake-claude.sh"):$PATH"
#   ln -sf "$PWD/scripts/fake-claude.sh" /usr/local/bin/claude
#
# On ne simule PAS `npx bmad-method install` — Hive a déjà Install()
# qui est un `exec.Command("npx", ...)` séparé. La CI qui veut
# stubber ça peut shadow `npx` dans le PATH.

set -e

# Collecte tout le stdin (prompt), les args à part.
ARGS=("$@")

# On attend --print + --output-format json. Si absent, ignore silencieux.
want_json=false
for a in "${ARGS[@]}"; do
	if [ "$a" = "json" ]; then want_json=true; fi
done

# Lis le prompt depuis stdin (ignoré par le stub, on simule juste).
PROMPT=$(cat || true)

# Détecte la slash-command dans le prompt.
CMD=$(echo "$PROMPT" | grep -oE '/bmad-[a-z-]+' | head -1 || true)

# Réponse canned.
RESULT="Stub claude — commande simulée : ${CMD:-(aucune)}. Aucun fichier écrit."

if ! $want_json; then
	echo "$RESULT"
	exit 0
fi

# Envelope JSON réaliste.
cat <<EOF
{
  "result": "$RESULT",
  "is_error": false,
  "duration_ms": 42,
  "num_turns": 1,
  "total_cost_usd": 0,
  "usage": { "input_tokens": 10, "output_tokens": 8 },
  "session_id": "fake-$(date +%s%N)"
}
EOF
