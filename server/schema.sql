-- Cue AI schema (slice 1)
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS decks (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner       text        NOT NULL DEFAULT 'anon',
    title       text        NOT NULL DEFAULT 'Untitled deck',
    prompt      text        NOT NULL,
    app_tsx     text        NOT NULL,
    tokens_css  text        NOT NULL DEFAULT '',
    is_public   boolean     NOT NULL DEFAULT true,
    embedding   vector(1536),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS decks_public_created_idx
    ON decks (is_public, created_at DESC);

-- ivfflat index for semantic gallery search (used in a later slice)
CREATE INDEX IF NOT EXISTS decks_embedding_idx
    ON decks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- No accounts/auth: Cue is open source, BYOK — each visitor's own OpenAI/
-- Anthropic key (entered client-side, encrypted in their own browser) is
-- used for their generations, or the server's own key as the default. See
-- CLAUDE.md's "BYOK" section. There was a brief magic-link auth system
-- (2026-07-22) that was fully removed — no tables to migrate, it never
-- shipped past local dev.
