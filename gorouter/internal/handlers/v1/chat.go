package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/combo"
	"github.com/gorouter/gorouter/internal/db"
	"github.com/gorouter/gorouter/internal/middleware"
	"github.com/gorouter/gorouter/internal/models"
	"github.com/gorouter/gorouter/internal/streaming"
	"github.com/gorouter/gorouter/internal/translator"
)

type ChatRequest struct {
	Model            string         `json:"model"`
	Messages         []ChatMessage  `json:"messages"`
	Stream           bool           `json:"stream"`
	Temperature      *float64       `json:"temperature,omitempty"`
	MaxTokens        *int           `json:"max_tokens,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty"`
	Stop             interface{}    `json:"stop,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatHandler struct {
	DB     db.DatabaseManager
	Combos *combo.ComboManager
	Logger *slog.Logger
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = generateID("chatcmpl")
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Failed to parse request body", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		writeError(w, "model is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	if req.Messages == nil || len(req.Messages) == 0 {
		writeError(w, "messages is required", "invalid_request_error", "invalid_request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	w.Header().Set("X-Request-ID", requestID)

	claims := middleware.GetUserClaims(ctx)
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	resolver := &models.ModelResolver{DB: h.DB}
	resolvedModel, resolvedProvider, err := resolver.Resolve(ctx, req.Model, userID)
	if err != nil {
		writeError(w, "failed to resolve model alias", "invalid_request_error", "alias_error", http.StatusBadRequest)
		return
	}
	if resolvedModel != "" {
		req.Model = resolvedModel
	}

	startTime := time.Now()
	_ = resolvedProvider

	if req.Stream {
		h.handleStreaming(ctx, w, &req, requestID, startTime)
	} else {
		h.handleNonStreaming(ctx, w, &req, requestID, startTime)
	}
}

func (h *ChatHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time) {
	streaming.WriteSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	ctx = context.WithValue(ctx, "request_id", requestID)

	finalModel := req.Model
	providerName := ""
	statusCode := 200

	comboResult, comboErr := h.tryComboExecution(ctx, req)
	if comboErr == nil && comboResult != nil && comboResult.Success {
		if flusher, ok := w.(http.Flusher); ok {
			respData, _ := json.Marshal(comboResult.Response)
			w.Write(respData)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
		if model, ok := comboResult.Response["model"].(string); ok {
			finalModel = model
		}
		if combo, ok := comboResult.Response["provider"].(string); ok {
			providerName = combo
		}
	} else {
		providerName, finalModel, statusCode = h.executeDirectRequest(ctx, w, req, requestID)
	}

	h.recordUsage(ctx, req, startTime, true, statusCode, finalModel, providerName, nil)
}

func (h *ChatHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time) {
	ctx = context.WithValue(ctx, "request_id", requestID)

	comboResult, err := h.tryComboExecution(ctx, req)
	if err == nil && comboResult != nil && comboResult.Success {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(comboResult.Response)
		h.recordUsage(ctx, req, startTime, false, 200, comboResult.Model, "", extractUsage(comboResult.Response))
		return
	}

	providerName, finalModel, statusCode := h.executeDirectRequest(ctx, w, req, requestID)
	h.recordUsage(ctx, req, startTime, false, statusCode, finalModel, providerName, nil)
}

func (h *ChatHandler) tryComboExecution(ctx context.Context, req *ChatRequest) (*combo.ExecuteResponse, error) {
	if h.Combos == nil || h.DB == nil {
		return nil, nil
	}

	comboList, err := h.Combos.LoadCombos(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range comboList {
		if c.Name == req.Model || c.ID == req.Model {
			execReq := combo.ExecuteRequest{
				Model:    req.Model,
				Messages: convertToMapMessages(req.Messages),
				Stream:   req.Stream,
			}
			return h.Combos.Execute(ctx, c.ID, execReq)
		}
	}

	return nil, nil
}

func (h *ChatHandler) executeDirectRequest(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string) (provider, model string, statusCode int) {
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

	apiKey := string(providerConn.EncryptedSecret)

	normReq := &translator.NormalizedChatRequest{
		Model:    targetModel.ModelID,
		Messages: convertChatMessages(req.Messages),
		Stream:   req.Stream,
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

	httpReq, err := adapter.TranslateRequest(normReq, providerConn.BaseURL, apiKey)
	if err != nil {
		writeError(w, "failed to translate request", "upstream_error", "translate_error", http.StatusBadGateway)
		return providerConn.Provider, targetModel.ModelID, http.StatusBadGateway
	}

	httpReq.Header.Set("X-Request-ID", requestID)

	client := &http.Client{Timeout: 60 * time.Second}
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

func (h *ChatHandler) recordUsage(ctx context.Context, req *ChatRequest, startTime time.Time, isStream bool, statusCode int, model, provider string, usage *Usage) {
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

	requestID := ""
	if rid := ctx.Value("request_id"); rid != nil {
		if s, ok := rid.(string); ok {
			requestID = s
		}
	}

	event := &db.UsageEvent{
		ID:             generateID("usage"),
		RequestID:      requestID,
		UserID:         userID,
		ApiKeyID:       apiKeyID,
		Provider:       provider,
		RequestedModel: req.Model,
		FinalModel:     model,
		StatusCode:     statusCode,
		IsStream:       isStream,
		CreatedAt:      time.Now(),
	}

	if usage != nil {
		event.PromptTokens = usage.PromptTokens
		event.CompletionTokens = usage.CompletionTokens
		event.TotalTokens = usage.TotalTokens
		event.EstimatedCost = calculateCost(usage.PromptTokens, usage.CompletionTokens, 0, 0)
	}

	event.LatencyMs = int(time.Since(startTime).Milliseconds())

	h.DB.UsageEvents().Create(ctx, event)
}

func convertToMapMessages(msgs []ChatMessage) []map[string]string {
	result := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}
	return result
}

func convertChatMessages(msgs []ChatMessage) []translator.NormalizedMessage {
	result := make([]translator.NormalizedMessage, len(msgs))
	for i, m := range msgs {
		result[i] = translator.NormalizedMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}

func extractUsage(data map[string]any) *Usage {
	if data == nil {
		return nil
	}

	if usageData, ok := data["usage"].(map[string]any); ok {
		usage := &Usage{}
		if pt, ok := usageData["prompt_tokens"].(float64); ok {
			usage.PromptTokens = int(pt)
		}
		if ct, ok := usageData["completion_tokens"].(float64); ok {
			usage.CompletionTokens = int(ct)
		}
		if tt, ok := usageData["total_tokens"].(float64); ok {
			usage.TotalTokens = int(tt)
		}
		return usage
	}

	return nil
}

func calculateCost(promptTokens, completionTokens, inputPrice, outputPrice int) float64 {
	return float64(promptTokens)*float64(inputPrice)/1000 + float64(completionTokens)*float64(outputPrice)/1000
}