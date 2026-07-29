# Nexlayer — Cue-AI

<!-- nexlayer:meta version=1 analyzed=2026-07-23T04:18:01Z repo=https://github.com/sasdeployer/Cue-AI branch=nexlayer -->

> **For AI agents (Claude Code, Cursor, Gemini CLI, Copilot):**
> This file is the **project context** for this Nexlayer deployment — tech stack, env vars, secrets, live URL.
> For full platform detail (nexlayer.yaml schema, Dockerfile rules, CI/CD, task recipes) read **`nexlayer.skills`** in this repo.
>
> **Critical rules (full detail in `nexlayer.skills`):**
> - Inter-pod refs: `${podName:port}` only — never `localhost` or bare hostnames
> - Docker Hub images: prefix with `mirror.gcr.io/library/` — bare tags fail on the cluster
> - Secrets: set in the Nexlayer dashboard — never commit to `nexlayer.yaml` or Dockerfile
>
> **This file:** `agent-managed` sections update automatically. `user-editable` sections (Local Development Setup, Nexlayer Deployment Plan, Build Notes) are yours — preserved across re-analysis.

## Project Summary
<!-- nexlayer:section agent-managed=project_summary -->
Cue-AI transforms a single prompt into an interactive, multi-slide presentation deck where every slide is a real React component, utilizing a custom slide engine and LLM-generated code.
<!-- nexlayer:end -->

## Technology Stack
<!-- nexlayer:section agent-managed=tech_stack -->
| Name | Kind | Version | Detected From |
|------|------|---------|---------------|
| React | framework | 18.3.1 | package.json |
| Vite | build | 5.4.0 | package.json |
| TypeScript | language | 5.5.3 | package.json |
| Go | language | 1.22+ | README.md |
| Postgres (pgvector) | database | 16 | docker-compose.yml |
<!-- nexlayer:end -->

## Repository Structure
<!-- nexlayer:section agent-managed=structure_map -->
- server/ — Go-based API server handling LLM orchestration and data
- web/ — React frontend and slide rendering engine
- src/ — Shared source code and components
- db/ — Database initialization scripts
<!-- nexlayer:end -->

## External Services Required
<!-- nexlayer:section agent-managed=external_deps -->
Services that must be configured separately (not deployed by Nexlayer):

- OpenAI API (OPENAI_API_KEY)
- Anthropic API (ANTHROPIC_API_KEY)
<!-- nexlayer:end -->

## Local Development Setup
<!-- nexlayer:section user-editable=local_setup -->
### Prerequisites

- Node.js >= 20
- Go >= 1.22
- Docker

### Environment variables

Copy `.env.example` to `.env.local` and fill in:

```
DATABASE_URL=postgres://cueai:cueai@localhost:5432/cueai?sslmode=disable
PORT=8080
ALLOW_ORIGIN=http://localhost:5273
OPENAI_API_KEY=your_key
```

### Steps

1. `npm install` — Install frontend dependencies
2. `./dev.sh` — Bootstrap database, server, and web app

<!-- nexlayer:end -->

## Nexlayer Setup
<!-- nexlayer:section agent-managed=nexlayer_setup -->
### Pod Environment Variables

| Pod | Variable | Value | Kind |
|-----|----------|-------|------|
| `app` | `PORT` | `"8080"` | plain |
| `app` | `HOSTNAME` | `"0.0.0.0"` | plain |
| `app` | `STATIC_DIR` | `"./dist"` | plain |
| `app` | `DATABASE_URL` | `"postgres://cueai:cueai@postgres.pod:5432/cueai?sslmode=disable"` | inter-pod |
| `app` | `ALLOW_ORIGIN` | `"<% URL %>"` | plain |
| `postgres` | `POSTGRES_USER` | `"cueai"` | plain |
| `postgres` | `POSTGRES_PASSWORD` | `"cueai"` | plain |
| `postgres` | `POSTGRES_DB` | `"cueai"` | plain |

No `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` — see Build Notes.

### nexlayer.yaml

