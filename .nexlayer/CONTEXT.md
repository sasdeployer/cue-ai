# cue-ai — production context

Written by the Nexlayer agent so any coding agent that opens this repo
starts with the same picture. Read this before proposing infrastructure
changes.

- **Repo** `https://github.com/sasdeployer/cue-ai` on `main`
- **Analyzed** 2026-10-14T22:27:18.398Z

## Stack

| Component | Version | How we know |
| --- | --- | --- |
| Go | 1.23 | read from `Dockerfile`, `server/go.mod` |
| React | 18.3 | read from `package.json` |
| Vite | 5.4 | read from `package.json`, `vite.config.ts` |
| PostgreSQL + pgvector | 16 | read from `docker-compose.yml`, `.env.example` |
| TypeScript | 5.5 | read from `tsconfig.json`, `package.json` |
| Framer Motion | 11.3 | read from `package.json` |
| OpenAI API | gpt-5.2 | read from `.env.example`, `README.md` |
| Anthropic API | claude-sonnet-5 | read from `.env.example` |

## How Nexlayer will run it

| Service | Reachable | Image | How we know |
| --- | --- | --- | --- |
| `app` | public | `registry.nexlayer.io/user_01kdnssb5ktgqr1mawtnz48s00/cue-ai:675630d` | read from `nexlayer.yaml` |
| `postgres` | internal only | `mirror.gcr.io/pgvector/pgvector:pg16` | read from `nexlayer.yaml` |

Reachability is inferred from service names and roles, not stated by the
analysis. Check it before relying on it — exposing something that should
be internal is not recoverable by editing this file afterwards.

Networking, HTTPS, and service discovery are handled.

## Secrets

This app uses 2 keys: 0 required to run, 2 optional.

Keys reach the app by name. In `nexlayer.yaml`, write `${NAME}` where the
value goes (e.g. `OPENAI_API_KEY: "${OPENAI_API_KEY}"`) — never the value.
When you deploy through the Nexlayer MCP, Nexlayer fills each name from this
app's Secrets. Values never go in this repo, the chat, or your context.

- `ANTHROPIC_API_KEY` — optional, not set. Alternative AI backend; if neither OpenAI nor Anthropic key is set, app starts successfully in canned mode per README
- `OPENAI_API_KEY` — optional, not set. Without it Cue runs in canned mode serving a real sample deck; users can also enter their own key per-request from the Settings UI

## What the human told us

**Stage.** This is a side project.

Said by a person, not derived from the code. Where this contradicts what
the repo looks like, the person is right about intent and the repo is
right about what exists today.

## Notes from the analysis

- App reaches the database pod via db.pod:5432 using the DATABASE_URL environment variable
- pgvector/pgvector:pg16 (docker-compose) is a Docker Hub namespaced image that fails on Nexlayer; use mirror.gcr.io/library/postgres:16-alpine and install pgvector extension via the server's schema.sql (CREATE EXTENSION IF NOT EXISTS vector)
- The Go binary applies server/schema.sql on every boot (NewStore), so no init script mount is needed on the db pod
- The frontend is statically compiled into the Go binary (served from ./dist), so a single app pod serves both API and UI
- The reference/ directory (deck engine templates) must be present in the image at runtime for compile.go to generate slides
- Set ALLOW_ORIGIN to the external URL of the app pod or use * for development; CORS is enforced by the Go server

## Talking to Nexlayer

Nexlayer is reachable over MCP. Call `nexlayer_get_deployment_workflow`
before deploying — it is the current procedure, and it changes more often
than this file does.
