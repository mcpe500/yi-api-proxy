package v1

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
	"github.com/gorouter/gorouter/internal/streaming"
	"github.com/gorouter/gorouter/internal/translator"
)

type MessagesHandler struct {
	DB        db.DatabaseManager
	Combos    *combo.ComboManager
	Logger    *slog.Logger
	Optimizer *TokenOptimizer
}

func NewMessagesHandler(db db.DatabaseManager, cm *combo.ComboManager, log *slog.Logger, opt *TokenOptimizer) *MessagesHandler {
	return &MessagesHandler{DB: db, Combos: cm, Logger: log, Optimizer: opt}
}

type MessagesRequest struct {
	Model         string            `json:"model"`
	Messages      []ClaudeMessage   `json:"messages"`
	Stream        bool              `json:"stream,omitempty"`
	System        interface{}       `json:"system,omitempty"`
	MaxTokens     *int              `json:"max_tokens,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
	Tools         []ClaudeTool      `json:"tools,omitempty"`
	ToolChoice    *ClaudeToolChoice `json:"tool_choice,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Metadata      interface{}       `json:"metadata,omitempty"`
}

type ClaudeToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool"
	Name string `json:"name,omitempty"`
}

type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // can be string or array of blocks
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

	if h.Optimizer != nil {
		req.Messages, req.System, _ = h.Optimizer.ApplyToClaudeMessages(r, req.Messages, req.System)
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

func parseSystemPrompt(system interface{}) string {
	if system == nil {
		return ""
	}
	switch v := system.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		result := ""
		for i, p := range parts {
			if i > 0 {
				result += "\n"
			}
			result += p
		}
		return result
	default:
		return ""
	}
}

func parseContentToString(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		result := ""
		for i, p := range parts {
			if i > 0 {
				result += "\n"
			}
			result += p
		}
		return result
	default:
		return ""
	}
}

func (h *MessagesHandler) convertToChatRequest(req *MessagesRequest) *ChatRequest {
	messages := make([]ChatMessage, 0, len(req.Messages)+1)

	if sys := parseSystemPrompt(req.System); sys != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: sys})
	}

	for _, msg := range req.Messages {
		messages = append(messages, ChatMessage{
			Role:    msg.Role,
			Content: parseContentToString(msg.Content),
		})
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
	}

	if len(req.StopSequences) > 0 {
		chatReq.Stop = req.StopSequences
	}

	return chatReq
}

func (h *MessagesHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	streamReq := *req
	streamReq.Stream = true

	providerName, finalModel, statusCode, upstreamReader := h.executeStreamingRequest(ctx, &streamReq, requestID)

	streaming.WriteSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	if statusCode != http.StatusOK || upstreamReader == nil {
		errJSON, _ := json.Marshal(map[string]interface{}{
			"type":  "error",
			"error": map[string]string{"type": "upstream_error", "message": "upstream request failed"},
		})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errJSON))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		h.recordUsage(ctx, req, startTime, true, statusCode, finalModel, providerName, originalModel, nil)
		return
	}
	defer upstreamReader.Close()

	msgID := requestID
	if msgID == "" {
		msgID = generateID("msg")
	}

	flusher, _ := w.(http.Flusher)

	fmt.Fprint(w, formatMessageStart(msgID, originalModel))
	if flusher != nil {
		flusher.Flush()
	}

	fmt.Fprint(w, formatContentBlockStart(0))
	if flusher != nil {
		flusher.Flush()
	}

	fmt.Fprint(w, formatPing())
	if flusher != nil {
		flusher.Flush()
	}

	scanner := bufio.NewScanner(upstreamReader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	outputTokens := 0
	var stopReason string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			break
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streaming.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fmt.Fprint(w, formatContentBlockDelta(choice.Index, choice.Delta.Content))
				outputTokens++
				if flusher != nil {
					flusher.Flush()
				}
			}
			if choice.FinishReason != nil {
				if fr, ok := choice.FinishReason.(string); ok {
					stopReason = mapFinishReason(fr)
				}
			}
		}
	}

	if stopReason == "" {
		stopReason = "end_turn"
	}

	fmt.Fprint(w, formatContentBlockStop(0))
	if flusher != nil {
		flusher.Flush()
	}

	fmt.Fprint(w, formatMessageDelta(stopReason, outputTokens))
	if flusher != nil {
		flusher.Flush()
	}

	fmt.Fprint(w, formatMessageStop())
	if flusher != nil {
		flusher.Flush()
	}

	h.recordUsage(ctx, req, startTime, true, http.StatusOK, finalModel, providerName, originalModel, nil)
}

