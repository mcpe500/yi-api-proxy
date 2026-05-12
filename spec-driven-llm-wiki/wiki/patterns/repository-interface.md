---
title: "Pattern: Repository Interface"
type: pattern
tags: [pattern, repository, interface, go]
---

# Pattern: Repository Interface

## Problem

Need to abstract database operations across 3 different drivers (JSON/SQLite/PostgreSQL).

## Solution

Define repository interfaces that each driver implements:

```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    List(ctx context.Context, filter UserFilter) ([]*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}
```

Each driver (JSON, SQLite, PG) implements this interface.

## Usage

```go
// Service depends on interface, not implementation
type UserService struct {
    users UserRepository
}

// At startup, inject correct implementation
svc := UserService{users: db.Users()}  // db is DatabaseManager
```

## Benefits

- Test with mock repository
- Swap database driver without changing business logic
- Each driver optimizes independently

## Used In

- All spec services (UserService, ApiKeyService, ProviderService, etc.)
- All 3 database drivers