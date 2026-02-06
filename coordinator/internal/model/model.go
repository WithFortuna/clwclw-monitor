package model

import "time"

type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusRunning AgentStatus = "running"
	AgentStatusWaiting AgentStatus = "waiting"
)

type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusFailed     TaskStatus = "failed"
)

type Agent struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Status        AgentStatus            `json:"status"`
	CurrentTaskID string                 `json:"current_task_id,omitempty"`
	LastSeen      time.Time              `json:"last_seen"`
	Meta          map[string]any          `json:"meta,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Task struct {
	ID              string     `json:"id"`
	ChannelID       string     `json:"channel_id"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Status          TaskStatus `json:"status"`
	Priority        int        `json:"priority"`
	AssignedAgentID string     `json:"assigned_agent_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ClaimedAt       *time.Time `json:"claimed_at,omitempty"`
	DoneAt          *time.Time `json:"done_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	AgentID        string    `json:"agent_id"`
	Kind           string    `json:"kind"`
	Text           string    `json:"text,omitempty"`
	SendEnter      bool      `json:"send_enter"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
}
