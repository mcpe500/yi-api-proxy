package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/translator"
)

type EmbeddingHandler struct {
	db db.DatabaseManager
}

func NewEmbeddingHandler(dbManager db.DatabaseManager, _ string) *EmbeddingHandler {
	return &EmbeddingHandler{db: dbManager}
}

type EmbeddingRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	User           string      `json:"user,omitempty"`
}

type EmbeddingResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  EmbeddingUsage     `json:"usage"`
}

type EmbeddingObject struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (h *EmbeddingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", "invalid_request_error", "", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, "Failed to read request body", "invalid_request_error", "", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req EmbeddingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, fmt.Sprintf("Invalid JSON: %v", err), "invalid_request_error", "", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "Model is required", "invalid_request_error", "", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	models, err := h.db.Models().ListEnabled(ctx)
	if err != nil {
		writeError(w, "failed to load models", "internal_error", "", http.StatusServiceUnavailable)
		return
	}

	var targetModel *db.Model
	for _, m := range models {
		if m.ModelName == req.Model || m.ID == req.Model || m.ModelID == req.Model {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		writeError(w, "model not found: "+req.Model, "invalid_request_error", "model_not_found", http.StatusBadRequest)
		return
	}

	providerConn, err := h.db.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		writeError(w, "provider not found", "upstream_error", "provider_not_found", http.StatusServiceUnavailable)
		return
	}

	adapter, err := translator.Get(providerConn.Provider)
	if err != nil {
		writeError(w, "adapter not found for provider: "+providerConn.Provider, "upstream_error", "adapter_error", http.StatusServiceUnavailable)
		return
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "failed to decrypt provider secret", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return
	}

	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				inputs = append(inputs, str)
			}
		}
	default:
		writeError(w, "Input must be a string or array of strings", "invalid_request_error", "", http.StatusBadRequest)
		return
	}

	if len(inputs) == 0 {
		writeError(w, "Input cannot be empty", "invalid_request_error", "", http.StatusBadRequest)
		return
	}

	httpReq, err := adapter.TranslateEmbeddingRequest(targetModel.ModelID, inputs, providerConn.BaseURL, decryptedKey)
	if err != nil {
		writeError(w, "failed to translate request", "upstream_error", "translate_error", http.StatusBadGateway)
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		writeError(w, "upstream request failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(bodyBytes)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}