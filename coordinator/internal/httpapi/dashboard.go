package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"clwclw-monitor/coordinator/internal/store"
)

const dashboardCacheTTL = 1 * time.Second

func (s *Server) invalidateDashboardCache() {
	// Dashboard cache is per-user now, so we just skip caching for simplicity.
	// In a high-traffic scenario, per-user caching could be added.
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
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
	agentResponses := make([]agentResponse, len(agents))
	for i, a := range agents {
		agentResponses[i] = agentResponse{
			Agent:        a,
			WorkerStatus: a.DerivedWorkerStatus(threshold),
		}
	}

	channels, err := s.store.ListChannels(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list channels")
		return
	}

	chains, err := s.store.ListChains(r.Context(), userID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list chains")
		return
	}

	tasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{UserID: userID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
		return
	}

	events, err := s.store.ListEvents(r.Context(), store.EventFilter{UserID: userID, Limit: 60})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list events")
		return
	}

	resp := map[string]any{
		"agents":   agentResponses,
		"channels": channels,
		"chains":   chains,
		"tasks":    tasks,
		"events":   events,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to encode response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
