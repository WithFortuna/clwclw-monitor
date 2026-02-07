-- clwclw-monitor: task inputs (interactive prompts)

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

