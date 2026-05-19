package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type UserCombosHandler struct {
	cm         *combo.ComboManager
	db         db.DatabaseManager
	jwtSecret  string
}

func NewUserCombosHandler(cm *combo.ComboManager, dbManager db.DatabaseManager, jwtSecret string) *UserCombosHandler {
	return &UserCombosHandler{
		cm:        cm,
		db:        dbManager,
		jwtSecret: jwtSecret,
	}
}

func (h *UserCombosHandler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.jwtSecret))
		r.Get("/combos", h.ListCombos)
		r.Post("/combos", h.CreateCombo)
		r.Get("/combos/{id}", h.GetCombo)
		r.Put("/combos/{id}", h.UpdateCombo)
		r.Delete("/combos/{id}", h.DeleteCombo)
		r.Post("/combos/{id}/items", h.AddComboItem)
		r.Post("/combos/auto-generate", h.AutoGenerateCombo)
		r.Delete("/combos/{id}/items/{itemId}", h.RemoveComboItem)
		r.Put("/combos/{id}/items/reorder", h.ReorderComboItems)
		r.Post("/combos/{id}/execute", h.ExecuteCombo)
	})
}

func (h *UserCombosHandler) RegisterAdmin(r chi.Router) {
	r.Route("/combos", func(r chi.Router) {
		r.Post("/auto-generate", h.AutoGenerateCombo)
	})
}

type CreateComboRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsActive    bool   `json:"is_active,omitempty"`
}

func (h *UserCombosHandler) ListCombos(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	combos, err := h.cm.ListByUser(r.Context(), user.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed to list combos"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(combos)
}

func (h *UserCombosHandler) CreateCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req CreateComboRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	c := &db.Combo{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		UserID:      user.UserID,
		IsActive:    req.IsActive,
		Items:       []*db.ComboItem{},
	}

	if err := h.cm.Create(r.Context(), c); err != nil {
		http.Error(w, `{"error":"failed to create combo"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *UserCombosHandler) GetCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.cm.GetCombo(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *UserCombosHandler) UpdateCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.cm.GetCombo(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req CreateComboRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	c.Name = req.Name
	c.Description = req.Description
	c.IsActive = req.IsActive

	if err := h.cm.Update(r.Context(), c); err != nil {
		http.Error(w, `{"error":"failed to update combo"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *UserCombosHandler) DeleteCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.cm.GetCombo(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if err := h.cm.Delete(r.Context(), id); err != nil {
		http.Error(w, `{"error":"failed to delete combo"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type AddItemRequest struct {
	ProviderID     string `json:"provider_id"`
	ModelID        string `json:"model_id"`
	Priority       int    `json:"priority,omitempty"`
	MaxRetries     int    `json:"max_retries,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type AutoGenerateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	ModelIDs    []string `json:"model_ids"`
	IsActive    bool     `json:"is_active,omitempty"`
	Priority    int      `json:"priority,omitempty"`
}

func (h *UserCombosHandler) AddComboItem(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	comboID := chi.URLParam(r, "id")
	c, err := h.cm.GetCombo(r.Context(), comboID)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		http.Error(w, `{"error":"model_id is required"}`, http.StatusBadRequest)
		return
	}

	if req.ProviderID == "" {
		// Implicit fallback: find all models with this model_id
		models, err := h.db.Models().FindByModelID(r.Context(), req.ModelID)
		if err != nil {
			http.Error(w, `{"error":"failed to find models"}`, http.StatusInternalServerError)
			return
		}
		if len(models) == 0 {
			http.Error(w, `{"error":"no enabled models found for model_id: `+req.ModelID+`"}`, http.StatusNotFound)
			return
		}

		var addedItems []*db.ComboItem
		for _, m := range models {
			item := &db.ComboItem{
				ID:             uuid.New().String(),
				ComboID:        comboID,
				ProviderID:     m.ProviderID,
				ModelID:        m.ModelID,
				Priority:       req.Priority,
				MaxRetries:     req.MaxRetries,
				TimeoutSeconds: req.TimeoutSeconds,
			}
			if err := h.cm.AddItem(r.Context(), item); err != nil {
				http.Error(w, `{"error":"failed to add item: `+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
			addedItems = append(addedItems, item)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(addedItems)
		return
	}

	item := &db.ComboItem{
		ID:             uuid.New().String(),
		ComboID:        comboID,
		ProviderID:     req.ProviderID,
		ModelID:        req.ModelID,
		Priority:       req.Priority,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	if err := h.cm.AddItem(r.Context(), item); err != nil {
		http.Error(w, `{"error":"failed to add item"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *UserCombosHandler) AutoGenerateCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req AutoGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || len(req.ModelIDs) == 0 {
		http.Error(w, `{"error":"name and model_ids are required"}`, http.StatusBadRequest)
		return
	}

	c := &db.Combo{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		UserID:      user.UserID,
		IsActive:    req.IsActive,
		Items:       []*db.ComboItem{},
	}

	if err := h.cm.Create(r.Context(), c); err != nil {
		http.Error(w, `{"error":"failed to create combo"}`, http.StatusInternalServerError)
		return
	}

	for _, modelID := range req.ModelIDs {
		models, err := h.db.Models().FindByModelID(r.Context(), modelID)
		if err != nil {
			continue
		}

		for _, m := range models {
			item := &db.ComboItem{
				ID:         uuid.New().String(),
				ComboID:    c.ID,
				ProviderID: m.ProviderID,
				ModelID:    m.ModelID,
				Priority:   req.Priority,
			}
			h.cm.AddItem(r.Context(), item)
		}
	}

	// Reload combo with items
	updated, _ := h.cm.GetCombo(r.Context(), c.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(updated)
}

func (h *UserCombosHandler) RemoveComboItem(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	itemID := chi.URLParam(r, "itemId")
	if err := h.cm.RemoveItem(r.Context(), itemID); err != nil {
		http.Error(w, `{"error":"failed to remove item"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type ReorderRequest struct {
	ItemIDs []string `json:"item_ids"`
}

func (h *UserCombosHandler) ReorderComboItems(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	comboID := chi.URLParam(r, "id")
	c, err := h.cm.GetCombo(r.Context(), comboID)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.cm.ReorderItems(r.Context(), comboID, req.ItemIDs); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserCombosHandler) ExecuteCombo(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	comboID := chi.URLParam(r, "id")

	c, err := h.cm.GetCombo(r.Context(), comboID)
	if err != nil || c == nil {
		http.Error(w, `{"error":"combo not found"}`, http.StatusNotFound)
		return
	}

	if c.UserID != user.UserID && c.UserID != "" && user.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req combo.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	result, err := h.cm.Execute(r.Context(), comboID, req)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
