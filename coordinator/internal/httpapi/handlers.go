package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"time": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

type agentsHeartbeatRequest struct {
	AgentID       string             `json:"agent_id"`
	Name          string             `json:"name"`
	Status        model.AgentStatus  `json:"status"` // Legacy: for backward compatibility
	ClaudeStatus  model.ClaudeStatus `json:"claude_status"`
	CurrentTaskID string             `json:"current_task_id"`
	Meta          map[string]any     `json:"meta"`
}

func (s *Server) handleAgentsHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req agentsHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	// Use ClaudeStatus if provided, otherwise fall back to Status for backward compatibility
	claudeStatus := req.ClaudeStatus
	if claudeStatus == "" && req.Status != "" {
		claudeStatus = model.ClaudeStatus(req.Status)
	}

	userID := userIDFromContext(r.Context())

	a := model.Agent{
		ID:            strings.TrimSpace(req.AgentID),
		UserID:        userID,
		Name:          strings.TrimSpace(req.Name),
		Status:        req.Status,   // Legacy field
		ClaudeStatus:  claudeStatus, // New field
		CurrentTaskID: strings.TrimSpace(req.CurrentTaskID),
		Meta:          req.Meta,
	}

	agent, err := s.store.UpsertAgent(r.Context(), a)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()

	// Detect setup_waiting state and store + publish notification
	metaState, _ := a.Meta["state"].(string)
	if metaState == "setup_waiting" {
		firstChan := ""
		subs, _ := a.Meta["subscriptions"].([]any)
		if len(subs) > 0 {
			firstChan, _ = subs[0].(string)
		}
		msg := fmt.Sprintf("Agent '%s' is waiting for a Claude Code session.", a.Name)
		if firstChan == "" {
			msg += " Assign a channel first, then start a session."
		} else {
			msg += " Start one?"
		}

		notif := Notification{
			Key:       a.ID + ":setup_waiting",
			UserID:    userID,
			AgentID:   a.ID,
			AgentName: a.Name,
			Type:      "setup_waiting",
			Channel:   firstChan,
			Message:   msg,
			CreatedAt: time.Now().UTC(),
		}
		s.notifTk.Add(notif)

		if s.notifTk.ShouldNotify(a.ID, "setup_waiting") {
			s.bus.PublishWithPayload(EventNotification, userID, map[string]any{
				"notification_type": "setup_waiting",
				"agent_id":          a.ID,
				"agent_name":        a.Name,
				"channel":           firstChan,
				"message":           msg,
			})
		}
	} else {
		s.notifTk.ClearByAgent(a.ID, "setup_waiting")
	}

	writeJSON(w, http.StatusOK, map[string]any{"agent": agent})
}

type agentResponse struct {
	model.Agent
	WorkerStatus model.WorkerStatus `json:"worker_status"`
}

func (s *Server) handleAgentsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	userID := userIDFromContext(r.Context())
	agents, err := s.store.ListAgents(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list agents")
		return
	}

	// Compute worker_status for each agent (30 second threshold = 2x heartbeat interval)
	threshold := 30 * time.Second
	response := make([]agentResponse, len(agents))
	for i, a := range agents {
		response[i] = agentResponse{
			Agent:        a,
			WorkerStatus: a.DerivedWorkerStatus(threshold),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"agents": response})
}

type requestSessionRequest struct {
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
}

