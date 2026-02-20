package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateAccessToken(ctx context.Context, t model.AccessToken) (model.AccessToken, error) {
	if strings.TrimSpace(t.UserID) == "" {
		return model.AccessToken{}, errors.New("user_id_required")
	}
	if strings.TrimSpace(t.Name) == "" {
		return model.AccessToken{}, errors.New("name_required")
	}
	if strings.TrimSpace(t.TokenHash) == "" {
		return model.AccessToken{}, errors.New("token_hash_required")
	}

	scopes := t.Scopes
	if len(scopes) == 0 {
		scopes = []string{"tasks:write"}
	}

	var out model.AccessToken
	var lastUsedAt *time.Time
	var revokedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		insert into public.access_tokens (user_id, name, token_hash, prefix, scopes)
		values ($1::uuid, $2, $3, $4, $5::text[])
		returning id::text, user_id::text, name, token_hash, prefix, scopes, created_at, last_used_at, revoked_at
	`, t.UserID, t.Name, t.TokenHash, t.Prefix, scopes).Scan(
		&out.ID,
		&out.UserID,
		&out.Name,
		&out.TokenHash,
		&out.Prefix,
		&out.Scopes,
		&out.CreatedAt,
		&lastUsedAt,
		&revokedAt,
	)
	if err != nil {
		return model.AccessToken{}, mapPgErr(err)
	}
	out.LastUsedAt = lastUsedAt
	out.RevokedAt = revokedAt
	return out, nil
}

func (s *Store) ListAccessTokens(ctx context.Context, userID string) ([]model.AccessToken, error) {
	rows, err := s.pool.Query(ctx, `
		select id::text, user_id::text, name, token_hash, prefix, scopes, created_at, last_used_at, revoked_at
		from public.access_tokens
		where user_id = $1::uuid
		order by created_at desc
	`, userID)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	var out []model.AccessToken
	for rows.Next() {
		var t model.AccessToken
		var lastUsedAt *time.Time
		var revokedAt *time.Time
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Name,
			&t.TokenHash,
			&t.Prefix,
			&t.Scopes,
			&t.CreatedAt,
			&lastUsedAt,
			&revokedAt,
		); err != nil {
			return nil, mapPgErr(err)
		}
		t.LastUsedAt = lastUsedAt
		t.RevokedAt = revokedAt
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) RevokeAccessToken(ctx context.Context, tokenID string, userID string) error {
	cmdTag, err := s.pool.Exec(ctx, `
		update public.access_tokens
		set revoked_at = now()
		where id = $1::uuid
		  and user_id = $2::uuid
		  and revoked_at is null
	`, tokenID, userID)
	if err != nil {
		return mapPgErr(err)
	}
	if cmdTag.RowsAffected() == 0 {
		var exists bool
		_ = s.pool.QueryRow(ctx, `
			select exists(select 1 from public.access_tokens where id = $1::uuid and user_id = $2::uuid)
		`, tokenID, userID).Scan(&exists)
		if !exists {
			return store.ErrNotFound
		}
		// Already revoked — idempotent, no error
	}
	return nil
}

func (s *Store) ValidateAccessToken(ctx context.Context, tokenHash string) (*model.AccessToken, error) {
	var t model.AccessToken
	var lastUsedAt *time.Time
	var revokedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		update public.access_tokens
		set last_used_at = now()
		where token_hash = $1
		  and revoked_at is null
		returning id::text, user_id::text, name, token_hash, prefix, scopes, created_at, last_used_at, revoked_at
	`, tokenHash).Scan(
		&t.ID,
		&t.UserID,
		&t.Name,
		&t.TokenHash,
		&t.Prefix,
		&t.Scopes,
		&t.CreatedAt,
		&lastUsedAt,
		&revokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, mapPgErr(err)
	}
	t.LastUsedAt = lastUsedAt
	t.RevokedAt = revokedAt
	return &t, nil
}

func (s *Store) GetChannelByNameAndUserID(ctx context.Context, name string, userID string) (model.Channel, error) {
	var ch model.Channel
	err := s.pool.QueryRow(ctx, `
		select id::text, coalesce(user_id::text, ''), name, coalesce(description, ''), created_at
		from public.channels
		where lower(name) = lower($1)
		  and user_id = $2::uuid
	`, name, userID).Scan(&ch.ID, &ch.UserID, &ch.Name, &ch.Description, &ch.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Channel{}, store.ErrNotFound
		}
		return model.Channel{}, mapPgErr(err)
	}
	return ch, nil
}

func (s *Store) GetChainByNameAndChannelID(ctx context.Context, name string, channelID string) (model.Chain, error) {
	var out model.Chain
	err := s.pool.QueryRow(ctx, `
		select id::text, coalesce(user_id::text, ''), channel_id::text, name, coalesce(description, ''), status,
		       coalesce(owner_agent_id::text, ''), created_at, updated_at
		from public.chains
		where lower(name) = lower($1)
		  and channel_id = $2::uuid
		order by created_at asc
		limit 1
	`, name, channelID).Scan(
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
