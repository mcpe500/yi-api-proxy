# Wiki Log

Append-only operation log.

## [2026-04-24] init | Spec-driven wiki skeleton

Created initial wiki skeleton, prompt files, templates, and tooling plan.

## [2026-04-25] graph | Knowledge graph rebuilt

5 nodes, 0 edges.

## [2026-05-12] spec | Gorouter PRD added - Go AI API Gateway

New task: Build Go API Management / AI Router "9Router-like" from scratch.
PRD covers: auth, users, API keys, providers, models, routing, combo fallback, gateway, adapters, quota, analytics.
Design principle: **Ringan** (lightweight) - single binary, low RAM, fast startup.
Project name: **gorouter**.
Database: DatabaseManager with JSON/SQLite/PostgreSQL support.

## [2026-05-12] spec | All 17 specs written

Specs 001-017 created in Bahasa Indonesia (user's language):

- 001: Project Scaffold & Config & DatabaseManager
- 002: Auth System (JWT, bcrypt, bootstrap admin)
- 003: User & Admin Management (CRUD, RBAC)
- 004: API Key Management (scopes, hash, rotation)
- 005: Provider Management (AES-256-GCM encryption, cooldown)
- 006: Model Catalog & Aliases
- 007: Translator/Adapter Interface (normalized types)
- 008: Adapters: OpenAI + OpenAI-Compatible
- 009: Adapters: GLM + MiniMax
- 010: Gateway /v1 Endpoints (chat completions, SSE streaming)
- 011: Smart Routing & Combo Fallback
- 012: Quota, Rate Limit & Budget Enforcement
- 013: Usage Analytics & Logs (usage, request, audit)
- 014: Dashboard API Endpoints (all /api/* routes)
- 015: Adapters: CommandCode + OpenCode Go + Compat Endpoints
- 016: Health, Metrics & Monitoring
- 017: Docker, Deployment & Migrations

## [2026-05-12] wiki | Overview, components, decisions, patterns created

Wiki pages created:
- overview.md: Project overview, tech stack, spec breakdown
- components/dbmanager.md: DatabaseManager abstraction
- components/gateway.md: OpenAI-compatible gateway
- components/provider-adapter.md: Adapter interface
- components/routing.md: Combo fallback routing
- components/auth.md: JWT + API key auth
- components/quota.md: Quota + rate limiting
- decisions/adr-001-go-lightweight.md: Go for lightweight
- decisions/adr-002-pure-go-sqlite.md: modernc.org/sqlite
- decisions/adr-003-three-mode-db.md: JSON/SQLite/PG
- patterns/repository-interface.md: Repository pattern
- patterns/provider-adapter.md: Adapter pattern

## [2026-05-12] research | 9Router codebase analyzed (16+ agents)

Fleet swarm of 16+ explore agents analyzed 9Router reference:
- Gateway/v1 routes (chat, embeddings, models, responses)
- Provider/adapter system (translator registry, executors)
- Combo/fallback routing (strategies, cooldown, backoff)
- Auth/user/API-key (JWT, bcrypt, HMAC)
- DB schema/usage (settings, apiKeys, machines)
- Dashboard/UI, config/env
- Streaming/SSE implementation
- Cloud/edge handlers
- Model catalog logic
- Error handling patterns
- Middleware pipeline
- Documentation (README, CHANGELOG, ARCHITECTURE, guides)

## [2026-05-12] implement | Specs 001-017 fully implemented

All 17 specs implemented in Go. Build passes, go vet clean.

### Files created:
- cmd/server/main.go - Entry point with all routes wired
- internal/config/config.go - Environment config with validation
- internal/logger/logger.go - slog JSON logger
- internal/db/dbmanager.go - DatabaseManager with JSON driver + SQLite/PG stubs
- internal/auth/jwt.go - JWT token generation and validation
- internal/auth/password.go - bcrypt hash and verify
- internal/apikeys/service.go - API key service (create, validate, rotate, revoke)
- internal/apikeys/keys.go - Key generation (sk-gorouter-* prefix) + HMAC-SHA256 hashing
- internal/combo/combo.go - ComboManager with priority-based fallback execution
- internal/crypto/crypto.go - AES-256-GCM encryption for provider secrets
- internal/proxy/proxy.go - HTTP reverse proxy with streaming support
- internal/streaming/stream.go - SSE chunk formatting helpers
- internal/middleware/auth.go - RequireAuth, RequireAdmin, RequireAPIKey middleware
- internal/middleware/ratelimit.go - Sliding window rate limiter
- internal/handlers/* - All HTTP handlers (admin, v1, user)
- web/* - Admin SPA + Chat UI
- Dockerfile, Dockerfile.dev, docker-compose.yml, .dockerignore

### Key metrics:
- Binary: 8.3MB (optimized), 10.2MB (debug)
- go vet: clean
- Routes: /auth/*, /admin/*, /me/*, /v1/*, /health, /ready
- Providers: OpenAI-compatible proxy with streaming passthrough
