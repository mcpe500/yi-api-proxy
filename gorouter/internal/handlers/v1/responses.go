package v1

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/crypto"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
	"github.com/gorouter/gorouter/internal/streaming"
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
	Model         string          `json:"model"`
	Input         interface{}     `json:"input"`
	Stream        bool            `json:"stream,omitempty"`
	StreamOptions *StreamOptions  `json:"stream_options,omitempty"`
	Tools         interface{}     `json:"tools,omitempty"`
	ToolChoice    interface{}     `json:"tool_choice,omitempty"`
	MaxTokens     *int            `json:"max_tokens,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	Seed          *int            `json:"seed,omitempty"`
	User          string          `json:"user,omitempty"`
	Metadata      interface{}     `json:"metadata,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ResponsesAPIResponse struct {
	ID             string           `json:"id"`
	Object         string           `json:"object"`
	Created        int64            `json:"created"`
	Model          string           `json:"model"`
	Output         []ResponseOutput `json:"output"`
	ServiceTier    string           `json:"service_tier,omitempty"`
	RequiredAction *Action          `json:"required_action,omitempty"`
	Usage          *ResponsesUsage  `json:"usage,omitempty"`
}

type ResponseOutput struct {
	Type         string          `json:"type"`
	ID           string          `json:"id,omitempty"`
	Message      *ResponseMessage `json:"message,omitempty"`
	FunctionCall *FunctionCall    `json:"function_call,omitempty"`
	Index        int             `json:"index"`
	Status       string          `json:"status,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

type ResponseMessage struct {
	Role    string           `json:"role"`
	Content []ResponseContent `json:"content"`
}

type ResponseContent struct {
	Type        string       `json:"type"`
	Text        string       `json:"text,omitempty"`
	Index       int          `json:"index,omitempty"`
	Audio       *AudioContent `json:"audio,omitempty"`
	Refusal     string       `json:"refusal,omitempty"`
}

type AudioContent struct {
	Id       string `json:"id"`
	OutputTs string `json:"output_ts,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

type FunctionCall struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Arguments     string `json:"arguments"`
	ParseError    bool   `json:"parse_error,omitempty"`
}

type Action struct {
	Type        string   `json:"type"`
	RequiredIDs []string `json:"required_ids"`
}

type ResponsesUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
	InputTokensDetails *InputTokensDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *OutputTokensDetails `json:"output_tokens_details,omitempty"`
}

type InputTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type OutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

type ResponsesStreamChunk struct {
	ID             string                  `json:"id"`
	Object         string                  `json:"object"`
	Created        int64                   `json:"created"`
	Model          string                  `json:"model"`
	Choices        []ResponsesStreamChoice `json:"choices"`
	Usage          *ResponsesUsage         `json:"usage,omitempty"`
	OutputIndex    int                     `json:"output_index,omitempty"`
	ContentIndex   int                     `json:"content_index,omitempty"`
	RefusalIndex   int                     `json:"refusal_index,omitempty"`
}

type ResponsesStreamChoice struct {
	Index        int                     `json:"index"`
	Delta        ResponseDelta           `json:"delta"`
	FinishReason string                  `json:"finish_reason,omitempty"`
}

type ResponseDelta struct {
	Type        string                  `json:"type"`
	Content     []ResponseContent       `json:"content,omitempty"`
	Refusal     string                  `json:"refusal,omitempty"`
	FunctionCall *FunctionCall           `json:"function_call,omitempty"`
}

type ResponsesErrorResponse struct {
	Error ResponsesError `json:"error"`
}

type ResponsesError struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func parseResponsesInput(input interface{}) ([]translator.NormalizedMessage, error) {
	switch v := input.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return []translator.NormalizedMessage{{Role: "user", Content: v}}, nil

	case []interface{}:
		if len(v) == 0 {
			return nil, nil
		}

		first, ok := v[0].(map[string]interface{})
		if !ok {
			var msgs []translator.NormalizedMessage
			for i, item := range v {
				s, ok := item.(string)
				if !ok {
					continue
				}
				role := "user"
				if i%2 == 1 {
					role = "assistant"
				}
				msgs = append(msgs, translator.NormalizedMessage{Role: role, Content: s})
			}
			return msgs, nil
		}

		if _, hasRole := first["role"]; hasRole {
			return parseMessageArray(v)
		}

		if _, hasType := first["type"]; hasType {
			return parsePartsArray(v)
		}

		return nil, nil

	default:
		return nil, nil
	}
}

func parseMessageArray(items []interface{}) ([]translator.NormalizedMessage, error) {
	var msgs []translator.NormalizedMessage
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := m["role"].(string)
		content := extractContent(m["content"])

		if role != "" && content != "" {
			msg := translator.NormalizedMessage{Role: role, Content: content}

			if tc, ok := m["tool_calls"]; ok {
				msg.ToolCalls = tc
			}
			if tr, ok := m["tool_role"]; ok {
				msg.ToolChoice = tr
			}

			msgs = append(msgs, msg)
		}
	}
	return msgs, nil
}

