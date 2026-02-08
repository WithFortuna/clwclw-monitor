package model

import "time"

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusRunning AgentStatus = "running"
	AgentStatusWaiting AgentStatus = "waiting"
)

// ClaudeStatus represents the execution state of Claude Code
type ClaudeStatus string

const (
	ClaudeStatusIdle    ClaudeStatus = "idle"    // No task or not actively executing
	ClaudeStatusRunning ClaudeStatus = "running" // Task assigned and actively executing
	ClaudeStatusWaiting ClaudeStatus = "waiting" // Task in progress, waiting for user input
)

// WorkerStatus represents the agent worker process lifecycle state (computed from last_seen)
type WorkerStatus string

const (
	WorkerStatusOnline  WorkerStatus = "online"  // Heartbeat within threshold
	WorkerStatusOffline WorkerStatus = "offline" // Heartbeat stale
)

type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

type ExecutionMode string

const (
	ExecutionModeAcceptEdits      ExecutionMode = "accept-edits"
	ExecutionModePlanMode         ExecutionMode = "plan-mode"
	ExecutionModeBypassPermission ExecutionMode = "bypass-permission"
)

type ChainStatus string

const (
	ChainStatusQueued     ChainStatus = "queued"
	ChainStatusInProgress ChainStatus = "in_progress"
	ChainStatusDone       ChainStatus = "done"
	ChainStatusFailed     ChainStatus = "failed"
)

type Agent struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Status        AgentStatus    `json:"status"` // DEPRECATED: Use ClaudeStatus instead
	ClaudeStatus  ClaudeStatus   `json:"claude_status"`
	CurrentTaskID string         `json:"current_task_id,omitempty"`
	LastSeen      time.Time      `json:"last_seen"`
	Meta          map[string]any `json:"meta,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// DerivedWorkerStatus computes worker status from last_seen timestamp
func (a Agent) DerivedWorkerStatus(threshold time.Duration) WorkerStatus {
	if time.Since(a.LastSeen) < threshold {
		return WorkerStatusOnline
	}
	return WorkerStatusOffline
}

type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Chain struct {
	ID          string      `json:"id"`
	ChannelID   string      `json:"channel_id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Status      ChainStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Task struct {
	ID              string        `json:"id"`
	ChainID         string        `json:"chain_id,omitempty"` // New field to link to a chain
	Sequence        int           `json:"sequence,omitempty"` // New field for order within a chain
	ChannelID       string        `json:"channel_id"`
	Title           string        `json:"title"`
	Description     string        `json:"description,omitempty"`
	Type            string        `json:"type,omitempty"`
	Status          TaskStatus    `json:"status"`
	Priority        int           `json:"priority"`
	AssignedAgentID string        `json:"assigned_agent_id,omitempty"`
	ExecutionMode   ExecutionMode `json:"execution_mode,omitempty"` // Claude Code execution mode
	CreatedAt       time.Time     `json:"created_at"`
	ClaimedAt       *time.Time    `json:"claimed_at,omitempty"`
	DoneAt          *time.Time    `json:"done_at,omitempty"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type Event struct {
	ID             string         `json:"id"`
	AgentID        string         `json:"agent_id"`
	TaskID         string         `json:"task_id,omitempty"`
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type TaskInput struct {
	ID             string     `json:"id"`
	TaskID         string     `json:"task_id"`
	AgentID        string     `json:"agent_id"`
	Kind           string     `json:"kind"`
	Text           string     `json:"text,omitempty"`
	SendEnter      bool       `json:"send_enter"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
}
