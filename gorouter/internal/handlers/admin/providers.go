package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gorouter/gorouter/internal/audit"
	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
)

type ProviderHandler struct {
	db          db.DatabaseManager
	auditLogger *audit.AuditLogger
	httpClient  *http.Client
}

func NewProviderHandler(dbMgr db.DatabaseManager, auditLogger *audit.AuditLogger) *ProviderHandler {
	return &ProviderHandler{
		db:          dbMgr,
		auditLogger: auditLogger,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *ProviderHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/test", h.Test)
	r.Post("/{id}/enable", h.Enable)
	r.Post("/{id}/disable", h.Disable)
	return r
}

type ProviderResponse struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	AuthType string `json:"auth_type"`
	BaseURL  string `json:"base_url,omitempty"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Status   string `json:"status"`
}

func providerToResponse(conn *db.ProviderConnection) *ProviderResponse {
	return &ProviderResponse{
		ID:       conn.ID,
		Provider: conn.Provider,
		Name:     conn.Name,
		AuthType: conn.AuthType,
		BaseURL:  conn.BaseURL,
		Priority: conn.Priority,
		Weight:   conn.Weight,
		Status:   conn.Status,
	}
}

func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	providers, err := h.db.Providers().List(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var resp []*ProviderResponse
	for _, p := range providers {
		resp = append(resp, providerToResponse(p))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   resp,
		"total":  len(resp),
	})
}

func (h *ProviderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	writeJSON(w, http.StatusOK, providerToResponse(conn))
}

type CreateProviderRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	AuthType string `json:"auth_type"`
	APIKey   string `json:"api_key,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
}

func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Provider == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider is required"})
		return
	}
	if req.AuthType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auth_type is required"})
		return
	}
	if req.APIKey == "" && req.BaseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api_key or base_url is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	encryptedSecret := []byte{}
	if req.APIKey != "" {
		encrypted, err := crypto.Encrypt(req.APIKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt secret"})
			return
		}
		encryptedSecret = []byte(encrypted)
	}

	conn := &db.ProviderConnection{
		ID:              uuid.New().String(),
		Name:            req.Name,
		Provider:        req.Provider,
		AuthType:        req.AuthType,
		EncryptedSecret: encryptedSecret,
		BaseURL:         req.BaseURL,
		Priority:        req.Priority,
		Weight:          req.Weight,
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.db.Providers().Create(ctx, conn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "provider_created", "provider", conn.ID, fmt.Sprintf("name=%s provider=%s", conn.Name, conn.Provider))

	writeJSON(w, http.StatusCreated, providerToResponse(conn))
}

type UpdateProviderRequest struct {
	Name     string `json:"name,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
}

func (h *ProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name != "" {
		conn.Name = req.Name
	}
	if req.BaseURL != "" {
		conn.BaseURL = req.BaseURL
	}
	if req.Priority > 0 {
		conn.Priority = req.Priority
	}
	if req.Weight > 0 {
		conn.Weight = req.Weight
	}

	if err := h.db.Providers().Update(ctx, conn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "provider_updated", "provider", conn.ID, fmt.Sprintf("name=%s", conn.Name))

	writeJSON(w, http.StatusOK, providerToResponse(conn))
}

func (h *ProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	if err := h.db.Providers().Delete(ctx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(r, "provider_deleted", "provider", id, fmt.Sprintf("name=%s", conn.Name))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ProviderHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	var apiKey string
	if len(conn.EncryptedSecret) > 0 {
		decrypted, err := crypto.Decrypt(string(conn.EncryptedSecret))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decrypt secret"})
			return
		}
		apiKey = decrypted
	}

	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = getProviderDefaultURL(conn.Provider)
	}

	if baseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no base_url configured and no default known"})
		return
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/models", nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	switch conn.AuthType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	case "api_key":
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("connection failed: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		h.logAudit(r, "provider_tested", "provider", id, fmt.Sprintf("name=%s status_code=%d", conn.Name, resp.StatusCode))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "success",
			"status_code": resp.StatusCode,
			"message":    "connection successful",
		})
		return
	}

	writeJSON(w, resp.StatusCode, map[string]interface{}{
		"status":     "error",
		"status_code": resp.StatusCode,
		"body":        string(body),
		"message":     "provider returned error",
	})
}

func (h *ProviderHandler) Enable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	if err := h.db.Providers().UpdateStatus(ctx, id, "active"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	conn.Status = "active"
	h.logAudit(r, "provider_enabled", "provider", id, fmt.Sprintf("name=%s", conn.Name))
	writeJSON(w, http.StatusOK, providerToResponse(conn))
}

func (h *ProviderHandler) Disable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	conn, err := h.db.Providers().FindByID(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if conn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	if err := h.db.Providers().UpdateStatus(ctx, id, "disabled"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	conn.Status = "disabled"
	h.logAudit(r, "provider_disabled", "provider", id, fmt.Sprintf("name=%s", conn.Name))
	writeJSON(w, http.StatusOK, providerToResponse(conn))
}

func getProviderDefaultURL(provider string) string {
	defaults := map[string]string{
		"openai":        "https://api.openai.com",
		"anthropic":     "https://api.anthropic.com",
		"google":        "https://generativelanguage.googleapis.com",
		"deepseek":      "https://api.deepseek.com",
		"mistral":       "https://api.mistral.ai",
		"groq":          "https://api.groq.com",
		"together":      "https://api.together.xyz",
		"perplexity":    "https://api.perplexity.ai",
		"ollama":        "http://localhost:11434",
		"lmstudio":      "http://localhost:1234",
		"azure":         "https://{resource}.openai.us",
	}
	if url, ok := defaults[provider]; ok {
		return url
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func copyHeader(dst, src http.Header, key string) {
	if v := src.Get(key); v != "" {
		dst.Set(key, v)
	}
}

func copyBody(dst *bytes.Buffer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}

func (h *ProviderHandler) logAudit(r *http.Request, action, targetType, targetID, details string) {
	if h.auditLogger == nil {
		return
	}
	actor := middleware.GetAuditActor(r.Context())
	h.auditLogger.Log(r.Context(), actor, action, targetType, targetID, details)
}