# cue-ai — production context

Written by the Nexlayer agent so any coding agent that opens this repo
starts with the same picture. Read this before proposing infrastructure
changes.

- **Repo** `https://github.com/sasdeployer/cue-ai` on `main`
- **Analyzed** 2026-10-06T06:21:21.034Z

## Stack

| Component | Version | How we know |
| --- | --- | --- |
| Go | 1.22+ (Docker build uses 1.23) | read from `README.md`, `Dockerfile` |
| TypeScript | ^5.5.3 | read from `package.json`, `tsconfig.json` |
| Node.js | 20+ (Docker build uses 22) | read from `README.md`, `Dockerfile` |
| React | ^18.3.1 | read from `package.json` |
| Vite | ^5.4.0 | read from `package.json`, `vite.config.ts` |
| framer-motion | ^11.3.0 | read from `package.json` |
| PostgreSQL + pgvector | 16 | read from `docker-compose.yml`, `.env.example` |
| Docker / Docker Compose |  | read from `docker-compose.yml`, `Dockerfile` |
| OpenAI / Anthropic LLM APIs | gpt-5.2 / claude-sonnet-5 | read from `.env.example` |

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

- `ANTHROPIC_API_KEY` — optional, not set. Alternative to OpenAI. Provider preference is OpenAI, then Anthropic, then canned sample decks.
- `OPENAI_API_KEY` — optional, not set. Without any provider key the server runs in canned mode and returns sample decks. Visitors can also supply their own key in Settings.

## What the human told us

**Purpose.** this is just a test

**Stage.** This is an experiment.

Said by a person, not derived from the code. Where this contradicts what
the repo looks like, the person is right about intent and the repo is
right about what exists today.

## Notes from the analysis

- Two pods: 'app' is the single container from the repo Dockerfile, with the Go server serving the API and the built web app from ./dist on 8080. 'db' is a separate Postgres pod. The alpine image listed for 'app' is only the Dockerfile's runtime base; deploy the image built from the Dockerfile.
- The app reaches Postgres at db.pod:5432. The Dockerfile bakes in DATABASE_URL pointing at localhost:5432, so it must be overridden at runtime with the db.pod form.
- The server applies server/schema.sql itself on every boot, so no init script or volume mount is needed on the db pod.
- docker-compose uses pgvector/pgvector:pg16, a namespaced Docker Hub image that fails on the cluster. The plan uses mirror.gcr.io/library/postgres:16 instead. If schema.sql runs CREATE EXTENSION vector, that image will fail at boot, so use a pgvector-enabled image that is reachable from the cluster, or remove the extension dependency.
- The Dockerfile sets OPENAI_API_KEY and ANTHROPIC_API_KEY to 'nexlayer-placeholder'. The server may treat that as a configured key and call the providers with an invalid credential instead of falling back to canned mode. Override both with real keys, or set them empty, at deploy time.
- ALLOW_ORIGIN (CORS) should be set to the public URL of the deployed app. It is the same origin when the Go server serves the web app.
- The runtime image must include ./reference (the deck engine). compile.go reads it at runtime, and the Dockerfile already copies it.
- The Dockerfile rewrites a bad 'go 1.26.5' directive in server/go.mod to 'go 1.23' during the build.

## Talking to Nexlayer

Nexlayer is reachable over MCP. Call `nexlayer_get_deployment_workflow`
before deploying — it is the current procedure, and it changes more often
than this file does.
