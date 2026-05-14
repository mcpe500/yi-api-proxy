# Gorouter Audit Report: Perbandingan vs 9router, Bug Inti, dan Review Dokumentasi

Tanggal audit: 2026-05-14

## Ringkasan Eksekutif

Kesimpulan utama: **gorouter sudah memiliki fondasi gateway yang bagus, tetapi belum setara fitur dengan 9router**. Implementasi saat ini kuat di area Go backend, DB multi-mode, admin CRUD, API key, provider/model management, dan basic OpenAI-compatible chat/embeddings. Namun fitur advanced 9router seperti banyak adapter provider, OAuth/subscription providers, Cloudflare Worker, raw forward, token saver, cloud sync, cache, images/audio/web tools, true fallback tiered routing, quota-aware routing, dan kompatibilitas streaming masih belum lengkap atau belum ada.

Status kualitas saat ini: **usable sebagai skeleton / MVP internal**, belum production-ready penuh. Ada beberapa bug kritis yang perlu diperbaiki sebelum dipakai serius.

Build terakhir diverifikasi: `cd gorouter && rtk go build ./...` → **Success**.

Commit terkait integrasi routing terbaru: `2521765 feat: integrate routing strategies + health monitor into gorouter`.

## Sumber Audit

- Kode lokal: `gorouter/`
- Referensi 9router: `https://github.com/decolua/9router`
- Snapshot referensi: `temp-references/decolua-9router-8a5edab282632443.txt`
- Dokumentasi/spec lokal: `spec-driven-llm-wiki/`

## Apakah Sudah Benar?

Jawaban singkat: **sebagian benar, tetapi belum lengkap dan masih ada bug inti**.

Yang sudah benar:

- Server Go + Chi berjalan dan build sukses.
- DB abstraction tersedia untuk JSON/SQLite/Postgres.
- Admin auth, API key auth, provider CRUD, model CRUD, audit, dashboard dasar sudah ada.
- Provider secret sudah didekripsi di chat dan embeddings path.
- Router sudah terhubung ke chat path lewat `GOROUTER_ROUTING_STRATEGY`.
- Health monitor sudah start/stop bersama lifecycle server.
- Latency provider sudah diperbarui dari real request chat direct path.

Yang belum benar / belum matang:

- Rate limiter punya bug unit waktu sehingga limit bisa salah total.
- `/v1/responses` dan `/v1/messages` memakai encrypted secret mentah, bukan hasil decrypt.
- Routing model-aware masih hanya mengambil provider pertama untuk model yang sama.
- Fallback strategy belum melakukan retry ke provider berikutnya.
- Streaming belum OpenAI-compatible secara penuh.
- Quota/budget belum benar-benar enforced.
- Dokumentasi mengklaim fitur lebih banyak daripada implementasi nyata.

## Bug Inti / Temuan Kritis

### 1. Docker compose env typo membuat default deployment rusak

Lokasi:

- `gorouter/docker-compose.yml:8`
- `gorouter/docker-compose.yml:13`

Masalah:

```yaml
GOROUTER_DRIVER=json
GOROUTAP_ADMIN_PASSWORD=changeme123
```

Kode config memakai:

- `GOROUTER_DATABASE_DRIVER`
- `GOROUTER_BOOTSTRAP_ADMIN_PASSWORD`

Dampak:

- Driver DB dari compose tidak terbaca.
- Bootstrap admin password tidak terbaca.
- Deployment Docker default bisa gagal atau berjalan dengan config tidak sesuai.

Prioritas: **Critical**.

### 2. Rate limiter salah unit waktu

Lokasi:

- `gorouter/internal/middleware/ratelimit.go:87`
- `gorouter/internal/middleware/ratelimit.go:128-131`
- `gorouter/internal/middleware/ratelimit.go:226-249`

Masalah:

- `now := time.Now().Unix()` memakai detik.
- Entry window disimpan dengan `UnixMilli()` memakai milidetik.
- Perbandingan cutoff detik vs timestamp milidetik membuat hitungan window tidak valid.

Dampak:

