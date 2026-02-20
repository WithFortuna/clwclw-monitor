package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	// Ping to fail fast.
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) UpsertAgent(ctx context.Context, a model.Agent) (model.Agent, error) {
	now := time.Now().UTC()

	metaJSON := []byte(`{}`)
	if a.Meta != nil {
		if b, err := json.Marshal(a.Meta); err == nil {
			metaJSON = b
		}
	}

	if strings.TrimSpace(a.ID) == "" {
		// Let DB generate UUID.
		var out model.Agent
		err := s.pool.QueryRow(ctx, `
			insert into public.agents (name, status, claude_status, current_task_id, last_seen, meta, user_id)
			values ($1, $2, $3, nullif($4, '')::uuid, $5, $6::jsonb, nullif($7, '')::uuid)
			returning id::text, coalesce(user_id::text, ''), name, status, claude_status, coalesce(current_task_id::text, ''), last_seen, meta, created_at, updated_at
		`, a.Name, string(a.Status), string(a.ClaudeStatus), a.CurrentTaskID, now, string(metaJSON), a.UserID).Scan(
			&out.ID,
			&out.UserID,
			&out.Name,
			&out.Status,
			&out.ClaudeStatus,
			&out.CurrentTaskID,
			&out.LastSeen,
			&metaJSON,
			&out.CreatedAt,
			&out.UpdatedAt,
		)
		if err != nil {
			return model.Agent{}, mapPgErr(err)
		}
		_ = json.Unmarshal(metaJSON, &out.Meta)
		return out, nil
	}

	var out model.Agent
	err := s.pool.QueryRow(ctx, `
		insert into public.agents (id, name, status, claude_status, current_task_id, last_seen, meta, user_id)
		values ($1::uuid, $2, $3, $4, nullif($5, '')::uuid, $6, $7::jsonb, nullif($8, '')::uuid)
		on conflict (id) do update
		set name = excluded.name,
		    status = excluded.status,
		    claude_status = excluded.claude_status,
		    current_task_id = excluded.current_task_id,
		    last_seen = excluded.last_seen,
		    meta = excluded.meta,
		    user_id = coalesce(excluded.user_id, public.agents.user_id),
		    updated_at = now()
		returning id::text, coalesce(user_id::text, ''), name, status, claude_status, coalesce(current_task_id::text, ''), last_seen, meta, created_at, updated_at
	`, a.ID, a.Name, string(a.Status), string(a.ClaudeStatus), a.CurrentTaskID, now, string(metaJSON), a.UserID).Scan(
		&out.ID,
		&out.UserID,
		&out.Name,
		&out.Status,
		&out.ClaudeStatus,
		&out.CurrentTaskID,
		&out.LastSeen,
		&metaJSON,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return model.Agent{}, mapPgErr(err)
	}
	_ = json.Unmarshal(metaJSON, &out.Meta)
	return out, nil
}

func (s *Store) ListAgents(ctx context.Context, userID string) ([]model.Agent, error) {
	query := `
		select id::text, coalesce(user_id::text, ''), name, status, claude_status, coalesce(current_task_id::text, ''), last_seen, meta, created_at, updated_at
		from public.agents
	`
	var args []any
	if strings.TrimSpace(userID) != "" {
		query += " where user_id = $1::uuid"
		args = append(args, userID)
	}
	query += " order by lower(name) asc, name asc, id asc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.Agent
	for rows.Next() {
		var a model.Agent
		var metaJSON []byte
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Status, &a.ClaudeStatus, &a.CurrentTaskID, &a.LastSeen, &metaJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, mapPgErr(err)
		}
		_ = json.Unmarshal(metaJSON, &a.Meta)
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) GetAgent(ctx context.Context, id string) (*model.Agent, error) {
	var a model.Agent
	var metaJSON []byte
	err := s.pool.QueryRow(ctx, `
		select id::text, coalesce(user_id::text, ''), name, status, claude_status, coalesce(current_task_id::text, ''), last_seen, meta, created_at, updated_at
		from public.agents
		where id = $1
	`, id).Scan(&a.ID, &a.UserID, &a.Name, &a.Status, &a.ClaudeStatus, &a.CurrentTaskID, &a.LastSeen, &metaJSON, &a.CreatedAt, &a.UpdatedAt)

	if err != nil {
		return nil, mapPgErr(err)
	}

	_ = json.Unmarshal(metaJSON, &a.Meta)
	return &a, nil
}

func (s *Store) CreateChannel(ctx context.Context, ch model.Channel) (model.Channel, error) {
	if strings.TrimSpace(ch.Name) == "" {
		return model.Channel{}, errors.New("name_required")
	}

	var out model.Channel
	err := s.pool.QueryRow(ctx, `
		insert into public.channels (name, description, user_id)
		values ($1, nullif($2, ''), nullif($3, '')::uuid)
		returning id::text, coalesce(user_id::text, ''), name, coalesce(description, ''), created_at
	`, ch.Name, ch.Description, ch.UserID).Scan(&out.ID, &out.UserID, &out.Name, &out.Description, &out.CreatedAt)
	if err != nil {
		return model.Channel{}, mapPgErr(err)
	}
	return out, nil
}