func (h *MessagesHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time, originalModel string) {
	cw := newCaptureWriter()
	providerName, finalModel, statusCode := h.executeDirectRequest(ctx, cw, req, requestID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if statusCode == http.StatusOK {
		var chatResp translator.NormalizedChatResponse
		if err := json.Unmarshal(cw.buf.Bytes(), &chatResp); err == nil {
			json.NewEncoder(w).Encode(convertChatToMessages(&chatResp))
		} else {
			writeAnthropicErrorBody(w, "failed to parse upstream response", "upstream_error")
		}
	} else {
		writeAnthropicErrorBody(w, strings.TrimSpace(cw.buf.String()), errorTypeForStatus(statusCode))
	}

	h.recordUsage(ctx, req, startTime, false, statusCode, finalModel, providerName, originalModel, nil)
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
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(bodyBytes)

	return providerConn.Provider, targetModel.ModelID, resp.StatusCode
}

func (h *MessagesHandler) executeStreamingRequest(ctx context.Context, req *ChatRequest, requestID string) (provider, model string, statusCode int, body io.ReadCloser) {
	statusCode = 200

	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		return "", req.Model, http.StatusServiceUnavailable, nil
	}

	var targetModel *db.Model
	for _, m := range models {
		if m.ModelID == req.Model || m.ModelName == req.Model || m.ID == req.Model {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		return "", req.Model, http.StatusBadRequest, nil
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		return "", targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	adapter, err := translator.Get(providerConn.Provider)
	if err != nil {
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	normReq := &translator.NormalizedChatRequest{
		Model:    targetModel.ModelID,
		Messages: convertChatMessages(req.Messages),
		Stream:   true,
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
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway, nil
	}

	httpReq.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway, nil
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return providerConn.Provider, targetModel.ModelID, resp.StatusCode, nil
	}

	return providerConn.Provider, targetModel.ModelID, http.StatusOK, resp.Body
}

func convertChatToMessages(chatResp *translator.NormalizedChatResponse) *MessagesAPIResponse {
	resp := &MessagesAPIResponse{
		ID:           chatResp.ID,
		Type:         "message",
		Role:         "assistant",
		Content:      []ContentBlock{},
		Model:        chatResp.Model,
		StopReason:   "end_turn",
		StopSequence: "",
		Usage:        &ClaudeUsage{},
	}

	for _, choice := range chatResp.Choices {
		if choice.Message != nil && choice.Message.Content != "" {
			resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: choice.Message.Content})
		}
		if choice.FinishReason != nil {
			resp.StopReason = mapFinishReason(*choice.FinishReason)
		}
	}

	if len(resp.Content) == 0 {
		resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: ""})
	}

	if chatResp.Usage != nil {
		resp.Usage.InputTokens = chatResp.Usage.PromptTokens
		resp.Usage.OutputTokens = chatResp.Usage.CompletionTokens
	}

	return resp
}

func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

func formatAnthropicEvent(eventType string, payload interface{}) string {
	data, _ := json.Marshal(payload)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(data))
}

func formatMessageStart(id, model string) string {
	return formatAnthropicEvent("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      id,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   model,
		},
	})
}

func formatContentBlockStart(index int) string {
	return formatAnthropicEvent("content_block_start", map[string]interface{}{
		"type":          "content_block_start",
		"index":         index,
		"content_block": map[string]string{"type": "text", "text": ""},
	})
}

func formatContentBlockDelta(index int, text string) string {
	return formatAnthropicEvent("content_block_delta", map[string]interface{}{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]string{"type": "text_delta", "text": text},
	})
}

func formatContentBlockStop(index int) string {
	return formatAnthropicEvent("content_block_stop", map[string]interface{}{
		"type":  "content_block_stop",
		"index": index,
	})
}

func formatMessageDelta(stopReason string, outputTokens int) string {
	return formatAnthropicEvent("message_delta", map[string]interface{}{
		"type":  "message_delta",
		"delta": map[string]interface{}{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outputTokens},
	})
}

func formatMessageStop() string {
	return formatAnthropicEvent("message_stop", map[string]string{"type": "message_stop"})
}

func formatPing() string {
	return formatAnthropicEvent("ping", map[string]string{"type": "ping"})
}

func writeAnthropicErrorBody(w io.Writer, message, errType string) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type":  "error",
		"error": map[string]string{"type": errType, "message": message},
	})
}

func (h *MessagesHandler) recordUsage(ctx context.Context, req *ChatRequest, startTime time.Time, isStream bool, statusCode int, model, provider, originalModel string, usage *translator.NormalizedUsage) {
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

	if usage != nil {
		event.PromptTokens = usage.PromptTokens
		event.CompletionTokens = usage.CompletionTokens
		event.TotalTokens = usage.TotalTokens
		event.EstimatedCost = h.calculateModelCost(ctx, model, usage.PromptTokens, usage.CompletionTokens)
	}

	h.DB.UsageEvents().Create(ctx, event)
}

func (h *MessagesHandler) calculateModelCost(ctx context.Context, modelID string, promptTokens, completionTokens int) float64 {
	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		return 0
	}
	for _, m := range models {
		if m.ModelID == modelID || m.ModelName == modelID || m.ID == modelID {
			return calculateCost(promptTokens, completionTokens, m.InputCostPer1k, m.OutputCostPer1k)
		}
	}
	return 0
}
