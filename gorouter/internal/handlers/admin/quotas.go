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
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type QuotaHandler struct {
	db          db.DatabaseManager
	auditLogger *audit.AuditLogger
}

func NewQuotaHandler(dbManager db.DatabaseManager, auditLogger *audit.AuditLogger) *QuotaHandler {
	return &QuotaHandler{db: dbManager, auditLogger: auditLogger}
}

func (h *QuotaHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/reset-usage", h.ResetUsage)
	return r
}

func (h *QuotaHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	quotas, err := h.db.Quotas().List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if quotas == nil {
		quotas = []*db.Quota{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   quotas,
	})
}

func (h *QuotaHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	quota, err := h.db.Quotas().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if quota == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "quota not found"})
		return
	}

	writeJSON(w, http.StatusOK, quota)
}

type CreateQuotaRequest struct {
	UserID          string  `json:"user_id"`
	MonthlyTokenCap int64   `json:"monthly_token_cap"`
	MonthlyCostCap  float64 `json:"monthly_cost_cap"`
}

func (h *QuotaHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateQuotaRequest
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

	existing, err := h.db.Quotas().FindByUserID(ctx, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "quota already exists for this user"})
		return
	}

	resetAt := time.Now().AddDate(0, 1, 0)
	quota := &db.Quota{
		ID:              uuid.New().String(),
		UserID:          req.UserID,
		MonthlyTokenCap: req.MonthlyTokenCap,
		MonthlyCostCap:  req.MonthlyCostCap,
		UsedTokens:      0,
		UsedCost:        0,
		ResetAt:         resetAt,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.db.Quotas().Create(ctx, quota); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "quota_created", "quota", quota.ID, fmt.Sprintf("user_id=%s token_cap=%d cost_cap=%.4f", quota.UserID, quota.MonthlyTokenCap, quota.MonthlyCostCap))

	writeJSON(w, http.StatusCreated, quota)
}

type UpdateQuotaRequest struct {
	MonthlyTokenCap *int64   `json:"monthly_token_cap,omitempty"`
	MonthlyCostCap  *float64 `json:"monthly_cost_cap,omitempty"`
}

func (h *QuotaHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Quotas().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "quota not found"})
		return
	}

	var req UpdateQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.MonthlyTokenCap != nil {
		existing.MonthlyTokenCap = *req.MonthlyTokenCap
	}
	if req.MonthlyCostCap != nil {
		existing.MonthlyCostCap = *req.MonthlyCostCap
	}
	existing.UpdatedAt = time.Now()

	if err := h.db.Quotas().Update(ctx, existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "quota_updated", "quota", existing.ID, fmt.Sprintf("user_id=%s token_cap=%d cost_cap=%.4f", existing.UserID, existing.MonthlyTokenCap, existing.MonthlyCostCap))

	writeJSON(w, http.StatusOK, existing)
}

func (h *QuotaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Quotas().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "quota not found"})
		return
	}

	if err := h.db.Quotas().Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "quota_deleted", "quota", id, fmt.Sprintf("deleted user_id=%s", existing.UserID))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *QuotaHandler) ResetUsage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	quota, err := h.db.Quotas().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if quota == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "quota not found"})
		return
	}

	if err := h.db.Quotas().ResetMonthly(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "quota_reset", "quota", id, fmt.Sprintf("user_id=%s", quota.UserID))

	quota.UsedTokens = 0
	quota.UsedCost = 0
	quota.ResetAt = time.Now().AddDate(0, 1, 0)
	writeJSON(w, http.StatusOK, quota)
}

func (h *QuotaHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}