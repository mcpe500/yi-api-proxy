---
title: "RTK - Rust Token Killer"
type: component
tags: [rtk, compression, token-optimization, output-filter]
---

# Component: RTK (Rust Token Killer)

## Overview

RTK adalah package untuk mengompres output tool (git diff, ls, grep, build output) sebelum dikirim ke LLM. Menggunakan filter-pattern spesifik untuk setiap jenis output, mengurangi token usage 60-90%.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Gateway Handler                       │
│  POST /v1/chat/completions, /v1/responses, /v1/messages  │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│              Upstream Provider Response                   │
│                    (raw output)                          │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│                    RTK Filter                             │
│           (applied based on X-RTK header)                 │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│              Compressed Response                          │
│               (sent to client)                           │
└─────────────────────────────────────────────────────────┘
```

## Activation

### Via Header

```
X-RTK: true
X-RTK-Filter: gitdiff|ls|grep|build|autodetect
```

### Via Config

```bash
GOROUTER_RTK_ENABLED=true
GOROUTER_RTK_DEFAULT_FILTER=autodetect
```

## Filters

### GitDiff Filter

Target: `git diff`, `git show`, `git log -p`

Transforms:
- Remove binary file indicators (minus content)
- Collapse context lines (`@@` sections)
- Show only file paths + changes summary
- Remove redundant `index` lines

Input:
```
diff --git a/src/main.go b/src/main.go
index 1234567..abcdefg 100644
--- a/src/main.go
+++ b/src/main.go
@@ -10,7 +10,8 @@ func main() {
-    fmt.Println("old")
+    fmt.Println("new")
+    fmt.Println("added line")
 }
```

Output:
```
src/main.go: -old +new +added line
```

### Ls Filter

Target: `ls -la`, directory listings

Transforms:
- Compact format: `drwxr-xr-x dir/`
- Remove timestamps for brevity
- Show file count summary
- Group by type

### Grep Filter

Target: `grep -rn`, ripgrep, `rg`

Transforms:
- Remove match context (filename:line: only)
- Collapse consecutive matches in same file
- Show match count per file
- Remove verbose headers

### Build Filter

Target: `cargo build`, `npm run build`, `tsc`, `go build`, `vite build`

Transforms:
- Show only status line (Compiling... Done)
- Summary: files changed, warnings, errors
- Collapsed compiler output
- Error lines preserved

## Auto-Detection

Jika filter tidak ditentukan, RTK mendeteksi format secara otomatis:

| Marker | Detected Type |
|--------|---------------|
| `diff `, `index `, `--- `, `+++ `, `@@` | gitdiff |
| `Compiling`, `Finished`, `cargo build` | build |
| `file:line:content` pattern | grep |
| `drwxr-xr-x` permission strings | ls |

## Integration

### v1 Handlers

RTK diintegrasikan pada response pipeline:

- `ChatHandler` - Compress streaming/non-streaming responses
- `ResponsesHandler` - Apply to response output
- `MessagesHandler` - Apply to Claude message output

### Compress Function

```go
import "github.com/gorouter/gorouter/internal/rtk"

compressed, saved := rtk.Compress(content)
compressed, saved := rtk.CompressWithFilter(content, "gitdiff")
```

### Available Filters

```go
filters := rtk.GetAvailableFilters()
// ["gitdiff", "ls", "grep", "build", "autodetect"]
```

## Related Specs

- [[spec:010-gateway]] - Gateway implementation
- [[spec:007-translator]] - Request/response translation