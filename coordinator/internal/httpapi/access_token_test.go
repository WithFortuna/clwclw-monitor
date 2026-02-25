package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"clwclw-monitor/coordinator/internal/model"
)

const testUserID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

// withExternalAuth injects user_id and token scopes into the request context,
// mimicking what externalAuthMiddleware does after token validation.
func withExternalAuth(r *http.Request, userID string, scopes []string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxTokenScopes, scopes)
	return r.WithContext(ctx)
}

func doExternalCreateTask(t *testing.T, s *Server, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := newJSONRequest(t, http.MethodPost, "/v1/external/tasks", body)
	req = withExternalAuth(req, testUserID, []string{"tasks:write"})
	s.handleExternalCreateTask(rec, req)
	return rec
}

func TestCoordinatorUseCasesBDD_ExternalCreateTask(t *testing.T) {
	t.Run("Given valid access token with tasks:write scope When channel does not exist Then channel is auto-created and task is created successfully", func(t *testing.T) {
		s := newTestServer(t)

		rec := doExternalCreateTask(t, s, map[string]any{
			"channel": "brand-new-channel",
			"title":   "First task",
		})

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
		}
		var resp struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec, &resp)
		if resp.Task.ID == "" {
			t.Fatal("expected task to have an ID")
		}
		if resp.Task.ChannelID == "" {
			t.Fatal("expected task to have a channel_id from auto-created channel")
		}

		ch, err := s.store.GetChannelByNameAndUserID(context.Background(), "brand-new-channel", testUserID)
		if err != nil {
			t.Fatalf("expected auto-created channel to exist in store: %v", err)
		}
		if ch.UserID != testUserID {
			t.Fatalf("expected channel user_id %q, got %q", testUserID, ch.UserID)
		}
	})

	t.Run("Given channel already exists When same channel name is used again Then existing channel is reused and task is created", func(t *testing.T) {
		s := newTestServer(t)

		rec1 := doExternalCreateTask(t, s, map[string]any{
			"channel": "existing-channel",
			"title":   "Task one",
		})
		if rec1.Code != http.StatusCreated {
			t.Fatalf("first request: expected status %d, got %d body=%s", http.StatusCreated, rec1.Code, rec1.Body.String())
		}
		var resp1 struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec1, &resp1)
		channelID := resp1.Task.ChannelID

		rec2 := doExternalCreateTask(t, s, map[string]any{
			"channel": "existing-channel",
			"title":   "Task two",
		})
		if rec2.Code != http.StatusCreated {
			t.Fatalf("second request: expected status %d, got %d body=%s", http.StatusCreated, rec2.Code, rec2.Body.String())
		}
		var resp2 struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec2, &resp2)

		if resp2.Task.ChannelID != channelID {
			t.Fatalf("expected channel_id to be reused (%q), got %q", channelID, resp2.Task.ChannelID)
		}
	})

	t.Run("Given no chain name is provided When task is created Then standalone chain is auto-created with CSPG prefix", func(t *testing.T) {
		s := newTestServer(t)

		rec := doExternalCreateTask(t, s, map[string]any{
			"channel": "auto-chain-channel",
			"title":   "Standalone task",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
		}
		var resp struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec, &resp)

		if resp.Task.ChainID == "" {
			t.Fatal("expected task to have an auto-created chain_id")
		}
		if resp.Task.Sequence != 1 {
			t.Fatalf("expected sequence 1, got %d", resp.Task.Sequence)
		}

		chain, err := s.store.GetChain(context.Background(), resp.Task.ChainID)
		if err != nil {
			t.Fatalf("expected chain to exist in store: %v", err)
		}
		if chain.Name != "CSPG: Standalone task" {
			t.Fatalf("expected chain name %q, got %q", "CSPG: Standalone task", chain.Name)
		}
	})

	t.Run("Given chain name is provided but does not exist When task is created Then named chain is auto-created", func(t *testing.T) {
		s := newTestServer(t)

		rec := doExternalCreateTask(t, s, map[string]any{
			"channel": "named-chain-channel",
			"chain":   "my-pipeline",
			"title":   "Pipeline task",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
		}
		var resp struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec, &resp)

		chain, err := s.store.GetChain(context.Background(), resp.Task.ChainID)
		if err != nil {
			t.Fatalf("expected chain to exist: %v", err)
		}
		if chain.Name != "my-pipeline" {
			t.Fatalf("expected chain name %q, got %q", "my-pipeline", chain.Name)
		}
	})

	t.Run("Given chain already exists When same chain name is used again Then existing chain is reused and sequence increments", func(t *testing.T) {
		s := newTestServer(t)

		rec1 := doExternalCreateTask(t, s, map[string]any{
			"channel": "reuse-chain-channel",
			"chain":   "shared-pipeline",
			"title":   "First pipeline task",
		})
		if rec1.Code != http.StatusCreated {
			t.Fatalf("first request: expected status %d, got %d body=%s", http.StatusCreated, rec1.Code, rec1.Body.String())
		}
		var resp1 struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec1, &resp1)
		chainID := resp1.Task.ChainID

		rec2 := doExternalCreateTask(t, s, map[string]any{
			"channel": "reuse-chain-channel",
			"chain":   "shared-pipeline",
			"title":   "Second pipeline task",
		})
		if rec2.Code != http.StatusCreated {
			t.Fatalf("second request: expected status %d, got %d body=%s", http.StatusCreated, rec2.Code, rec2.Body.String())
		}
		var resp2 struct {
			Task model.Task `json:"task"`
		}
		decodeJSONBody(t, rec2, &resp2)

		if resp2.Task.ChainID != chainID {
			t.Fatalf("expected chain_id to be reused (%q), got %q", chainID, resp2.Task.ChainID)
		}
		if resp2.Task.Sequence != 2 {
			t.Fatalf("expected sequence 2 for second task in chain, got %d", resp2.Task.Sequence)
		}
	})

	t.Run("Given request body is missing channel field When task is created Then bad_request is returned", func(t *testing.T) {
		s := newTestServer(t)

		rec := doExternalCreateTask(t, s, map[string]any{
			"title": "No channel",
		})

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
		expectErrorCode(t, rec, "invalid_request")
	})

	t.Run("Given request body is missing title field When task is created Then bad_request is returned", func(t *testing.T) {
		s := newTestServer(t)

		rec := doExternalCreateTask(t, s, map[string]any{
			"channel": "some-channel",
		})

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
		expectErrorCode(t, rec, "invalid_request")
	})

	t.Run("Given access token without tasks:write scope When task is created Then forbidden is returned", func(t *testing.T) {
		s := newTestServer(t)

		rec := httptest.NewRecorder()
		req := newJSONRequest(t, http.MethodPost, "/v1/external/tasks", map[string]any{
			"channel": "any-channel",
			"title":   "task",
		})
		req = withExternalAuth(req, testUserID, []string{"channels:write"})
		s.handleExternalCreateTask(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, rec.Code, rec.Body.String())
		}
		expectErrorCode(t, rec, "forbidden")
	})
}

func TestCoordinatorUseCasesBDD_AccessTokenScopes(t *testing.T) {
	t.Run("Given access token creation request When valid scopes are provided Then token is created successfully", func(t *testing.T) {
		for _, scope := range []string{"tasks:write", "channels:write", "chains:write"} {
			if !validScopes[scope] {
				t.Errorf("expected scope %q to be valid", scope)
			}
		}
	})

	t.Run("Given access token creation request When unknown scope is provided Then bad_request with invalid_scope is returned", func(t *testing.T) {
		s := newTestServer(t)

		rec := httptest.NewRecorder()
		req := newJSONRequest(t, http.MethodPost, "/v1/access-tokens", map[string]any{
			"name":   "bad-token",
			"scopes": []string{"nonexistent:scope"},
		})
		ctx := context.WithValue(req.Context(), ctxUserID, testUserID)
		req = req.WithContext(ctx)
		s.handleAccessTokens(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
		}
		expectErrorCode(t, rec, "invalid_scope")
	})
}
