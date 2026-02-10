package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"clwclw-monitor/coordinator/internal/model"

	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	AgentAuth bool   `json:"agent_auth"`
}

type authResponse struct {
	Token    string     `json:"token"`
	User     model.User `json:"user"`
	AuthCode string     `json:"auth_code,omitempty"`
}

type agentTokenRequest struct {
	Code string `json:"code"`
}

type agentTokenResponse struct {
	Token    string `json:"token"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

const agentJWTExpiry = 90 * 24 * time.Hour // 90 days for agent tokens
const authCodeExpiry = 5 * time.Minute

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

func generateAuthCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateAgentJWT(userID, username string) (string, error) {
	claims := map[string]any{
		"sub":      userID,
		"username": username,
		"agent":    true,
		"exp":      time.Now().Add(agentJWTExpiry).Unix(),
		"iat":      time.Now().Unix(),
	}
	return generateJWTFromClaims(claims)
}

func validatePassword(pw string) string {
	if len(pw) < 6 {
		return "password must be at least 6 characters"
	}
	hasUpper := false
	hasSpecial := false
	for _, r := range pw {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			hasSpecial = true
		}
	}
	if !hasUpper {
		return "password must contain at least one uppercase letter"
	}
	if !hasSpecial {
		return "password must contain at least one special character"
	}
	return ""
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if !usernameRegex.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "invalid_username", "username must be 3-30 characters (letters, numbers, _, -)")
		return
	}

	if msg := validatePassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_password", msg)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}

	user := model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
	}

	created, err := s.store.CreateUser(r.Context(), user)
	if err != nil {
		if err.Error() == "conflict" || strings.Contains(err.Error(), "conflict") {
			writeError(w, http.StatusConflict, "conflict", "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to create user")
		return
	}

	token, err := generateJWT(created.ID, created.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: created})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "username is required")
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}

	token, err := generateJWT(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to generate token")
		return
	}

	resp := authResponse{Token: token, User: *user}

	// If agent_auth is requested, generate an auth code
	if req.AgentAuth {
		code, err := generateAuthCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to generate auth code")
			return
		}

		authCode := model.AuthCode{
			Code:      code,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(authCodeExpiry),
		}
		if err := s.store.CreateAuthCode(r.Context(), authCode); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to create auth code")
			return
		}

		resp.AuthCode = code
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}

	// Try JWT from Authorization header
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "no token provided")
		return
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(auth, bearerPrefix) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token format")
		return
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(auth, bearerPrefix))
	userID, username, err := parseJWT(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	resp := map[string]string{
		"user_id":  userID,
		"username": username,
	}

	// If agent_auth is requested, generate an auth code
	if r.URL.Query().Get("agent_auth") == "true" {
		code, err := generateAuthCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to generate auth code")
			return
		}

		authCode := model.AuthCode{
			Code:      code,
			UserID:    userID,
			ExpiresAt: time.Now().Add(authCodeExpiry),
		}
		if err := s.store.CreateAuthCode(r.Context(), authCode); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "failed to create auth code")
			return
		}

		resp["auth_code"] = code
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAgentToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}

	var req agentTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "code is required")
		return
	}

	ac, err := s.store.ConsumeAuthCode(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired auth code")
		return
	}

	user, err := s.store.GetUserByID(r.Context(), ac.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to get user")
		return
	}

	token, err := generateAgentJWT(user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to generate agent token")
		return
	}

	writeJSON(w, http.StatusOK, agentTokenResponse{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
	})
}

// handleDebugToken validates a JWT token without auth middleware.
// POST /v1/auth/debug-token  {"token": "eyJ..."}
func (s *Server) handleDebugToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "token is required")
		return
	}

	userID, username, err := parseJWT(token)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"valid":    true,
		"user_id":  userID,
		"username": username,
	})
}
