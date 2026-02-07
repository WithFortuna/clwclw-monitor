-- 0005_update_claim_task_function.sql

-- Drop the old claim_task function
DROP FUNCTION IF EXISTS public.claim_task(uuid, uuid);

-- Recreate the claim_task function with chain-aware logic
CREATE OR REPLACE FUNCTION public.claim_task(p_channel_id uuid, p_agent_id uuid)
RETURNS SETOF public.tasks
LANGUAGE plpgsql
AS $$
DECLARE
    v_task_id uuid;
    v_chain_id uuid;
BEGIN
    -- 1. Try to claim the next sequential task in an existing 'in_progress' chain
    SELECT t.id, t.chain_id
    INTO v_task_id, v_chain_id
    FROM public.tasks t
    JOIN public.chains c ON t.chain_id = c.id
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND c.status = 'in_progress'
      AND NOT EXISTS (SELECT 1 FROM public.tasks WHERE chain_id = t.chain_id AND sequence < t.sequence AND status != 'done')
    ORDER BY c.created_at ASC, t.sequence ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1;

    IF v_task_id IS NOT NULL THEN
        UPDATE public.tasks
        SET status = 'in_progress',
            assigned_agent_id = p_agent_id,
            claimed_at = NOW(),
            updated_at = NOW()
        WHERE id = v_task_id
        RETURNING *;
        RETURN NEXT;
        RETURN;
    END IF;

    -- 2. Try to claim the first task (sequence 1) of a 'queued' chain
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
        -- Update chain status to 'in_progress'
        UPDATE public.chains
        SET status = 'in_progress',
            updated_at = NOW()
        WHERE id = v_chain_id;

        UPDATE public.tasks
        SET status = 'in_progress',
            assigned_agent_id = p_agent_id,
            claimed_at = NOW(),
            updated_at = NOW()
        WHERE id = v_task_id
        RETURNING *;
        RETURN NEXT;
        RETURN;
    END IF;

    -- 3. Fallback: Claim the oldest non-chain 'queued' task
    SELECT t.id
    INTO v_task_id
    FROM public.tasks t
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND t.chain_id IS NULL
    ORDER BY t.created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1;

    IF v_task_id IS NOT NULL THEN
        UPDATE public.tasks
        SET status = 'in_progress',
            assigned_agent_id = p_agent_id,
            claimed_at = NOW(),
            updated_at = NOW()
        WHERE id = v_task_id
        RETURNING *;
        RETURN NEXT;
        RETURN;
    END IF;

    RETURN;
END;
$$;