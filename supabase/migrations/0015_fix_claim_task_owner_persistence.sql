-- 0015_fix_claim_task_owner_persistence.sql
-- Keep chain ownership until explicit detach (or lease expiry policy),
-- and ensure only owner can claim newly added tasks in owned chains.

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
    v_owned_chain_id uuid;
BEGIN
    -- Ownership persists regardless of chain status.
    SELECT c.id INTO v_owned_chain_id
    FROM public.chains c
    WHERE c.owner_agent_id = p_agent_id
    LIMIT 1;

    -- Owner can only claim from the owned chain.
    IF v_owned_chain_id IS NOT NULL THEN
        SELECT t.id, t.chain_id
        INTO v_task_id, v_chain_id
        FROM public.tasks t
        WHERE t.channel_id = p_channel_id
          AND t.chain_id = v_owned_chain_id
          AND t.status = 'queued'
          AND NOT EXISTS (
            SELECT 1
            FROM public.tasks
            WHERE chain_id = t.chain_id
              AND status = 'locked'
        )
          AND NOT EXISTS (
            SELECT 1
            FROM public.tasks
            WHERE chain_id = t.chain_id
              AND sequence < t.sequence
              AND status != 'done'
        )
        ORDER BY t.sequence ASC
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
        END IF;

        RETURN;
    END IF;

    -- Unowned agents can claim from unowned/unlocked chains only.
    SELECT t.id, t.chain_id
    INTO v_task_id, v_chain_id
    FROM public.tasks t
             JOIN public.chains c ON t.chain_id = c.id
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND c.status != 'locked'
      AND c.owner_agent_id IS NULL
      AND NOT EXISTS (
        SELECT 1
        FROM public.tasks
        WHERE chain_id = t.chain_id
          AND status = 'locked'
    )
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
        UPDATE public.chains
        SET status = 'in_progress',
            owner_agent_id = p_agent_id,
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
    END IF;

    RETURN;
END;
$$;
