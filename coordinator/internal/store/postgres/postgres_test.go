package postgres

import (
	"context"
	"os"
	"testing"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a new PostgreSQL store for testing.
// It skips tests if DATABASE_URL is not set.
// It also applies migrations to the test database.
func setupTestDB(t *testing.T) (*Store, func()) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping PostgreSQL tests")
	}

	// Connect to a clean database (e.g., using a specific test database or template)
	// For simplicity, we'll connect to the main database and assume schema can be reset.
	// In a real scenario, you might create a unique schema/database per test.
	pool, err := pgxpool.New(context.Background(), databaseURL)
	require.NoError(t, err)

	// Apply migrations
	_, err = pool.Exec(context.Background(), `
		-- Drop all tables if they exist
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO postgres;
		GRANT ALL ON SCHEMA public TO public;

		-- UUID generation (Supabase typically supports pgcrypto)
		create extension if not exists pgcrypto;

		-- Common updated_at trigger
		create or replace function set_updated_at()
		returns trigger as $$
		begin
		new.updated_at = now();
		return new;
		end;
		$$ language plpgsql;

		-- Agents
		create table if not exists public.agents (
		id uuid primary key default gen_random_uuid(),
		name text not null,
		status text not null default 'idle',
		claude_status text not null default 'idle', -- Added during development
		current_task_id uuid null,
		last_seen timestamptz not null default now(),
		meta jsonb not null default '{}'::jsonb,
		created_at timestamptz not null default now(),
		updated_at timestamptz not null default now()
		);

		create index if not exists idx_agents_last_seen on public.agents (last_seen desc);

		create trigger trg_agents_updated_at
		before update on public.agents
		for each row execute function set_updated_at();

		-- Channels
		create table if not exists public.channels (
		id uuid primary key default gen_random_uuid(),
		name text not null unique,
		description text null,
		created_at timestamptz not null default now()
		);

		-- Chains (new table)
		CREATE TABLE chains (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE UNIQUE INDEX uq_chains_channel_name ON chains (channel_id, name);
		CREATE TRIGGER handle_updated_at_chains BEFORE UPDATE ON chains
		FOR EACH ROW EXECUTE FUNCTION set_updated_at();

		-- Tasks
		create table if not exists public.tasks (
		id uuid primary key default gen_random_uuid(),
		channel_id uuid not null references public.channels(id) on delete restrict,
		chain_id uuid not null references public.chains(id) on delete cascade, -- All tasks must belong to a chain
		sequence integer null, -- New field
		title text not null,
		description text null,
		status text not null default 'queued',
		priority int not null default 0,
		assigned_agent_id uuid null references public.agents(id) on delete set null,
		created_at timestamptz not null default now(),
		claimed_at timestamptz null,
		done_at timestamptz null,
		updated_at timestamptz not null default now()
		);

		create index if not exists idx_tasks_channel_status_created on public.tasks (channel_id, status, created_at asc);
		create index if not exists idx_tasks_status_created on public.tasks (status, created_at asc);
		create index if not exists idx_tasks_assigned_agent on public.tasks (assigned_agent_id);
		create index if not exists idx_tasks_chain_id on public.tasks(chain_id); -- New index

		create trigger trg_tasks_updated_at
		before update on public.tasks
		for each row execute function set_updated_at();

		-- Task claim idempotency (prevents duplicate claim on retries)
		create table if not exists public.task_claim_idempotency (
		agent_id uuid not null references public.agents(id) on delete cascade,
		idempotency_key text not null,
		channel_id uuid not null references public.channels(id) on delete cascade,
		task_id uuid null references public.tasks(id) on delete set null,
		created_at timestamptz not null default now(),
		primary key (agent_id, idempotency_key)
		);

		create index if not exists idx_task_claim_idem_created on public.task_claim_idempotency (created_at desc);
		create index if not exists idx_task_claim_idem_task on public.task_claim_idempotency (task_id);

		-- Events (작업 이력)
		create table if not exists public.events (
		id uuid primary key default gen_random_uuid(),
		agent_id uuid not null references public.agents(id) on delete cascade,
		task_id uuid null references public.tasks(id) on delete set null,
		type text not null,
		payload jsonb not null default '{}'::jsonb,
		idempotency_key text null,
		created_at timestamptz not null default now()
		);

		create index if not exists idx_events_agent_created on public.events (agent_id, created_at desc);
		create index if not exists idx_events_task_created on public.events (task_id, created_at desc);
		create index if not exists idx_events_created on public.events (created_at desc);

		-- Best-effort idempotency: allow client to provide a key and make it unique per agent.
		create unique index if not exists uq_events_agent_idempotency
		on public.events (agent_id, idempotency_key)
		where idempotency_key is not null;

		-- Task Inputs
		CREATE TABLE task_inputs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			text TEXT,
			send_enter BOOLEAN NOT NULL DEFAULT FALSE,
			idempotency_key TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			claimed_at TIMESTAMP WITH TIME ZONE
		);
		CREATE UNIQUE INDEX uq_task_inputs_task_idempotency ON task_inputs (task_id, idempotency_key) WHERE idempotency_key IS NOT NULL;


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

			-- No standalone fallback - all tasks must belong to a chain
			RETURN;
		END;
		$$;
	`)
	require.NoError(t, err)

	s := &Store{pool: pool}

	return s, func() {
		// Teardown: close pool
		pool.Close()
	}
}