- RPM/RPD/TPM bisa tidak bekerja sesuai ekspektasi.
- Retry-after bisa salah.
- Cleanup window juga tidak akurat.

Prioritas: **Critical**.

### 3. TPM bukan token limit sungguhan

Lokasi:

- `gorouter/internal/middleware/ratelimit.go:116-125`
- `gorouter/internal/middleware/ratelimit.go:297-301`

Masalah:

- `tokens` menyimpan timestamp request, bukan jumlah token.
- `TrackTokenUsage` no-op.

Dampak:

- `TokensPerMinute` sebenarnya bukan pembatas token.
- Quota token bisa dilanggar.

Prioritas: **Critical**.

### 4. `/v1/responses` dan `/v1/messages` mengirim encrypted secret mentah

Lokasi:

- `gorouter/internal/handlers/v1/responses.go:221`
- `gorouter/internal/handlers/v1/messages.go:187`

Masalah:

```go
apiKey := string(providerConn.EncryptedSecret)
```

Chat dan embeddings sudah benar memakai decrypt, tetapi responses/messages belum.

Dampak:

- Upstream auth gagal jika secret tersimpan terenkripsi.
- Endpoint kompatibilitas tidak reliable.

Prioritas: **Critical**.

### 5. Router hanya mengambil provider pertama untuk model yang sama

Lokasi:

- `gorouter/internal/routing/router.go:56-61`

Masalah:

```go
if m.ModelID == modelID || m.ModelName == modelID || m.ID == modelID {
    targetProviderIDs = append(targetProviderIDs, m.ProviderID)
    break
}
```

Dampak:

- Jika model sama tersedia di beberapa provider, hanya provider pertama dipakai.
- Weighted/latency/cost routing praktis tidak berguna untuk model duplikat.

Prioritas: **High**.

### 6. Fallback strategy belum fallback sungguhan

Lokasi:

- `gorouter/internal/routing/router.go:180-184`
- `gorouter/internal/handlers/v1/chat.go` direct request flow

Masalah:

- `byFallback()` hanya memilih provider priority tertinggi.
- Tidak ada retry ke provider berikutnya saat 429/5xx/network timeout.

Dampak:

- Nama `fallback` menyesatkan.
- Gateway gagal walau provider alternatif tersedia.

Prioritas: **High**.

### 7. Cooldown/inactive provider masih bisa dipakai saat semua non-active

Lokasi:

- `gorouter/internal/routing/router.go:37-45`
- `gorouter/internal/routing/router.go:87-95`

Masalah:

Jika tidak ada provider active, router fallback ke semua provider termasuk cooldown/error/disabled.

Dampak:

- Provider yang sedang cooldown bisa tetap menerima traffic.
- Circuit breaker tidak konsisten.

Prioritas: **High**.

### 8. Streaming belum kompatibel penuh

Lokasi:

- `gorouter/internal/handlers/v1/chat.go` streaming/direct path
- `gorouter/internal/handlers/v1/responses.go:178-180`
- `gorouter/internal/handlers/v1/messages.go:144-146`

Masalah:

- Direct streaming cenderung copy raw upstream body.
- Combo streaming menulis JSON penuh sebagai blok SSE-like, bukan chunk token stream.
- Responses/messages streaming eksplisit `not implemented`.

Dampak:

- Client OpenAI/Anthropic yang butuh SSE bisa gagal.
- Streaming tidak bisa diandalkan.

Prioritas: **High**.

### 9. Cost accounting selalu nol di chat direct

Lokasi:

- `gorouter/internal/handlers/v1/chat.go:337`

Masalah:

```go
calculateCost(..., 0, 0)
```

Dampak:

- Usage cost tidak akurat.
- Budget/cost analytics tidak berguna.

Prioritas: **Medium**.

### 10. CORS wildcard + credentials bermasalah

Lokasi:

- `gorouter/cmd/server/main.go:145-151`

Masalah:

`AllowedOrigins: ["*"]` dipakai bersama `AllowCredentials: true`.

Dampak:

- Browser credentialed CORS bisa ditolak.
- Secara keamanan terlalu longgar untuk dashboard/admin.

Prioritas: **Medium**.

### 11. Health/readiness terlalu dangkal

