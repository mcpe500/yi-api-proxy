---
title: "Caveman Mode"
type: component
tags: [caveman, prompt-injection, terse-mode, output-optimization]
---

# Component: Caveman Mode

## Overview

Caveman Mode menyuntikkan prompt system message yang memaksa model AI untuk merespons secara singkat dan minimal, mengurangi output tokens ~65%. Prompt berisi aturan untuk menghilangkan filler words, articles, dan verbose language.

## Activation

### Via Header

```
X-Caveman: lite|full|ultra
```

### Via Config

```bash
GOROUTER_CAVEMAN_LEVEL=full
```

## Modes

### Lite Mode

Prompt untuk pengurangan sedang (~50%):

```
You terse caveman. Technical substance stay exact, fluff die.
Drop articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries.
Fragments OK. Short synonyms (big not extensive).
Pattern: [thing] [action] [reason]. [next step].
```

### Full Mode

Prompt default untuk pengurangan optimal (~65%):

```
You are terse caveman. All technical substance stay exact, only fluff die.
Drop: articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries, hedging.
Fragments OK. Short synonyms (big not extensive, fix not implement a solution for).
Pattern: [thing] [action] [reason]. [next step].
Code blocks, file paths, commands, errors, URLs: keep exact.
```

### Ultra Mode

Prompt untuk pengurangan maksimal (~75%):

```
Terse caveman. Exact technical, fluff die.
Drop articles, filler, pleasantries.
Fragments OK. [thing] [action] [reason]. [next step].
```

## Injection Logic

```go
func Inject(messages []NormalizedMessage, level string) []NormalizedMessage {
    prompt := GetPrompt(level)
    injected := prependSystemMessage(messages, prompt)
    return injected
}
```

System message ditambahkan di awal array message, sebelum semua message user/assistant.

## Supported Endpoints

| Endpoint | Injection Method |
|-----------|-----------------|
| `POST /v1/chat/completions` | Prepend system message |
| `POST /v1/responses` | Prepend to input array |
| `POST /v1/messages` | Modify system field |

## Behavior

### What Gets Shortened

- Articles: `a`, `an`, `the`
- Fillers: `just`, `really`, `basically`, `actually`, `simply`
- Pleasantries: `thanks`, `please`, `sure`, `of course`
- Hedging: `I think`, `might be`, `could be`, `perhaps`

### What Stays Exact

- Code blocks and inline code
- File paths and URLs
- Commands and error messages
- Technical substance and accuracy

### Output Pattern

```
Before: "The function will iterate through the array and return the first element that matches the predicate."
After:  "iterate array, return first match."
```

## Integration Points

### v1 Handlers

- `ChatHandler` - Inject before request translation
- `ResponsesHandler` - Inject into input array
- `MessagesHandler` - Modify system field in request

### Prompt Retrieval

```go
prompt := caveman.GetPrompt("full")  // Returns full mode prompt
prompt := caveman.GetPrompt("lite")  // Returns lite mode prompt
prompt := caveman.GetPrompt("ultra") // Returns ultra mode prompt
```

## Related Specs

- [[spec:010-gateway]] - Gateway implementation
- [[spec:007-translator]] - Request/response translation
- [[components:rtk]] - RTK for output compression