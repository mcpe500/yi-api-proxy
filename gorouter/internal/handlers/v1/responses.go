package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
	"github.com/gorouter/gorouter/internal/translator"
)

type ResponsesHandler struct {
	DB     db.DatabaseManager
	Combos *combo.ComboManager
	Logger *slog.Logger
}

func NewResponsesHandler(db db.DatabaseManager, cm *combo.ComboManager, log *slog.Logger) *ResponsesHandler {
	return &ResponsesHandler{DB: db, Combos: cm, Logger: log}
}

type ResponsesRequest struct {
	Model       string      `json:"model"`
	Input       interface{} `json:"input"`
	Stream      bool        `json:"stream,omitempty"`
	Tools       interface{} `json:"tools,omitempty"`
	ToolChoice  interface{} `json:"tool_choice,omitempty"`
	MaxTokens   *int        `json:"max_tokens,omitempty"`
	Temperature *float64    `json:"temperature,omitempty"`
}

type ResponsesAPIResponse struct {
	ID             string            `json:"id"`
	Object         string            `json:"object"`
	Created        int64             `json:"created"`
	Model          string            `json:"model"`
	Output         []ResponseOutput  `json:"output"`
	ServiceTier    string            `json:"service_tier,omitempty"`
	RequiredAction *Action           `json:"required_action,omitempty"`
	Usage          *ResponsesUsage   `json:"usage,omitempty"`
}

type ResponseOutput struct {
	Type         string               `json:"type"`
	Message      *ResponseMessage     `json:"message,omitempty"`
	FunctionCall *FunctionCallResult  `json:"function_call,omitempty"`
	Index        int                  `json:"index"`
	Status       string               `json:"status"`
	FinishReason string               `json:"finish_reason"`
}

type ResponseMessage struct {
	Role    string       `json:"role"`
	Content []ContentRef `json:"content"`
}

type ContentRef struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type FunctionCallResult struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	ID        string `json:"id"`
}

type Action struct {
	Type        string   `json:"type"`
	RequiredIDs []string `json:"required_ids"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (h *ResponsesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateID("resp")
	}

	var req ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Failed to parse request body", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Input == nil {
		writeError(w, "input is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	w.Header().Set("X-Request-ID", requestID)
	startTime := time.Now()

	chatReq := h.convertToChatRequest(&req)
	if chatReq == nil {
		writeError(w, "invalid input format", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Stream {
		h.handleStreaming(ctx, w, chatReq, requestID, startTime, req.Model)
	} else {
		h.handleNonStreaming(ctx, w, chatReq, requestID, startTime, req.Model)
	}
}

func (h *ResponsesHandler) convertToChatRequest(req *ResponsesRequest) *ChatRequest {
	messages := h.extractMessages(req.Input)
	if len(messages) == 0 {
		return nil
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	return chatReq
}

func (h *ResponsesHandler) extractMessages(input interface{}) []ChatMessage {
	switch v := input.(type) {
	case string:
		return []ChatMessage{{Role: "user", Content: v}}
	case []interface{}:
		if len(v) == 0 {
			return nil
		}
		switch v[0].(type) {
		case string:
			var msgs []ChatMessage
			for i, item := range v {
				if s, ok := item.(string); ok {
					role := "user"
					if i%2 == 1 {
						role = "assistant"
					}
					msgs = append(msgs, ChatMessage{Role: role, Content: s})
				}
			}
			return msgs
		case map[string]interface{}:
			var msgs []ChatMessage
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					role, _ := m["role"].(string)
					content, _ := m["content"].(string)
					if role != "" && content != "" {
						msgs = append(msgs, ChatMessage{Role: role, Content: content})
					}
				}
			}
			return msgs
		}
	}
	return nil
}

func (h *ResponsesHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	writeError(w, "/v1/responses streaming not yet implemented", "not_implemented", "not_implemented", http.StatusNotImplemented)
}

func (h *ResponsesHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	providerName, finalModel, statusCode := h.executeDirectRequest(ctx, w, req, requestID)
	h.recordUsage(ctx, req, startTime, false, statusCode, finalModel, providerName, originalModel)
}

func (h *ResponsesHandler) executeDirectRequest(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string) (provider, model string, statusCode int) {
	statusCode = 200

	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		writeError(w, "failed to load models", "upstream_error", "provider_error", http.StatusServiceUnavailable)
		return "", req.Model, http.StatusServiceUnavailable
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
		return "", req.Model, http.StatusBadRequest
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		writeError(w, "provider not found", "upstream_error", "provider_not_found", http.StatusServiceUnavailable)
		return "", targetModel.ModelID, http.StatusServiceUnavailable
	}

	adapter, err := translator.Get(providerConn.Provider)
	if err != nil {
		writeError(w, "adapter not found for provider: "+providerConn.Provider, "upstream_error", "adapter_error", http.StatusServiceUnavailable)
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeError(w, "failed to decrypt provider secret", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable
	}

	normReq := &translator.NormalizedChatRequest{
		Model:    targetModel.ModelID,
		Messages: convertChatMessages(req.Messages),
		Stream:   false,
	}

	if req.Temperature != nil {
		normReq.Temperature = req.Temperature
	}
	if req.MaxTokens != nil {
		normReq.MaxTokens = req.MaxTokens
	}
	if req.TopP != nil {
		normReq.TopP = req.TopP
	}

	httpReq, err := adapter.TranslateRequest(normReq, providerConn.BaseURL, decryptedKey)
	if err != nil {
		writeError(w, "failed to translate request", "upstream_error", "translate_error", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway
	}

	httpReq.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		writeError(w, "upstream request failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(bodyBytes)
		return providerConn.Provider, targetModel.ModelID, resp.StatusCode
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)

	return providerConn.Provider, targetModel.ModelID, http.StatusOK
}

func (h *ResponsesHandler) recordUsage(ctx context.Context, req *ChatRequest, startTime time.Time, isStream bool, statusCode int, model, provider, originalModel string) {
	if h.DB == nil {
		return
	}

	claims := middleware.GetUserClaims(ctx)
	userID := ""
	apiKeyID := ""

	if claims != nil {
		userID = claims.UserID
	}

	if v := ctx.Value(middleware.ApiKeyValidationKey); v != nil {
		if av, ok := v.(*db.ApiKeyValidation); ok {
			if userID == "" {
				userID = av.UserID
			}
			apiKeyID = av.KeyID
		}
	}

	event := &db.UsageEvent{
		ID:             generateID("usage"),
		UserID:         userID,
		ApiKeyID:       apiKeyID,
		Provider:       provider,
		RequestedModel: originalModel,
		FinalModel:     model,
		StatusCode:     statusCode,
		IsStream:       isStream,
		LatencyMs:      int(time.Since(startTime).Milliseconds()),
		CreatedAt:      time.Now(),
	}

	h.DB.UsageEvents().Create(ctx, event)
}
