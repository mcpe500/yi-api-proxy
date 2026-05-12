---
title: "Gateway Component"
type: component
tags: [gateway, api, openai, streaming, sse]
---

# Component: Gateway

## Overview

Gateway adalah OpenAI-compatible API endpoint utama yang melayani request dari user/tools.

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (streaming + non-streaming) |
| `/v1/models` | GET | Model list (filtered by permission) |
| `/v1/responses` | POST | OpenAI Responses API |
| `/v1/messages` | POST | Claude Messages API |
| `/v1/embeddings` | POST | Embeddings |
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check |

## Request Pipeline

```
1. CORS middleware
2. API key auth middleware
3. Load user + permissions
4. Validate request
5. Resolve model (direct, alias, combo)
6. Check model permission
7. Get provider adapter
8. Get provider account (active, not cooldown)
9. Translate request via adapter
10. Execute upstream request
11. Fallback if error eligible (spec 011)
12. Normalize response via adapter
13. Stream/non-stream response
14. Record usage event
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