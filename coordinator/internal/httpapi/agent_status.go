package httpapi

import (
	"time"

	"clwclw-monitor/coordinator/internal/model"
)

func agentWithDerivedRuntimeStatus(a model.Agent, threshold time.Duration) agentResponse {
	workerStatus := a.DerivedWorkerStatus(threshold)
	effective := a

	// If worker is offline, Claude process cannot be running.
	if workerStatus == model.WorkerStatusOffline {
		effective.ClaudeStatus = model.ClaudeStatusIdle
		effective.Status = model.AgentStatusIdle
	}

	return agentResponse{
		Agent:        effective,
		WorkerStatus: workerStatus,
	}
}
