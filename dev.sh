#!/usr/bin/env bash
# Cue AI — run the whole stack locally (db + server + web).
# Usage: ./dev.sh
set -euo pipefail
cd "$(dirname "$0")"

export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"

# Keep the AI's reference in sync with the CANONICAL repo files so every generation
# uses the current slides skill, tokens, and component APIs — and the Sandpack preview
# runs the current engine. The .bolt skill + src/ are the single source of truth.
echo "▸ syncing slides skill + engine reference…"
cp .bolt/skills/slides/SKILL.md server/reference/SKILL.md
cp src/styles/tokens.css       server/reference/tokens.default.css
ls src/components > server/reference/components.list.txt
# API surface only (doc comment + prop types + signature), NOT full component
# source: the model authors just App.tsx, so shipping implementation detail cost
# ~21k input tokens per turn and made every generation slower for no gain.
node scripts/gen-component-api.mjs
rsync -a --delete src/ web/src/deck-template/src/   # pre-baked engine for the web runtime
node scripts/gen-registry.mjs                        # regenerate the module registry
# Mirror the engine where the server compile-check (esbuild) resolves deck imports.
rsync -a --delete src/deck/       server/reference/engine/deck/
rsync -a --delete src/components/ server/reference/engine/components/
rsync -a --delete src/styles/     server/reference/engine/styles/
echo "  reference in sync with .bolt skill + src/."

echo "▸ starting Postgres + pgvector (docker)…"
docker compose up -d db
until [ "$(docker inspect --format '{{.State.Health.Status}}' cueai-db 2>/dev/null)" = "healthy" ]; do
  sleep 1
done
echo "  db healthy."

echo "▸ starting Gin server on :8080…"
( cd server && go run . ) &
SERVER_PID=$!

echo "▸ starting web on :5273…"
( cd web && npm run dev ) &
WEB_PID=$!

trap 'echo; echo "stopping…"; kill $SERVER_PID $WEB_PID 2>/dev/null; exit 0' INT TERM
echo
echo "  Cue AI → http://localhost:5273"
echo "  (Ctrl-C to stop server + web; db keeps running — 'docker compose down' to stop it.)"
wait
