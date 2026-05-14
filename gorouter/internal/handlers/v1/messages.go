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
	"github.com/gorouter/gorouter/internal/streaming"
	"github.com/gorouter/gorouter/internal/translator"
)

type MessagesHandler struct {
	DB     db.DatabaseManager
	Combos *combo.ComboManager
	Logger *slog.Logger
}

func NewMessagesHandler(db db.DatabaseManager, cm *combo.ComboManager, log *slog.Logger) *MessagesHandler {
	return &MessagesHandler{DB: db, Combos: cm, Logger: log}
}

type MessagesRequest struct {
	Model       string          `json:"model"`
	Messages    []ClaudeMessage `json:"messages"`
	Stream      bool            `json:"stream,omitempty"`
	System      interface{}     `json:"system,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Tools       []ClaudeTool    `json:"tools,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type MessagesAPIResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []ContentBlock  `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason"`
	StopSequence string          `json:"stop_sequence,omitempty"`
	Usage        *ClaudeUsage    `json:"usage"`
}

type ContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateID("msg")
	}

	var req MessagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Failed to parse request body", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, "messages is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	w.Header().Set("X-Request-ID", requestID)
	startTime := time.Now()

	chatReq := h.convertToChatRequest(&req)

	if req.Stream {
		h.handleStreaming(ctx, w, chatReq, requestID, startTime, req.Model)
	} else {
		h.handleNonStreaming(ctx, w, chatReq, requestID, startTime, req.Model)
	}
}

func (h *MessagesHandler) convertToChatRequest(req *MessagesRequest) *ChatRequest {
	messages := make([]ChatMessage, 0, len(req.Messages)+1)

	if req.System != nil {
		switch v := req.System.(type) {
		case string:
			if v != "" {
				messages = append(messages, ChatMessage{Role: "system", Content: v})
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if text, ok := m["text"].(string); ok {
						messages = append(messages, ChatMessage{Role: "system", Content: text})
					}
				}
			}
		}
	}

	for _, msg := range req.Messages {
		messages = append(messages, ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
	}

	return chatReq
}

func (h *MessagesHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	nonStreamReq := *req
	nonStreamReq.Stream = false

	cw := newCaptureWriter()
	providerName, finalModel, statusCode := h.executeDirectRequest(ctx, cw, &nonStreamReq, requestID)

	streaming.WriteSSEHeaders(w)
	if statusCode == http.StatusOK {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: "))
		w.Write(cw.buf.Bytes())
		w.Write([]byte("\n\n"))
		w.Write(streaming.FormatDone())
	} else {
		w.WriteHeader(statusCode)
		w.Write(cw.buf.Bytes())
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	h.recordUsage(ctx, req, startTime, true, statusCode, finalModel, providerName, originalModel)
}

func (h *MessagesHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	providerName, finalModel, statusCode := h.executeDirectRequest(ctx, w, req, requestID)
	h.recordUsage(ctx, req, startTime, false, statusCode, finalModel, providerName, originalModel)
}

func (h *MessagesHandler) executeDirectRequest(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string) (provider, model string, statusCode int) {
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

func (h *MessagesHandler) recordUsage(ctx context.Context, req *ChatRequest, startTime time.Time, isStream bool, statusCode int, model, provider, originalModel string) {
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
