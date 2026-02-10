package postgres

import (
	"context"
	"errors"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) CreateUser(ctx context.Context, u model.User) (model.User, error) {
	var out model.User
	err := s.pool.QueryRow(ctx, `
		insert into public.users (username, password_hash)
		values ($1, $2)
		returning id::text, username, password_hash, created_at, updated_at
	`, u.Username, u.PasswordHash).Scan(
		&out.ID,
		&out.Username,
		&out.PasswordHash,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, store.ErrConflict
		}
		return model.User{}, err
	}
	return out, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx, `
		select id::text, username, password_hash, created_at, updated_at
		from public.users
		where lower(username) = lower($1)
	`, username).Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
