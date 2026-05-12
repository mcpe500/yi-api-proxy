package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type RateLimitHandler struct {
	db          db.DatabaseManager
	auditLogger *audit.AuditLogger
}

func NewRateLimitHandler(dbMgr db.DatabaseManager, auditLogger *audit.AuditLogger) *RateLimitHandler {
	return &RateLimitHandler{db: dbMgr, auditLogger: auditLogger}
}

func (h *RateLimitHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)

	return r
}

func (h *RateLimitHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	limits, err := h.db.RateLimits().List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if limits == nil {
		limits = []*db.RateLimit{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   limits,
	})
}

func (h *RateLimitHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	limit, err := h.db.RateLimits().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if limit == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "rate limit not found"})
		return
	}

	writeJSON(w, http.StatusOK, limit)
}

func (h *RateLimitHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRateLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.RateLimits().FindByUserID(ctx, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "rate limit already exists for this user"})
		return
	}

	id := generateRateLimitID()
	rl := &db.RateLimit{
		ID:                id,
		UserID:            req.UserID,
		RequestsPerMinute: req.RequestsPerMinute,
		RequestsPerDay:    req.RequestsPerDay,
		TokensPerMinute:   req.TokensPerMinute,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := h.db.RateLimits().Create(ctx, rl); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "ratelimit_updated", "rate_limit", rl.ID, fmt.Sprintf("user_id=%s rpm=%d rpd=%d tpm=%d", rl.UserID, rl.RequestsPerMinute, rl.RequestsPerDay, rl.TokensPerMinute))

	writeJSON(w, http.StatusCreated, rl)
}

func (h *RateLimitHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.RateLimits().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "rate limit not found"})
		return
	}

	var req updateRateLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.RequestsPerMinute != nil {
		existing.RequestsPerMinute = *req.RequestsPerMinute
	}
	if req.RequestsPerDay != nil {
		existing.RequestsPerDay = *req.RequestsPerDay
	}
	if req.TokensPerMinute != nil {
		existing.TokensPerMinute = *req.TokensPerMinute
	}

	if err := h.db.RateLimits().Update(ctx, existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "ratelimit_updated", "rate_limit", existing.ID, fmt.Sprintf("user_id=%s rpm=%d rpd=%d tpm=%d", existing.UserID, existing.RequestsPerMinute, existing.RequestsPerDay, existing.TokensPerMinute))

	writeJSON(w, http.StatusOK, existing)
}

func (h *RateLimitHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.RateLimits().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "rate limit not found"})
		return
	}

	if err := h.db.RateLimits().Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "ratelimit_updated", "rate_limit", id, fmt.Sprintf("deleted user_id=%s", existing.UserID))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type createRateLimitRequest struct {
	UserID            string `json:"user_id"`
	RequestsPerMinute int    `json:"requests_per_minute"`
	RequestsPerDay    int    `json:"requests_per_day"`
	TokensPerMinute   int    `json:"tokens_per_minute"`
}

type updateRateLimitRequest struct {
	RequestsPerMinute *int `json:"requests_per_minute,omitempty"`
	RequestsPerDay    *int `json:"requests_per_day,omitempty"`
	TokensPerMinute   *int `json:"tokens_per_minute,omitempty"`
}

func generateRateLimitID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "rl_" + hex.EncodeToString(b)
}

func getIntParam(r *http.Request, key string) int {
	val := chi.URLParam(r, key)
	if val == "" {
		val = r.URL.Query().Get(key)
	}
	n, _ := strconv.Atoi(val)
	return n
}

func getStringParam(r *http.Request, key string) string {
	val := chi.URLParam(r, key)
	if val == "" {
		val = r.URL.Query().Get(key)
	}
	return strings.TrimSpace(val)
}

func (h *RateLimitHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}
