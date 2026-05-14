---
title: "DatabaseManager Component"
type: component
tags: [database, abstraction, sqlite, postgresql]
---

# Component: DatabaseManager

## Overview

DatabaseManager adalah abstraksi database yang mendukung 3 driver: JSON, SQLite, dan PostgreSQL.

## Architecture

```go
type DatabaseManager interface {
    Driver() string
    Connect() error
    Close() error
    Migrate() error
    
    // Repositories
    Users() UserRepository
    ApiKeys() ApiKeyRepository
    Providers() ProviderRepository
    Models() ModelRepository
    Combos() ComboRepository
    UsageEvents() UsageRepository
    AuditLogs() AuditRepository
    Settings() SettingsRepository
    RateLimits() RateLimitRepository
    Quotas() QuotaRepository
}
```

## Drivers

### JSON Driver - IMPLEMENTED (`internal/db/dbmanager.go`)
- Single file persistence: `gorouter.json`
- In-memory map + file sync
- Zero external dependencies
- Full repo implementations including Quotas, Settings, Aliases
- Good for: testing, single-user

### SQLite Driver - IMPLEMENTED (`internal/db/sqlite.go`)
- modernc.org/sqlite (pure Go, no CGO)
- Single file: `gorouter.db`
- Schema migration on startup via `Migrate()`
- Tables: users, api_keys, providers, models, combos, combo_items, usage_events, audit_logs, settings, quotas, rate_limits
- Single-writer mode (MaxOpenConns=1)
- Full repository implementations for all 11 tables
- Good for: self-host, small team, single-node

### PostgreSQL Driver - IMPLEMENTED (`internal/db/postgres.go`)
- lib/pq driver, full ACID, concurrent access
- All repositories implemented with parameterized queries
- FK constraints with ON DELETE CASCADE
- Migration wrapped in transaction
- Good for: multi-user, HA, production

## Config

```bash
# JSON (default fallback)
GOROUTER_DATABASE_DRIVER=json
GOROUTER_DATABASE_DSN=gorouter.json

# SQLite
GOROUTER_DATABASE_DRIVER=sqlite
GOROUTER_DATABASE_DSN=gorouter.db

# PostgreSQL
GOROUTER_DATABASE_DRIVER=postgres
GOROUTER_DATABASE_DSN=postgres://user:pass@localhost/gorouter?sslmode=disable
```

## Related Specs

- [[spec:001-scaffold]] - DatabaseManager implementation

## Implementation

- JSON driver: `internal/db/dbmanager.go`
- SQLite driver: `internal/db/sqlite.go`
- PostgreSQL driver: `internal/db/postgres.go`
- Types/interfaces: `internal/db/dbmanager.go`