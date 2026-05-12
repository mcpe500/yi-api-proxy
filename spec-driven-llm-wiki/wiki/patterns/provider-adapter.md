---
title: "Pattern: Provider Adapter"
type: pattern
tags: [pattern, adapter, provider, translation]
---

# Pattern: Provider Adapter

## Problem

Need to support multiple AI providers with different API formats.

## Solution

Define a common `ProviderAdapter` interface. Each provider implements the interface to handle format translation.

```go
type ProviderAdapter interface {
    TranslateRequest(ctx, req, cred) (*http.Request, error)
    ParseResponse(ctx, resp, req) (*NormalizedChatResponse, error)
    StreamResponse(ctx, resp, req) (<-chan StreamEvent, error)
}
```

## Flow

```
OpenAI Request → NormalizedChatRequest → [Adapter.TranslateRequest] → Provider Request
Provider Response → [Adapter.ParseResponse] → NormalizedChatResponse → OpenAI Response
```

## Registration

```go
registry.Register(&OpenAIAdapter{})
registry.Register(&GLMAdapter{})
registry.Register(&MiniMaxAdapter{})

adapter := registry.Get("glm")  // Returns GLMAdapter
```

## Benefits

- Add new provider by implementing one interface
- Internal normalized format (OpenAI-compatible) as lingua franca
- Test each adapter independently

## Used In

- [[spec:007-translator]] - Interface definition
- [[spec:008]] to [[spec:009]] and [[spec:015]] - Adapter implementations