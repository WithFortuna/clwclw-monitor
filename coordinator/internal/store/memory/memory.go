package memory

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

type Store struct {
	mu sync.Mutex

	agents    map[string]model.Agent
	channels  map[string]model.Channel
	chains    map[string]model.Chain
	tasks     map[string]model.Task
	events    map[string]model.Event
	inputs    map[string]model.TaskInput
	users     map[string]model.User
	authCodes map[string]model.AuthCode

	claimIdem map[string]string
	inputIdem map[string]string

	// simple idempotency tracking (best-effort for in-memory phase)
	idem map[string]struct{}
}

func NewStore() *Store {
	return &Store{
		agents:    make(map[string]model.Agent),
		channels:  make(map[string]model.Channel),
		chains:    make(map[string]model.Chain),
		tasks:     make(map[string]model.Task),
		events:    make(map[string]model.Event),
		inputs:    make(map[string]model.TaskInput),
		users:     make(map[string]model.User),
		authCodes: make(map[string]model.AuthCode),
		claimIdem: make(map[string]string),
		inputIdem: make(map[string]string),
		idem:      make(map[string]struct{}),
	}
}

func (s *Store) UpsertAgent(_ context.Context, a model.Agent) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if strings.TrimSpace(a.ID) == "" {
		a.ID = newID()
	}

	existing, ok := s.agents[a.ID]
	if ok {
		if a.Name != "" {
			existing.Name = a.Name
		}
		if a.Status != "" {
			existing.Status = a.Status
		}
		if a.ClaudeStatus != "" {
			existing.ClaudeStatus = a.ClaudeStatus
		}
		if a.CurrentTaskID != "" {
			existing.CurrentTaskID = a.CurrentTaskID
		}
		if a.Meta != nil {
			existing.Meta = a.Meta
		}
		existing.LastSeen = now
		existing.UpdatedAt = now
		s.agents[a.ID] = existing
		return existing, nil
	}

	if a.Status == "" {
		a.Status = model.AgentStatusIdle
	}
	if a.ClaudeStatus == "" {
		a.ClaudeStatus = model.ClaudeStatusIdle
	}
	a.LastSeen = now
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Meta == nil {
		a.Meta = map[string]any{}
	}
	s.agents[a.ID] = a
	return a, nil
}

