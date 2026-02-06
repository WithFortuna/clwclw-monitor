package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"clwclw-monitor/coordinator/internal/config"
)

const requestIDHeader = "X-Request-Id"

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
		log.Printf("%s %s %s in %s", r.Method, r.URL.Path, r.Header.Get(requestIDHeader), time.Since(start).String())
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
	// auth disabled
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow health checks.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow UI/static assets without auth so operators can enter the API key in the dashboard.
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimSpace(cfg.AuthToken)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Query param token (for SSE/EventSource which can't set headers).
		if r.Method == http.MethodGet {
			if key := strings.TrimSpace(r.URL.Query().Get("api_key")); key != "" && key == token {
				next.ServeHTTP(w, r)
				return
			}
			if key := strings.TrimSpace(r.URL.Query().Get("token")); key != "" && key == token {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Authorization: Bearer <token>
		if auth := r.Header.Get("Authorization"); auth != "" {
			const prefix = "Bearer "
			if strings.HasPrefix(auth, prefix) && strings.TrimSpace(strings.TrimPrefix(auth, prefix)) == token {
				next.ServeHTTP(w, r)
				return
			}
		}

		// X-Api-Key: <token>
		if key := strings.TrimSpace(r.Header.Get("X-Api-Key")); key != "" && key == token {
			next.ServeHTTP(w, r)
			return
		}

		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid credentials")
	})
}
