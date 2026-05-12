package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorouter/gorouter/internal/proxy"
	"github.com/gorouter/gorouter/internal/streaming"
)

type ChatRequest struct {
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Stream     bool               `json:"stream"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	FrequencyPenalty *float64     `json:"frequency_penalty,omitempty"`
	PresencePenalty *float64      `json:"presence_penalty,omitempty"`
	Stop        interface{}       `json:"stop,omitempty"`
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
	Proxy  *proxy.Proxy
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

	startTime := time.Now()

	if req.Stream {
		h.handleStreaming(ctx, w, &req, requestID, startTime)
	} else {
		h.handleNonStreaming(ctx, w, &req, requestID, startTime)
	}
}

func (h *ChatHandler) handleStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time) {
	streaming.WriteSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	body, err := h.Proxy.StreamRequest(ctx, req, requestID)
	if err != nil {
		h.Logger.Error("Streaming request failed", "error", err, "request_id", requestID)
		fmt.Fprintf(w, "data: {\"error\":{\"message\":\"upstream error\",\"type\":\"upstream_error\",\"code\":\"upstream_unavailable\"}}\n\n")
		fmt.Fprint(w, streaming.FormatDone())
		return
	}
	defer body.Close()

	if err := streaming.CopyAndStream(w, body); err != nil {
		h.Logger.Error("Stream copy failed", "error", err, "request_id", requestID)
		return
	}

	h.logUsage(requestID, req, startTime, true)
}

func (h *ChatHandler) handleNonStreaming(ctx context.Context, w http.ResponseWriter, req *ChatRequest, requestID string, startTime time.Time) {
	body, statusCode, err := h.Proxy.DoRequest(ctx, req, requestID)
	if err != nil {
		h.Logger.Error("Non-streaming request failed", "error", err, "request_id", requestID)
		writeError(w, "Upstream request failed", errorTypeForStatus(http.StatusBadGateway), "upstream_unavailable", http.StatusBadGateway)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/json")

	if statusCode >= 400 {
		w.WriteHeader(statusCode)
		io.Copy(w, body)
		return
	}

	w.WriteHeader(http.StatusOK)
	io.Copy(w, body)

	h.logUsage(requestID, req, startTime, false)
}

func (h *ChatHandler) logUsage(requestID string, req *ChatRequest, startTime time.Time, isStream bool) {
	_ = requestID
	_ = req
	_ = startTime
	_ = isStream
}
