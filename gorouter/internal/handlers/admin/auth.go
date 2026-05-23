package admin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/auth"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type UserFinder interface {
	Create(ctx context.Context, user *db.User) error
	FindByEmail(ctx context.Context, email string) (*db.User, error)
	UpdateLastLogin(ctx context.Context, id string) error
}

type AuthHandler struct {
	db          UserFinder
	jwtSecret   string
	auditLogger *audit.AuditLogger
}

func NewAuthHandler(userRepo UserFinder, jwtSecret string, auditLogger *audit.AuditLogger) *AuthHandler {
	return &AuthHandler{db: userRepo, jwtSecret: jwtSecret, auditLogger: auditLogger}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.db.FindByEmail(ctx, req.Email)
	if err != nil || user == nil {
		h.logAudit(r, nil, "login_failed", "user", req.Email, "invalid email")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		h.logAudit(r, nil, "login_failed", "user", user.ID, fmt.Sprintf("email=%s", user.Email))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if user.Status == "suspended" || user.Status == "deleted" {
		h.logAudit(r, nil, "login_failed", "user", user.ID, fmt.Sprintf("email=%s status=%s", user.Email, user.Status))
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account is suspended"})
		return
	}

	_ = h.db.UpdateLastLogin(ctx, user.ID)

	h.logAudit(r, nil, "login_success", "user", user.ID, fmt.Sprintf("email=%s", user.Email))

	expiresAt := time.Now().Add(24 * time.Hour)
	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, h.jwtSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	resp := LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	}
	resp.User.ID = user.ID
	resp.User.Email = user.Email
	resp.User.Role = user.Role

	writeJSON(w, http.StatusOK, resp)
}

type RefreshRequest struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}

	claims, err := auth.ValidateToken(req.Token, h.jwtSecret)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	newToken, err := auth.GenerateToken(claims.UserID, claims.Email, claims.Role, h.jwtSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":      newToken,
		"expires_at": expiresAt.Unix(),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) logAudit(r *http.Request, overrideActor *audit.AuditActor, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := overrideActor
	if actor == nil {
		actor = middleware.GetAuditActor(r.Context())
	}
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	atIndex := -1
	dotAfterAt := false
	for i, ch := range email {
		if ch == '@' {
			if atIndex != -1 || i == 0 || i == len(email)-1 {
				return false
			}
			atIndex = i
		} else if ch == '.' && atIndex != -1 {
			dotAfterAt = true
		}
	}
	return atIndex > 0 && dotAfterAt && atIndex < len(email)-1
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate required fields
	if req.Email == "" || req.Name == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, name, and password are required"})
		return
	}

	// Validate email format
	if !isValidEmail(req.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}

	// Validate password min 8 chars
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check for duplicate email
	existing, err := h.db.FindByEmail(ctx, req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	// Generate user ID
	userID := generateUserID()

	// Create user
	user := &db.User{
		ID:           userID,
		Email:        req.Email,
		Name:         req.Name,
		Role:         "user",
		Status:       "active",
		PasswordHash: hash,
		CreatedBy:    &userID,
	}

	if err := h.db.Create(ctx, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	// Generate JWT token
	expiresAt := time.Now().Add(24 * time.Hour)
	token, err := auth.GenerateToken(user.ID, user.Email, user.Role, h.jwtSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Log audit
	h.logAudit(r, nil, "user_registered", "user", user.ID, fmt.Sprintf("email=%s", user.Email))

	// Return response
	resp := LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
	}
	resp.User.ID = user.ID
	resp.User.Email = user.Email
	resp.User.Role = user.Role

	writeJSON(w, http.StatusCreated, resp)
}

// generateUserID creates a unique user ID using crypto/rand
func generateUserID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// fallback to time-based if crypto/rand fails
		return fmt.Sprintf("user-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("user-%x", b)
}
