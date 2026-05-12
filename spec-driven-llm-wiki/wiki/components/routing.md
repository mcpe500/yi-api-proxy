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

## Account Selection

```go
GetActiveAccounts(provider, excludeIDs):
  accounts = providers.GetByProvider(provider)
  return accounts.Filter(a =>
    a.Status == "active" AND
    (a.CooldownUntil == nil OR a.CooldownUntil < now) AND
    a.ID NOT IN excludeIDs
  ).Sort(by priority ASC)
```

## Cooldown Mechanism

```go
// On error
cooldown = min(baseCooldown * (2 ^ backoffLevel), 300000)
providers.MarkCooldown(accountID, cooldown, errorText)

// On success
providers.ClearCooldown(accountID)
```

## Related Specs

- [[spec:011-routing]] - Routing implementation
- [[spec:005-providers]] - Provider management + cooldown
- [[spec:006-models]] - Model catalog + aliases