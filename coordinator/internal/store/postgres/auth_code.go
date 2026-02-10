package postgres

import (
	"context"
	"errors"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx, `
		select id::text, username, password_hash, created_at, updated_at
		from public.users
		where id = $1::uuid
	`, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, mapPgErr(err)
	}
	return &u, nil
}

func (s *Store) CreateAuthCode(ctx context.Context, code model.AuthCode) error {
	_, err := s.pool.Exec(ctx, `
		insert into public.auth_codes (code, user_id, agent_name, expires_at)
		values ($1, $2::uuid, $3, $4)
	`, code.Code, code.UserID, code.AgentName, code.ExpiresAt)
	if err != nil {
		return mapPgErr(err)
	}
	return nil
}

func (s *Store) ConsumeAuthCode(ctx context.Context, code string) (*model.AuthCode, error) {
	var ac model.AuthCode
	err := s.pool.QueryRow(ctx, `
		update public.auth_codes
		set used = true
		where code = $1
		  and used = false
		  and expires_at > now()
		returning code, user_id::text, agent_name, expires_at, used, created_at
	`, code).Scan(&ac.Code, &ac.UserID, &ac.AgentName, &ac.ExpiresAt, &ac.Used, &ac.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, mapPgErr(err)
	}
	return &ac, nil
}