func TestPostgresStore_ChainCRUD(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	// Create a channel first
	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel-pg"})
	assert.NoError(t, err)
	assert.NotEmpty(t, ch.ID)

	// Test CreateChain
	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "test-chain-pg-1",
		Description: "A test chain for postgres",
		Status:      model.ChainStatusQueued,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, chain.ID)
	assert.Equal(t, ch.ID, chain.ChannelID)
	assert.Equal(t, "test-chain-pg-1", chain.Name)
	assert.Equal(t, "A test chain for postgres", chain.Description)
	assert.Equal(t, model.ChainStatusQueued, chain.Status)
	assert.NotZero(t, chain.CreatedAt)
	assert.NotZero(t, chain.UpdatedAt)

	// Test GetChain
	fetchedChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, chain.ID, fetchedChain.ID)
	assert.Equal(t, chain.Name, fetchedChain.Name)

	// Test UpdateChain
	updatedChainName := "updated-chain-pg-1"
	updatedChainDesc := "Updated description for postgres"
	updatedChainStatus := model.ChainStatusInProgress
	updatedChain, err := s.UpdateChain(ctx, model.Chain{
		ID:          chain.ID,
		Name:        updatedChainName,
		Description: updatedChainDesc,
		Status:      updatedChainStatus,
	})
	assert.NoError(t, err)
	assert.Equal(t, updatedChainName, updatedChain.Name)
	assert.Equal(t, updatedChainDesc, updatedChain.Description)
	assert.Equal(t, updatedChainStatus, updatedChain.Status)
	assert.True(t, updatedChain.UpdatedAt.After(chain.UpdatedAt))

	// Test ListChains
	listedChains, err := s.ListChains(ctx, "", ch.ID)
	assert.NoError(t, err)
	assert.Len(t, listedChains, 1)
	assert.Equal(t, updatedChain.ID, listedChains[0].ID)

	// Test DeleteChain
	err = s.DeleteChain(ctx, chain.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = s.GetChain(ctx, chain.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestPostgresStore_CreateTaskRequiresChainID(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "chain-req-test-pg"})
	assert.NoError(t, err)

	// Creating a task without chain_id should fail
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, Title: "No chain task PG"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chain_id_required")
}

func TestPostgresStore_ClaimTaskWithChains(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "chain-test-channel-pg"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "e4414f52-8700-47e2-8926-27a92c4e5124", Name: "Test Agent PG"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	// Create a chain
	chain1, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-alpha-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain1
	task1_1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 1, Title: "Task 1.1 PG"})
	assert.NoError(t, err)
	task1_2, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 2, Title: "Task 1.2 PG"})
	assert.NoError(t, err)
	task1_3, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 3, Title: "Task 1.3 PG"})
	assert.NoError(t, err)

	// Create another chain
	chain2, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-beta-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain2
	task2_1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain2.ID, Sequence: 1, Title: "Task 2.1 PG"})
	assert.NoError(t, err)

	// Test case 1: Claim first task of chain-alpha (should make chain-alpha InProgress)
	claimedTask, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1_1.ID, claimedTask.ID)
	assert.Equal(t, model.TaskStatusInProgress, claimedTask.Status)

	// Check chain status updated
	updatedChain1, err := s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusInProgress, updatedChain1.Status)

	// Test case 2: Claim another task. Should be task 2.1 (from chain beta)
	claimedTask, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task2_1.ID, claimedTask.ID)
	assert.Equal(t, model.TaskStatusInProgress, claimedTask.Status)

	updatedChain2, err := s.GetChain(ctx, chain2.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusInProgress, updatedChain2.Status)

	// Test case 3: Complete task 1.1, then claim next (should be 1.2)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_1.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	claimedTask, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1_2.ID, claimedTask.ID)

	// Test case 4: Complete task 1.2, then claim next (should be 1.3)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_2.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	claimedTask, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1_3.ID, claimedTask.ID)

	// Test case 5: Complete task 1.3. Chain 1 should be done now.
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_3.ID, AgentID: agent.ID})
	assert.NoError(t, err)
	updatedChain1, err = s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, updatedChain1.Status)

	// Test case 6: No more queued tasks at all
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task2_1.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.ErrorIs(t, err, store.ErrNoQueuedTasks)
}

