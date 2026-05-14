package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
)

type ImagesHandler struct {
	DB     db.DatabaseManager
	Logger *slog.Logger
}

func NewImagesHandler(db db.DatabaseManager, log *slog.Logger) *ImagesHandler {
	return &ImagesHandler{DB: db, Logger: log}
}

type ImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`
}

type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

func (h *ImagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", "invalid_request_error", "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "failed to read request body", "invalid_request_error", "read_error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req ImageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, fmt.Sprintf("invalid JSON: %v", err), "invalid_request_error", "parse_error", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		writeError(w, "failed to load models", "internal_error", "db_error", http.StatusServiceUnavailable)
		return
	}

	var targetModel *db.Model
	for _, m := range models {
		if m.ModelID == req.Model || m.ModelName == req.Model || m.ID == req.Model {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		writeError(w, "model not found: "+req.Model, "invalid_request_error", "model_not_found", http.StatusBadRequest)
		return
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		writeError(w, "provider not found", "upstream_error", "provider_not_found", http.StatusServiceUnavailable)
		return
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "failed to decrypt provider secret", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return
	}

	proxyURL := providerConn.BaseURL + "/v1/images/generations"
	proxyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, "failed to create proxy request", "internal_error", "proxy_error", http.StatusInternalServerError)
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Authorization", "Bearer "+decryptedKey)
	if rid := r.Header.Get("X-Request-ID"); rid != "" {
		proxyReq.Header.Set("X-Request-ID", rid)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, "upstream request failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
