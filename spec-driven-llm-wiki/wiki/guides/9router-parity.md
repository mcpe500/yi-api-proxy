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

| Filter | Target Output | Typical Savings |
|--------|--------------|-----------------|
| `gitdiff` | `git diff`, `git show` | 80% |
| `ls` | `ls -la`, directory listings | 60-75% |
| `grep` | `grep -rn`, ripgrep output | 75% |
| `build` | `cargo build`, `npm run build`, `tsc` | 70-87% |
| `autodetect` | Auto-detect dari content markers | Varies |

### Auto-Detection

Jika `X-RTK-Filter` tidak ditentukan, RTK mendeteksi format output secara otomatis:
- **gitdiff**: Deteksi marker `diff `, `index `, `--- `, `+++ `, `@@`
- **build**: Deteksi keyword `Compiling`, `Finished`, `cargo build`, `npm build`, `go build`, `tsc`
- **grep**: Deteksi pola `file:line:content`
- **ls**: Deteksi permission string `drwxr-xr-x` dan timestamps

### Integration Point

RTK berjalan di response pipeline gateway, setelah upstream response diterima dan sebelum dikirim ke client. Filter diterapkan pada non-streaming responses. Untuk streaming, filter diterapkan per-chunk jika memungkinkan.

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

- `POST /v1/chat/completions` - System message injection
- `POST /v1/responses` - Prompt prepended to input
- `POST /v1/messages` - System field modification
