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
    // Metadata
    Name() string
    Provider() string
    Capabilities() Capabilities
    
    // Request handling
    TranslateRequest(ctx context.Context, req NormalizedChatRequest, cred *Credential) (*http.Request, error)
    ParseResponse(ctx context.Context, resp *http.Response, req NormalizedChatRequest) (*NormalizedChatResponse, error)
    StreamResponse(ctx context.Context, resp *http.Response, req NormalizedChatRequest) (<-chan StreamEvent, error)
    
    // Credential handling
    RefreshCredentials(ctx context.Context, cred *Credential) (*Credential, error)
    TestConnection(ctx context.Context, cred *Credential) error
    
    // Model info
    ListModels(ctx context.Context, cred *Credential) ([]Model, error)
}

type Capabilities struct {
    SupportsChat        bool
    SupportsStreaming   bool
    SupportsTools       bool
    SupportsVision      bool
    SupportsJSONMode    bool
    SupportsEmbeddings  bool
}
```

## Normalized Types

```go
type NormalizedChatRequest struct {
    RequestID       string
    UserID          string
    Model           string
    Messages        []Message
    Tools           []Tool
    Stream          bool
    Temperature      *float64
    MaxTokens        *int
    // ...
}

type NormalizedChatResponse struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   Usage
    Created int64
}
```

## Adapters

| Adapter | Provider | Notes |
|---------|----------|-------|
| openai | api.openai.com | Native, passthrough |
| openai_compatible | Custom base URL | Passthrough with custom endpoint |
| glm | OpenAI-compatible | Zhipu JWT auth |
| minimax | OpenAI-compatible | Different endpoint path |
| commandcode | CommandCode | Custom request/response mapping |
| opencodego | OpenCode Go | Local/remote, no-auth/API key |

## Registry

```go
type AdapterRegistry struct {
    adapters map[string]ProviderAdapter
}

func (r *AdapterRegistry) Register(adapter ProviderAdapter)
func (r *AdapterRegistry) Get(provider string) ProviderAdapter
func (r *AdapterRegistry) List() []ProviderAdapter
```

## Related Specs

- [[spec:007-translator]] - Interface definition
- [[spec:008-adapters-openai]] - OpenAI adapter
- [[spec:009-adapters-glm-minimax]] - GLM + MiniMax adapters
- [[spec:015-compat]] - CommandCode + OpenCode Go adapters