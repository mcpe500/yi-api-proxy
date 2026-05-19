---
title: "Online Sync Worker"
type: component
tags: [sync, online, cloud, 9router-parity]
---

# Component: Online Sync Worker

## Overview

Online Sync membuat gorouter **online-ready** seperti 9router: konfigurasi lokal dapat dikirim periodik ke endpoint cloud/self-hosted.

Ini bukan hosted 9router cloud penuh; ini fondasi sinkronisasi agar deployment Go yang awalnya offline bisa masuk mode online/cloud.

## Activation

```bash
GOROUTER_ONLINE_SYNC_URL=https://example.com/gorouter/sync
GOROUTER_ONLINE_SYNC_TOKEN=optional-secret-token
```

Jika `GOROUTER_ONLINE_SYNC_URL` kosong, worker tidak berjalan.

## Behavior

- Worker berjalan background dari `cmd/server/main.go`.
- Interval sync: 5 menit.
- Payload berisi konfigurasi lokal:
  - providers
  - models
  - combos
- Auth opsional via bearer token dari `GOROUTER_ONLINE_SYNC_TOKEN`.

## Data Flow

```text
gorouter local DB
  -> sync.Worker snapshot
  -> POST GOROUTER_ONLINE_SYNC_URL
  -> cloud/self-hosted sync endpoint
```

## Files

- `gorouter/internal/sync/worker.go`
- `gorouter/internal/config/config.go`
- `gorouter/cmd/server/main.go`

## Limitations / Next Work

- Saat ini one-way push, belum bidirectional merge.
- Conflict resolution belum ada.
- Cloud endpoint/server belum disediakan dalam repo.
- Secret redaction/encryption policy perlu diaudit sebelum public multi-tenant cloud.

## Related

- [[guides:9router-parity]]
- [[components:dbmanager]]
- [[components:routing]]
