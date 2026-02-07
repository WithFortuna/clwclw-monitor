-- Migration: Separate Agent Status and Claude Code Execution Status
-- Issue: #1 - [fix] 클로드 코드 에이전트 상태관리
--
-- Adds claude_status column to track Claude Code execution state separately
-- from worker lifecycle (which is derived from last_seen timestamp).

-- Add claude_status column to agents table
alter table public.agents
  add column claude_status text not null default 'idle'
  check (claude_status in ('idle', 'running', 'waiting'));

-- Migrate existing status values to claude_status
update public.agents set claude_status = status;

-- Add index on claude_status for filtering
create index if not exists idx_agents_claude_status on public.agents(claude_status);

-- Update claim_task() stored procedure to remove automatic agent status update
-- This makes agent heartbeat the sole source of truth for Claude status
create or replace function public.claim_task(
  p_channel_id uuid,
  p_agent_id text,
  p_idempotency_key text default null
)
returns table (
  task_id uuid,
  task_channel_id uuid,
  task_title text,
  task_description text,
  task_status text,
  task_priority int,
  task_assigned_agent_id text,
  task_created_at timestamptz,
  task_claimed_at timestamptz,
  task_done_at timestamptz,
  task_updated_at timestamptz
)
language plpgsql
as $$
declare
  v_task record;
  v_now timestamptz := now();
  v_existing_claim record;
begin
  -- Check for existing claim with this idempotency key
  if p_idempotency_key is not null then
    select * into v_existing_claim
    from public.task_claims
    where idempotency_key = p_idempotency_key
      and agent_id = p_agent_id
    limit 1;

    if found then
      -- Return the previously claimed task
      return query
      select
        t.id,
        t.channel_id,
        t.title,
        t.description,
        t.status,
        t.priority,
        t.assigned_agent_id,
        t.created_at,
        t.claimed_at,
        t.done_at,
        t.updated_at
      from public.tasks t
      where t.id = v_existing_claim.task_id;
      return;
    end if;
  end if;

  -- FIFO claim: oldest queued task in channel
  select * into v_task
  from public.tasks
  where channel_id = p_channel_id
    and status = 'queued'
  order by created_at asc
  for update skip locked
  limit 1;

  if not found then
    return; -- No tasks available
  end if;

  -- Update task to in_progress
  update public.tasks
  set
    status = 'in_progress',
    assigned_agent_id = p_agent_id,
    claimed_at = v_now,
    updated_at = v_now
  where id = v_task.id;

  -- Record claim for idempotency
  if p_idempotency_key is not null then
    insert into public.task_claims (task_id, agent_id, idempotency_key, claimed_at)
    values (v_task.id, p_agent_id, p_idempotency_key, v_now)
    on conflict (idempotency_key, agent_id) do nothing;
  end if;

  -- NOTE: Removed automatic agent status update
  -- Agent heartbeat is now sole source of truth for claude_status

  -- Return claimed task
  return query
  select
    v_task.id,
    v_task.channel_id,
    v_task.title,
    v_task.description,
    'in_progress'::text as status,
    v_task.priority,
    p_agent_id,
    v_task.created_at,
    v_now as claimed_at,
    v_task.done_at,
    v_now as updated_at;
end;
$$;

-- Add comment documenting the change
comment on column public.agents.claude_status is
  'Claude Code execution state (idle/running/waiting). Updated via agent heartbeat.';

comment on column public.agents.status is
  'DEPRECATED: Legacy status field. Use claude_status for Claude execution state and derive worker status from last_seen.';
