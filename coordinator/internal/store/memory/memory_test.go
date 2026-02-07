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
	allChains, err := s.ListChains(ctx, "")
	assert.NoError(t, err)
	assert.Len(t, allChains, 3)

	// Test case 2: List chains for ch1
	ch1Chains, err := s.ListChains(ctx, ch1.ID)
	assert.NoError(t, err)
	assert.Len(t, ch1Chains, 2)
	for _, c := range ch1Chains {
		assert.Equal(t, ch1.ID, c.ChannelID)
	}

	// Test case 3: List chains for ch2
	ch2Chains, err := s.ListChains(ctx, ch2.ID)
	assert.NoError(t, err)
	assert.Len(t, ch2Chains, 1)
	for _, c := range ch2Chains {
		assert.Equal(t, ch2.ID, c.ChannelID)
	}

	// Test case 4: List chains for non-existent channel
	nonExistentChains, err := s.ListChains(ctx, "non-existent-channel")
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
	task1_3, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain1.ID, Sequence: 3, Title: "Task 1.3"})
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

	// Create a non-chain task
	nonChainTask, err := s.CreateTask(ctx, model.Task{ChannelID: ch.ID, Title: "Non-chain Task"})
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

	// Test case 2: Claim another task. Should be task 2.1 (from chain beta) as chain alpha still has in-progress tasks
	// However, the current logic is to check if the previous task is done.
	// So, task 1.2 is not claimable yet.
	// The oldest *eligible* task should be task 2.1.
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

	// Test case 4: No more queued tasks in current chains, claim non-chain task
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_2.ID, AgentID: agent.ID})
	assert.NoError(t, err)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task1_3.ID, AgentID: agent.ID})
	assert.NoError(t, err)

	// Chain 1 should be done now
	updatedChain1, err = s.GetChain(ctx, chain1.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.ChainStatusDone, updatedChain1.Status)

	// There is no eligible task from Chain2 since Task2_1 is already claimed.
	// Therefore, the next claimed task should be the Non-Chain Task.
	claimedTask, err = s.ClaimTask(ctx, store.ClaimTaskRequest{AgentID: agent.ID, ChannelID: ch.ID})
	assert.NoError(t, err)
	assert.Equal(t, nonChainTask.ID, claimedTask.ID)

	// Test case 5: No more queued tasks at all
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: task2_1.ID, AgentID: agent.ID})
	assert.NoError(t, err)
	_, err = s.CompleteTask(ctx, store.CompleteTaskRequest{TaskID: nonChainTask.ID, AgentID: agent.ID})
	assert.NoError(t, err)

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
	_, err = s.CreateTask(ctx, model.Task{ChannelID: ch.ID, ChainID: chain.ID, Sequence: 2, Title: "Fail Task 2"})
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
