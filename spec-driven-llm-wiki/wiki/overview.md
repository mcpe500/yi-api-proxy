---
title: "Project Overview"
type: synthesis
tags: [overview, gorouter, go-api-gateway]
last_updated: 2026-05-12
---

# Project Overview: gorouter

**gorouter** adalah AI API Gateway berbasis Go yang ringan dan powerful, dibangun untuk menjadi alternatif self-hosted dari 9Router (Node.js).

## Ringkasan

gorouter menyediakan:
1. **OpenAI-compatible API gateway** untuk tool seperti OpenCode Go, CommandCode, Cline, Continue, Cursor, custom SDK
2. **Provider router** untuk OpenAI, GLM, MiniMax, OpenCode Go, CommandCode, dan provider OpenAI-compatible custom
3. **Multi-user admin dashboard** untuk mengelola user, admin, API key, provider, model, combo, quota
4. **Smart routing + fallback** otomatis saat provider gagal (rate limit, quota, error)
5. **Streaming SSE** untuk chat completions

## Design Principles

### Ringan (Lightweight)
- **Single binary**: ~15-25MB, no Node.js runtime, no npm_modules
- **Low RAM**: 20-50MB typical usage
- **Fast startup**: <100ms
- **Small Docker image**: ~20-30MB (distroless)

### Zero External Dependencies (Default)
- SQLite (modernc.org/sqlite - pure Go, no CGO)
- JSON mode untuk testing/single-user
- In-memory rate limiting default

### Optional Scale
- PostgreSQL untuk multi-user, HA
- Redis untuk distributed rate limiting

## Tech Stack

| Component | Choice | Why |
|-----------|--------|-----|
| Web Framework | chi (go-chi/chi/v5) | Lightweight, idiomatic |
| Database | modernc.org/sqlite | Pure Go, no CGO |
| Auth | golang-jwt/jwt/v5 + bcrypt | Simple, secure |
| Encryption | AES-256-GCM (crypto/aes) | Provider secrets |
| Logger | slog (stdlib) | Structured logging |
| Router | chi v5 | REST routing |

## Project Structure

```
gorouter/
├── cmd/server/main.go           # Entry point + route wiring
├── internal/
│   ├── config/config.go          # Env config
│   ├── logger/logger.go          # slog wrapper
│   ├── db/dbmanager.go           # DatabaseManager (JSON/SQLite/PG)
│   ├── db/sqlite.go              # SQLite driver with schema migration
│   ├── auth/jwt.go               # JWT token gen/validate
│   ├── auth/password.go          # bcrypt hash/verify
│   ├── apikeys/service.go        # API key CRUD service
│   ├── apikeys/keys.go           # Key generation + hashing
│   ├── combo/combo.go            # Combo manager + fallback
│   ├── crypto/crypto.go          # AES-256-GCM encrypt/decrypt
│   ├── translator/
│   │   ├── types.go               # Normalized types + ProviderAdapter
│   │   └── registry.go            # Thread-safe adapter registry
│   ├── adapters/
│   │   ├── init.go                # Auto-registers all adapters
│   │   ├── openai/                # OpenAI adapter
│   │   ├── openai_compatible/     # Generic OpenAI-compatible
│   │   ├── glm/                   # GLM/Zhipu adapter (JWT)
│   │   └── minimax/               # MiniMax adapter
│   ├── proxy/proxy.go             # HTTP reverse proxy
│   ├── streaming/stream.go       # SSE formatting helpers
│   ├── handlers/
│   │   ├── admin_apikeys.go      # Admin API key CRUD
│   │   ├── admin_models.go       # Admin model CRUD
│   │   ├── user_keys.go          # User API key management
│   │   ├── user_combos.go        # User combo CRUD
│   │   ├── admin/
│   │   │   ├── auth.go           # Login/refresh/logout
│   │   │   ├── users.go          # Admin user CRUD
│   │   │   ├── dashboard.go      # Dashboard stats
│   │   │   ├── system.go         # System info
│   │   │   ├── audit.go          # Audit logs
│   │   │   ├── ratelimits.go     # Rate limit admin
│   │   │   └── providers.go      # Provider CRUD + Test/Enable/Disable
│   │   ├── v1/
│   │   │   ├── chat.go           # /v1/chat/completions
│   │   │   ├── models.go         # /v1/models
│   │   │   ├── embeddings.go     # /v1/embeddings
│   │   │   └── helpers.go        # Shared helpers
│   │   └── user/
│   │       └── usage.go          # User usage endpoint
│   ├── middleware/
│   │   ├── auth.go               # JWT + RequireAdmin + RequireAPIKey
│   │   └── ratelimit.go          # Rate limiting middleware
│   └── models/models.go          # Domain model structs (package db)
├── web/
│   ├── index.html                # Admin SPA
│   ├── admin.js                  # SPA logic
│   ├── admin.css                 # Dark theme
│   ├── chat.html                 # Chat test UI
│   └── chat.js                   # Chat streaming logic
├── Dockerfile
├── Dockerfile.dev
├── docker-compose.yml
├── .dockerignore
├── go.mod
└── go.sum
```