Lokasi:

- `gorouter/cmd/server/main.go:154-162`

Masalah:

- `/health` mengembalikan `db: ok` tanpa ping DB nyata.
- `/ready` selalu ready.

Dampak:

- Orchestrator bisa menganggap service sehat padahal DB/provider rusak.

Prioritas: **Medium**.

### 12. Health monitor tidak validasi credential dan recovery belum jelas

Lokasi:

- `gorouter/internal/provider/health.go:83-99`
- `gorouter/internal/provider/health.go:102-113`

Masalah:

- Health check memakai `Authorization: Bearer test`.
- Status `<500` dianggap healthy, termasuk 401/403.
- Success latency update tidak jelas reset cooldown/backoff/last_error.

Dampak:

- Provider dengan credential salah bisa dianggap healthy.
- Provider cooldown bisa tidak recovery sesuai ekspektasi.

Prioritas: **Medium**.

## Feature Comparison vs 9router

| Area | 9router | gorouter saat ini | Status |
|---|---|---|---|
| OpenAI `/v1/chat/completions` | Ada | Ada basic | Partial/Present |
| Chat streaming | Ada | Ada tapi raw/kurang normalized | Partial |
| `/v1/models` | Ada | Ada | Present |
| `/v1/embeddings` | Ada | Ada | Partial |
| `/v1/responses` | Ada | Non-stream basic, streaming 501 | Partial |
| `/v1/messages` | Ada | Non-stream basic, streaming 501 | Partial |
| Raw forward/proxy generic | Ada | Tidak jelas/tidak ada | Missing |
| Provider adapters luas | Banyak / 40+ | 4 adapter: OpenAI, compatible, GLM, MiniMax | Missing besar |
| OAuth providers | Ada | Tidak ada | Missing |
| Auto token refresh | Ada | Tidak ada | Missing |
| Free/subscription provider support | Ada | Tidak ada native | Missing |
| OpenAI/Claude/Gemini/Cursor/Kiro/Vertex/Ollama translation | Ada | Normalized chat sederhana | Partial kecil |
| RTK token saver | Ada | Tidak ada | Missing |
| Caveman mode/token compression | Ada | Tidak ada | Missing |
| Smart 3-tier fallback | Ada | Combo fallback dasar | Partial |
| Custom combos | Ada | Ada CRUD + execute | Partial |
| Multi-account | Ada | Multiple provider records bisa | Partial |
| Routing priority/weighted/latency/cost | Ada | Ada basic | Partial |
| Quota-aware routing | Ada | Tidak ada | Missing |
| Circuit breaker/cooldown | Ada | Ada sebagian | Partial |
| Rate limiting | Ada | Ada tapi bug | Buggy |
| Usage analytics | Ada | Ada sebagian | Partial |
| Admin dashboard/API | Ada | Ada | Partial/Present |
| User API keys | Ada | Ada | Present |
| Cloud sync | Ada | Tidak ada | Missing |
| Cloudflare Worker | Ada | Tidak ada, Go server | Missing |
| Docker/VPS deploy | Ada | Ada file Docker | Partial |
| JSON/SQLite/Postgres | 9router LowDB/Cloud style | gorouter lebih kuat: JSON/SQLite/Postgres | gorouter lebih baik |
| Secret encryption | Ada | Ada, tapi inconsistent | Partial |
| Health/ready | Ada | Ada dangkal | Partial |
| Metrics Prometheus | Ada/di spec | Tidak ditemukan | Missing |
| Cache/semantic cache | Ada/indikasi | Tidak ditemukan | Missing |
| Token count endpoint | Ada | Tidak ditemukan | Missing |
| Provider verify endpoint | Ada | Tidak ditemukan jelas | Missing |
| Image API | Ada di ref tree | Tidak ada | Missing |
| Audio/STT/TTS | Ada di ref tree | Tidak ada | Missing |
| Web fetch/search | Ada | Tidak ada | Missing |

## Apakah Fitur Sudah Sebanyak 9router?

**Belum.** Estimasi kasar:

