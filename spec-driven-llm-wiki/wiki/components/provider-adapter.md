---
title: "ProviderAdapter Interface"
type: component
tags: [adapters, providers, interface, translation]
---

# Component: ProviderAdapter Interface

## Overview

ProviderAdapter adalah interface untuk semua provider adapters, memungkinkan request/response translation antara format internal (OpenAI-compatible) dan format provider spesifik.

## Interface

```go
type ProviderAdapter interface {
    Name() string
    Provider() string
    Capabilities() Capabilities

    // Chat translation
    TranslateRequest(req *NormalizedChatRequest, baseURL, apiKey string) (*http.Request, error)
    ParseResponse(resp *http.Response) (*NormalizedChatResponse, error)
    ParseStreamChunk(data []byte) (*StreamEvent, error)

    // Embeddings
    TranslateEmbeddingRequest(model string, input interface{}, baseURL, apiKey string) (*http.Request, error)
    ParseEmbeddingResponse(resp *http.Response) (interface{}, error)

    // Model mapping
    ModelID(model string) string
}
```

## Capabilities

```go
type Capabilities struct {
    Chat       bool
    Streaming  bool
    Tools      bool
    Vision     bool
    JSONMode   bool
    Embeddings bool
}
```

## Normalized Types

```go
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

type StreamEvent struct {
    ID      string             `json:"id"`
    Object  string             `json:"object"`
    Created int64              `json:"created"`
    Model   string             `json:"model"`
    Choices []NormalizedChoice `json:"choices"`
    Usage   *NormalizedUsage   `json:"usage,omitempty"`
    Done    bool               `json:"-"`
}
```

## Registered Adapters

| Adapter | Provider | Notes |
|---------|----------|-------|
| openai | OpenAI | Direct passthrough |
| openai_compatible | OpenAI-compatible | Custom base URL support |
| glm | Zhipu GLM | JWT auth |
| minimax | MiniMax | OpenAI-compatible endpoint |

## Translation Flow

```
1. Parse incoming request → NormalizedChatRequest
2. Lookup adapter by provider name
3. adapter.TranslateRequest() → provider-specific http.Request
4. Execute HTTP request upstream
5. adapter.ParseResponse() → NormalizedChatResponse (non-stream)
   OR adapter.ParseStreamChunk() → StreamEvent (streaming)
6. Return OpenAI-compatible response
```

## Related Specs

- [[spec:007-translator]] - Interface definition
- [[spec:008-adapters-openai]] - OpenAI adapter
- [[spec:009-adapters-glm-minimax]] - GLM + MiniMax adapters
- [[spec:015-compat]] - CommandCode + OpenCode Go adapters