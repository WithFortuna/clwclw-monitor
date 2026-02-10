-- Multi-tenancy: add user_id to main resources

-- agents: user_id nullable (existing agents without user)
ALTER TABLE public.agents ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES public.users(id);

-- channels: user_id required for new rows
ALTER TABLE public.channels ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES public.users(id);

-- chains: user_id required
ALTER TABLE public.chains ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES public.users(id);

-- tasks: user_id required
ALTER TABLE public.tasks ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES public.users(id);

-- indexes
CREATE INDEX IF NOT EXISTS idx_agents_user_id ON public.agents(user_id);
CREATE INDEX IF NOT EXISTS idx_channels_user_id ON public.channels(user_id);
CREATE INDEX IF NOT EXISTS idx_chains_user_id ON public.chains(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON public.tasks(user_id);

-- auth_codes table (one-time use, 5min expiry)
CREATE TABLE IF NOT EXISTS public.auth_codes (
  code TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  agent_name TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  used BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
