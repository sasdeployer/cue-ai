# Nexlayer — how `cue-ai` ships

You were asked to work on or deploy this app. The state below is already
resolved — do not re-derive it from the code. The procedure is not here;
ask Nexlayer for it (see "How to deploy").

## The app

| | |
| --- | --- |
| Name | `cue-ai` |
| Repo | `https://github.com/sasdeployer/cue-ai` on `main` |
| Planned | 2026-10-06T06:20:52.015Z |
| Registered with Nexlayer | yes |

`.nexlayer/plan.lock` pins the commit this plan was written against. If HEAD
has moved and you changed how the app starts, runs, or what it needs,
re-check before deploying.

## The deploy config

This repo already has a `nexlayer.yaml`, and it is the source of truth: the
services in this plan were read from it (`app`, `postgres`).
Deploy from it. Change it for a reason, never to match a fresh analysis —
the analysis infers; this file is what runs.

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

The `verify` list in `.nexlayer/pipeline.yaml` is what to check. Check it — do not
assume a deploy worked.

## Stop and ask the human

- A required key is missing (send the link above — never take the value).
- Something would become publicly reachable that is internal in this plan.
- Anything that deletes data or tears down a running deployment.

Everything else is yours to do. When something breaks, start at
`.nexlayer/TROUBLESHOOTING.md`.
