package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/auth"
	"github.com/gorouter/gorouter/internal/db"
)

type UserFinder interface {
	FindByEmail(ctx context.Context, email string) (*db.User, error)
	UpdateLastLogin(ctx context.Context, id string) error
}

type AuthHandler struct {
	db         UserFinder
	jwtSecret string
}

func NewAuthHandler(userRepo UserFinder, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: userRepo, jwtSecret: jwtSecret}
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if user.Status == "suspended" || user.Status == "deleted" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account is suspended"})
		return
	}

	_ = h.db.UpdateLastLogin(ctx, user.ID)

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
