package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type ModelHandler struct {
	db          db.DatabaseManager
	auditLogger *audit.AuditLogger
}

func NewModelHandler(dbManager db.DatabaseManager, auditLogger *audit.AuditLogger) *ModelHandler {
	return &ModelHandler{db: dbManager, auditLogger: auditLogger}
}

func (h *ModelHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListModels)
	r.Post("/", h.CreateModel)
	r.Get("/{id}", h.GetModel)
	r.Put("/{id}", h.UpdateModel)
	r.Delete("/{id}", h.DeleteModel)
	return r
}

func (h *ModelHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.db.Models().List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if models == nil {
		models = []*db.Model{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": models})
}

func (h *ModelHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	model, err := h.db.Models().FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if model == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model not found"})
		return
	}
	writeJSON(w, http.StatusOK, model)
}

type CreateModelRequest struct {
	ProviderID      string   `json:"provider_id"`
	ModelID         string   `json:"model_id"`
	DisplayName     string   `json:"display_name"`
	Mode            string   `json:"mode"`
	Capabilities    []string `json:"capabilities"`
	ContextWindow   int      `json:"context_window"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	InputCostPer1k  float64  `json:"input_cost_per_1k"`
	OutputCostPer1k float64  `json:"output_cost_per_1k"`
	IsActive        bool     `json:"is_active"`
	Tags            []string `json:"tags"`
	DefaultTimeoutMs int     `json:"default_timeout_ms"`
	SupportsStream  bool     `json:"supports_stream"`
}

func (h *ModelHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req CreateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ProviderID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider_id is required"})
		return
	}
	if req.ModelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_id is required"})
		return
	}
	if req.InputCostPer1k < 0 || req.OutputCostPer1k < 0 || math.IsNaN(req.InputCostPer1k) || math.IsNaN(req.OutputCostPer1k) || math.IsInf(req.InputCostPer1k, 0) || math.IsInf(req.OutputCostPer1k, 0) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_cost_per_1k and output_cost_per_1k must be non-negative finite numbers"})
		return
	}

	model := &db.Model{
		ID:               uuid.New().String(),
		ProviderID:       req.ProviderID,
		ModelID:          req.ModelID,
		DisplayName:      req.DisplayName,
		Provider:         req.ProviderID,
		ModelName:        req.ModelID,
		Mode:             req.Mode,
		Capabilities:     req.Capabilities,
		ContextWindow:    req.ContextWindow,
		MaxOutputTokens:  req.MaxOutputTokens,
		InputCostPer1k:   req.InputCostPer1k,
		OutputCostPer1k:  req.OutputCostPer1k,
		InputPrice:       req.InputCostPer1k * 1000,
		OutputPrice:      req.OutputCostPer1k * 1000,
		Enabled:          req.IsActive,
		IsActive:         req.IsActive,
		Tags:             req.Tags,
		DefaultTimeoutMs: req.DefaultTimeoutMs,
		SupportsStream:   req.SupportsStream,
	}

	if err := h.db.Models().Create(r.Context(), model); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.logAudit(r, "model_created", "model", model.ID, fmt.Sprintf("name=%s provider_id=%s", model.DisplayName, model.ProviderID))
	writeJSON(w, http.StatusCreated, model)
}

func (h *ModelHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.Models().FindByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "model not found"})
		return
	}
	var req CreateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.InputCostPer1k < 0 || req.OutputCostPer1k < 0 || math.IsNaN(req.InputCostPer1k) || math.IsNaN(req.OutputCostPer1k) || math.IsInf(req.InputCostPer1k, 0) || math.IsInf(req.OutputCostPer1k, 0) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_cost_per_1k and output_cost_per_1k must be non-negative finite numbers"})
		return
	}
	existing.ProviderID = req.ProviderID
	existing.ModelID = req.ModelID
	existing.DisplayName = req.DisplayName
	existing.Provider = req.ProviderID
	existing.ModelName = req.ModelID
	existing.Mode = req.Mode
	existing.Capabilities = req.Capabilities
	existing.ContextWindow = req.ContextWindow
	existing.MaxOutputTokens = req.MaxOutputTokens
	existing.InputCostPer1k = req.InputCostPer1k
	existing.OutputCostPer1k = req.OutputCostPer1k
	existing.InputPrice = req.InputCostPer1k * 1000
	existing.OutputPrice = req.OutputCostPer1k * 1000
	existing.IsActive = req.IsActive
	existing.Enabled = req.IsActive
	existing.Tags = req.Tags
	existing.DefaultTimeoutMs = req.DefaultTimeoutMs
	existing.SupportsStream = req.SupportsStream
	if err := h.db.Models().Update(r.Context(), existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.logAudit(r, "model_updated", "model", existing.ID, fmt.Sprintf("name=%s", existing.DisplayName))
	writeJSON(w, http.StatusOK, existing)
}

func (h *ModelHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.Models().Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.logAudit(r, "model_deleted", "model", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ModelHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}
