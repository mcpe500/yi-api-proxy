---
title: "9Router Parity Guide"
type: guide
tags: [9router, parity, workflow, models]
---

# Guide: 9Router Parity Workflow

Dokumen ini menjelaskan alur kerja untuk memanfaatkan fitur 9router parity dalam gorouter.

## Workflow Overview

Alur kerja utama untuk manajemen model dan provider:

1.  **Add Provider**: Daftarkan provider (OpenAI, Anthropic, dll) melalui Dashboard atau API.
2.  **Define Models Inline**: Saat mendefinisikan provider, masukkan daftar model yang didukung langsung di dalam resource provider (Nested Resources).
3.  **Request Model ID**: User melakukan request menggunakan `model_id` standar (misal: `gpt-4o` atau `claude-3-5-sonnet`).
4.  **Auto Fallback**: Sistem secara otomatis mencari semua provider yang mendukung `model_id` tersebut dan melakukan routing/fallback jika terjadi kegagalan pada provider utama.

## Advantages

- **Simplifikasi Konfigurasi**: Tidak perlu mendefinisikan model secara terpisah jika hanya ingin menggunakan model bawaan provider.
- **High Availability**: Fallback otomatis antar provider untuk model yang sama tanpa perlu membuat Combo secara manual.
- **Dynamic Scaling**: Cukup tambah provider baru dengan model yang sama, dan sistem akan otomatis menggunakannya sebagai kandidat routing.

## Token Optimization: RTK

RTK (Rust Token Killer) mengompres output tool (git diff, ls, grep, build) sebelum dikirim ke LLM, mengurangi token usage hingga 80% pada output panjang.

### Activation

- **Header**: `X-RTK: true` pada request ke `/v1/*`
- **Config**: `GOROUTER_RTK_ENABLED=true`
- **Per-filter**: `X-RTK-Filter: gitdiff|ls|grep|build|autodetect`

### Available Filters

| Filter | Target Output |
|--------|--------------|
| `gitdiff` | `git diff`, `git show` |
| `ls` | `ls -la`, directory listings |
| `grep` | `grep -rn`, ripgrep output |
| `build` | `cargo build`, `npm run build`, `tsc`, `go build` |
| `test` | vitest, playwright, cargo/go tests |
| `gitops` | `git status`, `git log`, branch/push/pull |
| `github` | `gh pr`, `gh run`, `gh issue` |
| `pkgmgr` | `npm`, `pnpm`, `npx` |
| `infra` | `docker`, `kubectl` |
| `network` | `curl`, `wget` |
| `err` / `log` / `json` / `summary` | generic analysis filters |
| `autodetect` | Auto-detect dari content markers |

### Auto-Detection

Jika `X-RTK-Filter` tidak ditentukan, RTK mendeteksi format output secara otomatis:
- **gitdiff**: Deteksi marker `diff `, `index `, `--- `, `+++ `, `@@`
- **build**: Deteksi keyword `Compiling`, `Finished`, `cargo build`, `npm build`, `go build`, `tsc`
- **grep**: Deteksi pola `file:line:content`
- **ls**: Deteksi permission string `drwxr-xr-x` dan timestamps

### Integration Point

RTK berjalan di input pipeline gateway, sebelum request dikirim ke provider upstream. Ini mengompres tool output yang ada di message content.

## Token Optimization: Caveman Mode

Caveman Mode menyuntikkan prompt pendek ke system message untuk memaksa model merespons secara singkat, mengurangi output tokens ~65%.

### Activation

- **Header**: `X-Caveman: full|lite|ultra` pada request ke `/v1/chat/completions`, `/v1/responses`, `/v1/messages`
- **Config**: `GOROUTER_CAVEMAN_LEVEL=lite|full|ultra`

### Modes

| Mode | Description | Token Reduction |
|------|------------|-----------------|
| `lite` | Drop articles, filler, pleasantries. Fragments OK. | ~50% |
| `full` | Full terse rules. Drop articles, filler, hedging. Short synonyms. | ~65% |
| `ultra` | Maximum compression. Minimal prompt. | ~75% |

### Behavior

Caveman injector menambahkan system message di awal message array. Prompt berisi instruksi untuk menghilangkan filler words, articles, dan verbose language sambil mempertahankan akurasi teknis.

### Supported Endpoints

- `POST /v1/chat/completions` - System message prepended
- `POST /v1/responses` - System message/input prepended
- `POST /v1/messages` - Anthropic `system` field modified

## Online Mode

Gorouter sekarang punya fondasi online-ready melalui [[components:sync]]:

```bash
GOROUTER_ONLINE_SYNC_URL=https://example.com/sync
GOROUTER_ONLINE_SYNC_TOKEN=secret
```

Worker melakukan push snapshot providers/models/combos tiap 5 menit. Ini menutup gap awal "Go masih offline", namun belum sama dengan hosted 9router cloud penuh karena belum ada bidirectional sync dan cloud dashboard service.

## 3-Tier Fallback

Provider punya `tier`:

1. `subscription` — prioritas utama jika tersedia.
2. `cheap` — fallback biaya rendah.
3. `free` — fallback terakhir/gratis.

Routing strategy `tier` memprioritaskan Subscription → Cheap → Free untuk mendekati pola fallback 9router.