func parsePartsArray(items []interface{}) ([]translator.NormalizedMessage, error) {
	var contentParts []map[string]interface{}
	for _, item := range items {
		part, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		contentParts = append(contentParts, part)
	}

	if len(contentParts) == 0 {
		return nil, nil
	}

	contentJSON, _ := json.Marshal(contentParts)
	return []translator.NormalizedMessage{{Role: "user", Content: string(contentJSON)}}, nil
}

func extractContent(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if m, ok := v.(map[string]interface{}); ok {
		if text, ok := m["text"].(string); ok {
			return text
		}
	}
	return ""
}

func (h *ResponsesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateID("resp")
	}

	var req ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResponsesError(w, "Failed to parse request body", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeResponsesError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Input == nil {
		writeResponsesError(w, "input is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	w.Header().Set("X-Request-ID", requestID)
	startTime := time.Now()

	messages, err := parseResponsesInput(req.Input)
	if err != nil || len(messages) == 0 {
		writeResponsesError(w, "invalid input format", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Stream {
		h.handleStreaming(ctx, w, &req, messages, requestID, startTime)
	} else {
		h.handleNonStreaming(ctx, w, &req, messages, requestID, startTime)
	}
}

func (h *ResponsesHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ResponsesRequest, messages []translator.NormalizedMessage, requestID string, startTime time.Time) {
	providerName, finalModel, statusCode, usage := h.executeChatRequest(ctx, w, req, messages, requestID, false)

	if statusCode != http.StatusOK {
		h.recordUsage(ctx, req, messages, startTime, false, statusCode, finalModel, providerName)
		return
	}

	if usage != nil {
		_ = usage
	}

	h.recordUsage(ctx, req, messages, startTime, false, statusCode, finalModel, providerName)
}

func (h *ResponsesHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ResponsesRequest, messages []translator.NormalizedMessage, requestID string, startTime time.Time) {
	streaming.WriteSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeResponsesError(w, "streaming not supported", "invalid_request_error", "streaming_not_supported", http.StatusBadRequest)
		return
	}

	includeUsage := req.StreamOptions != nil && req.StreamOptions.IncludeUsage

	upstream, err := h.executeStreamingRequest(ctx, req, messages, requestID)
	if err != nil {
		writeSSEError(w, "upstream request failed: "+err.Error())
		flusher.Flush()
		h.recordUsage(ctx, req, messages, startTime, true, http.StatusBadGateway, req.Model, "unknown")
		return
	}
	defer upstream.Close()

	scanner := bufio.NewScanner(upstream)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var totalUsage *ResponsesUsage
	created := time.Now().Unix()
	respID := requestID

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			if line == "" {
				fmt.Fprint(w, "\n")
				flusher.Flush()
			}
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			if includeUsage && totalUsage != nil {
				usageChunk := ResponsesStreamChunk{
					ID:      respID,
					Object:  "response.chat.completion.chunk",
					Created: created,
					Model:   req.Model,
					Choices: []ResponsesStreamChoice{{Index: 0, Delta: ResponseDelta{Type: "content", Content: []ResponseContent{}}}},
					Usage:   totalUsage,
				}
				usageJSON, _ := json.Marshal(usageChunk)
				fmt.Fprintf(w, "data: %s\n\n", string(usageJSON))
				flusher.Flush()
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		var chatChunk streaming.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chatChunk); err != nil {
			continue
		}

		if chatChunk.Created != 0 {
			created = chatChunk.Created
		}
		if chatChunk.ID != "" {
			respID = chatChunk.ID
		}

		respChunk := convertChatChunkToResponsesStream(chatChunk, respID, created)
		respJSON, _ := json.Marshal(respChunk)
		fmt.Fprintf(w, "data: %s\n\n", string(respJSON))
		flusher.Flush()

		var usageData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &usageData); err == nil {
			if u, ok := usageData["usage"].(map[string]interface{}); ok {
				if totalUsage == nil {
					totalUsage = &ResponsesUsage{}
				}
				if pt, ok := u["prompt_tokens"].(float64); ok {
					totalUsage.InputTokens += int(pt)
				}
				if ct, ok := u["completion_tokens"].(float64); ok {
					totalUsage.OutputTokens += int(ct)
				}
				if tt, ok := u["total_tokens"].(float64); ok {
					totalUsage.TotalTokens += int(tt)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		h.Logger.Error("streaming scan error", "err", err)
	}

	h.recordUsage(ctx, req, messages, startTime, true, http.StatusOK, req.Model, "unknown")
}

func convertChatChunkToResponsesStream(chunk streaming.ChatCompletionChunk, respID string, created int64) ResponsesStreamChunk {
	rc := ResponsesStreamChunk{
		ID:      respID,
		Object:  "response.chat.completion.chunk",
		Created: created,
		Model:   chunk.Model,
	}

	for i, choice := range chunk.Choices {
		var delta ResponseDelta
		if choice.Delta.Content != "" {
			delta.Type = "content"
			delta.Content = []ResponseContent{{Type: "output_text", Text: choice.Delta.Content}}
		}

		finishReason := ""
		if choice.FinishReason != nil {
			if fr, ok := choice.FinishReason.(string); ok {
				finishReason = fr
			} else if fr, ok := choice.FinishReason.(float64); ok && int(fr) == 1 {
				finishReason = "stop"
			}
		}

		rc.Choices = append(rc.Choices, ResponsesStreamChoice{
			Index:        i,
			Delta:        delta,
			FinishReason: finishReason,
		})
	}

	return rc
}

func convertChatToResponses(chatResp *translator.NormalizedChatResponse, respID string) *ResponsesAPIResponse {
	output := []ResponseOutput{}

	for i, choice := range chatResp.Choices {
		var content []ResponseContent
		var role string
		var finishReason string

		if choice.Message != nil {
			role = choice.Message.Role
			if choice.Message.Content != "" {
				content = []ResponseContent{{Type: "output_text", Text: choice.Message.Content}}
			}
			if choice.Message.ToolCalls != nil {
				if tc, ok := choice.Message.ToolCalls.([]interface{}); ok {
					for _, tcItem := range tc {
						if tcMap, ok := tcItem.(map[string]interface{}); ok {
							funcCall := &FunctionCall{
								ID:       getString(tcMap["id"]),
								Type:     "function",
								Name:     getString(tcMap["function"]),
								Arguments: getString(tcMap["arguments"]),
							}
							content = append(content, ResponseContent{Type: "function_call", Text: getString(tcMap["function"]) + "(" + getString(tcMap["arguments"]) + ")"})
							output = append(output, ResponseOutput{
								Type:         "function_call",
								ID:           getString(tcMap["id"]),
								FunctionCall: funcCall,
								Index:        i,
								FinishReason: "tool_calls",
							})
						}
					}
				}
			}
		}

		if choice.FinishReason != nil {
			finishReason = *choice.FinishReason
		}

		if len(content) > 0 || role != "" {
			msgID := generateID("msg")
			if len(chatResp.Choices) > 1 {
				msgID = generateID("msg")
			}

			status := "completed"
			if finishReason == "tool_calls" || finishReason == "function_call" {
				status = "incomplete"
			}

			output = append(output, ResponseOutput{
				Type:    "message",
				ID:      msgID,
				Status:  status,
				Message: &ResponseMessage{
					Role:    role,
					Content: content,
				},
				Index:        i,
				FinishReason: finishReason,
			})
		}
	}

	usage := &ResponsesUsage{}
	if chatResp.Usage != nil {
		usage.InputTokens = chatResp.Usage.PromptTokens
		usage.OutputTokens = chatResp.Usage.CompletionTokens
		usage.TotalTokens = chatResp.Usage.TotalTokens
	}

	return &ResponsesAPIResponse{
		ID:          respID,
		Object:      "response",
		Created:     chatResp.Created,
		Model:       chatResp.Model,
		Output:      output,
		ServiceTier: "default",
		Usage:       usage,
	}
}

func (h *ResponsesHandler) executeChatRequest(ctx context.Context, w http.ResponseWriter, req *ResponsesRequest, messages []translator.NormalizedMessage, requestID string, stream bool) (provider, model string, statusCode int, usage *translator.NormalizedUsage) {
	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		writeResponsesError(w, "failed to load models", "upstream_error", "provider_error", http.StatusServiceUnavailable)
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
		writeResponsesError(w, "model not found: "+req.Model, "invalid_request_error", "model_not_found", http.StatusBadRequest)
		return "", req.Model, http.StatusBadRequest, nil
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		writeResponsesError(w, "provider not found", "upstream_error", "provider_not_found", http.StatusServiceUnavailable)
		return "", targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	adapter, err := translator.Get(providerConn.Provider)
	if err != nil {
		writeResponsesError(w, "adapter not found for provider: "+providerConn.Provider, "upstream_error", "adapter_error", http.StatusServiceUnavailable)
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		writeResponsesError(w, "failed to decrypt provider secret", "upstream_error", "auth_error", http.StatusServiceUnavailable)
		return providerConn.Provider, targetModel.ModelID, http.StatusServiceUnavailable, nil
	}

	normReq := &translator.NormalizedChatRequest{
		Model:    targetModel.ModelID,
		Messages: messages,
		Stream:   stream,
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
	if req.Seed != nil {
		normReq.Extra = map[string]interface{}{"seed": *req.Seed}
	}
	if req.Tools != nil {
		normReq.Tools = req.Tools
	}
	if req.ToolChoice != nil {
		normReq.Extra = mergeExtra(normReq.Extra, map[string]interface{}{"tool_choice": req.ToolChoice})
	}

	httpReq, err := adapter.TranslateRequest(normReq, providerConn.BaseURL, decryptedKey)
	if err != nil {
		writeResponsesError(w, "failed to translate request", "upstream_error", "translate_error", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway, nil
	}

	httpReq.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		writeResponsesError(w, "upstream request failed", "upstream_error", "request_failed", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		convertUpstreamError(w, resp.StatusCode, bodyBytes)
		return providerConn.Provider, targetModel.ModelID, resp.StatusCode, nil
	}

	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
		return providerConn.Provider, targetModel.ModelID, http.StatusOK, nil
	}

	var chatResp translator.NormalizedChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		writeResponsesError(w, "failed to parse upstream response", "upstream_error", "parse_error", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway, nil
	}

	responsesResp := convertChatToResponses(&chatResp, requestID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsesResp)

	return providerConn.Provider, targetModel.ModelID, http.StatusOK, chatResp.Usage
}

func (h *ResponsesHandler) executeStreamingRequest(ctx context.Context, req *ResponsesRequest, messages []translator.NormalizedMessage, requestID string) (io.ReadCloser, error) {
	models, err := h.DB.Models().ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var targetModel *db.Model
	for _, m := range models {
		if m.ModelID == req.Model || m.ModelName == req.Model || m.ID == req.Model {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		return nil, fmt.Errorf("model not found")
	}

	providerConn, err := h.DB.Providers().FindByID(ctx, targetModel.ProviderID)
	if err != nil || providerConn == nil {
		return nil, err
	}

	adapter, err := translator.Get(providerConn.Provider)
	if err != nil {
		return nil, err
	}

	decryptedKey, err := crypto.Decrypt(string(providerConn.EncryptedSecret))
	if err != nil {
		return nil, err
	}

	normReq := &translator.NormalizedChatRequest{
		Model:    targetModel.ModelID,
		Messages: messages,
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
	if req.Tools != nil {
		normReq.Tools = req.Tools
	}

	httpReq, err := adapter.TranslateRequest(normReq, providerConn.BaseURL, decryptedKey)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func convertUpstreamError(w http.ResponseWriter, statusCode int, body []byte) {
	var upstreamErr struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code,omitempty"`
		} `json:"error"`
	}

	if json.Unmarshal(body, &upstreamErr) == nil && upstreamErr.Error.Message != "" {
		errType := errorTypeForStatus(statusCode)
		if upstreamErr.Error.Type != "" {
			errType = upstreamErr.Error.Type
		}
		code := upstreamErr.Error.Code
		if code == "" {
			code = strconv.Itoa(statusCode)
		}
		writeResponsesError(w, upstreamErr.Error.Message, errType, code, statusCode)
		return
	}

	writeResponsesError(w, string(body), errorTypeForStatus(statusCode), strconv.Itoa(statusCode), statusCode)
}

func writeResponsesError(w http.ResponseWriter, message, errType, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ResponsesErrorResponse{
		Error: ResponsesError{
			Type:    errType,
			Code:    code,
			Message: message,
		},
	})
}

func (h *ResponsesHandler) recordUsage(ctx context.Context, req *ResponsesRequest, messages []translator.NormalizedMessage, startTime time.Time, isStream bool, statusCode int, model, provider string) {
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
		RequestedModel: req.Model,
		FinalModel:     model,
		StatusCode:     statusCode,
		IsStream:       isStream,
		LatencyMs:      int(time.Since(startTime).Milliseconds()),
		CreatedAt:      time.Now(),
	}

	h.DB.UsageEvents().Create(ctx, event)
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func mergeExtra(existing, new map[string]interface{}) map[string]interface{} {
	if existing == nil {
		return new
	}
	for k, v := range new {
		existing[k] = v
	}
	return existing
}

type bodyReader struct {
	data   []byte
	pos    int
}

func (b *bodyReader) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bodyReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int
	switch whence {
	case 0:
		newPos = int(offset)
	case 1:
		newPos = b.pos + int(offset)
	case 2:
		newPos = len(b.data) + int(offset)
	default:
		return 0, nil
	}
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(b.data) {
		newPos = len(b.data)
	}
	b.pos = newPos
	return int64(newPos), nil
}

func convertUsageToResponses(body io.Reader, respID, model string, usage *translator.NormalizedUsage) *ResponsesAPIResponse {
	return nil
}