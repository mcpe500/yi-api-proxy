# Wiki Index

## Overview
- [Overview](overview.md) - gorouter: Go AI API Gateway (ringan, 9Router-like)

## Specs

17 specs covering full gorouter implementation:
- [001-010](../spec/docs/) - MVP v0.1 Core (scaffold, auth, users, keys, providers, models, adapters, gateway)
- [011-013](../spec/docs/) - MVP v0.2 Routing (combo fallback, quota, usage)
- [014](../spec/docs/) - v1.0 Dashboard API
- [015](../spec/docs/) - MVP v0.3 Compatibility (CommandCode, OpenCode Go, /v1/responses, /v1/messages)
- [016-017](../spec/docs/) - v1.0 Production (metrics, Docker)

## Components

- [DatabaseManager](components/dbmanager.md) - 3-mode DB abstraction (JSON/SQLite/PG)
- [Gateway](components/gateway.md) - OpenAI-compatible /v1/* endpoints
- [ProviderAdapter](components/provider-adapter.md) - Adapter interface for providers
- [Routing](components/routing.md) - Smart routing + combo fallback
- [Auth](components/auth.md) - JWT dashboard auth + API key gateway auth
- [Quota](components/quota.md) - Quota + rate limiting enforcement

## Decisions

- [ADR-001: Go for Lightweight](decisions/adr-001-go-lightweight.md)
- [ADR-002: Pure Go SQLite](decisions/adr-002-pure-go-sqlite.md)
- [ADR-003: Three-Mode DB](decisions/adr-003-three-mode-db.md)

## Patterns

- [Repository Interface](patterns/repository-interface.md) - DB abstraction pattern
- [Provider Adapter](patterns/provider-adapter.md) - Provider translation pattern

## Syntheses

Saved query answers live in `syntheses/`.

## Handoffs
- [Handoff 001: Full Implementation](../spec/handoff/001.full-implementation.md) - All 17 specs implemented
