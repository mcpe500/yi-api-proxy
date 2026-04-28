# Yi API Proxy v2.0

Modular AI API Proxy inspired by free-claude-code architecture. Supports multiple providers with per-model routing, automatic fallback, and advanced features.

## Features

- **Multi-Provider Routing**: Route requests to GLM, MiniMax, Z.ai, NVIDIA NIM, OpenRouter, DeepSeek, LM Studio, llama.cpp, or Ollama
- **Per-Model Routing**: Automatically routes Opus/Sonnet/Haiku tier models to optimal providers
- **Automatic Fallback**: Seamlessly switches between providers on errors
- **Vision Support**: Automatic vision fallback - converts images to text for non-vision providers
- **Streaming**: Full SSE streaming support for all providers
- **Tool Use**: Support for tool/function calling
- **Thinking Blocks**: Handle reasoning/thinking blocks from providers
- **Security**: API key validation and rate limiting
- **Interactive Setup**: Wizard CLI for easy configuration
- **File Logging**: JSON-structured logs for requests and providers
- **Web Dashboard**: Lightweight HTML/CSS/JS dashboard for monitoring and configuration

## Quick Start

```bash
# Install dependencies
npm install

# Interactive setup (creates config.json)
npm run setup

# Or manually copy and edit config
cp config.example.json config.json
# Then edit config.json with your API keys

# Start server
npm run dev
```

## Setup Commands

| Command | Description |
|---------|-------------|
| `npm run setup` | Interactive setup wizard |
| `npm run test:providers` | Test all provider API connections |
| `node setup.js --validate` | Test providers (non-interactive) |
| `node setup.js --advanced` | Edit config as raw JSON |
| `node setup.js --export` | Export config to stdout |
| `node setup.js --import <file>` | Import config from file |

## Configuration

Edit `config.json` or use `npm run setup` for interactive configuration:

```json
{
  "server": {
    "port": 3000,
    "host": "0.0.0.0",
    "corsOrigins": "*"
  },
  "security": {
    "apiKeys": ["your-proxy-api-key"],
    "rateLimit": {
      "windowMs": 60000,
      "maxRequests": 200
    }
  },
  "enableModelRouting": true,
  "modelRouting": {
    "opus": {
      "models": ["claude-3-opus", "gpt-4"],
      "primaryProvider": "glm",
      "fallbackProviders": ["nvidia_nim", "openrouter"]
    },
    "sonnet": {
      "models": ["claude-3-sonnet", "gpt-4o"],
      "primaryProvider": "glm",
      "fallbackProviders": ["nvidia_nim", "minimax"]
    },
    "haiku": {
      "models": ["claude-3-haiku", "gpt-4o-mini"],
      "primaryProvider": "minimax",
      "fallbackProviders": ["glm", "deepseek"]
    }
  },
  "providers": {
    "glm": {
      "name": "GLM (Z.ai Coding Plan)",
      "baseUrl": "https://api.z.ai/api/coding/paas/v4",
      "endpoint": "/chat/completions",
      "type": "openai",
      "apiKey": "your-glm-api-key",
      "models": ["glm-5.1", "glm-5-turbo", "glm-4.7"],
      "visionModels": ["glm-5.1"],
      "capabilities": ["vision", "streaming", "tools"]
    },
    "nvidia_nim": {
      "name": "NVIDIA NIM",
      "baseUrl": "https://integrate.api.nvidia.com/v1",
      "endpoint": "/chat/completions",
      "type": "openai",
      "apiKey": "your-nvidia-nim-api-key",
      "models": ["nvidia_nim/z-ai/glm4.7"],
      "capabilities": ["vision", "streaming", "tools"]
    },
    "openrouter": {
      "name": "OpenRouter",
      "baseUrl": "https://openrouter.ai/api/v1",
      "endpoint": "/chat/completions",
      "type": "openai",
      "apiKey": "your-openrouter-api-key",
      "models": ["anthropic/claude-3.5-sonnet"],
      "capabilities": ["vision", "streaming", "tools"]
    },
    "deepseek": {
      "name": "DeepSeek",
      "baseUrl": "https://api.deepseek.com/v1",
      "endpoint": "/chat/completions",
      "type": "openai",
      "apiKey": "your-deepseek-api-key",
      "models": ["deepseek-chat"],
      "capabilities": ["text-only", "streaming", "tools"]
    },
    "ollama": {
      "name": "Ollama (Local)",
      "baseUrl": "http://localhost:11434/v1",
      "endpoint": "/chat/completions",
      "type": "openai",
      "apiKey": "dummy",
      "models": ["llama3", "mistral"],
      "capabilities": ["vision", "streaming", "tools"]
    }
  },
  "visionFallback": {
    "enabled": true,
    "provider": "glm",
    "model": "glm-5.1"
  }
}
```

## API Usage

