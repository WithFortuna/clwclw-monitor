package memory

import (
	"context"
	"time"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

func (s *Store) GetUserByID(_ context.Context, id string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &u, nil
}

func (s *Store) CreateAuthCode(_ context.Context, code model.AuthCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.authCodes[code.Code] = code
	return nil
}

func (s *Store) ConsumeAuthCode(_ context.Context, code string) (*model.AuthCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ac, ok := s.authCodes[code]
	if !ok {
		return nil, store.ErrNotFound
	}

	if ac.Used {
		return nil, store.ErrConflict
	}

	if time.Now().After(ac.ExpiresAt) {
		return nil, store.ErrNotFound
	}

	ac.Used = true
	s.authCodes[code] = ac
	return &ac, nil
}
