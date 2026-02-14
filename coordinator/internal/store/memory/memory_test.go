package memory

import (
	"context"
	"strings"
	"testing"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/stretchr/testify/assert"
)

func TestCreateChain(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	// Create a channel first
	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel"})
	assert.NoError(t, err)
	assert.NotEmpty(t, ch.ID)

	// Test case 1: Valid chain creation
	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "test-chain-1",
		Description: "A test chain",
		Status:      model.ChainStatusQueued,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, chain.ID)
	assert.Equal(t, ch.ID, chain.ChannelID)
	assert.Equal(t, "test-chain-1", chain.Name)
	assert.Equal(t, "A test chain", chain.Description)
	assert.Equal(t, model.ChainStatusQueued, chain.Status)
	assert.NotZero(t, chain.CreatedAt)
	assert.NotZero(t, chain.UpdatedAt)

	// Test case 2: Duplicate chain name within the same channel
	_, err = s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "test-chain-1",
		Description: "Another test chain",
		Status:      model.ChainStatusQueued,
	})
	assert.ErrorIs(t, err, store.ErrConflict)

	// Test case 3: Missing channel ID
	_, err = s.CreateChain(ctx, model.Chain{
		Name:        "test-chain-no-channel",
		Description: "A test chain",
		Status:      model.ChainStatusQueued,
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "channel_id_required"))

	// Test case 4: Missing name
	_, err = s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Description: "A test chain",
		Status:      model.ChainStatusQueued,
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "name_required"))

	// Test case 5: Non-existent channel ID
	_, err = s.CreateChain(ctx, model.Chain{
		ChannelID:   "non-existent-channel",
		Name:        "test-chain-invalid-channel",
		Description: "A test chain",
		Status:      model.ChainStatusQueued,
	})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetChain(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel"})
	assert.NoError(t, err)

	createdChain, err := s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "get-chain-test",
		Description: "Test chain for get",
		Status:      model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Test case 1: Get existing chain
	fetchedChain, err := s.GetChain(ctx, createdChain.ID)
	assert.NoError(t, err)
	assert.Equal(t, createdChain.ID, fetchedChain.ID)
	assert.Equal(t, createdChain.Name, fetchedChain.Name)

	// Test case 2: Get non-existent chain
	_, err = s.GetChain(ctx, "non-existent-id")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestListChains(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch1, err := s.CreateChannel(ctx, model.Channel{Name: "list-channel-1"})
	assert.NoError(t, err)
	ch2, err := s.CreateChannel(ctx, model.Channel{Name: "list-channel-2"})
	assert.NoError(t, err)

	// Create chains for ch1
	_, err = s.CreateChain(ctx, model.Chain{ChannelID: ch1.ID, Name: "chain-1-1"})
	assert.NoError(t, err)
	_, err = s.CreateChain(ctx, model.Chain{ChannelID: ch1.ID, Name: "chain-1-2"})
	assert.NoError(t, err)

	// Create chains for ch2
	_, err = s.CreateChain(ctx, model.Chain{ChannelID: ch2.ID, Name: "chain-2-1"})
	assert.NoError(t, err)

	// Test case 1: List all chains
	allChains, err := s.ListChains(ctx, "", "")
	assert.NoError(t, err)
	assert.Len(t, allChains, 3)

	// Test case 2: List chains for ch1
	ch1Chains, err := s.ListChains(ctx, "", ch1.ID)
	assert.NoError(t, err)
	assert.Len(t, ch1Chains, 2)
	for _, c := range ch1Chains {
		assert.Equal(t, ch1.ID, c.ChannelID)
	}

	// Test case 3: List chains for ch2
	ch2Chains, err := s.ListChains(ctx, "", ch2.ID)
	assert.NoError(t, err)
	assert.Len(t, ch2Chains, 1)
	for _, c := range ch2Chains {
		assert.Equal(t, ch2.ID, c.ChannelID)
	}

	// Test case 4: List chains for non-existent channel
	nonExistentChains, err := s.ListChains(ctx, "", "non-existent-channel")
	assert.NoError(t, err) // Should return empty slice, not error
	assert.Len(t, nonExistentChains, 0)
}

func TestUpdateChain(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel"})
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "update-chain-test",
		Description: "Original description",
		Status:      model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Test case 1: Update name, description, status
	updatedChain, err := s.UpdateChain(ctx, model.Chain{
		ID:          chain.ID,
		Name:        "updated-chain-name",
		Description: "New description",
		Status:      model.ChainStatusInProgress,
	})
	assert.NoError(t, err)
	assert.Equal(t, "updated-chain-name", updatedChain.Name)
	assert.Equal(t, "New description", updatedChain.Description)
	assert.Equal(t, model.ChainStatusInProgress, updatedChain.Status)
	assert.True(t, updatedChain.UpdatedAt.After(chain.UpdatedAt))

	// Test case 2: Update only status
	finalChain, err := s.UpdateChain(ctx, model.Chain{
		ID:     chain.ID,
		Name:   updatedChain.Name, // Must provide existing name to avoid conflict/nil
		Status: model.ChainStatusDone,
	})
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, finalChain.Status)
	assert.True(t, finalChain.UpdatedAt.After(updatedChain.UpdatedAt))

	// Test case 3: Update non-existent chain
	_, err = s.UpdateChain(ctx, model.Chain{
		ID:     "non-existent-id",
		Name:   "any-name",
		Status: model.ChainStatusQueued,
	})
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Test case 4: Attempt to update name to a conflicting name within the same channel
	_, err = s.CreateChain(ctx, model.Chain{ChannelID: ch.ID, Name: "another-chain"})
	assert.NoError(t, err)

	_, err = s.UpdateChain(ctx, model.Chain{
		ID:     chain.ID,
		Name:   "another-chain", // This should conflict
		Status: model.ChainStatusQueued,
	})
	assert.ErrorIs(t, err, store.ErrConflict)
}

