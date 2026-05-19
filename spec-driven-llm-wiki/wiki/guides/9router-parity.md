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
