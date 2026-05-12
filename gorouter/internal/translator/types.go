package translator

import "net/http"

type NormalizedChatRequest struct {
	Model       string                 `json:"model"`
	Messages    []NormalizedMessage    `json:"messages"`
	Stream      bool                   `json:"stream"`
	Temperature *float64               `json:"temperature,omitempty"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	Stop        interface{}            `json:"stop,omitempty"`
	Tools       interface{}            `json:"tools,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

type NormalizedMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type NormalizedChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []NormalizedChoice `json:"choices"`
	Usage   *NormalizedUsage   `json:"usage,omitempty"`
}

type NormalizedChoice struct {
	Index        int                `json:"index"`
	Message      *NormalizedMessage `json:"message,omitempty"`
	Delta        *NormalizedMessage `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason"`
}

type NormalizedUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamEvent struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []NormalizedChoice `json:"choices"`
	Usage   *NormalizedUsage   `json:"usage,omitempty"`
	Done    bool               `json:"-"`
}

type Capabilities struct {
	Chat       bool
	Streaming  bool
	Tools      bool
	Vision     bool
	JSONMode   bool
	Embeddings bool
}

type ProviderAdapter interface {
	Name() string
	Provider() string
	Capabilities() Capabilities
	TranslateRequest(req *NormalizedChatRequest, baseURL, apiKey string) (*http.Request, error)
	ParseResponse(resp *http.Response) (*NormalizedChatResponse, error)
	ParseStreamChunk(data []byte) (*StreamEvent, error)
	TranslateEmbeddingRequest(model string, input interface{}, baseURL, apiKey string) (*http.Request, error)
	ParseEmbeddingResponse(resp *http.Response) (interface{}, error)
	ModelID(model string) string
}
