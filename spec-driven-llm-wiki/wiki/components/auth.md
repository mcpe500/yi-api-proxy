---
title: "Authentication & Authorization"
type: component
tags: [auth, jwt, session, rbac, api-keys]
---

# Component: Authentication & Authorization

## Overview

Sistem auth untuk dashboard (JWT) dan API gateway (API key).

## Dashboard Auth (JWT)

```go
// Login
POST /api/auth/login
Body: { "email": "admin@test.com", "password": "secret" }
Response: Sets httpOnly cookie "auth_token"

// JWT Claims
{
  "user_id": "uuid",
  "email": "admin@test.com",
  "role": "admin",
  "exp": 1234567890
}

// Middleware
RequireAuth(jwtSecret) → validates JWT cookie → injects user to context
RequireAdmin() → checks user.Role == "admin"
```

## API Key Auth (Gateway)

```go
// Key format
sk-gorouter-<prefix>_<secret>
Example: sk-gorouter-abc123_x4k9m2p8q1r7n3h5j6t0w5y2z4

// Request header
Authorization: Bearer sk-gorouter-abc123_x4k9m2p8q1r7n3h5j6t0w5y2z4

// Storage
key_prefix = "abc123"  // for lookup (indexed)
key_hash = bcrypt(hash)  // for validation (cost 10)

// Lookup strategy
1. Extract prefix from key (between sk-gorouter- and _)
2. Find active key by prefix (fast indexed lookup)
3. Verify full key using bcrypt.CompareHashAndPassword
```

## RBAC Matrix

| Action | Admin | User |
|--------|-------|------|
| Create user/admin | Yes | No |
| Self-service register | Yes | Yes |
| Edit/suspend/delete user | Yes | No |
| Create API key | Yes | Own only |
| Manage providers | Yes | No |
| Manage models/combos | Yes | No |
| Manage quotas/rate limits | Yes | No |
| View all usage | Yes | Own only |
| View audit logs | Yes | No |
| Use /v1/* gateway | If has key | If has key |

## Self-Service Registration

```go
// User registration
POST /auth/register
Body: { "email": "user@example.com", "name": "User Name", "password": "password123" }
Response: { "token": "...", "expires_at": ..., "user": { "id": "...", "email": "...", "role": "user" } }

// Validation
- Email: required, valid format
- Password: min 8 characters
- Name: required
- Duplicate email → 409 Conflict
- Creates user with role="user", status="active"
- Returns JWT token (24h expiry)
```

## Bootstrap Admin

```bash
# Via environment variables
GOROUTER_BOOTSTRAP_ADMIN_EMAIL=admin@test.com
GOROUTER_BOOTSTRAP_ADMIN_PASSWORD=change-me

# Creates admin user on first start if not exists
```

## Related Specs

- [[spec:002-auth]] - Auth system
- [[spec:003-users]] - User management + RBAC
- [[spec:004-apikeys]] - API key management