func (s *Server) handleAgentsRequestSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req requestSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())
	channelID := strings.TrimSpace(req.ChannelID)
	channelName := strings.TrimSpace(req.ChannelName)

	// If ID is missing, try to find it by name.
	if channelID == "" && channelName != "" {
		ch, err := s.store.GetChannelByName(r.Context(), channelName)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, "channel_not_found", fmt.Sprintf("channel with name '%s' not found", channelName))
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get channel by name")
			return
		}
		channelID = ch.ID
	}

	if channelID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "channel_id or channel_name is required")
		return
	}

	// Auto-create a chain for this session request task
	newChain, err := s.store.CreateChain(r.Context(), model.Chain{
		UserID:      userID,
		ChannelID:   channelID,
		Name:        "Session Request",
		Description: "Auto-created chain for agent session request",
		Status:      model.ChainStatusQueued,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to create chain for session request: %v", err))
		return
	}

	// This creates a special task that signals headless agents to prompt for setup.
	task, err := s.store.CreateTask(r.Context(), model.Task{
		UserID:                   userID,
		ChannelID:                channelID,
		ChainID:                  newChain.ID,
		Sequence:                 1,
		Title:                    "Agent Session Request",
		Description:              "Automatic request for an agent on this channel to start a new Claude session.",
		Type:                     "request_claude_session",
		AgentSessionRequestToken: newAgentSessionRequestToken(),
		Status:                   model.TaskStatusQueued,
		Priority:                 100, // High priority
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to create session request task: %v", err))
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusCreated, map[string]any{"task": task})
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	agentID := strings.TrimSpace(r.PathValue("id"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id_required", "agent ID is required")
		return
	}

	userID := userIDFromContext(r.Context())

	agent, err := s.store.GetAgent(r.Context(), agentID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to get agent")
		return
	}

	// Verify user owns this agent
	if agent.UserID != userID {
		writeError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"agent": agent})
}

type updateAgentChannelsRequest struct {
	Subscriptions []string `json:"subscriptions"`
}

func (s *Server) handleAgentUpdateChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	agentID := strings.TrimSpace(r.PathValue("id"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id_required", "agent ID is required")
		return
	}

	var req updateAgentChannelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	agent, err := s.store.GetAgent(r.Context(), agentID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to get agent")
		return
	}

	// Update meta.subscriptions
	if agent.Meta == nil {
		agent.Meta = map[string]any{}
	}

	// Clean and deduplicate subscriptions
	subs := make([]string, 0, len(req.Subscriptions))
	seen := map[string]bool{}
	for _, s := range req.Subscriptions {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			subs = append(subs, s)
			seen[s] = true
		}
	}
	agent.Meta["subscriptions"] = subs

	updated, err := s.store.UpsertAgent(r.Context(), *agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to update agent")
		return
	}

	userID := userIDFromContext(r.Context())
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"agent": updated})
}

func (s *Server) handleAgentCurrentTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	agentID := strings.TrimSpace(r.PathValue("id"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id_required", "agent ID is required")
		return
	}

	userID := userIDFromContext(r.Context())

	agent, err := s.store.GetAgent(r.Context(), agentID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to get agent")
		return
	}

	// Verify user owns this agent
	if agent.UserID != userID {
		writeError(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}

	// If agent has no current task, return 404
	if agent.CurrentTaskID == "" {
		writeError(w, http.StatusNotFound, "no_current_task", "agent has no current task")
		return
	}

	// Fetch the task details
	tasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
		return
	}

	var currentTask *model.Task
	for _, t := range tasks {
		if t.ID == agent.CurrentTaskID {
			currentTask = &t
			break
		}
	}

	if currentTask == nil {
		// Task ID exists but task not found (inconsistent state)
		writeError(w, http.StatusNotFound, "task_not_found", "current task not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"task": currentTask})
}

type createChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		channels, err := s.store.ListChannels(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list channels")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"channels": channels})
		return

	case http.MethodPost:
		var req createChannelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		ch, err := s.store.CreateChannel(r.Context(), model.Channel{
			UserID:      userID,
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
		})
		if err != nil {
			status := http.StatusBadRequest
			if err == store.ErrConflict {
				status = http.StatusConflict
			}
			writeError(w, status, "invalid_request", err.Error())
			return
		}

		s.bus.Publish(EventChannels, userID)
		s.invalidateDashboardCache()
		writeJSON(w, http.StatusCreated, map[string]any{"channel": ch})
		return

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) handleGetChannelByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "channel_name_required", "channel name is required")
		return
	}

	channel, err := s.store.GetChannelByName(r.Context(), name)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to get channel by name: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"channel": channel})
}