func (s *Store) GetAgent(_ context.Context, id string) (*model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.agents[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &a, nil
}

func (s *Store) ListAgents(_ context.Context, userID string) ([]model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Agent, 0, len(s.agents))
	for _, a := range s.agents {
		if userID != "" && a.UserID != userID {
			continue
		}
		out = append(out, a)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out, nil
}

func (s *Store) CreateChannel(_ context.Context, ch model.Channel) (model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(ch.Name) == "" {
		return model.Channel{}, errWithCode("name_required")
	}

	for _, existing := range s.channels {
		if strings.EqualFold(existing.Name, ch.Name) {
			return model.Channel{}, store.ErrConflict
		}
	}

	ch.ID = newID()
	ch.CreatedAt = time.Now().UTC()
	s.channels[ch.ID] = ch
	return ch, nil
}

func (s *Store) ListChannels(_ context.Context, userID string) ([]model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Channel, 0, len(s.channels))
	for _, c := range s.channels {
		if userID != "" && c.UserID != userID {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetChannelByName(_ context.Context, name string) (model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.channels {
		if ch.Name == name {
			return ch, nil
		}
	}
	return model.Channel{}, store.ErrNotFound
}

func (s *Store) CreateChain(_ context.Context, c model.Chain) (model.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(c.ChannelID) == "" {
		return model.Chain{}, errWithCode("channel_id_required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return model.Chain{}, errWithCode("name_required")
	}
	if _, ok := s.channels[c.ChannelID]; !ok {
		return model.Chain{}, store.ErrNotFound
	}

	// Removed: chain name uniqueness check - allow duplicate names within same channel

	now := time.Now().UTC()
	c.ID = newID()
	if c.Status == "" {
		c.Status = model.ChainStatusQueued
	}
	c.CreatedAt = now
	c.UpdatedAt = now
	s.chains[c.ID] = c
	return c, nil
}

func (s *Store) GetChain(_ context.Context, id string) (model.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.chains[id]
	if !ok {
		return model.Chain{}, store.ErrNotFound
	}
	return c, nil
}

func (s *Store) ListChains(_ context.Context, userID string, channelID string) ([]model.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Chain, 0, len(s.chains))
	for _, c := range s.chains {
		if userID != "" && c.UserID != userID {
			continue
		}
		if channelID != "" && c.ChannelID != channelID {
			continue
		}
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) UpdateChain(_ context.Context, c model.Chain) (model.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.chains[c.ID]
	if !ok {
		return model.Chain{}, store.ErrNotFound
	}

	// Removed: chain name uniqueness check - allow duplicate names within same channel
	if strings.TrimSpace(c.Name) != "" {
		existing.Name = c.Name
	}

	if c.Description != "" {
		existing.Description = c.Description
	}
	if c.Status != "" {
		existing.Status = c.Status
	}
	// Allow updating OwnerAgentID (including setting to empty string to release ownership)
	if c.OwnerAgentID != existing.OwnerAgentID {
		existing.OwnerAgentID = c.OwnerAgentID
	}

	existing.UpdatedAt = time.Now().UTC()
	s.chains[existing.ID] = existing
	return existing, nil
}

func (s *Store) DeleteChain(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.chains[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.chains, id)
	return nil
}

func (s *Store) DetachAgentFromChain(_ context.Context, req store.DetachAgentFromChainRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	chainID := strings.TrimSpace(req.ChainID)
	agentID := strings.TrimSpace(req.AgentID)
	if chainID == "" {
		return errWithCode("chain_id_required")
	}
	if agentID == "" {
		return errWithCode("agent_id_required")
	}

	chain, ok := s.chains[chainID]
	if !ok {
		return store.ErrNotFound
	}

	// Only the current owner can be detached
	if chain.OwnerAgentID != agentID {
		return store.ErrConflict
	}

	now := time.Now().UTC()

	// Find in_progress task in this chain and set it to locked
	for id, t := range s.tasks {
		if t.ChainID == chainID && t.Status == model.TaskStatusInProgress {
			t.Status = model.TaskStatusLocked
			t.UpdatedAt = now
			s.tasks[id] = t
			break
		}
	}

	// Clear chain ownership and set chain status to locked
	chain.OwnerAgentID = ""
	chain.Status = model.ChainStatusLocked
	chain.UpdatedAt = now
	s.chains[chainID] = chain

	// Clear agent's current_task_id
	if agent, ok := s.agents[agentID]; ok {
		agent.CurrentTaskID = ""
		agent.UpdatedAt = now
		s.agents[agentID] = agent
	}

	return nil
}

func (s *Store) UpdateTaskStatus(_ context.Context, taskID string, newStatus model.TaskStatus) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errWithCode("task_id_required")
	}

	t, ok := s.tasks[taskID]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Only allow transitions from locked
	if t.Status != model.TaskStatusLocked {
		return nil, store.ErrConflict
	}

	// Only allow locked → queued or locked → done
	if newStatus != model.TaskStatusQueued && newStatus != model.TaskStatusDone {
		return nil, store.ErrConflict
	}

	now := time.Now().UTC()

	if newStatus == model.TaskStatusQueued {
		t.Status = model.TaskStatusQueued
		t.AssignedAgentID = ""
		t.ClaimedAt = nil
		t.UpdatedAt = now
	} else { // done
		t.Status = model.TaskStatusDone
		t.DoneAt = &now
		t.UpdatedAt = now
	}
	s.tasks[taskID] = t

	// Re-evaluate chain status
	if t.ChainID != "" {
		s.reevaluateChainStatus(t.ChainID, now)
	}

	return &t, nil
}

// reevaluateChainStatus checks chain tasks and updates chain status accordingly.
// Must be called with s.mu held.
func (s *Store) reevaluateChainStatus(chainID string, now time.Time) {
	chain, ok := s.chains[chainID]
	if !ok {
		return
	}

	hasLocked := false
	hasInProgress := false
	hasQueued := false
	allDoneOrFailed := true
	hasFailed := false

	for _, t := range s.tasks {
		if t.ChainID != chainID {
			continue
		}
		switch t.Status {
		case model.TaskStatusLocked:
			hasLocked = true
			allDoneOrFailed = false
		case model.TaskStatusInProgress:
			hasInProgress = true
			allDoneOrFailed = false
		case model.TaskStatusQueued:
			hasQueued = true
			allDoneOrFailed = false
		case model.TaskStatusFailed:
			hasFailed = true
		case model.TaskStatusDone:
			// fine
		}
	}

	if allDoneOrFailed {
		chain.OwnerAgentID = ""
		if hasFailed {
			chain.Status = model.ChainStatusFailed
		} else {
			chain.Status = model.ChainStatusDone
		}
	} else if hasLocked {
		chain.Status = model.ChainStatusLocked
	} else if hasInProgress {
		chain.Status = model.ChainStatusInProgress
	} else if hasQueued {
		chain.Status = model.ChainStatusQueued
	}

	chain.UpdatedAt = now
	s.chains[chainID] = chain
}

func (s *Store) CreateTask(_ context.Context, t model.Task) (model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(t.ChannelID) == "" {
		return model.Task{}, errWithCode("channel_id_required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return model.Task{}, errWithCode("title_required")
	}
	if _, ok := s.channels[t.ChannelID]; !ok {
		return model.Task{}, store.ErrNotFound
	}
	if strings.TrimSpace(t.ChainID) == "" {
		return model.Task{}, errWithCode("chain_id_required")
	}
	if _, ok := s.chains[t.ChainID]; !ok {
		return model.Task{}, errWithCode("chain_id_not_found")
	}

	now := time.Now().UTC()
	t.ID = newID()
	if t.Status == "" {
		t.Status = model.TaskStatusQueued
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	s.tasks[t.ID] = t
	return t, nil
}

func (s *Store) ListTasks(_ context.Context, f store.TaskFilter) ([]model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if f.UserID != "" && t.UserID != f.UserID {
			continue
		}
		if f.ChannelID != "" && t.ChannelID != f.ChannelID {
			continue
		}
		if f.ChainID != "" && t.ChainID != f.ChainID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		out = append(out, t)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (s *Store) ClaimTask(_ context.Context, req store.ClaimTaskRequest) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(req.AgentID) == "" {
		return nil, errWithCode("agent_id_required")
	}

	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		key := req.AgentID + ":" + idemKey
		if taskID, ok := s.claimIdem[key]; ok && strings.TrimSpace(taskID) != "" {
			t, ok := s.tasks[taskID]
			if ok {
				return &t, nil
			}
			delete(s.claimIdem, key)
		}
	}

	channelID := strings.TrimSpace(req.ChannelID)
	if channelID == "" && strings.TrimSpace(req.Channel) != "" {
		for _, ch := range s.channels {
			if strings.EqualFold(ch.Name, req.Channel) {
				channelID = ch.ID
				break
			}
		}
	}
	if channelID == "" {
		return nil, errWithCode("channel_id_or_channel_required")
	}

	// Check if agent already owns a chain
	var ownedChainID string
	for _, chain := range s.chains {
		if chain.OwnerAgentID == req.AgentID && chain.Status == model.ChainStatusInProgress {
			ownedChainID = chain.ID
			break
		}
	}

	// Find eligible tasks within active chains (all tasks must belong to a chain)
	var eligibleChainTasks []model.Task
	for _, t := range s.tasks {
		if t.ChannelID != channelID || t.Status != model.TaskStatusQueued || t.ChainID == "" {
			continue
		}

		chain, ok := s.chains[t.ChainID]
		if !ok || (chain.Status != model.ChainStatusQueued && chain.Status != model.ChainStatusInProgress) {
			continue
		}

		// Skip chains that have any locked tasks
		hasLockedTask := false
		for _, ct := range s.tasks {
			if ct.ChainID == t.ChainID && ct.Status == model.TaskStatusLocked {
				hasLockedTask = true
				break
			}
		}
		if hasLockedTask {
			continue
		}

		// If agent owns a chain, only consider tasks from that chain
		if ownedChainID != "" && t.ChainID != ownedChainID {
			continue
		}

		// If agent doesn't own a chain, only consider chains that are not owned by anyone
		if ownedChainID == "" && chain.OwnerAgentID != "" {
			continue
		}

		// Check if this task is the next in sequence for its chain
		if t.Sequence == 1 {
			// If sequence is 1, it's eligible if no other task in this chain is in progress
			hasInProgressInChain := false
			for _, otherTask := range s.tasks {
				if otherTask.ChainID == t.ChainID && otherTask.Status == model.TaskStatusInProgress {
					hasInProgressInChain = true
					break
				}
			}
			if !hasInProgressInChain {
				eligibleChainTasks = append(eligibleChainTasks, t)
			}
		} else {
			// For tasks with sequence > 1, check if previous task is done
			prevTaskDone := false
			for _, otherTask := range s.tasks {
				if otherTask.ChainID == t.ChainID && otherTask.Sequence == t.Sequence-1 && otherTask.Status == model.TaskStatusDone {
					prevTaskDone = true
					break
				}
			}
			if prevTaskDone {
				eligibleChainTasks = append(eligibleChainTasks, t)
			}
		}
	}

	var taskToClaim *model.Task
	if len(eligibleChainTasks) > 0 {
		// Sort eligible chain tasks: by chain creation time (oldest first), then by task sequence
		sort.Slice(eligibleChainTasks, func(i, j int) bool {
			chainI := s.chains[eligibleChainTasks[i].ChainID]
			chainJ := s.chains[eligibleChainTasks[j].ChainID]
			if chainI.CreatedAt.Before(chainJ.CreatedAt) {
				return true
			}
			if chainI.CreatedAt.After(chainJ.CreatedAt) {
				return false
			}
			return eligibleChainTasks[i].Sequence < eligibleChainTasks[j].Sequence
		})
		taskToClaim = &eligibleChainTasks[0]
	}

	if taskToClaim == nil {
		return nil, store.ErrNoQueuedTasks
	}

	now := time.Now().UTC()
	taskToClaim.Status = model.TaskStatusInProgress
	taskToClaim.AssignedAgentID = req.AgentID
	taskToClaim.ClaimedAt = &now
	taskToClaim.UpdatedAt = now
	s.tasks[taskToClaim.ID] = *taskToClaim

	// Update chain status and ownership if this is the first task of a chain
	chain := s.chains[taskToClaim.ChainID]
	if chain.Status == model.ChainStatusQueued {
		chain.Status = model.ChainStatusInProgress
		chain.OwnerAgentID = req.AgentID // Set chain ownership
		chain.UpdatedAt = now
		s.chains[chain.ID] = chain
	}

	if idemKey != "" {
		key := req.AgentID + ":" + idemKey
		s.claimIdem[key] = taskToClaim.ID
	}

	// Update agent's current_task_id (task claimed)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	if agent, ok := s.agents[req.AgentID]; ok {
		agent.CurrentTaskID = taskToClaim.ID
		agent.UpdatedAt = now
		s.agents[req.AgentID] = agent
	}

	return taskToClaim, nil
}

func (s *Store) AssignTask(_ context.Context, req store.AssignTaskRequest) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errWithCode("task_id_required")
	}
	if strings.TrimSpace(req.AgentID) == "" {
		return nil, errWithCode("agent_id_required")
	}

	t, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, store.ErrNotFound
	}

	switch t.Status {
	case model.TaskStatusQueued:
		// ok
	case model.TaskStatusInProgress:
		if t.AssignedAgentID == req.AgentID {
			return &t, nil // idempotent
		}
		return nil, store.ErrConflict
	default:
		return nil, store.ErrConflict
	}

	now := time.Now().UTC()
	t.Status = model.TaskStatusInProgress
	t.AssignedAgentID = req.AgentID
	if t.ClaimedAt == nil {
		t.ClaimedAt = &now
	}
	t.UpdatedAt = now
	s.tasks[t.ID] = t

	// Update agent's current_task_id (task assigned)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	if agent, ok := s.agents[req.AgentID]; ok {
		agent.CurrentTaskID = t.ID
		agent.UpdatedAt = now
		s.agents[req.AgentID] = agent
	}

	return &t, nil
}

