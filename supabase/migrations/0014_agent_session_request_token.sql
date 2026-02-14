-- Add token used to correlate request_claude_session task completion events.
alter table public.tasks
add column if not exists agent_session_request_token text null;

create unique index if not exists uq_tasks_agent_session_request_token
on public.tasks (agent_session_request_token)
where agent_session_request_token is not null;
