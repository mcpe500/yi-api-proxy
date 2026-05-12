---
title: "Routing & Combo Fallback"
type: component
tags: [routing, combo, fallback, smart-routing]
---

# Component: Routing & Combo Fallback

## Overview

Routing engine menangani model resolution, provider selection, dan automatic fallback saat provider gagal.

## Combo Model

```go
type Combo struct {
    ID          string
    Name        string       // "coding-default", "cheap-combo"
    Strategy    string       // "priority_fallback" | "weighted_round_robin"
    Items       []ComboItem  // Ordered list
}

type ComboItem struct {
    ModelID  string  // "glm/glm-4.5"
    Priority int    // Lower = higher priority
    Weight   int    // For weighted round-robin
}
```

## Routing Strategies

| Strategy | Description |
|----------|-------------|
| priority_fallback | Try in order, fallback if failed |
| weighted_round_robin | Weight-based, sticky sessions |
| least_cost | Sort by input_price + output_price |
| quota_aware | Avoid models with low remaining quota |
| latency_aware | Sort by p95 latency |

## Fallback Eligibility

### Eligible for Fallback
- 429 Rate Limit
- 502/503/504 Server Error
- Timeout
- Quota exceeded (try another account)

### NOT Eligible for Fallback
- 400 Bad Request (invalid JSON, missing field)
- 401 Auth error (invalid API key)
- 403 Forbidden (model not allowed)
- Content policy error (non-provider-specific)

### 401/403 Special Case
- If can refresh token → retry once
- If different account available → try that account

## Combo Execution Flow

When a combo is resolved:

```
1. Load combo items ordered by priority
2. For each item:
   a. Find provider via ProviderRepository.FindByID
   b. Get adapter from registry by provider name
   c. adapter.TranslateRequest() → provider-specific request
   d. Execute HTTP request upstream
   e. adapter.ParseResponse() → NormalizedChatResponse
   f. On success → clear cooldown, return response
   g. On error (429, 5xx) → mark cooldown, continue to next item
3. Return response from first successful provider
```

## Cooldown Mechanism

```go
// On error (429, 5xx)
provider.MarkCooldown(id, until, errorMsg)
// Status → "cooldown", sets CooldownUntil, LastError

// On success
provider.ClearCooldown(id)
// Status → "active", clears all error state
```

## Account Selection (Implemented)

Account selection uses `ProviderRepository.FindByProvider` and filters by:
- `Status == "active"`
- `CooldownUntil == nil OR CooldownUntil < now`

Fallback candidates are sorted by `Priority` DESC.

## Related Specs

- [[spec:011-routing]] - Routing implementation
- [[spec:005-providers]] - Provider management + cooldown
- [[spec:006-models]] - Model catalog + aliases