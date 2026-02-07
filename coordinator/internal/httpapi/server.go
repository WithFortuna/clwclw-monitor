package httpapi

import (
	"net/http"

	"clwclw-monitor/coordinator/internal/config"
	"clwclw-monitor/coordinator/internal/store"
)

type Server struct {
	cfg   config.Config
	store store.Store
	mux   *http.ServeMux
	bus   *eventBus
	dash  dashboardCache
}

func NewServer(cfg config.Config, st store.Store) *Server {
	s := &Server{
		cfg:   cfg,
		store: st,
		mux:   http.NewServeMux(),
		bus:   newEventBus(),
		dash:  newDashboardCache(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = recoverMiddleware(h)
	h = requestIDMiddleware(h)
	h = loggingMiddleware(h)
	h = authMiddleware(s.cfg, h)
	return h
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)

	s.mux.HandleFunc("/v1/agents/heartbeat", s.handleAgentsHeartbeat)
	s.mux.HandleFunc("/v1/agents", s.handleAgentsList)

	s.mux.HandleFunc("/v1/channels", s.handleChannels)
	s.mux.HandleFunc("/v1/chains", s.handleChains)
	s.mux.HandleFunc("/v1/chains/{id}", s.handleChain)
	s.mux.HandleFunc("/v1/tasks", s.handleTasks)
	s.mux.HandleFunc("/v1/tasks/claim", s.handleTasksClaim)
	s.mux.HandleFunc("/v1/tasks/assign", s.handleTasksAssign)
	s.mux.HandleFunc("/v1/tasks/complete", s.handleTasksComplete)
	s.mux.HandleFunc("/v1/tasks/fail", s.handleTasksFail)
	s.mux.HandleFunc("/v1/tasks/inputs", s.handleTaskInputs)
	s.mux.HandleFunc("/v1/tasks/inputs/claim", s.handleTaskInputsClaim)

	s.mux.HandleFunc("/v1/events", s.handleEvents)
	s.mux.HandleFunc("/v1/stream", s.handleStream)
	s.mux.HandleFunc("/v1/dashboard", s.handleDashboard)

	s.registerUI()
}
