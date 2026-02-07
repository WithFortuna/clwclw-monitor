-- Add execution_mode column to tasks table
-- This allows specifying Claude Code permission mode per task
-- Valid values: 'accept-edits', 'plan-mode', 'bypass-permission', null (default)

alter table public.tasks
add column execution_mode text null;

-- Add comment for documentation
comment on column public.tasks.execution_mode is
'Claude Code execution mode: accept-edits, plan-mode, bypass-permission, or null (use current mode)';
