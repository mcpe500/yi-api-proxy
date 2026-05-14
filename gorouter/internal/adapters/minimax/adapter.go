package minimax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorouter/gorouter/internal/translator"
)

type MiniMaxAdapter struct{}

func New() *MiniMaxAdapter { return &MiniMaxAdapter{} }

func (a *MiniMaxAdapter) Name() string     { return "MiniMax" }
func (a *MiniMaxAdapter) Provider() string { return "minimax" }
func (a *MiniMaxAdapter) Capabilities() translator.Capabilities {
	return translator.Capabilities{
		Chat: true, Streaming: true, Tools: true,
		Vision: true, JSONMode: true, Embeddings: true,
	}
}
func (a *MiniMaxAdapter) ModelID(model string) string { return model }

func (a *MiniMaxAdapter) TranslateRequest(req *translator.NormalizedChatRequest, baseURL, apiKey string) (*http.Request, error) {
	body := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		body["stop"] = req.Stop
	}
	if req.Tools != nil {
		body["tools"] = req.Tools
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/text/chatcompletion_v2"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	return httpReq, nil
}

func (a *MiniMaxAdapter) ParseResponse(resp *http.Response) (*translator.NormalizedChatResponse, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(body))
	}

	var result translator.NormalizedChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *MiniMaxAdapter) ParseStreamChunk(data []byte) (*translator.StreamEvent, error) {
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "data: ") {
		s = strings.TrimPrefix(s, "data: ")
	}
	s = strings.TrimSpace(s)
	if s == "[DONE]" {
		return &translator.StreamEvent{Done: true}, nil
	}

	var event translator.StreamEvent
	if err := json.Unmarshal([]byte(s), &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (a *MiniMaxAdapter) TranslateEmbeddingRequest(model string, input interface{}, baseURL, apiKey string) (*http.Request, error) {
	body := map[string]interface{}{"model": model, "input": input}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/embeddings"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	return httpReq, nil
}

func (a *MiniMaxAdapter) ParseEmbeddingResponse(resp *http.Response) (interface{}, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}
	return result, nil
}

func init() {
	translator.Register(New())
}
