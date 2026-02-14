-- 0012_chain_ownership.sql
-- Add chain ownership to enforce single-chain-per-agent constraint

-- Step 1: Add owner_agent_id to chains table
ALTER TABLE public.chains ADD COLUMN owner_agent_id UUID REFERENCES public.agents(id) ON DELETE SET NULL;

-- Step 2: Create index for faster ownership lookups
CREATE INDEX idx_chains_owner_agent_id ON public.chains(owner_agent_id) WHERE owner_agent_id IS NOT NULL;

-- Step 3: Update claim_task function to enforce chain ownership
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
    -- Check if agent already owns a chain
    SELECT c.id INTO v_owned_chain_id
    FROM public.chains c
    WHERE c.owner_agent_id = p_agent_id
    LIMIT 1;

    -- Case 1: Agent already owns a chain - can only claim tasks from that chain
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

            RETURN;
        END IF;

        -- Agent owns another chain but cannot claim from any other chain.
        RETURN;
    END IF;

    -- Case 2: Agent doesn't own a chain - claim next eligible task from unowned/unlocked chain
    SELECT t.id, t.chain_id
    INTO v_task_id, v_chain_id
    FROM public.tasks t
             JOIN public.chains c ON t.chain_id = c.id
    WHERE t.channel_id = p_channel_id
      AND t.status = 'queued'
      AND c.status != 'locked'
      AND c.owner_agent_id IS NULL  -- Chain must not be owned by anyone
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
        -- Set chain ownership and status
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

        RETURN;
    END IF;

    -- No task available
    RETURN;
END;
$$;

-- Step 4: Create helper function to update chain status (used manually, not auto-triggered)
-- This function updates chain status based on task completion but does NOT release ownership
-- Ownership must be released explicitly via detach API
CREATE OR REPLACE FUNCTION public.update_chain_status(p_chain_id uuid)
    RETURNS VOID
    LANGUAGE plpgsql
AS $$
DECLARE
    v_all_done boolean;
    v_has_failed boolean;
BEGIN
    -- Check if all tasks are done or failed
    SELECT NOT EXISTS (
        SELECT 1
        FROM public.tasks
        WHERE chain_id = p_chain_id
          AND status NOT IN ('done', 'failed')
    ) INTO v_all_done;

    -- Check if any task failed
    SELECT EXISTS (
        SELECT 1
        FROM public.tasks
        WHERE chain_id = p_chain_id
          AND status = 'failed'
    ) INTO v_has_failed;

    -- Update chain status ONLY (ownership remains with agent until explicit detach)
    IF v_all_done THEN
        UPDATE public.chains
        SET status = CASE WHEN v_has_failed THEN 'failed' ELSE 'done' END,
            updated_at = NOW()
        WHERE id = p_chain_id;
    END IF;
END;
$$;

-- Step 5: Create manual detach function (called by API)
CREATE OR REPLACE FUNCTION public.detach_chain_ownership(p_chain_id uuid, p_agent_id uuid)
    RETURNS VOID
    LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE public.chains
    SET owner_agent_id = NULL,
        updated_at = NOW()
    WHERE id = p_chain_id
      AND owner_agent_id = p_agent_id; -- Only owner can detach
END;
$$;
