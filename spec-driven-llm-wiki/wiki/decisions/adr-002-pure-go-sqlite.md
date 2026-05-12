---
title: "ADR-002: Pure Go SQLite (modernc.org/sqlite)"
type: decision
tags: [database, sqlite, pure-go]
---

# ADR-002: Pure Go SQLite (modernc.org/sqlite)

## Decision

Use `modernc.org/sqlite` instead of `mattn/go-sqlite3` for SQLite driver.

## Rationale

- **No CGO**: `modernc.org/sqlite` is pure Go, compiles without C toolchain
- **Cross-compile**: Easy to build for linux/amd64, linux/arm64, darwin, windows
- **Static binary**: No external C libraries needed
- **Slightly slower**: But sufficient for gorouter use case (not high-throughput OLTP)

## Consequences

- **Positive**: True static binary, easy cross-compile
- **Positive**: No CGO complexity in CI/CD
- **Negative**: ~10-20% slower than mattn/go-sqlite3
- **Mitigation**: Performance acceptable for API gateway use case

## Related

- [[spec:001-scaffold]] - DatabaseManager
- [[ADR-001]] - Go for lightweight