Canonical copy lives in `nexlayer.yaml` at the repo root; it matches what is
deployed. See Build Notes for why the LLM keys are absent and why the postgres
password is a committed throwaway.
<!-- nexlayer:end -->

## Nexlayer Deployment Plan
<!-- nexlayer:section user-editable=deployment_plan -->
### Pod Topology

| Pod | Image | Port | Role |
|-----|-------|------|------|
| web | mirror.gcr.io/library/node:22-alpine | 5273 | web |
| api | mirror.gcr.io/library/golang:1.22-alpine | 8080 | web |
| db | mirror.gcr.io/pgvector/pgvector:pg16 | 5432 | database |

### Deployment notes

- The API server connects to the database using the Nexlayer pod address: db.pod:5432
- The web frontend communicates with the backend using api.pod:8080
- pgvector is required; the official postgres image may need the pgvector extension installed via a custom Dockerfile using mirror.gcr.io/library/postgres as base

<!-- nexlayer:end -->

## Build Notes
<!-- nexlayer:section user-editable=build_notes -->
<!-- Add notes for future builds here — preserved across re-analysis -->

### The runtime image needs `reference/engine`, not just the binary

`server/compile.go` compile-checks every generated deck with esbuild against the
mirrored bolt-slides engine, and it resolves that engine **from disk at runtime**
(`findEngineDir()` looks for `reference/engine` in the working directory, then
next to the executable). A final stage that copies only `main` and `dist` builds
and boots fine, serves the frontend fine, and answers `GET /api/decks` fine —
then fails *every* generation with `event: error` ("we couldn't build a working
deck"), because the compile check has nothing to resolve deck imports against.
This shipped live once and was only caught by POSTing a real generate. The
Dockerfile's final stage must keep:

```dockerfile
COPY --from=backend-builder /app-backend/reference ./reference
```

**Verify a deploy with a POST, not a GET.** `GET /` + `GET /api/decks` returning
200 does not exercise the generation pipeline at all:

```bash
curl -s -N -X POST "$URL/api/decks" -H 'Content-Type: application/json' \
  -d '{"prompt":"a three-slide deck introducing Cue"}' | grep -E '^event: (done|error)'
```

### No LLM key is set on purpose

`config.go` picks a provider on "key is non-empty", so a placeholder value is
worse than an absent one — it selects OpenAI and every generation 400s. Live runs
with both keys unset: canned sample decks server-side, BYOK for visitors. Set a
real key as an encrypted dashboard var if server-side generation is wanted.

### The database is ephemeral

No `volumes:` are declared, so the postgres pod re-initialises on every deploy
and all saved decks are lost. `server/schema.sql` is embedded and applied on
every boot, so this self-heals rather than 500ing — but a shared deck link does
not survive a redeploy. Declaring a volume is the fix if that matters.
<!-- nexlayer:end -->

## Nexlayer Configuration
<!-- nexlayer:section agent-managed=nexlayer_config -->
**Last deployed:** 2026-07-29  
**Live URL:** https://zen-antelope-cue-ai.cloud.nexlayer.ai  
**Image:** `registry.nexlayer.io/user_01kdnssb5ktgqr1mawtnz48s00/cue-ai:8d5fb14-engine`  
**Runtime:** Go binary serving `./dist` · **Port:** 8080  
**Deploy branch:** nexlayer (kept fast-forwarded to `main`)  

Deployed config is `nexlayer.yaml` at the repo root, verbatim.
<!-- nexlayer:end -->

## Build History
<!-- nexlayer:section agent-managed=build_history -->
| Date | Status | Notes |
|------|--------|-------|
| 2026-07-23T04:18:01Z | analyzed | initial repo analysis |
| 2026-07-23T04:36:56Z | success | deployed https://zen-antelope-cue-ai.cloud.nexlayer.ai |
| 2026-07-29 | partial | tag `8d5fb14`: frontend + schema-on-boot fixed (`/` and `GET /api/decks` 200), but every generation failed the compile check — `reference/engine` was missing from the runtime image |
| 2026-07-29 | success | tag `8d5fb14-engine`: engine copied into the final stage; live generate verified end to end (`event: done`, deck persisted) |
<!-- nexlayer:end -->