- Core gateway/admin skeleton: **60-70%** dari target lokal/spec.
- Advanced routing/quota/fallback/monitoring: **35-45%**.
- Feature breadth vs 9router asli: **25-35%**.

gorouter unggul di:

- Go binary sederhana.
- DB abstraction lebih serius: JSON/SQLite/Postgres.
- Struktur backend admin/API cukup rapi.
- Cocok sebagai base self-hosted API gateway.

9router unggul jauh di:

- Jumlah provider dan adapter.
- OAuth/subscription provider support.
- Cloudflare Worker/cloud sync ecosystem.
- Raw forward dan multi-format compatibility.
- Token saving / RTK / Caveman-specific tooling.
- Images/audio/web/search/cache/token-count/verify endpoints.

## Review Kode per Modul

### Config

File utama: `gorouter/internal/config/config.go`

Status: **Partial good**.

Kuat:

- Env-based config jelas.
- DB driver/DSN configurable.
- Routing strategy env sudah ada.

Masalah:

- Compose env tidak sinkron.
- `mustGenerateOrPanic()` misleading karena return empty string.
- Secret encryption key perlu validasi lebih ketat untuk production/provider usage.

### Main Server / Routing HTTP

File utama: `gorouter/cmd/server/main.go`

Status: **Good skeleton, but ops readiness partial**.

Kuat:

- Routes admin/user/v1 tertata.
- Middleware API key/rate limit/audit terpasang.
- Router + health monitor lifecycle sudah wired.

Masalah:

- CORS wildcard + credentials.
- `/health` dan `/ready` statis.
- Scope enforcement belum dipakai penuh untuk `/v1` inference.

### DB Manager

File utama:

- `gorouter/internal/db/dbmanager.go`
- `gorouter/internal/db/sqlite.go`
- `gorouter/internal/db/postgres.go`

Status: **Relatif kuat**.

Kuat:

- JSON/SQLite/Postgres ada.
- Repo pattern tersedia untuk users, API keys, providers, models, aliases, usage, audit, quota/rate limits.

Masalah:

- Beberapa repo/driver lama terlihat dead/confusing.
- Migration/versioning belum tampak matang untuk production lifecycle.
- Quota repo ada tetapi enforcement belum jelas.

### Auth / API Keys

File utama:

- `gorouter/internal/middleware/auth.go`
- `gorouter/internal/middleware/apikey.go`
- `gorouter/internal/auth/jwt.go`
- `gorouter/internal/apikeys/service.go`

Status: **Mostly present**.

Kuat:

- JWT dashboard auth ada.
- API key validation ada.
- Scope function tersedia.

Masalah:

- Inference routes belum memakai `RequireScope` secara konsisten.
- Cookie-only dashboard auth kurang fleksibel untuk API client.
- CORS/cookie settings perlu hardening.

### Rate Limit / Quota

File utama:

- `gorouter/internal/middleware/ratelimit.go`

Status: **Buggy / partial**.

Masalah utama:

- Detik vs milidetik bercampur.
- TPM bukan token usage.
- `TrackTokenUsage` no-op.
- Quota/budget enforcement belum nyata.

### Provider Adapter

File utama:

- `gorouter/internal/adapters/init.go`
- `gorouter/internal/adapters/openai/adapter.go`
- `gorouter/internal/adapters/openai_compatible/adapter.go`
- `gorouter/internal/adapters/glm/adapter.go`
- `gorouter/internal/adapters/minimax/adapter.go`
- `gorouter/internal/translator/types.go`

Status: **Basic but narrow**.

Kuat:

- Adapter interface ada.
- 4 adapter registered.
- Normalized request/response pattern tersedia.

Masalah:

- Provider breadth jauh dari 9router.
- Tool calls, multimodal content, response_format, logprobs, n, stop, penalties belum lengkap.
- MiniMax/GLM compatibility perlu diuji real API.
- Error response parsing embedding/chat belum matang.

### Chat / Gateway

File utama:

- `gorouter/internal/handlers/v1/chat.go`
- `gorouter/internal/handlers/v1/embeddings.go`
- `gorouter/internal/handlers/v1/models.go`
- `gorouter/internal/handlers/v1/responses.go`
- `gorouter/internal/handlers/v1/messages.go`

