package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/auth"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type UserHandler struct {
	db          db.DatabaseManager
	auditLogger *audit.AuditLogger
}

func NewUserHandler(dbManager db.DatabaseManager, auditLogger *audit.AuditLogger) *UserHandler {
	return &UserHandler{db: dbManager, auditLogger: auditLogger}
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Put("/{id}/status", h.UpdateStatus)
	return r
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := db.UserFilter{}
	if role := r.URL.Query().Get("role"); role != "" {
		filter.Role = role
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = status
	}

	users, err := h.db.Users().List(ctx, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	for _, u := range users {
		u.PasswordHash = ""
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   users,
		"total":  len(users),
	})
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.db.Users().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

type CreateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Users().FindByEmail(ctx, req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	role := "user"
	if req.Role == "admin" {
		role = "admin"
	}

	user := &db.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		Name:         req.Name,
		Role:         role,
		Status:       "active",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.Users().Create(ctx, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "user_created", "user", user.ID, fmt.Sprintf("email=%s", user.Email))

	user.PasswordHash = ""
	writeJSON(w, http.StatusCreated, user)
}

type UpdateUserRequest struct {
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role,omitempty"`
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.db.Users().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			return
		}
		user.PasswordHash = hash
	}

	if err := h.db.Users().Update(ctx, user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "user_updated", "user", user.ID, fmt.Sprintf("email=%s", user.Email))

	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.db.Users().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	if err := h.db.Users().Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "user_deleted", "user", id, fmt.Sprintf("previous_email=%s", user.Email))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type StatusUpdateRequest struct {
	Status string `json:"status"`
}

func (h *UserHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.db.Users().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	var req StatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Status != "active" && req.Status != "suspended" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be 'active' or 'suspended'"})
		return
	}

	if err := h.db.Users().UpdateStatus(ctx, id, req.Status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	action := "user_suspended"
	if req.Status == "active" {
		action = "user_activated"
	}
	h.logAudit(r, action, "user", id, fmt.Sprintf("email=%s status=%s", user.Email, req.Status))

	user.PasswordHash = ""
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}
