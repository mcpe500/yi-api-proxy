package glm

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorouter/gorouter/internal/translator"
)

type GLMAdapter struct{}

func New() *GLMAdapter { return &GLMAdapter{} }

func (a *GLMAdapter) Name() string     { return "GLM (Zhipu)" }
func (a *GLMAdapter) Provider() string { return "glm" }
func (a *GLMAdapter) Capabilities() translator.Capabilities {
	return translator.Capabilities{
		Chat: true, Streaming: true, Tools: true,
	}
}
func (a *GLMAdapter) ModelID(model string) string { return model }

func generateToken(apiKey string) (string, error) {
	parts := strings.SplitN(apiKey, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid GLM API key format")
	}

	ts := time.Now().UnixMilli()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","sign_type":"SIGN"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"api_key":"%s","exp":%d,"timestamp":%d}`, parts[0], ts+3600000, ts),
	))

	h := hmac.New(sha256.New, []byte(parts[1]))
	h.Write([]byte(header + "." + payload))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return header + "." + payload + "." + signature, nil
}

func (a *GLMAdapter) TranslateRequest(req *translator.NormalizedChatRequest, baseURL, apiKey string) (*http.Request, error) {
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

	token, err := generateToken(apiKey)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", token)
	return httpReq, nil
}

func (a *GLMAdapter) ParseResponse(resp *http.Response) (*translator.NormalizedChatResponse, error) {
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

func (a *GLMAdapter) ParseStreamChunk(data []byte) (*translator.StreamEvent, error) {
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

func (a *GLMAdapter) TranslateEmbeddingRequest(model string, input interface{}, baseURL, apiKey string) (*http.Request, error) {
	body := map[string]interface{}{"model": model, "input": input}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	token, err := generateToken(apiKey)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/embeddings"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", token)
	return httpReq, nil
}

func (a *GLMAdapter) ParseEmbeddingResponse(resp *http.Response) (interface{}, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

func init() {
	translator.Register(New())
}