func TestPostgresStore_CompleteChainTaskUpdatesChainStatus(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "completion-channel-pg"})
	assert.NoError(t, err)
	agent := model.Agent{ID: "d846c827-0248-43e8-a3f2-17726359e931", Name: "Completion Agent PG"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "completion-chain-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Comp Task 1 PG"})
	assert.NoError(t, err)
	task2, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Comp Task 2 PG"})
	assert.NoError(t, err)

	// Claim and complete task 1
	claimed1, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1.ID, claimed1.ID)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// Chain should still be InProgress
	updatedChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusInProgress, updatedChain.Status)

	// Claim and complete task 2 (last task)
	claimed2, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task2.ID, claimed2.ID)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task2.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// Chain should now be Done
	updatedChain, err = s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, updatedChain.Status)
}

func TestPostgresStore_FailChainTaskUpdatesChainStatus(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "failure-channel-pg"})
	assert.NoError(t, err)
	agent := model.Agent{ID: "c18f3d13-6447-49f3-b3c0-3b4f9f23e61c", Name: "Failure Agent PG"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "failure-chain-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Fail Task 1 PG"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Fail Task 2 PG"})
	assert.NoError(t, err)

	// Claim task 1
	claimed1, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1.ID, claimed1.ID)

	// Fail task 1
	_, err = s.FailTask(ctx, store.FailTaskRequest{TaskID: task1.ID, AgentID: agent.ID, Reason: "simulated failure"})
	assert.NoError(t, err)

	// Chain should now be Failed
	updatedChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusFailed, updatedChain.Status)

	// Try to claim task 2 (should not be claimable as chain is failed)
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.ErrorIs(t, err, store.ErrNoQueuedTasks) // Expect no more queued tasks for this chain
}

func TestPostgresStore_DetachAgentFromChain(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "detach-channel-pg"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", Name: "Detach Agent PG"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "detach-chain-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Detach Task 1 PG"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Detach Task 2 PG"})
	assert.NoError(t, err)

	// Claim first task
	claimed, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1.ID, claimed.ID)

	// Detach agent
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	// Verify chain is locked with no owner
	updChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusLocked, updChain.Status)
	assert.Equal(t, "", updChain.OwnerAgentID)

	// Verify task is locked
	tasks, err := s.ListTasks(ctx, store.TaskFilter{ChainID: chain.ID})
	assert.NoError(t, err)
	var lockedTask *model.Task
	for _, tk := range tasks {
		if tk.Status == model.TaskStatusLocked {
			lockedTask = &tk
			break
		}
	}
	assert.NotNil(t, lockedTask)
	assert.Equal(t, task1.ID, lockedTask.ID)

	// Verify agent's current_task_id is cleared
	ag, err := s.GetAgent(ctx, agent.ID)
	assert.NoError(t, err)
	assert.Equal(t, "", ag.CurrentTaskID)
}

func TestPostgresStore_UpdateTaskStatus(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "status-channel-pg"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "b2c3d4e5-f6a7-8901-bcde-f12345678901", Name: "Status Agent PG"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "status-chain-pg",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Status Task 1 PG"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Status Task 2 PG"})
	assert.NoError(t, err)

	// Claim and detach to get a locked task
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	// Test locked → queued
	updated, err := s.UpdateTaskStatus(ctx, task1.ID, model.TaskStatusQueued)
	assert.NoError(t, err)
	assert.Equal(t, model.TaskStatusQueued, updated.Status)
	assert.Equal(t, "", updated.AssignedAgentID)

	// Test locked → done (claim and detach again first)
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	updated, err = s.UpdateTaskStatus(ctx, task1.ID, model.TaskStatusDone)
	assert.NoError(t, err)
	assert.Equal(t, model.TaskStatusDone, updated.Status)

	// Cannot update non-locked task
	_, err = s.UpdateTaskStatus(ctx, task1.ID, model.TaskStatusQueued)
	assert.ErrorIs(t, err, store.ErrConflict)
}
