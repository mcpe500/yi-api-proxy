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
}
```

## Drivers

### JSON Driver
- Single file persistence: `gorouter.json`
- In-memory map + file sync
- Zero external dependencies
- Good for: testing, single-user

### SQLite Driver
- modernc.org/sqlite (pure Go, no CGO)
- Single file: `gorouter.db`
- Production-ready for self-host
- Good for: small team, single-node

### PostgreSQL Driver
- lib/pq driver
- Full ACID, concurrent access
- Good for: multi-user, HA

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

See `internal/db/dbmanager.go` in gorouter codebase.