func (s *Store) ListChannels(ctx context.Context, userID string) ([]model.Channel, error) {
	query := `
		select id::text, coalesce(user_id::text, ''), name, coalesce(description, ''), created_at
		from public.channels
	`
	var args []any
	if strings.TrimSpace(userID) != "" {
		query += " where user_id = $1::uuid"
		args = append(args, userID)
	}
	query += " order by created_at asc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.Channel
	for rows.Next() {
		var ch model.Channel
		if err := rows.Scan(&ch.ID, &ch.UserID, &ch.Name, &ch.Description, &ch.CreatedAt); err != nil {
			return nil, mapPgErr(err)
		}
		out = append(out, ch)
	}
	return out, nil
}

func (s *Store) GetChannelByName(ctx context.Context, name string) (model.Channel, error) {
	var ch model.Channel
	err := s.pool.QueryRow(ctx, `
		select id::text, name, coalesce(description, ''), created_at
		from public.channels
		where name = $1
	`, name).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Channel{}, store.ErrNotFound
		}
		return model.Channel{}, mapPgErr(err)
	}
	return ch, nil
}

func (s *Store) CreateChain(ctx context.Context, c model.Chain) (model.Chain, error) {
	if strings.TrimSpace(c.ChannelID) == "" {
		return model.Chain{}, errors.New("channel_id_required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return model.Chain{}, errors.New("name_required")
	}

	status := c.Status
	if status == "" {
		status = model.ChainStatusQueued
	}

	var out model.Chain
	err := s.pool.QueryRow(ctx, `
		insert into public.chains (channel_id, name, description, status, user_id, owner_agent_id)
		values ($1::uuid, $2, nullif($3, ''), $4, nullif($5, '')::uuid, nullif($6, '')::uuid)
		returning id::text, coalesce(user_id::text, ''), channel_id::text, name, coalesce(description, ''), status, coalesce(owner_agent_id::text, ''), created_at, updated_at
	`, c.ChannelID, c.Name, c.Description, string(status), c.UserID, c.OwnerAgentID).Scan(
		&out.ID,
		&out.UserID,
		&out.ChannelID,
		&out.Name,
		&out.Description,
		&out.Status,
		&out.OwnerAgentID,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return model.Chain{}, mapPgErr(err)
	}
	return out, nil
}

func (s *Store) GetChain(ctx context.Context, id string) (model.Chain, error) {
	var out model.Chain
	err := s.pool.QueryRow(ctx, `
		select id::text, coalesce(user_id::text, ''), channel_id::text, name, coalesce(description, ''), status, coalesce(owner_agent_id::text, ''), created_at, updated_at
		from public.chains
		where id = $1::uuid
	`, id).Scan(
		&out.ID,
		&out.UserID,
		&out.ChannelID,
		&out.Name,
		&out.Description,
		&out.Status,
		&out.OwnerAgentID,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Chain{}, store.ErrNotFound
		}
		return model.Chain{}, mapPgErr(err)
	}
	return out, nil
}

