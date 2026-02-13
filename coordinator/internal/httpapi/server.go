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
}

func NewServer(cfg config.Config, st store.Store) *Server {
	initJWTKey(cfg.JWTSecret)
	s := &Server{
		cfg:   cfg,
		store: st,
		mux:   http.NewServeMux(),
		bus:   newEventBus(),
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

	// Auth routes
	s.mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /v1/auth/verify", s.handleAuthVerify)
	s.mux.HandleFunc("POST /v1/auth/agent-token", s.handleAgentToken)
	s.mux.HandleFunc("POST /v1/auth/debug-token", s.handleDebugToken)

	s.mux.HandleFunc("POST /v1/agents/heartbeat", s.handleAgentsHeartbeat)
	s.mux.HandleFunc("POST /v1/agents/request-session", s.handleAgentsRequestSession)
	s.mux.HandleFunc("GET /v1/agents/{id}/current-task", s.handleAgentCurrentTask)
	s.mux.HandleFunc("GET /v1/agents/{id}", s.handleGetAgent)
	s.mux.HandleFunc("PATCH /v1/agents/{id}/channels", s.handleAgentUpdateChannels)
	s.mux.HandleFunc("GET /v1/agents", s.handleAgentsList)

	s.mux.HandleFunc("/v1/channels", s.handleChannels)
	s.mux.HandleFunc("GET /v1/channels/by-name/{name}", s.handleGetChannelByName)
	s.mux.HandleFunc("/v1/chains", s.handleChains)
	s.mux.HandleFunc("/v1/chains/{id}", s.handleChain)
	s.mux.HandleFunc("/v1/tasks", s.handleTasks)
	s.mux.HandleFunc("/v1/tasks/claim", s.handleTasksClaim)
	s.mux.HandleFunc("/v1/tasks/assign", s.handleTasksAssign)
	s.mux.HandleFunc("/v1/tasks/complete", s.handleTasksComplete)
	s.mux.HandleFunc("/v1/tasks/fail", s.handleTasksFail)
	s.mux.HandleFunc("/v1/tasks/inputs", s.handleTaskInputs)
	s.mux.HandleFunc("/v1/tasks/inputs/claim", s.handleTaskInputsClaim)

	s.mux.HandleFunc("GET /v1/notifications", s.handleNotificationsList)
	s.mux.HandleFunc("POST /v1/notifications/dismiss", s.handleNotificationDismiss)

	s.mux.HandleFunc("/v1/events", s.handleEvents)
	s.mux.HandleFunc("/v1/stream", s.handleStream)
	s.mux.HandleFunc("/v1/dashboard", s.handleDashboard)

	s.registerUI()
}