### Chat Completions (OpenAI-compatible)

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-proxy-api-key" \
  -d '{
    "model": "glm-5.1",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Anthropic Messages API

```bash
curl -X POST http://localhost:3000/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-proxy-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "glm-5.1",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 256
  }'
```

### With Image (Vision)

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-proxy-api-key" \
  -d '{
    "model": "glm-5.1",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this image:"},
        {"type": "image_url", "image_url": {"url": "https://example.com/image.jpg"}}
      ]
    }]
  }'
```

### Auto-Routing

Model is auto-detected from the request body:

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-proxy-api-key" \
  -d '{"model": "claude-haiku", "messages": [...]}'
```

### Manual Provider Selection

```bash
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-proxy-api-key" \
  -H "X-Provider: minimax" \
  -d '{"model": "MiniMax-M2.7", "messages": [...]}'
```

## Vision Handling

When you send an image to a non-vision provider (like MiniMax):

1. Proxy detects the image
2. Uses GLM's vision model (glm-5.1) to describe the image
3. Converts description to text
4. Sends text-only request to MiniMax

This allows you to use vision-capable models without changing your code.

## Supported Providers

### Cloud Providers
- **GLM (Z.ai Coding Plan)** - Vision, streaming, tools
- **MiniMax** - Text-only, streaming, tools
- **Z.ai Direct** - Vision, streaming, tools
- **NVIDIA NIM** - Vision, streaming, tools
- **OpenRouter** - Vision, streaming, tools (aggregates many models)
- **DeepSeek** - Text-only, streaming, tools (cost-effective)

### Local Providers
- **LM Studio** - Vision, streaming, tools (localhost:1234)
- **llama.cpp** - Text-only, streaming (localhost:8080)
- **Ollama** - Vision, streaming, tools (localhost:11434)

## Model Routing

The proxy automatically routes models to optimal providers based on tier:

- **Opus Tier** (claude-3-opus, gpt-4) → GLM → NVIDIA NIM → OpenRouter
- **Sonnet Tier** (claude-3-sonnet, gpt-4o) → GLM → NVIDIA NIM → MiniMax
- **Haiku Tier** (claude-3-haiku, gpt-4o-mini) → MiniMax → GLM → DeepSeek

Override routing with `X-Provider` header.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/` | API info and features |
| POST | `/v1/chat/completions` | Chat completion proxy (OpenAI format) |
| POST | `/v1/messages` | Messages proxy (Anthropic format) |
| GET | `/v1/providers` | List all configured providers |
| GET | `/v1/models` | List available models |
| GET | `/v1/health/:provider` | Check specific provider status |
| GET | `/dashboard` | Web dashboard (HTML) |
| GET | `/api/config` | Get current configuration (API) |
| POST | `/api/config` | Update configuration (API) |
| GET | `/api/logs` | Get recent logs (API) |

## Claude Code Setup

Configure Claude Code to use this proxy:

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your-proxy-api-key",
    "ANTHROPIC_BASE_URL": "http://localhost:3000",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.1",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5-turbo",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-4-flash"
  }
}
```

The proxy will automatically route requests to the best available provider based on the model tier.

## Web Dashboard

Access the lightweight web dashboard at `http://localhost:3000/dashboard`

**Dashboard Features:**
- **Provider Status**: View all configured providers and their status
- **Configuration Editor**: Edit config.json directly through the web UI
- **Logs Viewer**: View recent request and provider logs
- **Server Info**: View API endpoints and features

The dashboard is built with plain HTML/CSS/JavaScript - no build step required, lightweight and fast.

## Architecture

Inspired by free-claude-code with modular Node.js architecture:

```
src/
├── api/              # API routes and service layer
│   └── chatRoutes.js # Chat completion endpoints
├── core/             # Shared protocol helpers
│   ├── anthropicProtocol.js  # Anthropic format utilities
│   └── sseHelper.js         # SSE streaming support
├── providers/        # Provider management
│   └── providerRegistry.js  # Provider registry and routing
├── config/           # Configuration
│   ├── providerCatalog.js   # Provider metadata
│   └── modelRouter.js       # Per-model routing
├── middleware/       # Express middleware
│   ├── auth.js              # API key validation
│   ├── mediaHandler.js      # Vision/image handling
│   └── rateLimit.js         # Rate limiting
└── lib/              # Utilities
    └── logger.js            # Logging system
```

## Logging

Logs are written to `logs/` directory:
- `logs/yi-api.YYYYMMDD.log` - Combined proxy logs
- `logs/requests.YYYYMMDD.log` - Request details
- `logs/providers/{provider}.YYYYMMDD.log` - Per-provider logs

## License

MIT

## Inspired By

This project is inspired by [free-claude-code](https://github.com/Alishahryar1/free-claude-code), adapted to Node.js with additional provider support and modular architecture.