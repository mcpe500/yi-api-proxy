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
| `/v1/responses` | POST | OpenAI Responses API (schema conversion + streaming) | IMPLEMENTED |
| `/v1/messages` | POST | Claude Messages API (schema conversion + Anthropic SSE) | IMPLEMENTED |
| `/v1/images/generations` | POST | OpenAI-compatible image generation proxy | IMPLEMENTED |
| `/v1/audio/speech` | POST | OpenAI-compatible text-to-speech proxy | IMPLEMENTED |
| `/v1/audio/transcriptions` | POST | OpenAI-compatible transcription proxy | IMPLEMENTED |
| `/v1/search` | POST | Web search proxy | IMPLEMENTED |
| `/v1/web/search` | POST | Web search proxy alias | IMPLEMENTED |
| `/health` | GET | Health check | IMPLEMENTED |
| `/ready` | GET | Readiness check | IMPLEMENTED |
| `/metrics` | GET | Prometheus metrics endpoint | IMPLEMENTED |

## Request Pipeline

```
1. API key auth middleware (RequireAPIKey)
2. Rate limiter middleware
3. Inject Caveman prompt (if X-Caveman header present)
4. Resolve model (combo first, then direct model lookup, then implicit fallback)
5. Get adapter from registry
6. Translate request via adapter
7. Execute upstream HTTP request
8. Apply RTK compression (if X-RTK header present)
9. Record usage event
10. Stream/JSON response
```

## Model Resolution Logic

Gateway mendukung dua mekanisme resolusi model:
- **Explicit Combos**: Mencari model di dalam daftar Combo yang didefinisikan user.
- **Implicit Fallback**: Jika `model_id` bukan combo, sistem mencari semua provider aktif yang mendukung `model_id` tersebut dan melakukan fallback otomatis di antara mereka.

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