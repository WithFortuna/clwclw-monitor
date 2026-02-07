package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"clwclw-monitor/coordinator/internal/store"
)

const dashboardCacheTTL = 1 * time.Second

type dashboardCache struct {
	mu        sync.Mutex
	expiresAt time.Time
	payload   []byte
}

func newDashboardCache() dashboardCache {
	return dashboardCache{}
}

func (c *dashboardCache) Get(now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.payload == nil {
		return nil, false
	}
	if now.After(c.expiresAt) {
		return nil, false
	}
	return c.payload, true
}

func (c *dashboardCache) Set(now time.Time, ttl time.Duration, payload []byte) {
	c.mu.Lock()
	c.payload = payload
	c.expiresAt = now.Add(ttl)
	c.mu.Unlock()
}

func (c *dashboardCache) Invalidate() {
	c.mu.Lock()
	c.payload = nil
	c.expiresAt = time.Time{}
	c.mu.Unlock()
}

func (s *Server) invalidateDashboardCache() {
	s.dash.Invalidate()
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	now := time.Now().UTC()
	if b, ok := s.dash.Get(now); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}

	agents, err := s.store.ListAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list agents")
		return
	}

	channels, err := s.store.ListChannels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list channels")
		return
	}

	chains, err := s.store.ListChains(r.Context(), "") // Get all chains
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list chains")
		return
	}

	tasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
		return
	}

	events, err := s.store.ListEvents(r.Context(), store.EventFilter{Limit: 60})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list events")
		return
	}

	resp := map[string]any{
		"agents":   agents,
		"channels": channels,
		"tasks":    tasks,
		"events":   events,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to encode response")
		return
	}

	s.dash.Set(now, dashboardCacheTTL, b)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

