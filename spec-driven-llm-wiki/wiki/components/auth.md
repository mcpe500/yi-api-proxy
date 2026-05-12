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
Example: sk-gorouter-prod_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

// Request header
Authorization: Bearer sk-gorouter-xxx

// Storage
key_prefix = "sk-gorouter-prod"  // for lookup
key_hash = SHA256(full_key)      // for validation (never store plaintext)
```

## RBAC Matrix

| Action | Admin | User |
|--------|-------|------|
| Create user/admin | Yes | No |
| Edit/suspend/delete user | Yes | No |
| Create API key | Yes | Own only |
| Manage providers | Yes | No |
| Manage models/combos | Yes | No |
| View all usage | Yes | Own only |
| View audit logs | Yes | No |
| Use /v1/* gateway | If has key | If has key |

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