func (s *Store) CompleteTask(_ context.Context, req store.CompleteTaskRequest) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errWithCode("task_id_required")
	}

	t, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Verify task ownership: request agent must match assigned agent
	if strings.TrimSpace(req.AgentID) != "" && t.AssignedAgentID != req.AgentID {
		log.Printf("[store] CompleteTask rejected: request agent_id=%s != task assigned_agent_id=%s (task_id=%s)",
			req.AgentID, t.AssignedAgentID, req.TaskID)
		return nil, store.ErrConflict
	}

	// Additional verification: agent's current_task_id must match this task
	// (Prevents completing other agent's tasks due to state directory confusion)
	if agentID := strings.TrimSpace(req.AgentID); agentID != "" {
		if agent, ok := s.agents[agentID]; ok {
			if agent.CurrentTaskID != "" && agent.CurrentTaskID != req.TaskID {
				log.Printf("[store] CompleteTask rejected: agent %s current_task_id=%s != request task_id=%s",
					agentID, agent.CurrentTaskID, req.TaskID)
				return nil, store.ErrConflict
			}
		}
	}

	now := time.Now().UTC()
	switch t.Status {
	case model.TaskStatusDone:
		// idempotent: already done; no state changes.
	case model.TaskStatusInProgress:
		t.Status = model.TaskStatusDone
		if t.DoneAt == nil {
			t.DoneAt = &now
		}
		t.UpdatedAt = now
		s.tasks[t.ID] = t
	default:
		return nil, store.ErrConflict
	}

	// Clear agent's current_task_id (task is complete)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		if agent, ok := s.agents[agentID]; ok {
			agent.CurrentTaskID = ""
			agent.UpdatedAt = now
			s.agents[agentID] = agent
		}
	}

	// Update chain status (but NOT ownership - ownership persists until explicit detach)
	if t.ChainID != "" {
		s.updateChainStatus(t.ChainID, now)
	}

	return &t, nil
}