func (s *Store) ListChains(ctx context.Context, userID string, channelID string) ([]model.Chain, error) {
	query := `
		select id::text, coalesce(user_id::text, ''), channel_id::text, name, coalesce(description, ''), status, coalesce(owner_agent_id::text, ''), created_at, updated_at
		from public.chains
	`
	var args []any
	var where []string
	if strings.TrimSpace(userID) != "" {
		args = append(args, userID)
		where = append(where, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(channelID) != "" {
		args = append(args, channelID)
		where = append(where, fmt.Sprintf("channel_id = $%d::uuid", len(args)))
	}
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by created_at asc"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.Chain
	for rows.Next() {
		var c model.Chain
		if err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.ChannelID,
			&c.Name,
			&c.Description,
			&c.Status,
			&c.OwnerAgentID,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, mapPgErr(err)
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) UpdateChain(ctx context.Context, c model.Chain) (model.Chain, error) {
	var out model.Chain
	err := s.pool.QueryRow(ctx, `
		update public.chains
		set name = $2,
		    description = nullif($3, ''),
		    status = $4,
		    owner_agent_id = nullif($5, '')::uuid,
		    updated_at = now()
		where id = $1::uuid
		returning id::text, coalesce(user_id::text, ''), channel_id::text, name, coalesce(description, ''), status, coalesce(owner_agent_id::text, ''), created_at, updated_at
	`, c.ID, c.Name, c.Description, string(c.Status), c.OwnerAgentID).Scan(
		&out.ID,
		&out.UserID,
		&out.ChannelID,
		&out.Name,
		&out.Description,
		&out.Status,
		&out.OwnerAgentID,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Chain{}, store.ErrNotFound
		}
		return model.Chain{}, mapPgErr(err)
	}
	return out, nil
}

func (s *Store) DeleteChain(ctx context.Context, id string) error {
	cmdTag, err := s.pool.Exec(ctx, `
		delete from public.chains
		where id = $1::uuid
	`, id)
	if err != nil {
		return mapPgErr(err)
	}
	if cmdTag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DetachAgentFromChain(ctx context.Context, req store.DetachAgentFromChainRequest) error {
	if strings.TrimSpace(req.ChainID) == "" {
		return errors.New("chain_id_required")
	}
	if strings.TrimSpace(req.AgentID) == "" {
		return errors.New("agent_id_required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Verify chain ownership
	var currentOwner string
	err = tx.QueryRow(ctx, `
		select coalesce(owner_agent_id::text, '')
		from public.chains
		where id = $1::uuid
	`, req.ChainID).Scan(&currentOwner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		return mapPgErr(err)
	}

	if currentOwner != req.AgentID {
		return store.ErrConflict
	}

	// Set in_progress task to locked
	_, err = tx.Exec(ctx, `
		update public.tasks
		set status = 'locked', updated_at = now()
		where chain_id = $1::uuid
		  and status = 'in_progress'
	`, req.ChainID)
	if err != nil {
		return mapPgErr(err)
	}

	// Clear chain ownership first.
	_, err = tx.Exec(ctx, `
		update public.chains
		set owner_agent_id = null,
		    updated_at = now()
		where id = $1::uuid
	`, req.ChainID)
	if err != nil {
		return mapPgErr(err)
	}

	// Re-evaluate chain status from current task states.
	// Chain becomes locked only when at least one task is locked.
	if err := s.reevaluateChainStatusTx(ctx, tx, req.ChainID, false); err != nil {
		return err
	}

	// Clear agent's current_task_id
	_, err = tx.Exec(ctx, `
		update public.agents
		set current_task_id = null, updated_at = now()
		where id = $1::uuid
	`, req.AgentID)
	if err != nil {
		return mapPgErr(err)
	}

	return tx.Commit(ctx)
}

func (s *Store) UpdateTaskStatus(ctx context.Context, taskID string, newStatus model.TaskStatus) (*model.Task, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task_id_required")
	}

	// Only allow locked → queued or locked → done
	if newStatus != model.TaskStatusQueued && newStatus != model.TaskStatusDone {
		return nil, store.ErrConflict
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var t model.Task
	if newStatus == model.TaskStatusQueued {
		err = tx.QueryRow(ctx, `
			update public.tasks
			set status = 'queued',
			    assigned_agent_id = null,
			    claimed_at = null,
			    updated_at = now()
			where id = $1::uuid and status = 'locked'
			returning id::text, coalesce(user_id::text, ''), channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0),
			          title, coalesce(description, ''), coalesce(type, ''), status, priority,
			          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
		`, taskID).Scan(
			&t.ID, &t.UserID, &t.ChannelID, &t.ChainID, &t.Sequence,
			&t.Title, &t.Description, &t.Type, &t.Status, &t.Priority,
			&t.AssignedAgentID, &t.ExecutionMode, &t.CreatedAt, &t.ClaimedAt, &t.DoneAt, &t.UpdatedAt,
		)
	} else {
		err = tx.QueryRow(ctx, `
			update public.tasks
			set status = 'done',
			    done_at = now(),
			    updated_at = now()
			where id = $1::uuid and status = 'locked'
			returning id::text, coalesce(user_id::text, ''), channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0),
			          title, coalesce(description, ''), coalesce(type, ''), status, priority,
			          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
		`, taskID).Scan(
			&t.ID, &t.UserID, &t.ChannelID, &t.ChainID, &t.Sequence,
			&t.Title, &t.Description, &t.Type, &t.Status, &t.Priority,
			&t.AssignedAgentID, &t.ExecutionMode, &t.CreatedAt, &t.ClaimedAt, &t.DoneAt, &t.UpdatedAt,
		)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Check if task exists but isn't locked
			var exists bool
			_ = tx.QueryRow(ctx, `select exists(select 1 from public.tasks where id = $1::uuid)`, taskID).Scan(&exists)
			if !exists {
				return nil, store.ErrNotFound
			}
			return nil, store.ErrConflict
		}
		return nil, mapPgErr(err)
	}

	// Re-evaluate chain status
	if t.ChainID != "" {
		if err := s.reevaluateChainStatusTx(ctx, tx, t.ChainID, true); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}

	return &t, nil
}

func (s *Store) reevaluateChainStatusTx(ctx context.Context, tx pgx.Tx, chainID string, clearOwnerOnCompletion bool) error {
	var hasLocked, hasInProgress, hasQueued bool
	allDoneOrFailed := true
	var hasFailed bool

	rows, err := tx.Query(ctx, `
		select status from public.tasks where chain_id = $1::uuid
	`, chainID)
	if err != nil {
		return mapPgErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var st model.TaskStatus
		if err := rows.Scan(&st); err != nil {
			return mapPgErr(err)
		}
		switch st {
		case model.TaskStatusLocked:
			hasLocked = true
			allDoneOrFailed = false
		case model.TaskStatusInProgress:
			hasInProgress = true
			allDoneOrFailed = false
		case model.TaskStatusQueued:
			hasQueued = true
			allDoneOrFailed = false
		case model.TaskStatusFailed:
			hasFailed = true
		}
	}
	if err := rows.Err(); err != nil {
		return mapPgErr(err)
	}

	var newChainStatus model.ChainStatus
	clearOwner := false
	if allDoneOrFailed {
		clearOwner = clearOwnerOnCompletion
		if hasFailed {
			newChainStatus = model.ChainStatusFailed
		} else {
			newChainStatus = model.ChainStatusDone
		}
	} else if hasLocked {
		newChainStatus = model.ChainStatusLocked
	} else if hasInProgress {
		newChainStatus = model.ChainStatusInProgress
	} else if hasQueued {
		newChainStatus = model.ChainStatusQueued
	}

	if newChainStatus == "" {
		return nil
	}
	if clearOwner {
		_, err = tx.Exec(ctx, `
			update public.chains set status = $1, owner_agent_id = null, updated_at = now() where id = $2::uuid
		`, string(newChainStatus), chainID)
	} else {
		_, err = tx.Exec(ctx, `
			update public.chains set status = $1, updated_at = now() where id = $2::uuid
		`, string(newChainStatus), chainID)
	}
	if err != nil {
		return mapPgErr(err)
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, t model.Task) (model.Task, error) {
	if strings.TrimSpace(t.ChannelID) == "" {
		return model.Task{}, errors.New("channel_id_required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return model.Task{}, errors.New("title_required")
	}
	if strings.TrimSpace(t.ChainID) == "" {
		return model.Task{}, errors.New("chain_id_required")
	}
	// Verify chain exists
	if _, err := s.GetChain(ctx, t.ChainID); err != nil {
		return model.Task{}, fmt.Errorf("chain_id not found: %w", err)
	}

	status := t.Status
	if status == "" {
		status = model.TaskStatusQueued
	}

	var out model.Task
	err := s.pool.QueryRow(ctx, `
		insert into public.tasks (channel_id, chain_id, sequence, title, description, type, agent_session_request_token, status, priority, execution_mode, user_id)
		values ($1::uuid, nullif($2, '')::uuid, $3, $4, nullif($5, ''), nullif($6, ''), nullif($7, ''), $8, $9, nullif($10, ''), nullif($11, '')::uuid)
		returning id::text, coalesce(user_id::text, ''), channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
		          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), coalesce(agent_session_request_token, ''), created_at, claimed_at, done_at, updated_at
	`, t.ChannelID, t.ChainID, t.Sequence, t.Title, t.Description, t.Type, t.AgentSessionRequestToken, string(status), t.Priority, string(t.ExecutionMode), t.UserID).Scan(
		&out.ID,
		&out.UserID,
		&out.ChannelID,
		&out.ChainID,
		&out.Sequence,
		&out.Title,
		&out.Description,
		&out.Type,
		&out.Status,
		&out.Priority,
		&out.AssignedAgentID,
		&out.ExecutionMode,
		&out.AgentSessionRequestToken,
		&out.CreatedAt,
		&out.ClaimedAt,
		&out.DoneAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return model.Task{}, mapPgErr(err)
	}

	// If task belongs to a chain and chain is done, reactivate it to queued
	if strings.TrimSpace(out.ChainID) != "" {
		_, _ = s.pool.Exec(ctx, `
			UPDATE public.chains SET status = 'queued', updated_at = NOW()
			WHERE id = $1::uuid AND status = 'done'
		`, out.ChainID)
	}

	return out, nil
}

func (s *Store) ListTasks(ctx context.Context, f store.TaskFilter) ([]model.Task, error) {
	query := `
		select id::text, coalesce(user_id::text, ''), channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
		       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), coalesce(agent_session_request_token, ''), created_at, claimed_at, done_at, updated_at
		from public.tasks
	`
	var where []string
	args := []any{}

	if strings.TrimSpace(f.UserID) != "" {
		args = append(args, f.UserID)
		where = append(where, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(f.ChannelID) != "" {
		args = append(args, f.ChannelID)
		where = append(where, fmt.Sprintf("channel_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(f.ChainID) != "" { // New filter for ChainID
		args = append(args, f.ChainID)
		where = append(where, fmt.Sprintf("chain_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(string(f.Status)) != "" {
		args = append(args, string(f.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by created_at asc"
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(" limit $%d", len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.ChannelID,
			&t.ChainID,
			&t.Sequence,
			&t.Title,
			&t.Description,
			&t.Type,
			&t.Status,
			&t.Priority,
			&t.AssignedAgentID,
			&t.ExecutionMode,
			&t.AgentSessionRequestToken,
			&t.CreatedAt,
			&t.ClaimedAt,
			&t.DoneAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, mapPgErr(err)
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) ClaimTask(ctx context.Context, req store.ClaimTaskRequest) (*model.Task, error) {
	if strings.TrimSpace(req.AgentID) == "" {
		return nil, errors.New("agent_id_required")
	}

	channelID := strings.TrimSpace(req.ChannelID)
	if channelID == "" && strings.TrimSpace(req.Channel) != "" {
		var id string
		if err := s.pool.QueryRow(ctx, `select id::text from public.channels where lower(name) = lower($1)`, req.Channel).Scan(&id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, store.ErrNotFound
			}
			return nil, mapPgErr(err)
		}
		channelID = id
	}
	if channelID == "" {
		return nil, errors.New("channel_id_or_channel_required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey != "" {
		// Reserve an idempotency row so concurrent retries with the same key converge to one task.
		_, _ = tx.Exec(ctx, `
			insert into public.task_claim_idempotency (agent_id, idempotency_key, channel_id)
			values ($1::uuid, $2, $3::uuid)
			on conflict (agent_id, idempotency_key) do nothing
		`, req.AgentID, idemKey, channelID)

		var existingTaskID string
		if err := tx.QueryRow(ctx, `
			select coalesce(task_id::text, '')
			from public.task_claim_idempotency
			where agent_id = $1::uuid and idempotency_key = $2
		`, req.AgentID, idemKey).Scan(&existingTaskID); err != nil {
			return nil, mapPgErr(err)
		}

		if strings.TrimSpace(existingTaskID) != "" {
			var out model.Task
			err := tx.QueryRow(ctx, `
				select id::text, channel_id::text, title, coalesce(description, ''), coalesce(type, ''), status, priority,
				       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
				from public.tasks
				where id = $1::uuid
			`, existingTaskID).Scan(
				&out.ID,
				&out.ChannelID,
				&out.Title,
				&out.Description,
				&out.Type,
				&out.Status,
				&out.Priority,
				&out.AssignedAgentID,
				&out.ExecutionMode,
				&out.CreatedAt,
				&out.ClaimedAt,
				&out.DoneAt,
				&out.UpdatedAt,
			)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, store.ErrNotFound
				}
				return nil, mapPgErr(err)
			}

			// Update agent's current_task_id.
			// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
			_, _ = tx.Exec(ctx, `
				update public.agents
				set current_task_id = $2::uuid,
				    updated_at = now()
				where id = $1::uuid
			`, req.AgentID, out.ID)

			if err := tx.Commit(ctx); err != nil {
				return nil, mapPgErr(err)
			}

			return &out, nil
		}
	}

	// Claim next queued task atomically (requires migration function claim_task).
	var t model.Task
	err = tx.QueryRow(ctx, `
		select id::text, channel_id::text, title, coalesce(description, ''), coalesce(type, ''), status, priority,
		       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
		from public.claim_task($1::uuid, $2::uuid)
	`, channelID, req.AgentID).Scan(
		&t.ID,
		&t.ChannelID,
		&t.Title,
		&t.Description,
		&t.Type,
		&t.Status,
		&t.Priority,
		&t.AssignedAgentID,
		&t.ExecutionMode,
		&t.CreatedAt,
		&t.ClaimedAt,
		&t.DoneAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNoQueuedTasks
		}
		return nil, mapPgErr(err)
	}

	if idemKey != "" {
		_, _ = tx.Exec(ctx, `
			update public.task_claim_idempotency
			set task_id = $3::uuid
			where agent_id = $1::uuid and idempotency_key = $2 and task_id is null
		`, req.AgentID, idemKey, t.ID)
	}

	// Update agent's current_task_id.
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	_, _ = tx.Exec(ctx, `
		update public.agents
		set current_task_id = $2::uuid,
		    updated_at = now()
		where id = $1::uuid
	`, req.AgentID, t.ID)

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}

	return &t, nil
}

func (s *Store) AssignTask(ctx context.Context, req store.AssignTaskRequest) (*model.Task, error) {
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errors.New("task_id_required")
	}
	if strings.TrimSpace(req.AgentID) == "" {
		return nil, errors.New("agent_id_required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var t model.Task
	err = tx.QueryRow(ctx, `
		update public.tasks
		set status = 'in_progress',
		    assigned_agent_id = $2::uuid,
		    claimed_at = coalesce(claimed_at, now()),
		    updated_at = now()
		where id = $1::uuid
		  and status = 'queued'
		returning id::text, channel_id::text, title, coalesce(description, ''), coalesce(type, ''), status, priority,
		          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
	`, req.TaskID, req.AgentID).Scan(
		&t.ID,
		&t.ChannelID,
		&t.Title,
		&t.Description,
		&t.Type,
		&t.Status,
		&t.Priority,
		&t.AssignedAgentID,
		&t.ExecutionMode,
		&t.CreatedAt,
		&t.ClaimedAt,
		&t.DoneAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var existing model.Task
			errGet := tx.QueryRow(ctx, `
				select id::text, channel_id::text, title, coalesce(description, ''), coalesce(type, ''), status, priority,
				       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
				from public.tasks
				where id = $1::uuid
			`, req.TaskID).Scan(
				&existing.ID,
				&existing.ChannelID,
				&existing.Title,
				&existing.Description,
				&existing.Type,
				&existing.Status,
				&existing.Priority,
				&existing.AssignedAgentID,
				&existing.ExecutionMode,
				&existing.CreatedAt,
				&existing.ClaimedAt,
				&existing.DoneAt,
				&existing.UpdatedAt,
			)
			if errGet != nil {
				if errors.Is(errGet, pgx.ErrNoRows) {
					return nil, store.ErrNotFound
				}
				return nil, mapPgErr(errGet)
			}

			if existing.Status == model.TaskStatusInProgress && existing.AssignedAgentID == strings.TrimSpace(req.AgentID) {
				t = existing // idempotent
			} else {
				return nil, store.ErrConflict
			}
		} else {
			return nil, mapPgErr(err)
		}
	}

	// Update agent's current_task_id.
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	_, _ = tx.Exec(ctx, `
		update public.agents
		set current_task_id = $2::uuid,
		    updated_at = now()
		where id = $1::uuid
	`, req.AgentID, t.ID)

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}

	return &t, nil
}

func (s *Store) CompleteTask(ctx context.Context, req store.CompleteTaskRequest) (*model.Task, error) {
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errors.New("task_id_required")
	}

	// Additional verification: agent's current_task_id must match this task
	// (Prevents completing other agent's tasks due to state directory confusion)
	if agentID := strings.TrimSpace(req.AgentID); agentID != "" {
		var currentTaskID string
		err := s.pool.QueryRow(ctx, `
			select coalesce(current_task_id::text, '')
			from public.agents
			where id = $1::uuid
		`, agentID).Scan(&currentTaskID)

		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, mapPgErr(err)
		}

		// If agent has a current_task_id, it must match the task being completed
		if currentTaskID != "" && currentTaskID != req.TaskID {
			log.Printf("[store] CompleteTask rejected: agent %s current_task_id=%s != request task_id=%s",
				agentID, currentTaskID, req.TaskID)
			return nil, store.ErrConflict
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		update public.tasks
		set status = 'done',
		    done_at = now(),
		    updated_at = now()
		where id = $1::uuid
	`
	args := []any{req.TaskID}
	if strings.TrimSpace(req.AgentID) != "" {
		args = append(args, req.AgentID)
		query += fmt.Sprintf(" and assigned_agent_id = $%d::uuid", len(args))
	}
	query += " and status = 'in_progress'"
	query += `
		returning id::text, channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
		          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
	`

	var t model.Task
	err = tx.QueryRow(ctx, query, args...).Scan(
		&t.ID,
		&t.ChannelID,
		&t.ChainID,
		&t.Sequence,
		&t.Title,
		&t.Description,
		&t.Type,
		&t.Status,
		&t.Priority,
		&t.AssignedAgentID,
		&t.ExecutionMode,
		&t.CreatedAt,
		&t.ClaimedAt,
		&t.DoneAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either task doesn't exist, agent mismatch, or already done.
			var existing model.Task
			errGet := tx.QueryRow(ctx, `
				select id::text, channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
				       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
				from public.tasks
				where id = $1::uuid
			`, req.TaskID).Scan(
				&existing.ID,
				&existing.ChannelID,
				&existing.ChainID,
				&existing.Sequence,
				&existing.Title,
				&existing.Description,
				&existing.Type,
				&existing.Status,
				&existing.Priority,
				&existing.AssignedAgentID,
				&existing.ExecutionMode,
				&existing.CreatedAt,
				&existing.ClaimedAt,
				&existing.DoneAt,
				&existing.UpdatedAt,
			)
			if errGet != nil {
				if errors.Is(errGet, pgx.ErrNoRows) {
					return nil, store.ErrNotFound
				}
				return nil, mapPgErr(errGet)
			}

			if strings.TrimSpace(req.AgentID) != "" && existing.AssignedAgentID != strings.TrimSpace(req.AgentID) {
				log.Printf("[store] CompleteTask rejected: UPDATE returned 0 rows - request agent_id=%s != task assigned_agent_id=%s (task_id=%s, status=%s)",
					req.AgentID, existing.AssignedAgentID, req.TaskID, existing.Status)
				return nil, store.ErrConflict
			}
			if existing.Status != model.TaskStatusDone {
				log.Printf("[store] CompleteTask rejected: UPDATE returned 0 rows - task status=%s (not in_progress, task_id=%s)",
					existing.Status, req.TaskID)
				return nil, store.ErrConflict
			}
			t = existing
		} else {
			return nil, mapPgErr(err)
		}
	}

	// If the task was part of a chain, check if all tasks in that chain are done
	if t.ChainID != "" {
		var inProgressTasksInChain int
		err := tx.QueryRow(ctx, `
			select count(id) from public.tasks
			where chain_id = $1::uuid
			and status != 'done' and status != 'failed'
		`, t.ChainID).Scan(&inProgressTasksInChain)
		if err != nil {
			return nil, mapPgErr(err)
		}

		if inProgressTasksInChain == 0 {
			// All tasks in the chain are done, update chain status
			_, err := tx.Exec(ctx, `
				update public.chains
				set status = $1, updated_at = now()
				where id = $2::uuid
			`, model.ChainStatusDone, t.ChainID)
			if err != nil {
				return nil, mapPgErr(err)
			}
		}
	}

	// Clear agent's current_task_id (task is complete)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		_, _ = tx.Exec(ctx, `
			update public.agents
			set current_task_id = null,
			    updated_at = now()
			where id = $1::uuid
		`, agentID)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}

	return &t, nil
}

func (s *Store) FailTask(ctx context.Context, req store.FailTaskRequest) (*model.Task, error) {
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, errors.New("task_id_required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		update public.tasks
		set status = 'failed',
		    done_at = null,
		    updated_at = now()
		where id = $1::uuid
	`
	args := []any{req.TaskID}
	if strings.TrimSpace(req.AgentID) != "" {
		args = append(args, req.AgentID)
		query += fmt.Sprintf(" and assigned_agent_id = $%d::uuid", len(args))
	}
	query += " and status = 'in_progress'"
	query += `
		returning id::text, channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
		          coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
	`

	var t model.Task
	err = tx.QueryRow(ctx, query, args...).Scan(
		&t.ID,
		&t.ChannelID,
		&t.ChainID,
		&t.Sequence,
		&t.Title,
		&t.Description,
		&t.Type,
		&t.Status,
		&t.Priority,
		&t.AssignedAgentID,
		&t.ExecutionMode,
		&t.CreatedAt,
		&t.ClaimedAt,
		&t.DoneAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either task doesn't exist, agent mismatch, or already failed.
			var existing model.Task
			errGet := tx.QueryRow(ctx, `
				select id::text, channel_id::text, coalesce(chain_id::text, ''), coalesce(sequence, 0), title, coalesce(description, ''), coalesce(type, ''), status, priority,
				       coalesce(assigned_agent_id::text, ''), coalesce(execution_mode, ''), created_at, claimed_at, done_at, updated_at
				from public.tasks
				where id = $1::uuid
			`, req.TaskID).Scan(
				&existing.ID,
				&existing.ChannelID,
				&existing.ChainID,
				&existing.Sequence,
				&existing.Title,
				&existing.Description,
				&existing.Type,
				&existing.Status,
				&existing.Priority,
				&existing.AssignedAgentID,
				&existing.ExecutionMode,
				&existing.CreatedAt,
				&existing.ClaimedAt,
				&existing.DoneAt,
				&existing.UpdatedAt,
			)
			if errGet != nil {
				if errors.Is(errGet, pgx.ErrNoRows) {
					return nil, store.ErrNotFound
				}
				return nil, mapPgErr(errGet)
			}

			if strings.TrimSpace(req.AgentID) != "" && existing.AssignedAgentID != strings.TrimSpace(req.AgentID) {
				return nil, store.ErrConflict
			}
			if existing.Status != model.TaskStatusFailed {
				return nil, store.ErrConflict
			}
			t = existing
		} else {
			return nil, mapPgErr(err)
		}
	}

	// If the task was part of a chain, update chain status to failed
	if t.ChainID != "" {
		_, err := tx.Exec(ctx, `
			update public.chains
			set status = $1, updated_at = now()
			where id = $2::uuid
		`, model.ChainStatusFailed, t.ChainID)
		if err != nil {
			return nil, mapPgErr(err)
		}
	}

	// Clear agent's current_task_id (task failed)
	// NOTE: Do NOT update claude_status - heartbeat is sole source of truth
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(t.AssignedAgentID)
	}
	if agentID != "" {
		_, _ = tx.Exec(ctx, `
			update public.agents
			set current_task_id = null,
			    updated_at = now()
			where id = $1::uuid
		`, agentID)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}

	return &t, nil
}

func (s *Store) CreateEvent(ctx context.Context, e model.Event) (model.Event, error) {
	if strings.TrimSpace(e.AgentID) == "" {
		return model.Event{}, errors.New("agent_id_required")
	}
	if strings.TrimSpace(e.Type) == "" {
		return model.Event{}, errors.New("type_required")
	}

	payloadJSON := []byte(`{}`)
	if e.Payload != nil {
		if b, err := json.Marshal(e.Payload); err == nil {
			payloadJSON = b
		}
	}

	var out model.Event
	var outPayload []byte
	err := s.pool.QueryRow(ctx, `
		insert into public.events (agent_id, task_id, type, payload, idempotency_key)
		values ($1::uuid, nullif($2, '')::uuid, $3, $4::jsonb, nullif($5, ''))
		returning id::text, agent_id::text, coalesce(task_id::text, ''), type, payload, coalesce(idempotency_key, ''), created_at
	`, e.AgentID, e.TaskID, e.Type, string(payloadJSON), e.IdempotencyKey).Scan(
		&out.ID,
		&out.AgentID,
		&out.TaskID,
		&out.Type,
		&outPayload,
		&out.IdempotencyKey,
		&out.CreatedAt,
	)
	if err != nil {
		return model.Event{}, mapPgErr(err)
	}
	_ = json.Unmarshal(outPayload, &out.Payload)
	return out, nil
}

func (s *Store) ListEvents(ctx context.Context, f store.EventFilter) ([]model.Event, error) {
	query := `
		select e.id::text, e.agent_id::text, coalesce(e.task_id::text, ''), e.type, e.payload,
		       coalesce(e.idempotency_key, ''), e.created_at
		from public.events e
	`
	var where []string
	args := []any{}

	if strings.TrimSpace(f.UserID) != "" {
		args = append(args, f.UserID)
		query = `
			select e.id::text, e.agent_id::text, coalesce(e.task_id::text, ''), e.type, e.payload,
			       coalesce(e.idempotency_key, ''), e.created_at
			from public.events e
			join public.agents a on a.id = e.agent_id
		`
		where = append(where, fmt.Sprintf("a.user_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(f.AgentID) != "" {
		args = append(args, f.AgentID)
		where = append(where, fmt.Sprintf("e.agent_id = $%d::uuid", len(args)))
	}
	if strings.TrimSpace(f.TaskID) != "" {
		args = append(args, f.TaskID)
		where = append(where, fmt.Sprintf("e.task_id = $%d::uuid", len(args)))
	}
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by e.created_at desc"
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(" limit $%d", len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.Event
	for rows.Next() {
		var e model.Event
		var payload []byte
		if err := rows.Scan(&e.ID, &e.AgentID, &e.TaskID, &e.Type, &payload, &e.IdempotencyKey, &e.CreatedAt); err != nil {
			return nil, mapPgErr(err)
		}
		_ = json.Unmarshal(payload, &e.Payload)
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) PurgeEventsBefore(ctx context.Context, before time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		with d as (
		  delete from public.events
		  where created_at < $1
		  returning 1
		)
		select count(*) from d
	`, before).Scan(&n)
	if err != nil {
		return 0, mapPgErr(err)
	}
	return n, nil
}

func (s *Store) CreateTaskInput(ctx context.Context, req store.CreateTaskInputRequest) (model.TaskInput, error) {
	taskID := strings.TrimSpace(req.TaskID)
	agentID := strings.TrimSpace(req.AgentID)
	if taskID == "" {
		return model.TaskInput{}, errors.New("task_id_required")
	}
	if agentID == "" {
		return model.TaskInput{}, errors.New("agent_id_required")
	}

	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "text"
	}

	if strings.TrimSpace(req.Text) == "" && !req.SendEnter {
		return model.TaskInput{}, errors.New("text_or_send_enter_required")
	}

	var out model.TaskInput
	var idem string
	var claimedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		insert into public.task_inputs (task_id, agent_id, kind, text, send_enter, idempotency_key)
		values ($1::uuid, $2::uuid, $3, nullif($4, ''), $5, nullif($6, ''))
		on conflict (task_id, idempotency_key) do update
		set agent_id = excluded.agent_id,
		    kind = excluded.kind,
		    text = excluded.text,
		    send_enter = excluded.send_enter
		returning id::text, task_id::text, agent_id::text, kind, coalesce(text, ''), send_enter, coalesce(idempotency_key, ''), created_at, claimed_at
	`, taskID, agentID, kind, req.Text, req.SendEnter, strings.TrimSpace(req.IdempotencyKey)).Scan(
		&out.ID,
		&out.TaskID,
		&out.AgentID,
		&out.Kind,
		&out.Text,
		&out.SendEnter,
		&idem,
		&out.CreatedAt,
		&claimedAt,
	)
	if err != nil {
		return model.TaskInput{}, mapPgErr(err)
	}
	out.IdempotencyKey = idem
	out.ClaimedAt = claimedAt
	return out, nil
}

func (s *Store) ClaimTaskInput(ctx context.Context, req store.ClaimTaskInputRequest) (*model.TaskInput, error) {
	taskID := strings.TrimSpace(req.TaskID)
	agentID := strings.TrimSpace(req.AgentID)
	if taskID == "" {
		return nil, errors.New("task_id_required")
	}
	if agentID == "" {
		return nil, errors.New("agent_id_required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out model.TaskInput
	var idem string
	var claimedAt *time.Time
	err = tx.QueryRow(ctx, `
		with next as (
		  select id
		  from public.task_inputs
		  where task_id = $1::uuid
		    and agent_id = $2::uuid
		    and claimed_at is null
		  order by created_at asc
		  limit 1
		  for update skip locked
		)
		update public.task_inputs ti
		set claimed_at = now()
		from next
		where ti.id = next.id
		returning ti.id::text, ti.task_id::text, ti.agent_id::text, ti.kind, coalesce(ti.text, ''), ti.send_enter, coalesce(ti.idempotency_key, ''), ti.created_at, ti.claimed_at
	`, taskID, agentID).Scan(
		&out.ID,
		&out.TaskID,
		&out.AgentID,
		&out.Kind,
		&out.Text,
		&out.SendEnter,
		&idem,
		&out.CreatedAt,
		&claimedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNoPendingInputs
		}
		return nil, mapPgErr(err)
	}

	out.IdempotencyKey = idem
	out.ClaimedAt = claimedAt

	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}
	return &out, nil
}

func mapPgErr(err error) error {
	// Unique violation, etc.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return store.ErrConflict
		case "23503":
			return store.ErrNotFound
		default:
			return fmt.Errorf("db_error %s: %s", pgErr.Code, pgErr.Message)
		}
	}
	return err
}
