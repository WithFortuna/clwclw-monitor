package memory

import (
	"context"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

func (s *Store) CreateUser(_ context.Context, u model.User) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := strings.TrimSpace(u.Username)
	if username == "" {
		return model.User{}, errWithCode("username_required")
	}

	for _, existing := range s.users {
		if strings.EqualFold(existing.Username, username) {
			return model.User{}, store.ErrConflict
		}
	}

	now := time.Now().UTC()
	u.ID = newID()
	u.Username = username
	u.CreatedAt = now
	u.UpdatedAt = now
	s.users[u.ID] = u
	return u, nil
}

func (s *Store) GetUserByUsername(_ context.Context, username string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if strings.EqualFold(u.Username, username) {
			return &u, nil
		}
	}
	return nil, store.ErrNotFound
}
