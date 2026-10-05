# Nexlayer — how `cue-ai` ships

You were asked to work on or deploy this app. The state below is already
resolved — do not re-derive it from the code. The procedure is not here;
ask Nexlayer for it (see "How to deploy").

## The app

| | |
| --- | --- |
| Name | `cue-ai` |
| Repo | `https://github.com/sasdeployer/cue-ai` on `main` |
| Planned | 2026-10-05T17:21:12.401Z |
| Registered with Nexlayer | not yet |

`.nexlayer/plan.lock` pins the commit this plan was written against. If HEAD
has moved and you changed how the app starts, runs, or what it needs,
re-check before deploying.

## Running now

`cue-ai` is already deployed on Nexlayer (running when this plan was written).

| | |
| --- | --- |
| applicationName | `cue-ai` |
| environment | `zen-antelope` |
| URL | <https://zen-antelope-cue-ai.cloud.nexlayer.ai> |

Every Nexlayer tool that targets this app — status, logs, events, the debug
proxy — takes these two values. Use them as written; do not ask for them.
A redeploy updates this deployment in place.

## The deploy config

This repo already has a `nexlayer.yaml`, and it is the source of truth: the
services in this plan were read from it (`app`, `postgres`).
Deploy from it. Change it for a reason, never to match a fresh analysis —
the analysis infers; this file is what runs.

## What this app is for

An AI-powered slide generation tool powered by the OpenAI API.

The human calls this headed for production.

## Can this deploy right now?

**Yes.** Nothing is blocking.

## How to deploy

Call `nexlayer_get_deployment_workflow` first. It returns the current
procedure — building and pushing the image included — and it is kept up to
date in a way this file is not. Do not infer the steps from here, and do
not skip it because the app looks simple.

If Nexlayer tools are not available to you, the human runs
`npx @nexlayer/mcp-install` once.

## Secrets

| Key | Status |
| --- | --- |
| `ANTHROPIC_API_KEY` | not set — the live app runs without it (optional) |
| `OPENAI_API_KEY` | not set — the live app runs without it (optional) |

Keys marked supplied are handled. Do not ask for them again.

## What was inferred rather than read

Nothing. Every claim in this plan was read from the repo.

## What "it worked" means

`.nexlayer/checks.json` lists what to verify per service. Verify it — do not
assume a deploy worked.

## Stop and ask the human

- A required key is missing (send the link above — never take the value).
- Something would become publicly reachable that is internal in this plan.
- Anything that deletes data or tears down a running deployment.

Everything else is yours to do. When something breaks, start at
`.nexlayer/TROUBLESHOOTING.md`.