// updateChainStatus updates chain status based on task completion
// Does NOT release ownership - ownership persists until explicit detach.
// Must be called with s.mu held.
func (s *Store) updateChainStatus(chainID string, now time.Time) {
	chain, ok := s.chains[chainID]
	if !ok {
		return
	}

	allDone := true
	hasFailed := false
	for _, t := range s.tasks {
		if t.ChainID != chainID {
			continue
		}
		if t.Status == model.TaskStatusFailed {
			hasFailed = true
		}
		if t.Status != model.TaskStatusDone && t.Status != model.TaskStatusFailed {
			allDone = false
		}
	}

	// If any task failed, immediately mark chain as failed (halts the chain)
	if hasFailed {
		chain.Status = model.ChainStatusFailed
		chain.UpdatedAt = now
		s.chains[chainID] = chain
		return
	}

	// If all tasks are done, mark chain as done
	if allDone {
		chain.Status = model.ChainStatusDone
		chain.UpdatedAt = now
		s.chains[chainID] = chain
	}
}

func (s *Store) FailTask(_ context.Context, req store.FailTaskRequest) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errWithCode("task_id_required")
	}

	t, ok := s.tasks[req.TaskID]
	if !ok {
		return nil, store.ErrNotFound
	}

	if strings.TrimSpace(req.AgentID) != "" && t.AssignedAgentID != req.AgentID {
		return nil, store.ErrConflict
	}

	now := time.Now().UTC()
	switch t.Status {
	case model.TaskStatusFailed:
		// idempotent: already failed; no state changes.
	case model.TaskStatusInProgress:
		t.Status = model.TaskStatusFailed
		t.DoneAt = nil
		t.UpdatedAt = now
		s.tasks[t.ID] = t
	default:
		return nil, store.ErrConflict
	}

	// Clear agent's current_task_id (task failed)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		if agent, ok := s.agents[agentID]; ok {
			agent.CurrentTaskID = ""
			agent.UpdatedAt = now
			s.agents[agentID] = agent
		}
	}

	// Update chain status (but NOT ownership - ownership persists until explicit detach)
	if t.ChainID != "" {
		s.updateChainStatus(t.ChainID, now)
	}

	return &t, nil
}

