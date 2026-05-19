---
title: "RTK - Rust Token Killer"
type: component
tags: [rtk, compression, token-optimization, input-filter]
---

# Component: RTK (Rust Token Killer)

## Overview

RTK mengompres **input/tool output** sebelum request diteruskan ke upstream LLM. Target utama: output panjang dari tool coding (`git diff`, `rg`, `go test`, `docker logs`, `curl`, dll) agar token prompt lebih kecil.

## Activation

```http
X-RTK: true
X-RTK-Filter: autodetect|gitdiff|ls|grep|build|test|gitops|github|pkgmgr|infra|network|err|log|json|summary
```

Global config:

```bash
GOROUTER_RTK_ENABLED=true
GOROUTER_RTK_DEFAULT_FILTER=autodetect
```

## Pipeline

```text
client request
  -> TokenOptimizer
  -> RTK compress message content
  -> Caveman inject (optional)
  -> provider routing/fallback
  -> upstream provider
```

## Filters

| Filter | Target | Behavior |
|---|---|---|
| `gitdiff` | `git diff`, `git show` | strip headers/noise, preserve changed lines |
| `ls` | `ls`, `dir` | compact permissions/date/listing noise |
| `grep` | `grep`, `rg` | preserve file/line hits, remove repeated context |
| `build` | `go build`, `npm build`, `cargo build`, `tsc` | keep warnings/errors/summary, drop progress |
| `test` | vitest, playwright, cargo/go tests | drop passing noise, keep failures/summaries |
| `gitops` | `git status`, `git log`, branch/push/pull | compact VCS status/log output |
| `github` | `gh pr`, `gh run`, `gh issue` | compact GitHub CLI rows/status |
| `pkgmgr` | `npm`, `pnpm`, `npx` | strip install/progress noise |
| `infra` | `docker`, `kubectl` | compact ps/get/log style output |
| `network` | `curl`, `wget` | remove transfer/progress meters |
| `err` | logs/errors | keep error-like lines |
| `log` | logs | deduplicate noisy log lines |
| `json` | JSON blobs | compact structural JSON |
| `summary` | generic long text | summarize repeated lines/noise |
| `autodetect` | default | choose best filter by markers |

## Implementation

Main package: `gorouter/internal/rtk/`

```go
compressed, saved := rtk.Compress(content)
compressed, saved := rtk.CompressWithFilter(content, "gitdiff")
filters := rtk.GetAvailableFilters()
```

Integrated through `gorouter/internal/handlers/v1/tokenizer.go` into:

- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

## Caveats

- RTK is lossy by design; use only for tool/log output, not user text needing exact preservation.
- Streaming response chunks are not retro-compressed; optimization happens before upstream request.

## Related

- [[components:caveman]]
- [[components:gateway]]
- [[components:routing]]