## Spec Breakdown (17 specs)

| # | Spec | MVP Phase | Status |
|---|------|-----------|--------|
| 001 | Project Scaffold & DatabaseManager | v0.1 | IMPLEMENTED |
| 002 | Auth System (JWT, bootstrap admin) | v0.1 | IMPLEMENTED |
| 003 | User & Admin Management (CRUD, RBAC) | v0.1 | IMPLEMENTED |
| 004 | API Key Management | v0.1 | IMPLEMENTED |
| 005 | Provider Management (encryption, cooldown) | v0.1 | IMPLEMENTED |
| 006 | Model Catalog & Aliases | v0.1 | IMPLEMENTED |
| 007 | Translator/Adapter Interface | v0.1 | IMPLEMENTED |
| 008 | Adapters: OpenAI + Compatible | v0.1 | IMPLEMENTED |
| 009 | Adapters: GLM + MiniMax | v0.1 | IMPLEMENTED |
| 010 | Gateway /v1 Endpoints (chat, streaming) | v0.1 | IMPLEMENTED |
| 011 | Smart Routing & Combo Fallback | v0.2 | IMPLEMENTED |
| 012 | Quota, Rate Limit & Budget | v0.2 | IMPLEMENTED |
| 013 | Usage Analytics & Logs | v0.2 | IMPLEMENTED |
| 014 | Dashboard API Endpoints | v1.0 | IMPLEMENTED |
| 015 | Adapters: CC + OCGo + Compat Endpoints | v0.3 | IMPLEMENTED |
| 016 | Health, Metrics & Monitoring | v1.0 | IMPLEMENTED |
| 017 | Docker, Deployment & Migrations | v1.0 | IMPLEMENTED |

## MVP Phases

### v0.1 Core
Specs 001-010: Foundation, auth, users, keys, providers, models, adapters, gateway

### v0.2 Routing
Specs 011-013: Combo fallback, quota, usage analytics

### v0.3 Compatibility
Spec 015: CommandCode, OpenCode Go, /v1/responses, /v1/messages

### v1.0 Production
Specs 014, 016, 017: Dashboard API, metrics, Docker deployment

## Key Features

- [x] [[spec:001-scaffold]] Project scaffold + DatabaseManager (JSON/SQLite/PG)
- [x] [[spec:002-auth]] JWT auth + bootstrap admin from env
- [x] [[spec:003-users]] User CRUD + RBAC (admin/user)
- [x] [[spec:004-apikeys]] API key dengan scopes, hash storage
- [x] [[spec:005-providers]] Provider connections + AES-256-GCM encryption
- [x] [[spec:006-models]] Model catalog + aliases + permission filtering
- [x] [[spec:007-translator]] ProviderAdapter interface + normalized types
- [x] [[spec:008-adapters-openai]] OpenAI + OpenAI-compatible adapters
- [x] [[spec:009-adapters-glm-minimax]] GLM + MiniMax adapters
- [x] [[spec:010-gateway]] POST /v1/chat/completions + GET /v1/models + SSE streaming
- [x] [[spec:011-routing]] Smart routing + combo fallback
- [x] [[spec:012-quota]] Quota + rate limit enforcement
- [x] [[spec:013-usage]] Usage events + request logs + audit logs
- [x] [[spec:014-dashboard-api]] All /api/* management endpoints
- [x] [[spec:015-compat]] CommandCode + OpenCode Go + compat endpoints
- [x] [[spec:016-metrics]] Health check + Prometheus metrics
- [x] [[spec:017-deploy]] Docker + deployment + migrations

## Quick Start

```bash
# Run with default SQLite
./gorouter

# With env config
GOROUTER_BOOTSTRAP_ADMIN_EMAIL=admin@test.com \
GOROUTER_BOOTSTRAP_ADMIN_PASSWORD=secret123 \
./gorouter

# Docker
docker-compose up -d
```

## Reference

- [[spec:000]] Spec-Driven LLM Wiki Architecture
- [[9Router Reference|external:temp-references/decolua-9router-8a5edab282632443.txt]] Original 9Router codebase reference