Status: **Basic OpenAI-compatible, advanced compat partial**.

Kuat:

- Chat non-streaming path ada.
- Embeddings ada.
- Models list ada.
- Responses/messages endpoint ada sebagai compatibility layer awal.

Masalah:

- Direct streaming belum normalized SSE.
- Responses/messages streaming not implemented.
- Responses/messages encrypted secret bug.
- Usage/cost extraction tidak lengkap.
- `/v1/models` fallback dummy model bisa misleading saat DB kosong.

### Routing

File utama:

- `gorouter/internal/routing/router.go`

Status: **Wired but not production-grade**.

Kuat:

- Strategy enum ada: priority, weighted, latency, cost, fallback.
- Chat path sudah memakai router.
- Model-aware selection sudah dimulai.

Masalah:

- `SelectProviderForModel()` break setelah match pertama.
- Fallback bukan retry chain.
- Cost memakai `context.Background()` dan ignore error.
- Sorting in-place bisa punya side effect.
- Cooldown handling belum strict.

### Combo

File utama:

- `gorouter/internal/combo/combo.go`
- `gorouter/internal/handlers/user_combos.go`

Status: **Partial**.

Kuat:

- Combo/fallback concept ada.
- Ordered provider execution ada.
- Cooldown marking ada.

Masalah:

- Streaming tidak real.
- Secret decrypt perlu diverifikasi; ada indikasi raw encrypted secret dipakai di beberapa path.
- Tidak ada tiered quota/cost-aware fallback seperti 9router.

### Health Monitoring

File utama:

- `gorouter/internal/provider/health.go`

Status: **Partial**.

Kuat:

- Loop monitor ada.
- Latency tracking ada.
- Failure/cooldown status ada.

Masalah:

- Fake bearer token.
- 401/403 dianggap healthy karena `<500`.
- Recovery/reset status belum kuat.
- Tidak expose metrics real.

### Docker / Deploy

File utama:

- `gorouter/Dockerfile`
- `gorouter/Dockerfile.dev`
- `gorouter/docker-compose.yml`

Status: **Partial**.

Masalah:

- Compose env typo critical.
- Healthcheck hanya shallow endpoint.
- Production secret defaults unsafe.

## Review Dokumentasi `spec-driven-llm-wiki/`

Kesimpulan: dokumentasi/spec **berguna sebagai arah desain**, tetapi **terlalu optimistis** dan tidak sinkron penuh dengan kode.

### Dokumen yang direview

- `spec-driven-llm-wiki/wiki/overview.md`
- `spec-driven-llm-wiki/wiki/components/*.md`
- `spec-driven-llm-wiki/wiki/patterns/*.md`
- `spec-driven-llm-wiki/wiki/decisions/*.md`
- `spec-driven-llm-wiki/spec/docs/*.md`
- `spec-driven-llm-wiki/spec/handoff/*.md`

### Klaim yang tidak sinkron

| Dokumen / area | Klaim | Realita kode |
|---|---|---|
| Overview status | Semua 17 spec implemented | Banyak fitur partial/missing |
| Gateway docs | Responses/messages status inconsistent | Routes ada, streaming 501, schema conversion tipis |
| DB docs | Postgres stub | `internal/db/postgres.go` sudah ada implementasi |
| Auth docs | `/api/auth/login` | Implementasi `/auth/login` |
| Dashboard docs | `/api/*` | Implementasi `/admin/*`, `/me/*`, `/auth/*` |
| Routing docs | `priority_fallback`, `weighted_round_robin`, `least_cost`, `quota_aware`, `latency_aware` | Kode: `priority`, `weighted`, `latency`, `cost`, `fallback`; quota-aware missing |
| Quota docs | token bucket + pre-request quota | Kode sliding window buggy + quota enforcement belum jelas |
| Metrics docs | Prometheus `/metrics` | Tidak ditemukan |
| Adapter docs | CommandCode/OpenCode Go | Tidak registered |

### Coverage terhadap spec lokal

