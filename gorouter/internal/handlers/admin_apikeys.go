package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gorouter/gorouter/internal/apikeys"
	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/middleware"
)

type AdminApiKeyHandler struct {
	svc          *apikeys.ApiKeyService
	auditLogger  *audit.AuditLogger
}

func NewAdminApiKeyHandler(svc *apikeys.ApiKeyService, auditLogger *audit.AuditLogger) *AdminApiKeyHandler {
	return &AdminApiKeyHandler{svc: svc, auditLogger: auditLogger}
}

func (h *AdminApiKeyHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.ListAllKeys(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, k := range keys {
		k.KeyHash = ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"keys": keys, "total": len(keys)})
}

type adminCreateKeyRequest struct {
	UserID    string   `json:"user_id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at,omitempty"`
}

func (h *AdminApiKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req adminCreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if userID == "" {
		userID = req.UserID
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

	created, err := h.svc.CreateKey(r.Context(), userID, req.Name, req.Scopes, expiresAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "apikey_created", "api_key", created.ID, fmt.Sprintf("user_id=%s name=%s", userID, req.Name))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "API key created. Save the key now - it will not be shown again.",
		"data":    created,
	})
}

func (h *AdminApiKeyHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	key.KeyHash = ""
	writeJSON(w, http.StatusOK, key)
}

func (h *AdminApiKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyID")
	if err := h.svc.RevokeKey(r.Context(), keyID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.logAudit(r, "apikey_revoked", "api_key", keyID, "")
	writeJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}

func (h *AdminApiKeyHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyID")
	created, err := h.svc.RotateKey(r.Context(), keyID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.logAudit(r, "apikey_rotated", "api_key", keyID, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "API key rotated. Save the new key now - it will not be shown again.",
		"data":    created,
	})
}

func (h *AdminApiKeyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListAll)
	r.Post("/", h.ListAll)
	r.Post("/create", h.Create)
	r.Post("/{keyID}/revoke", h.Revoke)
	r.Post("/{keyID}/rotate", h.Rotate)
	r.Get("/{keyID}", h.Get)
	return r
}

func (h *AdminApiKeyHandler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/api-keys", func(r chi.Router) {
		r.Use(middleware.RequireAdmin())
		r.Get("/", h.ListAll)
		r.Post("/create", h.Create)
		r.Post("/{keyID}/revoke", h.Revoke)
		r.Post("/{keyID}/rotate", h.Rotate)
		r.Get("/{keyID}", h.Get)
	})
}

func (h *AdminApiKeyHandler) RegisterUserKeyRoutes(r chi.Router) {
	r.Route("/users/{userID}/api-keys", func(r chi.Router) {
		r.Post("/", h.Create)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *AdminApiKeyHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}
