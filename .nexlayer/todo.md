# cue-ai — before this ships

Two lists, split by who can actually close the item.

## Needs code — the coding agent

- [ ] **Blocker:** Replace literal Postgres password with ${POSTGRES_PASSWORD} (`nexlayer.yaml`)
      _The yaml currently has POSTGRES_PASSWORD: "cueai" and embeds cueai:cueai in DATABASE_URL — anyone with access to the deployment config gets database credentials. This is a secret exposure._

## Needs the human

- [ ] Decide: Should an OpenAI or Anthropic key be set server-side for a production demo, or is BYOK-only acceptable for the target users?
- [ ] Decide: The pgvector image (pgvector/pgvector:pg16) is hosted on Docker Hub and may hit rate limits — confirm it pulls cleanly through the GCR mirror on first deploy, or switch to building a custom Postgres image with pgvector compiled in.
- Nothing blocking. Some optional keys are not set; the app runs without them.

## Check after the deploy — the coding agent

- [ ] GET /api/health on the app URL returns 200 with {"ok": true, "llm": "canned"}
- [ ] GET / on the app URL returns 200 with the React landing page HTML
- [ ] POST /api/decks with a test prompt and a valid X-User-OpenAI-Key header returns a deck JSON (confirms DB writes succeed and the pgvector extension loaded)
- [ ] Restart the postgres pod and confirm GET /api/decks still returns previously created decks (confirms volume persistence)

---

Machine-readable: `.nexlayer/findings.json`.
