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
| `app` | `DATABASE_URL` | `"postgres://cueai:${POSTGRES_PASSWORD}@postgres.pod:5432/cueai?sslmode=disable"` | inter-pod |
| `app` | `ALLOW_ORIGIN` | `"<% URL %>"` | plain |
| `app` | `OPENAI_API_KEY` | `"${OPENAI_API_KEY}"` | inter-pod |
| `app` | `ANTHROPIC_API_KEY` | `"${ANTHROPIC_API_KEY}"` | inter-pod |
| `postgres` | `POSTGRES_USER` | `"cueai"` | plain |
| `postgres` | `POSTGRES_PASSWORD` | `"${POSTGRES_PASSWORD}"` | inter-pod |
| `postgres` | `POSTGRES_DB` | `"cueai"` | plain |

### nexlayer.yaml

```yaml
application:
  name: cue-ai
  pods:
    - name: app
      image: "registry.nexlayer.io/user_01kdnssb5ktgqr1mawtnz48s00/cue-ai:9f8d315-fix5"
      path: /
      servicePorts:
        - 8080
      vars:
        PORT: "8080"
        HOSTNAME: "0.0.0.0"
        DATABASE_URL: "postgres://cueai:${POSTGRES_PASSWORD}@postgres.pod:5432/cueai?sslmode=disable"
        ALLOW_ORIGIN: "<% URL %>"
        OPENAI_API_KEY: "${OPENAI_API_KEY}"
        ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    - name: postgres
      image: mirror.gcr.io/library/postgres:16-alpine
      servicePorts:
        - 5432
      vars:
        POSTGRES_USER: "cueai"
        POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
        POSTGRES_DB: "cueai"
```
<!-- nexlayer:end -->

## Nexlayer Deployment Plan
<!-- nexlayer:section user-editable=deployment_plan -->
### Pod Topology

| Pod | Image | Port | Role |
|-----|-------|------|------|
| web | mirror.gcr.io/library/node:22-alpine | 5273 | web |
| api | mirror.gcr.io/library/golang:1.22-alpine | 8080 | web |
| db | mirror.gcr.io/library/postgres:16-alpine | 5432 | database |

### Deployment notes

- The API server connects to the database using the Nexlayer pod address: db.pod:5432
- The web frontend communicates with the backend using api.pod:8080
- pgvector is required; the official postgres image may need the pgvector extension installed via a custom Dockerfile using mirror.gcr.io/library/postgres as base

<!-- nexlayer:end -->

## Build Notes
<!-- nexlayer:section user-editable=build_notes -->
<!-- Add notes for future builds here — preserved across re-analysis -->
<!-- nexlayer:end -->

## Nexlayer Configuration
<!-- nexlayer:section agent-managed=nexlayer_config -->
**Last deployed:** 2026-07-23T04:36:56Z  
**Live URL:** https://zen-antelope-cue-ai.cloud.nexlayer.ai  
**Runtime:**  · **Port:** auto-detected  
**Deploy branch:** nexlayer  

```yaml
application:
  name: cue-ai
  pods:
    - name: app
      image: "registry.nexlayer.io/user_01kdnssb5ktgqr1mawtnz48s00/cue-ai:9f8d315-fix5"
      path: /
      servicePorts:
        - 8080
      vars:
        PORT: "8080"
        HOSTNAME: "0.0.0.0"
        DATABASE_URL: "postgres://cueai:${POSTGRES_PASSWORD}@postgres.pod:5432/cueai?sslmode=disable"
        ALLOW_ORIGIN: "<% URL %>"
        OPENAI_API_KEY: "${OPENAI_API_KEY}"
        ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    - name: postgres
      image: mirror.gcr.io/library/postgres:16-alpine
      servicePorts:
        - 5432
      vars:
        POSTGRES_USER: "cueai"
        POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
        POSTGRES_DB: "cueai"
```
<!-- nexlayer:end -->

## Build History
<!-- nexlayer:section agent-managed=build_history -->
| Date | Status | Notes |
|------|--------|-------|
| 2026-07-23T04:18:01Z | analyzed | initial repo analysis |
| 2026-07-23T04:36:56Z | success | deployed https://zen-antelope-cue-ai.cloud.nexlayer.ai |
<!-- nexlayer:end -->