func TestDeleteChain(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel"})
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID:   ch.ID,
		Name:        "delete-chain-test",
		Description: "Test chain for delete",
		Status:      model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Test case 1: Delete existing chain
	err = s.DeleteChain(ctx, chain.ID)
	assert.NoError(t, err)

	// Verify it's deleted
	_, err = s.GetChain(ctx, chain.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)

	// Test case 2: Delete non-existent chain
	err = s.DeleteChain(ctx, "non-existent-id")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestCreateTaskRequiresChainID(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel-chain-req"})
	assert.NoError(t, err)

	// Creating a task without chain_id should fail
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, Title: "No chain task"})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "chain_id_required"))
}

func TestClaimTaskWithChains(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "chain-test-channel"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "agent-1", Name: "Test Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	// Create a chain
	chain1, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-alpha",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain1
	task1_1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 1, Title: "Task 1.1"})
	assert.NoError(t, err)
	task1_2, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 2, Title: "Task 1.2"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 3, Title: "Task 1.3"})
	assert.NoError(t, err)

	// Create another chain
	chain2, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-beta",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain2
	task2_1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain2.ID, Sequence: 1, Title: "Task 2.1"})
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

	// Test case 4: No more queued tasks at all after completing remaining
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_2.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// task1_3 should now be claimable
	claimedTask, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, "Task 1.3", claimedTask.Title)

	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: claimedTask.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// Chain 1 should be done now
	updatedChain1, err = s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, updatedChain1.Status)

	// Complete remaining chain2 tasks
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task2_1.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// No more queued tasks
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.ErrorIs(t, err, store.ErrNoQueuedTasks)
}

func TestCompleteChainTaskUpdatesChainStatus(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "completion-channel"})
	assert.NoError(t, err)
	agent := model.Agent{ID: "agent-comp", Name: "Completion Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "completion-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Comp Task 1"})
	assert.NoError(t, err)
	task2, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Comp Task 2"})
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

func TestFailChainTaskUpdatesChainStatus(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "failure-channel"})
	assert.NoError(t, err)
	agent := model.Agent{ID: "agent-fail", Name: "Failure Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "failure-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Fail Task 1"})
	assert.NoError(t, err)
	task2, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Fail Task 2"})
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

	// Task 2 should still be claimable because predecessor(1) is terminal(failed)
	claimed2, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task2.ID, claimed2.ID)
	assert.Equal(t, model.TaskStatusInProgress, claimed2.Status)
}

