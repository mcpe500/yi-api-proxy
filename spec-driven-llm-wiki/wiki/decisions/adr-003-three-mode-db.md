---
title: "ADR-003: Three-Mode DatabaseManager"
type: decision
tags: [database, json, sqlite, postgresql, abstraction]
---

# ADR-003: Three-Mode DatabaseManager (JSON/SQLite/PostgreSQL)

## Decision

Support 3 database modes via DatabaseManager interface: JSON, SQLite, PostgreSQL.

## Rationale

- **JSON**: Zero setup, good for testing and single-user (like 9Router's db.json)
- **SQLite**: Production-ready for self-host, single binary deployment
- **PostgreSQL**: Scale deployment, multi-user, full ACID

User requested all three: "sqlite, postgre, json jadi nanti ada DatabaseManager".

## Consequences

- **Positive**: Flexible deployment options
- **Positive**: Developer can start with JSON, upgrade to SQLite, then PG
- **Negative**: Must maintain 3 implementations of Repository interface
- **Negative**: Testing matrix multiplied by 3
- **Mitigation**: Shared test suite across drivers; JSON driver is simplest

## Related

- [[spec:001-scaffold]] - DatabaseManager implementation
- [[Component:DatabaseManager]] - Component documentation