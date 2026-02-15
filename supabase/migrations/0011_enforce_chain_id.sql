-- 0011_enforce_chain_id.sql
-- Enforce that all tasks must belong to a chain.

-- Step 1: Auto-create chains for any existing standalone tasks
DO $$
DECLARE
    r RECORD;
    v_chain_id UUID;
BEGIN
    FOR r IN
        SELECT DISTINCT t.channel_id, t.user_id
        FROM public.tasks t
        WHERE t.chain_id IS NULL
    LOOP
        INSERT INTO public.chains (channel_id, name, description, status, user_id)
        VALUES (r.channel_id, 'Migrated Standalone Tasks', 'Auto-created chain for previously standalone tasks', 'queued', r.user_id)
        RETURNING id INTO v_chain_id;

        UPDATE public.tasks
        SET chain_id = v_chain_id, sequence = 1
        WHERE chain_id IS NULL AND channel_id = r.channel_id;
    END LOOP;
END;
$$;

-- Step 2: Make chain_id NOT NULL
ALTER TABLE public.tasks ALTER COLUMN chain_id SET NOT NULL;

-- Step 3: Change ON DELETE SET NULL to ON DELETE CASCADE
ALTER TABLE public.tasks DROP CONSTRAINT IF EXISTS tasks_chain_id_fkey;
ALTER TABLE public.tasks ADD CONSTRAINT tasks_chain_id_fkey
    FOREIGN KEY (chain_id) REFERENCES public.chains(id) ON DELETE CASCADE;

-- Step 4: Update claim_task function to remove standalone fallback
DROP FUNCTION IF EXISTS public.claim_task(uuid, uuid);

CREATE OR REPLACE FUNCTION public.claim_task(
    p_channel_id uuid,
    p_agent_id uuid
)
    RETURNS SETOF public.tasks
    LANGUAGE plpgsql
AS $$
DECLARE
    v_task_id uuid;
    v_chain_id uuid;
BEGIN
    -- 1. in_progress chain의 다음 task
    SELECT t.id, t.chain_id
    INTO v_task_id, v_chain_id
    FROM public.tasks t
             JOIN public.chains c ON t.chain_id = c.id
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND c.status = 'in_progress'
      AND NOT EXISTS (
        SELECT 1
        FROM public.tasks
        WHERE chain_id = t.chain_id
          AND sequence < t.sequence
          AND status != 'done'
    )
    ORDER BY c.created_at ASC, t.sequence ASC
        FOR UPDATE SKIP LOCKED
    LIMIT 1;

    IF v_task_id IS NOT NULL THEN
        RETURN QUERY
            UPDATE public.tasks
                SET status = 'in_progress',
                    assigned_agent_id = p_agent_id,
                    claimed_at = NOW(),
                    updated_at = NOW()
                WHERE id = v_task_id
                RETURNING *;

        RETURN;
    END IF;

    -- 2. queued chain의 첫 task
    SELECT t.id, t.chain_id
    INTO v_task_id, v_chain_id
    FROM public.tasks t
             JOIN public.chains c ON t.chain_id = c.id
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND c.status = 'queued'
      AND t.sequence = 1
    ORDER BY c.created_at ASC, t.sequence ASC
        FOR UPDATE SKIP LOCKED
    LIMIT 1;

    IF v_task_id IS NOT NULL THEN
        UPDATE public.chains
        SET status = 'in_progress',
            updated_at = NOW()
        WHERE id = v_chain_id;

        RETURN QUERY
            UPDATE public.tasks
                SET status = 'in_progress',
                    assigned_agent_id = p_agent_id,
                    claimed_at = NOW(),
                    updated_at = NOW()
                WHERE id = v_task_id
                RETURNING *;

        RETURN;
    END IF;

    -- 아무 것도 못 잡았을 때
    RETURN;
END;
$$;
