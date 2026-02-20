package httpapi

import (
	"testing"
	"time"

	"clwclw-monitor/coordinator/internal/model"
)

func TestAgentWithDerivedRuntimeStatus_OfflineForcesIdle(t *testing.T) {
	threshold := 30 * time.Second
	agent := model.Agent{
		Status:       model.AgentStatusRunning,
		ClaudeStatus: model.ClaudeStatusRunning,
		LastSeen:     time.Now().UTC().Add(-31 * time.Second),
	}

	got := agentWithDerivedRuntimeStatus(agent, threshold)
	if got.WorkerStatus != model.WorkerStatusOffline {
		t.Fatalf("expected worker offline, got %q", got.WorkerStatus)
	}
	if got.ClaudeStatus != model.ClaudeStatusIdle {
		t.Fatalf("expected claude status idle when worker offline, got %q", got.ClaudeStatus)
	}
	if got.Status != model.AgentStatusIdle {
		t.Fatalf("expected legacy status idle when worker offline, got %q", got.Status)
	}
}

func TestAgentWithDerivedRuntimeStatus_OnlineKeepsClaudeStatus(t *testing.T) {
	threshold := 30 * time.Second
	agent := model.Agent{
		Status:       model.AgentStatusRunning,
		ClaudeStatus: model.ClaudeStatusRunning,
		LastSeen:     time.Now().UTC().Add(-10 * time.Second),
	}

	got := agentWithDerivedRuntimeStatus(agent, threshold)
	if got.WorkerStatus != model.WorkerStatusOnline {
		t.Fatalf("expected worker online, got %q", got.WorkerStatus)
	}
	if got.ClaudeStatus != model.ClaudeStatusRunning {
		t.Fatalf("expected claude status running to be preserved, got %q", got.ClaudeStatus)
	}
	if got.Status != model.AgentStatusRunning {
		t.Fatalf("expected legacy status running to be preserved, got %q", got.Status)
	}
}
