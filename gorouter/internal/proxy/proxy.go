package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Proxy struct {
	BaseURL     string
	Timeout     time.Duration
	APIKey      string
	httpClient  *http.Client
}

type ProxyConfig struct {
	BaseURL string
	Timeout time.Duration
	APIKey  string
}

func NewProxy(cfg ProxyConfig) *Proxy {
	return &Proxy{
		BaseURL: cfg.BaseURL,
		Timeout: cfg.Timeout,
		APIKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

type ChatRequest struct {
	Model       string            `json:"model"`
	Messages    []map[string]string `json:"messages"`
	Stream      bool               `json:"stream"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	FrequencyPenalty *float64    `json:"frequency_penalty,omitempty"`
	PresencePenalty *float64     `json:"presence_penalty,omitempty"`
	Stop        interface{}       `json:"stop,omitempty"`
}

type UpstreamRequest struct {
	Model       string               `json:"model"`
	Messages    []map[string]string  `json:"messages"`
	Stream      bool                  `json:"stream"`
	Temperature *float64             `json:"temperature,omitempty"`
	MaxTokens   *int                 `json:"max_tokens,omitempty"`
	TopP        *float64             `json:"top_p,omitempty"`
}

func (p *Proxy) DoRequest(ctx context.Context, req interface{}, requestID string) (io.ReadCloser, int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq, requestID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("upstream request failed: %w", err)
	}

	return resp.Body, resp.StatusCode, nil
}

func (p *Proxy) StreamRequest(ctx context.Context, req interface{}, requestID string) (io.ReadCloser, error) {
	upstreamReq := convertToUpstreamRequest(req)
	
	body, err := json.Marshal(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq, requestID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}

	return resp.Body, nil
}

func (p *Proxy) setHeaders(req *http.Request, requestID string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
}

func convertToUpstreamRequest(req interface{}) *UpstreamRequest {
	switch r := req.(type) {
	case map[string]interface{}:
		model := ""
		messages := []map[string]string{}
		stream := false
		var temp *float64
		var maxTokens *int

		if m, ok := r["model"].(string); ok {
			model = m
		}
		if msgList, ok := r["messages"].([]interface{}); ok {
			for _, m := range msgList {
				if msg, ok := m.(map[string]interface{}); ok {
					msgMap := map[string]string{}
					for k, v := range msg {
						if s, ok := v.(string); ok {
							msgMap[k] = s
						}
					}
					messages = append(messages, msgMap)
				}
			}
		}
		if s, ok := r["stream"].(bool); ok {
			stream = s
		}
		if t, ok := r["temperature"].(float64); ok {
			temp = &t
		}
		if mt, ok := r["max_tokens"].(float64); ok {
			m := int(mt)
			maxTokens = &m
		}
		
		return &UpstreamRequest{
			Model:       model,
			Messages:    messages,
			Stream:      stream,
			Temperature: temp,
			MaxTokens:   maxTokens,
		}
	default:
		return &UpstreamRequest{Stream: true}
	}
}

func StreamPassthrough(w http.ResponseWriter, upstream io.Reader) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("flusher not available")
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := upstream.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}