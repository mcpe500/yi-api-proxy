---
title: "ADR-001: Go for Lightweight API Gateway"
type: decision
tags: [architecture, golang, lightweight]
---

# ADR-001: Go for Lightweight API Gateway

## Decision

Implement gorouter in Go instead of continuing with Node.js (9Router).

## Rationale

| Aspect | Node.js (9Router) | Go (gorouter) |
|--------|-------------------|---------------|
| Binary | 300MB+ runtime + npm_modules | ~15-25MB single binary |
| RAM | 100-300MB | 20-50MB |
| Startup | 2-5 seconds | <100ms |
| Docker | ~500MB image | ~20-30MB (distroless) |
| Dependencies | 500+ npm packages | ~7 Go modules |
| Deployment | Node.js runtime required | Single static binary |

Primary goal: **ringan** (lightweight). User explicitly chose Go for lighter resource usage.

## Consequences

- **Positive**: Smaller footprint, faster startup, easier deployment, no runtime dependency
- **Positive**: Static typing catches errors at compile time
- **Positive**: Built-in concurrency (goroutines) for streaming
- **Negative**: Smaller ecosystem than Node.js for web
- **Negative**: More verbose code for simple operations
- **Mitigation**: chi framework is lightweight enough; Go stdlib is comprehensive

## Related

- [[spec:001-scaffold]] - Project foundation