| Spec area | Coverage |
|---|---|
| Scaffold/config | Mostly |
| DB manager | Mostly |
| JWT auth | Mostly |
| API key auth | Mostly |
| Provider CRUD | Mostly |
| Model catalog | Mostly |
| Aliases | Partial |
| OpenAI chat | Partial/mostly |
| Embeddings | Partial |
| Responses compatibility | Partial |
| Anthropic messages compatibility | Partial |
| Routing strategies | Partial |
| Combo fallback | Partial |
| Rate limit | Buggy partial |
| Quota/budget | Low/partial |
| Usage analytics | Partial |
| Audit logs | Partial/mostly |
| Health/readiness | Partial |
| Prometheus metrics | Missing |
| Docker/deploy | Partial |

## Prioritas Perbaikan

### P0 - Harus diperbaiki sebelum production

1. Fix `docker-compose.yml` env names.
2. Fix rate limiter timestamp unit; pilih detik atau milidetik konsisten.
3. Implement real token tracking untuk TPM atau disable klaim TPM.
4. Decrypt secret di `/v1/responses` dan `/v1/messages`.
5. Fix router agar semua provider yang punya model sama masuk kandidat.
6. Implement real fallback retry pada direct chat request.
7. Pastikan cooldown/inactive provider tidak dipilih kecuali explicit override.

### P1 - Penting untuk kompatibilitas

1. Implement OpenAI SSE streaming normalization.
2. Implement `/v1/responses` streaming.
3. Implement `/v1/messages` streaming.
4. Tambah support tools/tool_choice/response_format/stop/penalties/multimodal content.
5. Perbaiki usage token extraction dan cost calculation dari model pricing.
6. Enforce API key scopes di semua endpoint inference.

### P2 - Penting untuk parity 9router

1. Tambah adapter provider lebih banyak.
2. Tambah OAuth/token refresh provider flow jika ingin setara 9router.
3. Tambah raw forward/proxy endpoint.
4. Tambah token count endpoint.
5. Tambah provider verify/test endpoint.
6. Tambah images/audio/web/search jika memang target parity 9router.
7. Tambah cloud sync jika dibutuhkan.
8. Tambah cache/semantic cache.

### P3 - Operasional/observability

1. `/health` ping DB nyata.
2. `/ready` cek DB + migration + required config.
3. Add Prometheus `/metrics` jika mengikuti spec.
4. Health monitor validasi auth/config lebih baik.
5. Tambah tests untuk routing/rate-limit/auth/adapters.
6. Sinkronkan dokumentasi dengan implementasi.

## Rekomendasi Roadmap

### Tahap 1: Stabilkan MVP

- Fix bug critical P0.
- Tambah tests minimal untuk rate limiter, router, decrypt flow, fallback.
- Update docs agar status tidak misleading.

### Tahap 2: Benarkan gateway compatibility

- OpenAI streaming proper SSE.
- Responses/messages proper conversion + streaming.
- Tool calls dan multimodal content.
- Usage/cost real extraction.

### Tahap 3: Routing dan quota production-grade

- Real fallback chain.
- Quota-aware + budget-aware routing.
- Provider cooldown recovery.
- Model/provider health-aware selection.

### Tahap 4: Parity 9router bertahap

- Tambah adapter sesuai prioritas provider.
- OAuth/subscription providers.
- Raw forward, token count, verify endpoint.
- Images/audio/web/search/cache/cloud sync bila memang masuk scope.

## Verdict Akhir

- **Ada bug?** Ya, ada beberapa bug kritis: docker compose env typo, rate limiter unit mismatch, encrypted secret di responses/messages, routing hanya provider pertama, fallback belum real.
- **Apakah sudah benar?** Belum sepenuhnya. Fondasi benar, tetapi beberapa fitur diklaim selesai padahal partial/missing.
- **Apakah fitur sudah sebanyak 9router?** Belum. gorouter baru mencakup sebagian kecil-menengah dari breadth 9router, walau arsitektur Go + DB multi-mode cukup kuat sebagai base.
- **Apakah layak dilanjutkan?** Ya. Struktur cukup baik untuk dilanjutkan, tetapi perlu stabilisasi critical path sebelum menambah fitur baru.
