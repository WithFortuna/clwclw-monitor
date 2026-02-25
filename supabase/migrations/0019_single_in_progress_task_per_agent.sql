-- Enforce at most one in-progress task per agent.
-- done/failed tasks may keep assigned_agent_id for history.
create unique index if not exists uq_tasks_single_in_progress_per_agent
on public.tasks (assigned_agent_id)
where status = 'in_progress' and assigned_agent_id is not null;
