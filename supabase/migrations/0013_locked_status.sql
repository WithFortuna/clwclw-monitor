-- Add 'locked' to task and chain status values.
-- PostgreSQL text columns don't have enum constraints by default in this schema,
-- so this migration is mainly documentation. If check constraints exist, update them:

-- Update task status check constraint if it exists
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_name = 'tasks_status_check' AND table_name = 'tasks'
  ) THEN
    ALTER TABLE public.tasks DROP CONSTRAINT tasks_status_check;
    ALTER TABLE public.tasks ADD CONSTRAINT tasks_status_check
      CHECK (status IN ('queued', 'in_progress', 'done', 'failed', 'locked'));
  END IF;
END $$;

-- Update chain status check constraint if it exists
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_name = 'chains_status_check' AND table_name = 'chains'
  ) THEN
    ALTER TABLE public.chains DROP CONSTRAINT chains_status_check;
    ALTER TABLE public.chains ADD CONSTRAINT chains_status_check
      CHECK (status IN ('queued', 'in_progress', 'done', 'failed', 'locked'));
  END IF;
END $$;
