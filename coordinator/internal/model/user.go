package model

import "time"

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AuthCode struct {
	Code      string    `json:"code"`
	UserID    string    `json:"user_id"`
	AgentName string    `json:"agent_name"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type AccessToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`      // DB storage only; never serialized
	Prefix     string     `json:"prefix"` // First 8 chars of raw token, for display
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