func (s *Store) CreateEvent(_ context.Context, e model.Event) (model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(e.AgentID) == "" {
		return model.Event{}, errWithCode("agent_id_required")
	}
	if strings.TrimSpace(e.Type) == "" {
		return model.Event{}, errWithCode("type_required")
	}

	// best-effort idempotency for events
	if strings.TrimSpace(e.IdempotencyKey) != "" {
		key := "event:" + e.AgentID + ":" + e.IdempotencyKey
		if _, ok := s.idem[key]; ok {
			return model.Event{}, store.ErrConflict
		}
		s.idem[key] = struct{}{}
	}

	now := time.Now().UTC()
	e.ID = newID()
	e.CreatedAt = now
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}
	s.events[e.ID] = e
	return e, nil
}

type errWithCode string

func (e errWithCode) Error() string { return string(e) }

func (s *Store) ListEvents(_ context.Context, f store.EventFilter) ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build set of agent IDs belonging to the user for filtering
	var userAgentIDs map[string]struct{}
	if f.UserID != "" {
		userAgentIDs = make(map[string]struct{})
		for _, a := range s.agents {
			if a.UserID == f.UserID {
				userAgentIDs[a.ID] = struct{}{}
			}
		}
	}

	out := make([]model.Event, 0, len(s.events))
	for _, e := range s.events {
		if f.UserID != "" {
			if _, ok := userAgentIDs[e.AgentID]; !ok {
				continue
			}
		}
		if f.AgentID != "" && e.AgentID != f.AgentID {
			continue
		}
		if f.TaskID != "" && e.TaskID != f.TaskID {
			continue
		}
		out = append(out, e)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}

	return out, nil
}

