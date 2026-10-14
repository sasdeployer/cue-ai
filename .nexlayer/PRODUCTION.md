# Nexlayer — how `cue-ai` ships

You were asked to work on or deploy this app. The state below is already
resolved — do not re-derive it from the code. The procedure is not here;
ask Nexlayer for it (see "How to deploy").

## The app

| | |
| --- | --- |
| Name | `cue-ai` |
| Repo | `https://github.com/sasdeployer/cue-ai` on `main` |
| Planned | 2026-10-14T22:27:18.398Z |
| Registered with Nexlayer | yes |

`.nexlayer/plan.lock` pins the commit this plan was written against. If HEAD
has moved and you changed how the app starts, runs, or what it needs,
re-check before deploying.

## The production plan

Written by the Nexlayer agent from this repo. Every decision cites the files it
rests on; if the code has changed since, re-check those files first.

Cue is a Go backend that serves both a compiled React frontend and an AI-powered deck generation API on port 8080, backed by PostgreSQL with pgvector for deck persistence. The Go binary embeds the built frontend from ./dist and the deck engine templates from ./reference — a single app pod handles everything. The one thing that matters most: the PostgreSQL password must never be a literal in the yaml, or the deploy will run with throwaway credentials that anyone can guess.

- **services: Two pods: app (Go binary serving API + static frontend on 8080) and postgres (pgvector). No separate frontend pod needed because the Go binary serves the built Vite output from ./dist at runtime.** — server/main.go registers both API routes and a static file handler for ./dist in the same Gin server, so one process serves everything. (`server/main.go`, `Dockerfile`)
- **database: PostgreSQL with pgvector extension (pgvector/pgvector:pg16) in its own pod with a 10 Gi volume at /var/lib/postgresql. PGDATA is set to a subdirectory to survive the lost+found problem.** — schema.sql runs CREATE EXTENSION IF NOT EXISTS vector on every boot, which requires the pgvector binary to be present in the image; stock postgres:16-alpine does not include it. (`server/schema.sql`, `docker-compose.yml`, `nexlayer.yaml`)
- **keys: OPENAI_API_KEY and ANTHROPIC_API_KEY are intentionally left unset in the yaml — the app runs in canned mode (sample decks) and visitors supply their own keys per-request via X-User-OpenAI-Key header.** — main.go selects a provider only when the key is non-empty; setting a placeholder would select OpenAI and then every generation fails. The BYOK path in handlers.go lets each visitor use their own key without any server-side secret. (`server/main.go`, `.env.example`)
- **build: The existing multi-stage Dockerfile builds the frontend with node:22-alpine, the Go backend with golang:1.23-alpine, and produces a minimal alpine:3.20 runtime image. It patches the go.mod version typo and copies the reference/ engine directory for runtime template resolution.** — compile.go's findEngineDir resolves deck templates from ./reference/engine at runtime — without it every slide generation fails the compile check. (`Dockerfile`, `server/go.mod`)
- **health: GET /api/health returns 200 with {ok: true, llm: '<provider name>'} — use this as the deployment readiness check.** — main.go registers r.GET("/api/health", …) unconditionally; it confirms both HTTP serving and database connectivity (store is initialized before routes are registered). (`server/main.go`)

### Fix before production

- **Blocker** — Replace literal Postgres password with ${POSTGRES_PASSWORD}: The yaml currently has POSTGRES_PASSWORD: "cueai" and embeds cueai:cueai in DATABASE_URL — anyone with access to the deployment config gets database credentials. This is a secret exposure. (`nexlayer.yaml`)

### Verify after the deploy

1. GET /api/health on the app URL returns 200 with {"ok": true, "llm": "canned"}
2. GET / on the app URL returns 200 with the React landing page HTML
3. POST /api/decks with a test prompt and a valid X-User-OpenAI-Key header returns a deck JSON (confirms DB writes succeed and the pgvector extension loaded)
4. Restart the postgres pod and confirm GET /api/decks still returns previously created decks (confirms volume persistence)

### Ask the human

- Should an OpenAI or Anthropic key be set server-side for a production demo, or is BYOK-only acceptable for the target users?
- The pgvector image (pgvector/pgvector:pg16) is hosted on Docker Hub and may hit rate limits — confirm it pulls cleanly through the GCR mirror on first deploy, or switch to building a custom Postgres image with pgvector compiled in.

## The deploy config

This repo already has a `nexlayer.yaml`, and it is the source of truth: the
services in this plan were read from it (`app`, `postgres`).
Deploy from it. Change it for a reason, never to match a fresh analysis —
the analysis infers; this file is what runs.

## Can this deploy right now?

**Not yet — 1 blocker under "Fix before production" (yours).** Full list in `.nexlayer/todo.md`.

Fix those first. Deploying with them open ships a known problem.

## How to deploy

Call `nexlayer_get_deployment_workflow` first. It returns the current
procedure — building and pushing the image included — and it is kept up to
date in a way this file is not. Do not infer the steps from here, and do
not skip it because the app looks simple.

If Nexlayer tools are not available to you, the human runs
`npx @nexlayer/mcp-install` once.

## Secrets

Keys reach the app by name. In `nexlayer.yaml`, write `${NAME}` where the
value goes (e.g. `OPENAI_API_KEY: "${OPENAI_API_KEY}"`) — never the value.
When you deploy through the Nexlayer MCP, Nexlayer fills each name from this
app's Secrets. Values never go in this repo, the chat, or your context.

| Key | Status |
| --- | --- |
| `ANTHROPIC_API_KEY` | not set — optional, the app runs without it |
| `OPENAI_API_KEY` | not set — optional, the app runs without it |

Keys marked supplied are handled. Do not ask for them again.

Optional keys: leave them out of `nexlayer.yaml` unless the human wants that
feature on — then they add the key in the same Secrets page and you add its
`${NAME}` reference.

## What was inferred rather than read

Nothing. Every claim in this plan was read from the repo.

## What "it worked" means

After every deploy, run `nexlayer_verify_deploy`. A deploy is accepted before it is
known to work; only "✓ Working" means it worked. It runs the `verify` list in
`.nexlayer/pipeline.yaml`, and when the app is broken it says what broke and why.

## Stop and ask the human

- A required key is missing (send the link above — never take the value).
- Something would become publicly reachable that is internal in this plan.
- Anything that deletes data or tears down a running deployment.

Everything else is yours to do. When something breaks, start at
`.nexlayer/TROUBLESHOOTING.md`.
