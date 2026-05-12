package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gorouter/gorouter/internal/apikeys"
	"github.com/gorouter/gorouter/internal/middleware"
)

type UserKeyHandler struct {
	svc       *apikeys.ApiKeyService
	jwtSecret string
}

func NewUserKeyHandler(svc *apikeys.ApiKeyService, jwtSecret string) *UserKeyHandler {
	return &UserKeyHandler{svc: svc, jwtSecret: jwtSecret}
}

type createKeyRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at,omitempty"`
}

func (h *UserKeyHandler) ListOwnKeys(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	keys, err := h.svc.ListKeys(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	for _, k := range keys {
		k.KeyHash = ""
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"keys": keys, "total": len(keys)})
}

func (h *UserKeyHandler) CreateOwnKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		req.Name = "api-key"
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expires_at format, use RFC3339"})
			return
		}
		expiresAt = &t
	}

	created, err := h.svc.CreateKey(r.Context(), claims.UserID, req.Name, req.Scopes, expiresAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "API key created. Save the key now - it will not be shown again.",
		"data":    created,
	})
}

func (h *UserKeyHandler) RevokeOwnKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	keyID := chi.URLParam(r, "keyID")
	key, err := h.svc.GetKey(r.Context(), keyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if key == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "API key not found"})
		return
	}
	if key.UserID != claims.UserID && claims.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot revoke key owned by another user"})
		return
	}

	if err := h.svc.RevokeKey(r.Context(), keyID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}

func (h *UserKeyHandler) RegisterRoutes(r chi.Router) {
	r.Route("/me/api-keys", func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.jwtSecret))
		r.Get("/", h.ListOwnKeys)
		r.Post("/", h.CreateOwnKey)
		r.Post("/{keyID}/revoke", h.RevokeOwnKey)
	})
}