func TestChainOwnership(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	// Setup: Create channel and agents
	ch, err := s.CreateChannel(ctx, model.Channel{Name: "test-channel"})
	assert.NoError(t, err)

	agent1 := model.Agent{ID: "agent-1", Name: "Agent 1"}
	agent2 := model.Agent{ID: "agent-2", Name: "Agent 2"}
	_, err = s.UpsertAgent(ctx, agent1)
	assert.NoError(t, err)
	_, err = s.UpsertAgent(ctx, agent2)
	assert.NoError(t, err)

	// Create two chains with tasks
	chain1, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-1",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	chain2, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "chain-2",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain1
	task1_1, err := s.CreateTask(ctx, model.Task{
		ChannelID: ch.ID,
		ChainID:   chain1.ID,
		Sequence:  1,
		Title:     "Chain 1 Task 1",
		Status:    model.TaskStatusQueued,
	})
	assert.NoError(t, err)

	task1_2, err := s.CreateTask(ctx, model.Task{
		ChannelID: ch.ID,
		ChainID:   chain1.ID,
		Sequence:  2,
		Title:     "Chain 1 Task 2",
		Status:    model.TaskStatusQueued,
	})
	assert.NoError(t, err)

	// Create tasks for chain2
	task2_1, err := s.CreateTask(ctx, model.Task{
		ChannelID: ch.ID,
		ChainID:   chain2.ID,
		Sequence:  1,
		Title:     "Chain 2 Task 1",
		Status:    model.TaskStatusQueued,
	})
	assert.NoError(t, err)

	// Test 1: Agent1 claims first task of chain1
	claimed, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent1.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1_1.ID, claimed.ID)

	// Verify chain1 is now owned by agent1
	updatedChain1, err := s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, agent1.ID, updatedChain1.OwnerAgentID)
	assert.Equal(t, model.ChainStatusInProgress, updatedChain1.Status)

	// Test 2: Agent2 tries to claim from same channel - should get chain2 (not chain1)
	claimed2, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent2.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task2_1.ID, claimed2.ID) // Should get chain2's first task
	assert.NotEqual(t, chain1.ID, claimed2.ChainID)

	// Verify chain2 is now owned by agent2
	updatedChain2, err := s.GetChain(ctx, chain2.ID)
	assert.NoError(t, err)
	assert.Equal(t, agent2.ID, updatedChain2.OwnerAgentID)

	// Test 3: Agent1 tries to claim again - should get next task from chain1 (not chain2)
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent1.ID, ChannelID: ch.ID})
	assert.ErrorIs(t, err, store.ErrNoQueuedTasks) // task1_2 not ready yet (task1_1 not done)

	// Complete task1_1
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_1.ID, AgentID: agent1.ID})
	assert.NoError(t, err)

	// Now agent1 should be able to claim task1_2
	claimed4, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent1.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1_2.ID, claimed4.ID)

	// Test 4: Complete all tasks and verify ownership is released
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_2.ID, AgentID: agent1.ID})
	assert.NoError(t, err)

	// Verify chain1 status is done (ownership persists until explicit detach)
	finalChain1, err := s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, agent1.ID, finalChain1.OwnerAgentID) // Ownership persists
	assert.Equal(t, model.ChainStatusDone, finalChain1.Status)

	// Test 5: Test chain failure on task failure
	_, err = s.FailTask(ctx, store.FailTaskRequest{TaskID: task2_1.ID, AgentID: agent2.ID})
	assert.NoError(t, err)

	// Verify chain2 status is failed (ownership persists until explicit detach)
	finalChain2, err := s.GetChain(ctx, chain2.ID)
	assert.NoError(t, err)
	assert.Equal(t, agent2.ID, finalChain2.OwnerAgentID) // Ownership persists
	assert.Equal(t, model.ChainStatusFailed, finalChain2.Status)
}

