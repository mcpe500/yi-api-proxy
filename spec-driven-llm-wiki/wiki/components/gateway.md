---
title: "Gateway Component"
type: component
tags: [gateway, api, openai, streaming, sse]
---

# Component: Gateway

## Overview

Gateway adalah OpenAI-compatible API endpoint utama yang melayani request dari user/tools.

## Endpoints

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/v1/chat/completions` | POST | Chat completions (streaming + non-streaming) | IMPLEMENTED |
| `/v1/models` | GET | Model list | IMPLEMENTED |
| `/v1/embeddings` | POST | Embeddings | IMPLEMENTED |
| `/v1/responses` | POST | OpenAI Responses API | IMPLEMENTED (Basic) |
| `/v1/messages` | POST | Claude Messages API | IMPLEMENTED (Basic) |
| `/health` | GET | Health check | IMPLEMENTED |
| `/ready` | GET | Readiness check | IMPLEMENTED |

## Request Pipeline

```
1. API key auth middleware (RequireAPIKey)
2. Rate limiter middleware
3. Resolve model (combo first, then direct model lookup)
4. Get adapter from registry
5. Translate request via adapter
6. Execute upstream HTTP request
7. Record usage event
8. Stream/JSON response
```

## Streaming SSE

```go
// SSE Headers
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
Access-Control-Allow-Origin: *

// Chunk format (OpenAI compatible)
data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"..."}}]}

data: [DONE]
```

## Related Specs

- [[spec:010-gateway]] - Gateway implementation
- [[spec:004-apikeys]] - API key auth
- [[spec:007-translator]] - Request/response translation
- [[spec:011-routing]] - Smart routing + fallback