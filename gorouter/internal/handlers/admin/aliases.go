package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gorouter/gorouter/internal/db"
)

type AliasHandler struct {
	db db.DatabaseManager
}

func NewAliasHandler(dbMgr db.DatabaseManager) *AliasHandler {
	return &AliasHandler{db: dbMgr}
}

func (h *AliasHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)

	return r
}

func (h *AliasHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	global, err := h.db.Aliases().ListGlobal(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if global == nil {
		global = []*db.ModelAlias{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   global,
	})
}

func (h *AliasHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	alias, err := h.db.Aliases().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if alias == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alias not found"})
		return
	}

	writeJSON(w, http.StatusOK, alias)
}

func (h *AliasHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		TargetID    string `json:"target_id"`
		Provider    string `json:"provider"`
		Description string `json:"description"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name == "" || req.TargetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and target_id are required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Aliases().FindByName(ctx, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "alias name already exists"})
		return
	}

	b := make([]byte, 16)
	rand.Read(b)

	alias := &db.ModelAlias{
		ID:          "alias_" + hex.EncodeToString(b),
		Name:        req.Name,
		TargetID:    req.TargetID,
		Provider:    req.Provider,
		Description: req.Description,
		UserID:      req.UserID,
		CreatedBy:   "",
		CreatedAt:   time.Now(),
	}

	if err := h.db.Aliases().Create(ctx, alias); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, alias)
}

func (h *AliasHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Aliases().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alias not found"})
		return
	}

	var req struct {
		Name        *string `json:"name,omitempty"`
		TargetID    *string `json:"target_id,omitempty"`
		Provider    *string `json:"provider,omitempty"`
		Description *string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.TargetID != nil {
		existing.TargetID = *req.TargetID
	}
	if req.Provider != nil {
		existing.Provider = *req.Provider
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}

	if err := h.db.Aliases().Update(ctx, existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *AliasHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	existing, err := h.db.Aliases().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alias not found"})
		return
	}

	if err := h.db.Aliases().Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
