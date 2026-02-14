-- =============================================================================
-- clwclw-monitor: Full DB initialization (unified from migrations 0001~0009)
-- Run this on a clean Supabase DB via SQL Editor.
-- =============================================================================

-- UUID generation
create extension if not exists pgcrypto;

-- ─────────────────────────────────────────────────────────────────────────────
-- Common updated_at trigger function
-- ─────────────────────────────────────────────────────────────────────────────
create or replace function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

-- ─────────────────────────────────────────────────────────────────────────────
-- Users (0009)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.users (
  id uuid primary key default gen_random_uuid(),
  username text not null,
  password_hash text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists users_username_lower_idx on public.users (lower(username));

create trigger trg_users_updated_at
before update on public.users
for each row execute function set_updated_at();

-- ─────────────────────────────────────────────────────────────────────────────
-- Agents (0001 + 0003: claude_status column)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.agents (
  id uuid primary key default gen_random_uuid(),
  user_id uuid null references public.users(id),
  name text not null,
  status text not null default 'idle',
  claude_status text not null default 'idle'
    check (claude_status in ('idle', 'running', 'waiting')),
  current_task_id uuid null,
  last_seen timestamptz not null default now(),
  meta jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists idx_agents_last_seen on public.agents (last_seen desc);
create index if not exists idx_agents_claude_status on public.agents (claude_status);
create index if not exists idx_agents_user_id on public.agents (user_id);

create trigger trg_agents_updated_at
before update on public.agents
for each row execute function set_updated_at();

comment on column public.agents.claude_status is
  'Claude Code execution state (idle/running/waiting). Updated via agent heartbeat.';
comment on column public.agents.status is
  'DEPRECATED: Legacy status field. Use claude_status for Claude execution state and derive worker status from last_seen.';

-- ─────────────────────────────────────────────────────────────────────────────
-- Channels (0001, unique name already in DDL)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.channels (
  id uuid primary key default gen_random_uuid(),
  user_id uuid null references public.users(id),
  name text not null unique,
  description text null,
  created_at timestamptz not null default now()
);

create index if not exists idx_channels_user_id on public.channels (user_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- Chains (0004, using set_updated_at instead of moddatetime)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.chains (
  id uuid primary key default gen_random_uuid(),
  user_id uuid null references public.users(id),
  channel_id uuid not null references public.channels(id) on delete cascade,
  name text not null,
  description text,
  status text not null default 'queued',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists idx_chains_user_id on public.chains (user_id);

create trigger trg_chains_updated_at
before update on public.chains
for each row execute function set_updated_at();

-- ─────────────────────────────────────────────────────────────────────────────
-- Tasks (0001 + 0004: chain_id/sequence + 0006: execution_mode + 0007: type)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.tasks (
  id uuid primary key default gen_random_uuid(),
  user_id uuid null references public.users(id),
  channel_id uuid not null references public.channels(id) on delete restrict,
  chain_id uuid null references public.chains(id) on delete set null,
  sequence integer null,
  title text not null,
  description text null,
  type text null,
  agent_session_request_token text null,
  status text not null default 'queued',
  priority int not null default 0,
  assigned_agent_id uuid null references public.agents(id) on delete set null,
  execution_mode text null,
  created_at timestamptz not null default now(),
  claimed_at timestamptz null,
  done_at timestamptz null,
  updated_at timestamptz not null default now()
);

create index if not exists idx_tasks_user_id on public.tasks (user_id);
create index if not exists idx_tasks_channel_status_created on public.tasks (channel_id, status, created_at asc);
create index if not exists idx_tasks_status_created on public.tasks (status, created_at asc);
create index if not exists idx_tasks_assigned_agent on public.tasks (assigned_agent_id);
create index if not exists idx_tasks_chain_id on public.tasks (chain_id);
create unique index if not exists uq_tasks_agent_session_request_token
on public.tasks (agent_session_request_token)
where agent_session_request_token is not null;

create trigger trg_tasks_updated_at
before update on public.tasks
for each row execute function set_updated_at();

comment on column public.tasks.execution_mode is
  'Claude Code execution mode: accept-edits, plan-mode, bypass-permission, or null (use current mode)';

-- ─────────────────────────────────────────────────────────────────────────────
-- Task claim idempotency (0001)
-- ─────────────────────────────────────────────────────────────────────────────
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

-- ─────────────────────────────────────────────────────────────────────────────
-- Events (0001)
-- ─────────────────────────────────────────────────────────────────────────────
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

create unique index if not exists uq_events_agent_idempotency
on public.events (agent_id, idempotency_key)
where idempotency_key is not null;

-- ─────────────────────────────────────────────────────────────────────────────
-- Task inputs (0002)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.task_inputs (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references public.tasks(id) on delete cascade,
  agent_id uuid not null references public.agents(id) on delete cascade,
  kind text not null default 'text',
  text text null,
  send_enter boolean not null default true,
  idempotency_key text null,
  created_at timestamptz not null default now(),
  claimed_at timestamptz null,
  unique (task_id, idempotency_key)
);

create index if not exists idx_task_inputs_pending
  on public.task_inputs (task_id, agent_id, claimed_at, created_at);

-- ─────────────────────────────────────────────────────────────────────────────
-- FIFO claim function (0005: chain-aware version)
-- Takes (channel_id uuid, agent_id uuid), returns SETOF public.tasks
-- ─────────────────────────────────────────────────────────────────────────────
create or replace function public.claim_task(p_channel_id uuid, p_agent_id uuid)
returns setof public.tasks
language plpgsql
as $$
declare
  v_task_id uuid;
  v_chain_id uuid;
begin
  -- 1. Try to claim the next sequential task in an existing 'in_progress' chain
  select t.id, t.chain_id
  into v_task_id, v_chain_id
  from public.tasks t
  join public.chains c on t.chain_id = c.id
  where t.channel_id = p_channel_id
    and t.status = 'queued'
    and c.status = 'in_progress'
    and not exists (
      select 1 from public.tasks
      where chain_id = t.chain_id and sequence < t.sequence and status != 'done'
    )
  order by c.created_at asc, t.sequence asc
  for update skip locked
  limit 1;

  if v_task_id is not null then
    return query
    update public.tasks
    set status = 'in_progress',
        assigned_agent_id = p_agent_id,
        claimed_at = now(),
        updated_at = now()
    where id = v_task_id
    returning *;
    return;
  end if;

  -- 2. Try to claim the first task (sequence 1) of a 'queued' chain
  select t.id, t.chain_id
  into v_task_id, v_chain_id
  from public.tasks t
  join public.chains c on t.chain_id = c.id
  where t.channel_id = p_channel_id
    and t.status = 'queued'
    and c.status = 'queued'
    and t.sequence = 1
  order by c.created_at asc, t.sequence asc
  for update skip locked
  limit 1;

  if v_task_id is not null then
    -- Update chain status to 'in_progress'
    update public.chains
    set status = 'in_progress',
        updated_at = now()
    where id = v_chain_id;

    return query
    update public.tasks
    set status = 'in_progress',
        assigned_agent_id = p_agent_id,
        claimed_at = now(),
        updated_at = now()
    where id = v_task_id
    returning *;
    return;
  end if;

  -- 3. Fallback: Claim the oldest non-chain 'queued' task
  return query
  with cte as (
    select t.id
    from public.tasks t
    where t.channel_id = p_channel_id
      and t.status = 'queued'
      and t.chain_id is null
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

-- ─────────────────────────────────────────────────────────────────────────────
-- Auth codes (0010: agent authentication flow)
-- ─────────────────────────────────────────────────────────────────────────────
create table if not exists public.auth_codes (
  code text primary key,
  user_id uuid not null references public.users(id) on delete cascade,
  agent_name text not null default '',
  expires_at timestamptz not null,
  used boolean not null default false,
  created_at timestamptz not null default now()
);

-- =============================================================================
-- Done. All tables, indexes, triggers, and functions created.
-- =============================================================================
