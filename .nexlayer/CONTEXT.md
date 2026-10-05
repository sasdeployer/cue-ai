# cue-ai — production context

Written by the Nexlayer agent so any coding agent that opens this repo
starts with the same picture. Read this before proposing infrastructure
changes.

- **Repo** `https://github.com/sasdeployer/cue-ai` on `main`
- **Analyzed** 2026-10-05T18:14:44.983Z

## Stack

| Component | Version | How we know |
| --- | --- | --- |
| React | 18.3.1 | read from `package.json` |
| Vite | 5.4.0 | read from `package.json` |
| Go | 1.23 | read from `Dockerfile` |
| PostgreSQL | 16 | read from `docker-compose.yml` |
| pgvector | pg16 | read from `docker-compose.yml` |

## How Nexlayer will run it

| Service | Reachable | Image | How we know |
| --- | --- | --- | --- |
| `app` | public | `registry.nexlayer.io/user_01kdnssb5ktgqr1mawtnz48s00/cue-ai:acee236-sameorigin` | read from `nexlayer.yaml` |
| `postgres` | internal only | `mirror.gcr.io/pgvector/pgvector:pg16` | read from `nexlayer.yaml` |

Reachability is inferred from service names and roles, not stated by the
analysis. Check it before relying on it — exposing something that should
be internal is not recoverable by editing this file afterwards.

Networking, HTTPS, and service discovery are handled.

## Secrets

0 of 2 keys were supplied by the
human on the web and handed to Nexlayer, which injects them at runtime. No
values are in this repo, and none ever will be.

- `ANTHROPIC_API_KEY` — not set; the live app runs without it, so it's optional.
- `OPENAI_API_KEY` — not set; the live app runs without it, so it's optional.

## What the human told us

**Purpose.** An AI-powered slide generation tool powered by the OpenAI API.

**Stage.** This is headed for production.

Said by a person, not derived from the code. Where this contradicts what
the repo looks like, the person is right about intent and the repo is
right about what exists today.

## Notes from the analysis

- Database pod is separate and accessed by API via db.pod:5432.
- API pod serves the built frontend static files from /app/dist, so a separate web pod is optional; if used, it must proxy API calls to api.pod:8080.
- The API pod must include the reference engine directory (server/reference) for runtime compilation.
- Use mirror.gcr.io/library/ images for all Docker Hub images.
- No init script for database; the API server applies schema.sql on boot.
- For production, set real API keys via environment variables, not placeholders.

## Talking to Nexlayer

Nexlayer is reachable over MCP. Call `nexlayer_get_deployment_workflow`
before deploying — it is the current procedure, and it changes more often
than this file does.