func (s *Store) PurgeEventsBefore(_ context.Context, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for id, e := range s.events {
		if e.CreatedAt.Before(before) {
			delete(s.events, id)
			removed++
		}
	}
	return removed, nil
}

func (s *Store) CreateTaskInput(_ context.Context, req store.CreateTaskInputRequest) (model.TaskInput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := strings.TrimSpace(req.TaskID)
	agentID := strings.TrimSpace(req.AgentID)
	if taskID == "" {
		return model.TaskInput{}, errWithCode("task_id_required")
	}
	if agentID == "" {
		return model.TaskInput{}, errWithCode("agent_id_required")
	}

	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "text"
	}

	text := req.Text
	if strings.TrimSpace(text) == "" && !req.SendEnter {
		return model.TaskInput{}, errWithCode("text_or_send_enter_required")
	}

	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		key := taskID + ":" + idemKey
		if existingID, ok := s.inputIdem[key]; ok {
			if existing, ok := s.inputs[existingID]; ok {
				return existing, nil
			}
			delete(s.inputIdem, key)
		}
	}

	now := time.Now().UTC()
	in := model.TaskInput{
		ID:             newID(),
		TaskID:         taskID,
		AgentID:        agentID,
		Kind:           kind,
		Text:           text,
		SendEnter:      req.SendEnter,
		IdempotencyKey: idemKey,
		CreatedAt:      now,
	}

	s.inputs[in.ID] = in
	if idemKey != "" {
		key := taskID + ":" + idemKey
		s.inputIdem[key] = in.ID
	}
	return in, nil
}

func (s *Store) ClaimTaskInput(_ context.Context, req store.ClaimTaskInputRequest) (*model.TaskInput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := strings.TrimSpace(req.TaskID)
	agentID := strings.TrimSpace(req.AgentID)
	if taskID == "" {
		return nil, errWithCode("task_id_required")
	}
	if agentID == "" {
		return nil, errWithCode("agent_id_required")
	}

	var candidates []model.TaskInput
	for _, in := range s.inputs {
		if in.TaskID != taskID {
			continue
		}
		if in.AgentID != agentID {
			continue
		}
		if in.ClaimedAt != nil {
			continue
		}
		candidates = append(candidates, in)
	}

	if len(candidates) == 0 {
		return nil, store.ErrNoPendingInputs
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	selected := candidates[0]
	now := time.Now().UTC()
	selected.ClaimedAt = &now
	s.inputs[selected.ID] = selected
	return &selected, nil
}
