package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/config"
	"clwclw-monitor/coordinator/internal/store"
)

const requestIDHeader = "X-Request-Id"

type contextKey string

const ctxUserID contextKey = "user_id"
const ctxUsername contextKey = "username"

func userIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(requestIDHeader) == "" {
			var b [12]byte
			_, _ = rand.Read(b[:])
			r.Header.Set(requestIDHeader, hex.EncodeToString(b[:]))
		}
		w.Header().Set(requestIDHeader, r.Header.Get(requestIDHeader))
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("http request", "method", r.Method, "path", r.URL.Path, "request_id", r.Header.Get(requestIDHeader), "duration", time.Since(start).String())
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "panic", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(cfg config.Config, next http.Handler) http.Handler {
	apiToken := strings.TrimSpace(cfg.AuthToken)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow health checks.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow auth endpoints without auth (register, login, agent-token).
		if strings.HasPrefix(r.URL.Path, "/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// External API uses its own access token auth middleware per-route.
		if strings.HasPrefix(r.URL.Path, "/v1/external/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow UI/static assets without auth (landing page, CSS, JS).
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		// --- Check JWT token first ---
		if auth := r.Header.Get("Authorization"); auth != "" {
			const prefix = "Bearer "
			if strings.HasPrefix(auth, prefix) {
				tokenStr := strings.TrimSpace(strings.TrimPrefix(auth, prefix))

				// Try JWT parse
				if userID, username, err := parseJWT(tokenStr); err == nil {
					ctx := context.WithValue(r.Context(), ctxUserID, userID)
					ctx = context.WithValue(ctx, ctxUsername, username)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				} else {
					slog.Warn("JWT parse failed", "error", err)
				}

				// Try API token match (admin mode, no user_id)
				if apiToken != "" && tokenStr == apiToken {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// --- Query param tokens (for SSE/EventSource which can't set headers) ---
		if r.Method == http.MethodGet {
			// Try JWT via query param
			if qToken := strings.TrimSpace(r.URL.Query().Get("token")); qToken != "" {
				if userID, username, err := parseJWT(qToken); err == nil {
					ctx := context.WithValue(r.Context(), ctxUserID, userID)
					ctx = context.WithValue(ctx, ctxUsername, username)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				} else {
					slog.Warn("JWT parse failed via query param", "error", err)
				}
				// Also try as API token
				if apiToken != "" && qToken == apiToken {
					next.ServeHTTP(w, r)
					return
				}
			}
			if key := strings.TrimSpace(r.URL.Query().Get("api_key")); key != "" && apiToken != "" && key == apiToken {
				next.ServeHTTP(w, r)
				return
			}
		}

		// X-Api-Key: <token>
		if key := strings.TrimSpace(r.Header.Get("X-Api-Key")); key != "" && key == apiToken {
			next.ServeHTTP(w, r)
			return
		}

		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid credentials")
	})
}

const ctxTokenID contextKey = "token_id"
const ctxTokenScopes contextKey = "token_scopes"

func externalAuthMiddleware(st store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing access token")
			return
		}
		rawToken := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if rawToken == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing access token")
			return
		}

		hashBytes := sha256.Sum256([]byte(rawToken))
		tokenHash := hex.EncodeToString(hashBytes[:])

		token, err := st.ValidateAccessToken(r.Context(), tokenHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or revoked access token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, token.UserID)
		ctx = context.WithValue(ctx, ctxTokenID, token.ID)
		ctx = context.WithValue(ctx, ctxTokenScopes, token.Scopes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func contextHasScope(ctx context.Context, scope string) bool {
	scopes, _ := ctx.Value(ctxTokenScopes).([]string)
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}
