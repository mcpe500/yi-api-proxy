---
title: "Quota & Rate Limiting"
type: component
tags: [quota, rate-limit, budget, enforcement]
---

# Component: Quota & Rate Limiting

## Overview

Quota dan rate limiting enforcement sebelum request ke provider (pre-request validation).

## Quota Policies

```go
type QuotaPolicy struct {
    TargetType   string  // "user" | "api_key"
    TargetID     string
    PolicyType   string  // "daily_requests" | "monthly_tokens" | "monthly_cost"
    Limit        int64
    ModelPattern string  // "*" for all, "glm/*"
    Period       string  // "day" | "month"
}
```

## Rate Limit Rules

```go
type RateLimitRule struct {
    TargetType string  // "user" | "api_key"
    TargetID   string
    LimitType  string  // "rpm" | "tpm"
    Limit      int
    Window     int     // seconds
}
```

## Token Bucket (In-Memory)

```go
type TokenBucket struct {
    Tokens     float64
    LastRefill time.Time
    Limit      int
    Window     int
}

func (s *MemoryStore) Allow(key string, limit int, window int) (bool, int) {
    // Refill based on elapsed time
    elapsed := now.Sub(bucket.LastRefill).Seconds()
    refill := (elapsed / float64(window)) * float64(bucket.Limit)
    bucket.Tokens = min(float64(bucket.Limit), bucket.Tokens + refill)
    
    if bucket.Tokens >= 1 {
        bucket.Tokens--
        return true, 0
    }
    retryAfter := int((1 - bucket.Tokens) / (float64(bucket.Limit) / float64(window)))
    return false, retryAfter
}
```

## Pre-Request Validation

```
ValidateRequest(userID, apiKeyID, modelID, tokens):
  1. Check model permission (CanUseModel)
     → Error: model_forbidden (403)
  
  2. Check quota policies
     → Error: quota_exceeded (429)
  
  3. Check rate limit (RPM)
     → Error: rate_limited (429)
  
  4. Check rate limit (TPM)
     → Error: rate_limited (429)
  
  → Pass: execute request
```

## Config

```bash
# User quota (via API)
POST /api/admin/users/{id}/quota

# Rate limit (via API)
POST /api/admin/users/{id}/rate-limits

# Admin can reset quota
POST /api/admin/users/{id}/reset-quota
```

## Related Specs

- [[spec:012-quota]] - Quota implementation
- [[spec:004-apikeys]] - API key scoping
- [[spec:006-models]] - Model permissions