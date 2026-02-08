package store

import (
	"context"
	"errors"

	"clwclw-monitor/coordinator/internal/model"
)

var (
	ErrNotFound        = errors.New("not_found")
	ErrConflict        = errors.New("conflict")
	ErrNoQueuedTasks   = errors.New("no_queued_tasks")
	ErrNoPendingInputs = errors.New("no_pending_inputs")
)

type TaskFilter struct {
	ChannelID string
	ChainID   string // New field for filtering by chain
	Status    model.TaskStatus
	Limit     int
}

type EventFilter struct {
	AgentID string
	TaskID  string
	Limit   int
}

type ClaimTaskRequest struct {
	AgentID        string `json:"agent_id"`
	ChannelID      string `json:"channel_id,omitempty"`
	Channel        string `json:"channel,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type CompleteTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type FailTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id,omitempty"`
	Reason         string `json:"reason,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type AssignTaskRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type CreateTaskInputRequest struct {
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	Kind           string `json:"kind,omitempty"`
	Text           string `json:"text,omitempty"`
	SendEnter      bool   `json:"send_enter"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type ClaimTaskInputRequest struct {
	TaskID  string `json:"task_id"`
	AgentID string `json:"agent_id"`
}

type Store interface {
	UpsertAgent(ctx context.Context, a model.Agent) (model.Agent, error)
	GetAgent(ctx context.Context, id string) (*model.Agent, error)
	ListAgents(ctx context.Context) ([]model.Agent, error)

	CreateChannel(ctx context.Context, ch model.Channel) (model.Channel, error)
	ListChannels(ctx context.Context) ([]model.Channel, error)
	GetChannelByName(ctx context.Context, name string) (model.Channel, error)

	CreateChain(ctx context.Context, c model.Chain) (model.Chain, error)
	GetChain(ctx context.Context, id string) (model.Chain, error)
	ListChains(ctx context.Context, channelID string) ([]model.Chain, error)
	UpdateChain(ctx context.Context, c model.Chain) (model.Chain, error)
	DeleteChain(ctx context.Context, id string) error

	CreateTask(ctx context.Context, t model.Task) (model.Task, error)
	ListTasks(ctx context.Context, f TaskFilter) ([]model.Task, error)
	ClaimTask(ctx context.Context, req ClaimTaskRequest) (*model.Task, error)
	AssignTask(ctx context.Context, req AssignTaskRequest) (*model.Task, error)
	CompleteTask(ctx context.Context, req CompleteTaskRequest) (*model.Task, error)
	FailTask(ctx context.Context, req FailTaskRequest) (*model.Task, error)

	CreateEvent(ctx context.Context, e model.Event) (model.Event, error)
	ListEvents(ctx context.Context, f EventFilter) ([]model.Event, error)

	CreateTaskInput(ctx context.Context, req CreateTaskInputRequest) (model.TaskInput, error)
	ClaimTaskInput(ctx context.Context, req ClaimTaskInputRequest) (*model.TaskInput, error)
}