type createChainRequest struct {
	ChannelID   string            `json:"channel_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      model.ChainStatus `json:"status"`
}

type updateChainRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      model.ChainStatus `json:"status"`
}

func (s *Server) handleChains(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		channelID := strings.TrimSpace(r.URL.Query().Get("channel_id"))
		chains, err := s.store.ListChains(r.Context(), userID, channelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list chains")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"chains": chains})
		return

	case http.MethodPost:
		var req createChainRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		chain, err := s.store.CreateChain(r.Context(), model.Chain{
			UserID:      userID,
			ChannelID:   strings.TrimSpace(req.ChannelID),
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			Status:      req.Status,
		})
		if err != nil {
			status := http.StatusBadRequest
			if err == store.ErrNotFound { // e.g. channel_id not found
				status = http.StatusNotFound
			} else if err == store.ErrConflict { // e.g. duplicate name
				status = http.StatusConflict
			}
			writeError(w, status, "invalid_request", err.Error())
			return
		}

		s.bus.Publish(EventChains, userID)
		s.invalidateDashboardCache()
		writeJSON(w, http.StatusCreated, map[string]any{"chain": chain})
		return

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) handleChain(w http.ResponseWriter, r *http.Request) {
	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeError(w, http.StatusBadRequest, "chain_id_required", "chain ID is required")
		return
	}

	userID := userIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		chain, err := s.store.GetChain(r.Context(), chainID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, "not_found", "chain not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to get chain")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"chain": chain})
		return

	case http.MethodPut:
		var req updateChainRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		chain, err := s.store.UpdateChain(r.Context(), model.Chain{
			ID:          chainID,
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			Status:      req.Status,
		})
		if err != nil {
			status := http.StatusBadRequest
			if err == store.ErrNotFound {
				status = http.StatusNotFound
			} else if err == store.ErrConflict {
				status = http.StatusConflict
			}
			writeError(w, status, "invalid_request", err.Error())
			return
		}

		s.bus.Publish(EventChains, userID)
		s.invalidateDashboardCache()
		writeJSON(w, http.StatusOK, map[string]any{"chain": chain})
		return

	case http.MethodDelete:
		err := s.store.DeleteChain(r.Context(), chainID)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, "not_found", "chain not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to delete chain")
			return
		}

		s.bus.Publish(EventChains, userID)
		s.invalidateDashboardCache()
		w.WriteHeader(http.StatusNoContent)
		return

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) handleChainDetach(w http.ResponseWriter, r *http.Request) {
	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeError(w, http.StatusBadRequest, "chain_id_required", "chain ID is required")
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	if strings.TrimSpace(req.AgentID) == "" {
		writeError(w, http.StatusBadRequest, "agent_id_required", "agent_id is required")
		return
	}

	err := s.store.DetachAgentFromChain(r.Context(), store.DetachAgentFromChainRequest{
		ChainID: chainID,
		AgentID: req.AgentID,
	})
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "chain not found")
			return
		}
		if err == store.ErrConflict {
			writeError(w, http.StatusConflict, "not_owner", "agent is not the owner of this chain")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to detach chain")
		return
	}

	userID := userIDFromContext(r.Context())
	s.bus.Publish(EventChains, userID)
	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type createTaskRequest struct {
	ChannelID     string              `json:"channel_id"`
	ChainID       string              `json:"chain_id"` // New field for chain association
	Sequence      int                 `json:"sequence"` // New field for order within a chain
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	Priority      int                 `json:"priority"`
	Status        model.TaskStatus    `json:"status"`
	ExecutionMode model.ExecutionMode `json:"execution_mode,omitempty"` // Claude Code execution mode
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		filter := store.TaskFilter{UserID: userID}
		if v := strings.TrimSpace(r.URL.Query().Get("channel_id")); v != "" {
			filter.ChannelID = v
		}
		if v := strings.TrimSpace(r.URL.Query().Get("chain_id")); v != "" {
			filter.ChainID = v
		}
		if v := strings.TrimSpace(r.URL.Query().Get("status")); v != "" {
			filter.Status = model.TaskStatus(v)
		}
		tasks, err := s.store.ListTasks(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
		return

	case http.MethodPost:
		var req createTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		// Check if ChainID is empty, if so, create a new chain for this task
		if strings.TrimSpace(req.ChainID) == "" {
			newChain, err := s.store.CreateChain(r.Context(), model.Chain{
				UserID:      userID,
				ChannelID:   strings.TrimSpace(req.ChannelID),
				Name:        fmt.Sprintf("Standalone Chain for %s", strings.TrimSpace(req.Title)),
				Description: fmt.Sprintf("Auto-created chain for single task: %s", strings.TrimSpace(req.Title)),
				Status:      model.ChainStatusQueued,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to create chain for task: %v", err))
				return
			}
			req.ChainID = newChain.ID
			req.Sequence = 1 // First and only task in this new chain
		}

		t, err := s.store.CreateTask(r.Context(), model.Task{
			UserID:        userID,
			ChannelID:     strings.TrimSpace(req.ChannelID),
			ChainID:       strings.TrimSpace(req.ChainID),
			Sequence:      req.Sequence,
			Title:         strings.TrimSpace(req.Title),
			Description:   strings.TrimSpace(req.Description),
			Priority:      req.Priority,
			Status:        req.Status,
			ExecutionMode: req.ExecutionMode,
		})
		if err != nil {
			status := http.StatusBadRequest
			if err == store.ErrNotFound { // e.g. channel_id or chain_id not found
				status = http.StatusNotFound
			} else if err == store.ErrConflict { // e.g. duplicate sequence in chain
				status = http.StatusConflict
			}
			writeError(w, status, "invalid_request", err.Error())
			return
		}
		s.bus.Publish(EventTasks, userID)
		s.invalidateDashboardCache()
		writeJSON(w, http.StatusCreated, map[string]any{"task": t})
		return

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type claimTaskRequest struct {
	AgentID        string `json:"agent_id"`
	ChannelID      string `json:"channel_id"`
	Channel        string `json:"channel"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *Server) handleTasksClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req claimTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	t, err := s.store.ClaimTask(r.Context(), store.ClaimTaskRequest{
		AgentID:        strings.TrimSpace(req.AgentID),
		ChannelID:      strings.TrimSpace(req.ChannelID),
		Channel:        strings.TrimSpace(req.Channel),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		switch err {
		case store.ErrNoQueuedTasks:
			writeError(w, http.StatusNotFound, "no_tasks", "no queued tasks")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, "conflict", "duplicate claim (idempotency)")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

type assignTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *Server) handleTasksAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req assignTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	t, err := s.store.AssignTask(r.Context(), store.AssignTaskRequest{
		TaskID:         strings.TrimSpace(req.TaskID),
		AgentID:        strings.TrimSpace(req.AgentID),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "task not found")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, "conflict", "task conflict")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

type completeTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *Server) handleTasksComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req completeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	t, err := s.store.CompleteTask(r.Context(), store.CompleteTaskRequest{
		TaskID:         strings.TrimSpace(req.TaskID),
		AgentID:        strings.TrimSpace(req.AgentID),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "task not found")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, "conflict", "task/agent conflict")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

type failTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *Server) handleTasksFail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req failTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	t, err := s.store.FailTask(r.Context(), store.FailTaskRequest{
		TaskID:         strings.TrimSpace(req.TaskID),
		AgentID:        strings.TrimSpace(req.AgentID),
		Reason:         strings.TrimSpace(req.Reason),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "task not found")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, "conflict", "task/agent conflict")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

type createTaskInputRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	Kind           string `json:"kind"`
	Text           string `json:"text"`
	SendEnter      bool   `json:"send_enter"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *Server) handleTaskInputs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req createTaskInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	in, err := s.store.CreateTaskInput(r.Context(), store.CreateTaskInputRequest{
		TaskID:         strings.TrimSpace(req.TaskID),
		AgentID:        strings.TrimSpace(req.AgentID),
		Kind:           strings.TrimSpace(req.Kind),
		Text:           req.Text,
		SendEnter:      req.SendEnter,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == store.ErrNotFound {
			status = http.StatusNotFound
		} else if err == store.ErrConflict {
			status = http.StatusConflict
		}
		writeError(w, status, "invalid_request", err.Error())
		return
	}

	s.bus.Publish(EventInputs, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusCreated, map[string]any{"input": in})
}

type claimTaskInputRequest struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
}

func (s *Server) handleTaskInputsClaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req claimTaskInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	in, err := s.store.ClaimTaskInput(r.Context(), store.ClaimTaskInputRequest{
		TaskID:  strings.TrimSpace(req.TaskID),
		AgentID: strings.TrimSpace(req.AgentID),
	})
	if err != nil {
		switch err {
		case store.ErrNoPendingInputs:
			writeError(w, http.StatusNotFound, "no_inputs", "no pending inputs")
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "not found")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventInputs, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"input": in})
}

type createEventRequest struct {
	AgentID        string         `json:"agent_id"`
	TaskID         string         `json:"task_id"`
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	IdempotencyKey string         `json:"idempotency_key"`
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		filter := store.EventFilter{UserID: userID}
		if v := strings.TrimSpace(r.URL.Query().Get("agent_id")); v != "" {
			filter.AgentID = v
		}
		if v := strings.TrimSpace(r.URL.Query().Get("task_id")); v != "" {
			filter.TaskID = v
		}
		if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
			// Ignore parsing errors and keep 0.
			var n int
			_, _ = fmt.Sscanf(v, "%d", &n)
			if n > 0 {
				filter.Limit = n
			}
		}

		events, err := s.store.ListEvents(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list events")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": events})
		return

	case http.MethodPost:
		var req createEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		eventTaskID := strings.TrimSpace(req.TaskID)

		if strings.EqualFold(strings.TrimSpace(req.Type), "agent.automation.session_request.completed") {
			token := extractSessionRequestToken(req.Payload)
			if token == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "agent_session_request_token is required for session request completion event")
				return
			}

			// Find session request task by token (+optional task_id), then complete deterministically.
			candidateTaskID := strings.TrimSpace(req.TaskID)
			if candidateTaskID == "" {
				candidateTaskID = extractSessionRequestTaskID(req.Payload)
			}
			if eventTaskID == "" {
				eventTaskID = candidateTaskID
			}

			allTasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{
				UserID: userID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
				return
			}

			var selected *model.Task
			var selectedByID *model.Task
			var selectedInProgress *model.Task
			var selectedDone *model.Task
			for i := range allTasks {
				t := &allTasks[i]
				if t.Type != "request_claude_session" {
					continue
				}
				if strings.TrimSpace(t.AgentSessionRequestToken) != token {
					continue
				}
				if candidateTaskID != "" && t.ID == candidateTaskID {
					selectedByID = t
				}
				if t.Status == model.TaskStatusInProgress && selectedInProgress == nil {
					selectedInProgress = t
				}
				if t.Status == model.TaskStatusDone && selectedDone == nil {
					selectedDone = t
				}
				if selected == nil {
					selected = t
				}
			}

			if selected == nil {
				writeError(w, http.StatusNotFound, "not_found", "session request task not found for token")
				return
			}

			target := selected
			if candidateTaskID != "" {
				if selectedByID == nil {
					writeError(w, http.StatusConflict, "conflict", "task_id does not match token owner task")
					return
				}
				target = selectedByID
			} else if selectedInProgress != nil {
				target = selectedInProgress
			} else if selectedDone != nil {
				target = selectedDone
			}
			eventTaskID = target.ID

			switch target.Status {
			case model.TaskStatusDone:
				// already completed; idempotent no-op
			case model.TaskStatusInProgress:
				// For token-gated session request completion, bypass current_task_id coupling.
				_, err = s.store.CompleteTask(r.Context(), store.CompleteTaskRequest{
					TaskID:  target.ID,
					AgentID: "",
				})
				if err != nil {
					writeError(w, http.StatusConflict, "conflict", fmt.Sprintf("failed to complete session request task: %v", err))
					return
				}
				s.bus.Publish(EventTasks, userID)
				s.bus.Publish(EventChains, userID)
			default:
				writeError(w, http.StatusConflict, "conflict", "session request task is not in progress")
				return
			}
		}

		e, err := s.store.CreateEvent(r.Context(), model.Event{
			AgentID:        strings.TrimSpace(req.AgentID),
			TaskID:         eventTaskID,
			Type:           strings.TrimSpace(req.Type),
			Payload:        req.Payload,
			IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		})
		if err != nil {
			if err == store.ErrConflict {
				// idempotency: event already exists; treat as success.
				writeJSON(w, http.StatusOK, map[string]any{"deduped": true})
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		s.bus.Publish(EventEvents, userID)
		s.invalidateDashboardCache()
		writeJSON(w, http.StatusCreated, map[string]any{"event": e})
		return

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func newAgentSessionRequestToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "asr_" + hex.EncodeToString(b[:])
}

func extractSessionRequestToken(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	keys := []string{
		"agent_session_request_token",
		"agentSessionRequestToken",
		"agent_session_token",
		"agentSessionToken",
	}
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func extractSessionRequestTaskID(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["task_id"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	if v, ok := payload["taskId"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

type taskUpdateStatusRequest struct {
	Status model.TaskStatus `json:"status"`
}

func (s *Server) handleTaskUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	taskID := strings.TrimSpace(r.PathValue("id"))
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task_id_required", "task ID is required")
		return
	}

	var req taskUpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	userID := userIDFromContext(r.Context())

	t, err := s.store.UpdateTaskStatus(r.Context(), taskID, req.Status)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "task not found")
		case store.ErrConflict:
			writeError(w, http.StatusConflict, "conflict", "invalid status transition (only locked -> queued or locked -> done)")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.bus.Publish(EventChains, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

type chainAssignAgentRequest struct {
	AgentID string `json:"agent_id"`
}

func (s *Server) handleChainAssignAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeError(w, http.StatusBadRequest, "chain_id_required", "chain ID is required")
		return
	}

	var req chainAssignAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id_required", "agent_id is required")
		return
	}

	userID := userIDFromContext(r.Context())

	// Read existing chain first to preserve name/description/status
	existing, err := s.store.GetChain(r.Context(), chainID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "chain not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to get chain")
		return
	}

	existing.OwnerAgentID = agentID

	chain, err := s.store.UpdateChain(r.Context(), existing)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			writeError(w, http.StatusNotFound, "not_found", "chain not found")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		}
		return
	}

	s.bus.Publish(EventChains, userID)
	s.bus.Publish(EventAgents, userID)
	s.invalidateDashboardCache()
	writeJSON(w, http.StatusOK, map[string]any{"chain": chain})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unsupported", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	userID := userIDFromContext(r.Context())
	ch := s.bus.Subscribe(userID)
	defer s.bus.Unsubscribe(ch)

	// Initial event so the client knows the stream is up.
	_, _ = fmt.Fprintf(w, "event: hello\ndata: {}\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(ev)
			eventName := "update"
			if ev.Type == EventNotification {
				eventName = "notification"
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, string(b))
			flusher.Flush()
		}
	}
}

func (s *Server) handleNotificationsList(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	list := s.notifTk.List(userID)
	writeJSON(w, http.StatusOK, map[string]any{"notifications": list})
}

type dismissNotificationRequest struct {
	AgentID string `json:"agent_id"`
	Type    string `json:"type"`
}

func (s *Server) handleNotificationDismiss(w http.ResponseWriter, r *http.Request) {
	var req dismissNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	typ := strings.TrimSpace(req.Type)
	if agentID == "" || typ == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "agent_id and type are required")
		return
	}

	userID := userIDFromContext(r.Context())
	s.notifTk.Dismiss(userID, agentID, typ)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
