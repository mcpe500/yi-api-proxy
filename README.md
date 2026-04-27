# Yi API Proxy

Lightweight API proxy router for AI providers. Supports GLM (Z.ai Coding Plan), MiniMax, and Z.ai Direct with automatic vision handling.

## Features

- **Multi-Provider Routing**: Route requests to GLM, MiniMax, or Z.ai Direct based on model name
- **Vision Fallback**: Automatically converts images to text descriptions when using non-vision providers
- **Security**: API key validation and rate limiting
- **Interactive Setup**: Wizard CLI for easy configuration
- **File Logging**: JSON-structured logs for requests and providers

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
    "env": "development"
  },
  "security": {
    "apiKeys": ["your-proxy-api-key"],
    "rateLimit": {
      "windowMs": 60000,
      "maxRequests": 200
    }
  },
  "providers": {
    "glm": {
      "name": "GLM (Z.ai Coding Plan)",
      "baseUrl": "https://api.z.ai/api/anthropic",
      "endpoint": "/v1/messages",
      "apiKey": "your-glm-api-key",
      "models": ["glm-5.1", "glm-5-turbo", "glm-4.7"],
      "visionModels": ["glm-5.1"]
    },
    "minimax": {
      "name": "MiniMax",
      "baseUrl": "https://api.minimax.io",
      "endpoint": "/anthropic/v1/messages",
      "apiKey": "your-minimax-api-key",
      "models": ["MiniMax-M2.7"]
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

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/` | API info |
| POST | `/v1/chat/completions` | Chat completion proxy (OpenAI) |
| POST | `/v1/messages` | Messages proxy (Anthropic) |
| GET | `/v1/providers` | List providers |
| GET | `/v1/models` | List models |
| GET | `/v1/health/:provider` | Check provider status |

## Claude Code / OpenCode Setup

For Claude Code with Z.ai Coding Plan, configure your AI client's settings:

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your-zai-api-key",
    "ANTHROPIC_BASE_URL": "http://localhost:3000",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.1",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5-turbo",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-4-air"
  }
}
```

Or use direct Z.ai endpoint (no proxy):

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "your-zai-api-key",
    "ANTHROPIC_BASE_URL": "https://api.z.ai/api/anthropic",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.1"
  }
}
```

## Logging

Logs are written to `logs/` directory:
- `logs/yi-api.YYYYMMDD.log` - Combined proxy logs
- `logs/requests.YYYYMMDD.log` - Request details
- `logs/providers/{provider}.YYYYMMDD.log` - Per-provider logs

## License

MIT