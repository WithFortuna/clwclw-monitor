package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store"
)

var validScopes = map[string]bool{
	"tasks:write": true,
}

// --- Token management handlers (JWT auth) ---

type createAccessTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type createAccessTokenResponse struct {
	AccessToken model.AccessToken `json:"access_token"`
	Token       string            `json:"token"` // raw token, shown once
}

func (s *Server) handleAccessTokens(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		tokens, err := s.store.ListAccessTokens(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to list access tokens")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"access_tokens": tokens})

	case http.MethodPost:
		var req createAccessTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
			return
		}

		scopes := req.Scopes
		if len(scopes) == 0 {
			scopes = []string{"tasks:write"}
		}
		for _, sc := range scopes {
			if !validScopes[sc] {
				writeError(w, http.StatusBadRequest, "invalid_scope", fmt.Sprintf("unknown scope: %q", sc))
				return
			}
		}

		// Generate 32-byte random token (64 hex chars)
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to generate token")
			return
		}
		rawHex := hex.EncodeToString(raw[:])
		prefix := rawHex[:8]

		hashBytes := sha256.Sum256([]byte(rawHex))
		tokenHash := hex.EncodeToString(hashBytes[:])

		t, err := s.store.CreateAccessToken(r.Context(), model.AccessToken{
			UserID:    userID,
			Name:      name,
			TokenHash: tokenHash,
			Prefix:    prefix,
			Scopes:    scopes,
		})
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				writeError(w, http.StatusConflict, "conflict", "token already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "failed to create access token")
			return
		}

		writeJSON(w, http.StatusCreated, createAccessTokenResponse{
			AccessToken: t,
			Token:       rawHex,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *Server) handleRevokeAccessToken(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tokenID := r.PathValue("id")
	if strings.TrimSpace(tokenID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "token id is required")
		return
	}

	err := s.store.RevokeAccessToken(r.Context(), tokenID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "access token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to revoke access token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- External API handler (access token auth) ---

type externalCreateTaskRequest struct {
	Channel       string `json:"channel"`
	Chain         string `json:"chain"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Priority      int    `json:"priority"`
	ExecutionMode string `json:"execution_mode"`
}

func (s *Server) handleExternalCreateTask(w http.ResponseWriter, r *http.Request) {
	if !contextHasScope(r.Context(), "tasks:write") {
		writeError(w, http.StatusForbidden, "forbidden", "scope tasks:write required")
		return
	}

	userID := userIDFromContext(r.Context())

	var req externalCreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", "invalid json")
		return
	}

	channelName := strings.TrimSpace(req.Channel)
	title := strings.TrimSpace(req.Title)

	if channelName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "channel is required")
		return
	}
	if title == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	// Resolve channel by name + userID (owner-scoped)
	channel, err := s.store.GetChannelByNameAndUserID(r.Context(), channelName, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel_not_found", fmt.Sprintf("channel %q not found", channelName))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to lookup channel")
		return
	}

	// Resolve or create chain
	chainName := strings.TrimSpace(req.Chain)
	var chain model.Chain

	if chainName != "" {
		existing, lookupErr := s.store.GetChainByNameAndChannelID(r.Context(), chainName, channel.ID)
		if lookupErr == nil {
			chain = existing
		} else if errors.Is(lookupErr, store.ErrNotFound) {
			chain, err = s.store.CreateChain(r.Context(), model.Chain{
				UserID:    userID,
				ChannelID: channel.ID,
				Name:      chainName,
				Status:    model.ChainStatusQueued,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "failed to create chain")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, "internal", "failed to lookup chain")
			return
		}
	} else {
		autoName := "CSPG: " + title
		if len(autoName) > 56 {
			autoName = autoName[:56]
		}
		chain, err = s.store.CreateChain(r.Context(), model.Chain{
			UserID:    userID,
			ChannelID: channel.ID,
			Name:      autoName,
			Status:    model.ChainStatusQueued,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to create chain")
			return
		}
	}

	// Calculate next sequence in chain
	chainTasks, err := s.store.ListTasks(r.Context(), store.TaskFilter{ChainID: chain.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list chain tasks")
		return
	}
	maxSeq := 0
	for _, ct := range chainTasks {
		if ct.Sequence > maxSeq {
			maxSeq = ct.Sequence
		}
	}
	sequence := maxSeq + 1

	task, err := s.store.CreateTask(r.Context(), model.Task{
		UserID:        userID,
		ChannelID:     channel.ID,
		ChainID:       chain.ID,
		Sequence:      sequence,
		Title:         title,
		Description:   strings.TrimSpace(req.Description),
		Priority:      req.Priority,
		ExecutionMode: model.ExecutionMode(strings.TrimSpace(req.ExecutionMode)),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create task")
		return
	}

	s.bus.Publish(EventTasks, userID)
	s.invalidateDashboardCache()

	writeJSON(w, http.StatusCreated, map[string]any{"task": task})
}
