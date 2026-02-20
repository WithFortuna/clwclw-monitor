-- Access tokens for external service integration (M2M authentication)
-- Raw tokens are never stored; only SHA-256 hashes are persisted.

CREATE TABLE public.access_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,  -- SHA-256 hex of raw token
    prefix       TEXT NOT NULL,          -- First 8 chars of raw token (display only)
    scopes       TEXT[] NOT NULL DEFAULT '{"tasks:write"}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_access_tokens_hash ON public.access_tokens (token_hash);
CREATE INDEX idx_access_tokens_user_id ON public.access_tokens (user_id);