func TestDetachAgentFromChain(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "detach-channel"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "agent-detach", Name: "Detach Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "detach-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Detach Task 1"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Detach Task 2"})
	assert.NoError(t, err)

	// Claim first task
	claimed, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1.ID, claimed.ID)

	// Verify chain is in_progress with owner
	updChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusInProgress, updChain.Status)
	assert.Equal(t, agent.ID, updChain.OwnerAgentID)

	// Detach agent from chain
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	// Verify chain is locked with no owner
	updChain, err = s.GetChain(ctx, chain.ID)
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

	// Non-owner cannot detach
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: "some-other-agent",
	})
	assert.ErrorIs(t, err, store.ErrConflict)
}

func TestDetachAgentFromChain_NoInProgressTaskKeepsChainStatus(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "detach-no-progress-channel"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "agent-detach-no-progress", Name: "Detach No Progress Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "detach-no-progress-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{
		ChannelID: ch.ID,
		ChainID:   chain.ID,
		Sequence:  1,
		Title:     "Detach No Progress Task 1",
	})
	assert.NoError(t, err)

	claimed, err := s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, task1.ID, claimed.ID)

	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	chainBeforeDetach, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, chainBeforeDetach.Status)
	assert.Equal(t, agent.ID, chainBeforeDetach.OwnerAgentID)

	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	chainAfterDetach, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, "", chainAfterDetach.OwnerAgentID)
	assert.Equal(t, model.ChainStatusDone, chainAfterDetach.Status)

	tasksAfterDetach, err := s.ListTasks(ctx, store.TaskFilter{ChainID: chain.ID})
	assert.NoError(t, err)
	assert.Len(t, tasksAfterDetach, 1)
	assert.Equal(t, task1.ID, tasksAfterDetach[0].ID)
	assert.Equal(t, model.TaskStatusDone, tasksAfterDetach[0].Status)

	ag, err := s.GetAgent(ctx, agent.ID)
	assert.NoError(t, err)
	assert.Equal(t, "", ag.CurrentTaskID)
}

func TestUpdateTaskStatus(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "status-channel"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "agent-status", Name: "Status Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "status-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	task1, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Status Task 1"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Status Task 2"})
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
	assert.Nil(t, updated.ClaimedAt)

	// Chain should be back to queued
	updChain, err := s.GetChain(ctx, chain.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusQueued, updChain.Status)

	// Claim and detach again
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	// Test locked → done
	updated, err = s.UpdateTaskStatus(ctx, task1.ID, model.TaskStatusDone)
	assert.NoError(t, err)
	assert.Equal(t, model.TaskStatusDone, updated.Status)
	assert.NotNil(t, updated.DoneAt)

	// Cannot update non-locked task
	_, err = s.UpdateTaskStatus(ctx, task1.ID, model.TaskStatusQueued)
	assert.ErrorIs(t, err, store.ErrConflict)

	// Cannot transition to invalid status
	tasks, err := s.ListTasks(ctx, store.TaskFilter{ChainID: chain.ID, Status: model.TaskStatusQueued})
	assert.NoError(t, err)
	// task2 should still be queued
	if len(tasks) > 0 {
		_, err = s.UpdateTaskStatus(ctx, tasks[0].ID, model.TaskStatusDone)
		assert.ErrorIs(t, err, store.ErrConflict) // not locked
	}
}

func TestClaimTaskBlockedByLockedTask(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	ch, err := s.CreateChannel(ctx, model.Channel{Name: "locked-block-channel"})
	assert.NoError(t, err)

	agent := model.Agent{ID: "agent-lock", Name: "Lock Agent"}
	_, err = s.UpsertAgent(ctx, agent)
	assert.NoError(t, err)

	chain, err := s.CreateChain(ctx, model.Chain{
		ChannelID: ch.ID,
		Name:      "locked-block-chain",
		Status:    model.ChainStatusQueued,
	})
	assert.NoError(t, err)

	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 1, Title: "Lock Task 1"})
	assert.NoError(t, err)
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Lock Task 2"})
	assert.NoError(t, err)

	// Claim first task then detach
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	err = s.DetachAgentFromChain(ctx, store.DetachAgentFromChainRequest{
		ChainID: chain.ID,
		AgentID: agent.ID,
	})
	assert.NoError(t, err)

	// Try to claim - should fail because chain has locked task
	_, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.ErrorIs(t, err, store.ErrNoQueuedTasks)
}
