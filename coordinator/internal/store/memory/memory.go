package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

type Store struct {
	mu sync.Mutex

	agents   map[string]model.Agent
	channels map[string]model.Channel
	tasks    map[string]model.Task
	events   map[string]model.Event
	inputs   map[string]model.TaskInput

	claimIdem map[string]string
	inputIdem map[string]string

	// simple idempotency tracking (best-effort for in-memory phase)
	idem map[string]struct{}
}

func NewStore() *Store {
	return &Store{
		agents:   make(map[string]model.Agent),
		channels: make(map[string]model.Channel),
		tasks:    make(map[string]model.Task),
		events:   make(map[string]model.Event),
		inputs:   make(map[string]model.TaskInput),
		claimIdem: make(map[string]string),
		inputIdem: make(map[string]string),
		idem:     make(map[string]struct{}),
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
	a.LastSeen = now
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Meta == nil {
		a.Meta = map[string]any{}
	}
	s.agents[a.ID] = a
	return a, nil
}

func (s *Store) ListAgents(_ context.Context) ([]model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Agent, 0, len(s.agents))
	for _, a := range s.agents {
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

func (s *Store) ListChannels(_ context.Context) ([]model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.Channel, 0, len(s.channels))
	for _, c := range s.channels {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
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
		if f.ChannelID != "" && t.ChannelID != f.ChannelID {
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

	var queued []model.Task
	for _, t := range s.tasks {
		if t.ChannelID != channelID {
			continue
		}
		if t.Status != model.TaskStatusQueued {
			continue
		}
		queued = append(queued, t)
	}
	if len(queued) == 0 {
		return nil, store.ErrNoQueuedTasks
	}

	sort.Slice(queued, func(i, j int) bool {
		return queued[i].CreatedAt.Before(queued[j].CreatedAt)
	})

	t := queued[0]
	now := time.Now().UTC()
	t.Status = model.TaskStatusInProgress
	t.AssignedAgentID = req.AgentID
	t.ClaimedAt = &now
	t.UpdatedAt = now
	s.tasks[t.ID] = t

	if idemKey != "" {
		key := req.AgentID + ":" + idemKey
		s.claimIdem[key] = t.ID
	}

	// Optional: also update agent's current task if the agent exists.
	if a, ok := s.agents[req.AgentID]; ok {
		a.CurrentTaskID = t.ID
		a.Status = model.AgentStatusRunning
		a.LastSeen = now
		a.UpdatedAt = now
		s.agents[a.ID] = a
	}

	return &t, nil
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

	// Optional: update agent's current task if the agent exists.
	if a, ok := s.agents[req.AgentID]; ok {
		a.CurrentTaskID = t.ID
		a.Status = model.AgentStatusRunning
		a.LastSeen = now
		a.UpdatedAt = now
		s.agents[a.ID] = a
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

	if strings.TrimSpace(req.AgentID) != "" && t.AssignedAgentID != req.AgentID {
		return nil, store.ErrConflict
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

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		if a, ok := s.agents[agentID]; ok {
			if a.CurrentTaskID == t.ID {
				a.CurrentTaskID = ""
			}
			a.Status = model.AgentStatusIdle
			a.LastSeen = now
			a.UpdatedAt = now
			s.agents[a.ID] = a
		}
	}

	return &t, nil
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

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		if a, ok := s.agents[agentID]; ok {
			if a.CurrentTaskID == t.ID {
				a.CurrentTaskID = ""
			}
			a.Status = model.AgentStatusIdle
			a.LastSeen = now
			a.UpdatedAt = now
			s.agents[a.ID] = a
		}
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

	out := make([]model.Event, 0, len(s.events))
	for _, e := range s.events {
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
