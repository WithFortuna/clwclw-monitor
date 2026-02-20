package memory

import (
	"context"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

func (s *Store) CreateAccessToken(_ context.Context, t model.AccessToken) (model.AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(t.UserID) == "" {
		return model.AccessToken{}, errWithCode("user_id_required")
	}
	if strings.TrimSpace(t.Name) == "" {
		return model.AccessToken{}, errWithCode("name_required")
	}
	if strings.TrimSpace(t.TokenHash) == "" {
		return model.AccessToken{}, errWithCode("token_hash_required")
	}

	if _, exists := s.tokenHashes[t.TokenHash]; exists {
		return model.AccessToken{}, store.ErrConflict
	}

	t.ID = newID()
	t.CreatedAt = time.Now().UTC()
	if len(t.Scopes) == 0 {
		t.Scopes = []string{"tasks:write"}
	}

	s.accessTokens[t.ID] = t
	s.tokenHashes[t.TokenHash] = t.ID
	return t, nil
}

func (s *Store) ListAccessTokens(_ context.Context, userID string) ([]model.AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []model.AccessToken
	for _, t := range s.accessTokens {
		if userID != "" && t.UserID != userID {
			continue
		}
		out = append(out, t)
	}

	// Sort by created_at descending
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].CreatedAt.Before(out[j].CreatedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (s *Store) RevokeAccessToken(_ context.Context, tokenID string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.accessTokens[tokenID]
	if !ok {
		return store.ErrNotFound
	}
	if userID != "" && t.UserID != userID {
		return store.ErrNotFound
	}
	if t.RevokedAt != nil {
		return nil // already revoked; idempotent
	}

	now := time.Now().UTC()
	t.RevokedAt = &now
	s.accessTokens[tokenID] = t
	return nil
}

func (s *Store) ValidateAccessToken(_ context.Context, tokenHash string) (*model.AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.tokenHashes[tokenHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	t, ok := s.accessTokens[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if t.RevokedAt != nil {
		return nil, store.ErrNotFound
	}

	// Update last_used_at
	now := time.Now().UTC()
	t.LastUsedAt = &now
	s.accessTokens[id] = t

	return &t, nil
}

func (s *Store) GetChannelByNameAndUserID(_ context.Context, name string, userID string) (model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.channels {
		if strings.EqualFold(ch.Name, name) && ch.UserID == userID {
			return ch, nil
		}
	}
	return model.Channel{}, store.ErrNotFound
}

func (s *Store) GetChainByNameAndChannelID(_ context.Context, name string, channelID string) (model.Chain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.chains {
		if strings.EqualFold(c.Name, name) && c.ChannelID == channelID {
			return c, nil
		}
	}
	return model.Chain{}, store.ErrNotFound
}
