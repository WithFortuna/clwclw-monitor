-- clwclw-monitor: initial schema draft
-- Source of truth: REQUIREMENTS.md

-- UUID generation (Supabase typically supports pgcrypto)
create extension if not exists pgcrypto;

-- Common updated_at trigger
create or replace function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

-- Agents
create table if not exists public.agents (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  status text not null default 'idle',
  current_task_id uuid null,
  last_seen timestamptz not null default now(),
  meta jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists idx_agents_last_seen on public.agents (last_seen desc);

create trigger trg_agents_updated_at
before update on public.agents
for each row execute function set_updated_at();

-- Channels
create table if not exists public.channels (
  id uuid primary key default gen_random_uuid(),
  name text not null unique,
  description text null,
  created_at timestamptz not null default now()
);

-- Tasks
create table if not exists public.tasks (
  id uuid primary key default gen_random_uuid(),
  channel_id uuid not null references public.channels(id) on delete restrict,
  title text not null,
  description text null,
  status text not null default 'queued',
  priority int not null default 0,
  assigned_agent_id uuid null references public.agents(id) on delete set null,
  created_at timestamptz not null default now(),
  claimed_at timestamptz null,
  done_at timestamptz null,
  updated_at timestamptz not null default now()
);

create index if not exists idx_tasks_channel_status_created on public.tasks (channel_id, status, created_at asc);
create index if not exists idx_tasks_status_created on public.tasks (status, created_at asc);
create index if not exists idx_tasks_assigned_agent on public.tasks (assigned_agent_id);

create trigger trg_tasks_updated_at
before update on public.tasks
for each row execute function set_updated_at();

-- Task claim idempotency (prevents duplicate claim on retries)
create table if not exists public.task_claim_idempotency (
  agent_id uuid not null references public.agents(id) on delete cascade,
  idempotency_key text not null,
  channel_id uuid not null references public.channels(id) on delete cascade,
  task_id uuid null references public.tasks(id) on delete set null,
  created_at timestamptz not null default now(),
  primary key (agent_id, idempotency_key)
);

create index if not exists idx_task_claim_idem_created on public.task_claim_idempotency (created_at desc);
create index if not exists idx_task_claim_idem_task on public.task_claim_idempotency (task_id);

-- Events (작업 이력)
create table if not exists public.events (
  id uuid primary key default gen_random_uuid(),
  agent_id uuid not null references public.agents(id) on delete cascade,
  task_id uuid null references public.tasks(id) on delete set null,
  type text not null,
  payload jsonb not null default '{}'::jsonb,
  idempotency_key text null,
  created_at timestamptz not null default now()
);

create index if not exists idx_events_agent_created on public.events (agent_id, created_at desc);
create index if not exists idx_events_task_created on public.events (task_id, created_at desc);
create index if not exists idx_events_created on public.events (created_at desc);

-- Best-effort idempotency: allow client to provide a key and make it unique per agent.
create unique index if not exists uq_events_agent_idempotency
on public.events (agent_id, idempotency_key)
where idempotency_key is not null;

-- FIFO claim helper (returns 0 or 1 rows)
create or replace function public.claim_task(p_channel_id uuid, p_agent_id uuid)
returns setof public.tasks
language plpgsql
as $$
begin
  return query
  with cte as (
    select t.id
    from public.tasks t
    where t.channel_id = p_channel_id
      and t.status = 'queued'
    order by t.created_at asc
    for update skip locked
    limit 1
  )
  update public.tasks t
  set status = 'in_progress',
      assigned_agent_id = p_agent_id,
      claimed_at = now(),
      updated_at = now()
  where t.id in (select id from cte)
  returning t.*;
end;
$$;

-- Retention policy (30 days)
-- Implementation detail depends on the environment:
-- - Option A) pg_cron (if enabled) to run DELETE daily
-- - Option B) Supabase Scheduled Function / Edge Function
-- - Option C) Coordinator background purge loop (MVP default in this repo)
--
-- Example deletion query:
--   delete from public.events where created_at < now() - interval '30 days';
