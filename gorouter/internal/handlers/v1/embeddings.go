package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/db"
)

type EmbeddingHandler struct {
	db          db.DatabaseManager
	httpClient  *http.Client
	upstreamURL string
}

func NewEmbeddingHandler(dbManager db.DatabaseManager, upstreamURL string) *EmbeddingHandler {
	return &EmbeddingHandler{
		db: dbManager,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		upstreamURL: upstreamURL,
	}
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

	if err := h.validateModel(r.Context(), req.Model); err != nil {
		writeError(w, err.Error(), "invalid_request_error", "", http.StatusBadRequest)
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

	if h.upstreamURL != "" {
		h.forwardToProvider(w, r, body, req.Model)
		return
	}

	h.sendMockResponse(w, req.Model, inputs, req.EncodingFormat)
}

func (h *EmbeddingHandler) validateModel(ctx context.Context, modelName string) error {
	models, err := h.db.Models().ListEnabled(ctx)
	if err != nil {
		return nil
	}

	for _, m := range models {
		if m.ModelName == modelName || m.ID == modelName {
			return nil
		}
	}

	return fmt.Errorf("model not found: %s", modelName)
}

func (h *EmbeddingHandler) forwardToProvider(w http.ResponseWriter, r *http.Request, body []byte, model string) {
	url := h.upstreamURL + "/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		writeError(w, "Failed to create request", "internal_error", "", http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", r.Header.Get("Authorization"))

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		writeError(w, "Upstream request failed", "upstream_error", "", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *EmbeddingHandler) sendMockResponse(w http.ResponseWriter, model string, inputs []string, encodingFormat string) {
	data := make([]EmbeddingObject, len(inputs))
	for i := range inputs {
		embedding := make([]float64, 1536)
		for j := range embedding {
			embedding[j] = 0.001 * float64(i+j)
		}
		data[i] = EmbeddingObject{
			Object:    "embedding",
			Embedding: embedding,
			Index:     i,
		}
	}

	response := EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  model,
		Usage: EmbeddingUsage{
			PromptTokens: len(inputs) * 10,
			TotalTokens:  len(inputs) * 10,
		},
	}

	if encodingFormat == "base64" {
		response.Data[0].Embedding = nil
		response.Data[0].Object = "embedding"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func HandleEmbeddings(w http.ResponseWriter, r *http.Request) {
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

	data := make([]EmbeddingObject, len(inputs))
	for i := range inputs {
		embedding := make([]float64, 1536)
		for j := range embedding {
			embedding[j] = 0.001 * float64(i+j)
		}
		data[i] = EmbeddingObject{
			Object:    "embedding",
			Embedding: embedding,
			Index:     i,
		}
	}

	response := EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  req.Model,
		Usage: EmbeddingUsage{
			PromptTokens: len(inputs) * 10,
			TotalTokens:  len(inputs) * 10,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
