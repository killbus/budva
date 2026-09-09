# Telegram / TDLib Integration Guidelines

> Conventions for `internal/infra/telegram` — the go-tdlib (v0.7.6) adapter
> shared by facade and engine.

Content below comes from the chat-warmup task (Sept 2026, warmup-lazy-loadchats)
and its investigation of go-tdlib's actual behavior.

---

## Scenario: Wrapping TDLib calls in the Repo adapter

### 1. Scope / Trigger

- Trigger: any change to `internal/infra/telegram/` — new TDLib calls, error
  handling, or lifecycle logic.
- TDLib (via tdjson) is a **push-heavy** API: after `authorizationStateReady`
  it does NOT load the full chat list into the local database. Any
  **pull-based** call on a chat not yet in the DB fails with
  `400 Chat not found`. Affected surfaces: facade reads/writes on cold start,
  engine forwards to silent destinations, ruleset hot-reload destinations, and
  pure-output channels (deadlock: the only writer is the engine itself).

### 2. Signatures

```go
// internal/infra/telegram/client_adapter.go — the only go-tdlib surface
type clientAdapter interface {
    SendMessage(*client.SendMessageRequest) (*client.Message, error)
    // ... only methods with a live consumer; never the full go-tdlib surface
}

// Repo embeds clientAdapter; same-name methods on *Repo SHADOW the interface
// method — that is the established wrapper pattern:
func (r *Repo) GetMessage(req *client.GetMessageRequest) (*client.Message, error)

// internal/infra/telegram/warmup.go — lazy warmup entry point
func (r *Repo) warmOnChatNotFound(err error, method string, chatID int64) bool
```

### 3. Contracts

- **go-tdlib error model (verified against v0.7.6 source)**:
  `buildResponseError` returns `client.ResponseError` **by value** —
  `type ResponseError struct { Err *Error }`, with `Code int32` / `Message
  string` inside the nested pointer. Therefore:
  - `errors.As` must target the **value type** (`var e client.ResponseError`),
    never `*client.ResponseError`.
  - code/message live at `e.Err.Code` / `e.Err.Message` (nil-guard `e.Err`).
  - `ResponseError` has NO `Is()`/`Unwrap()`: `errors.Is` compares the inner
    `*Error` **pointer** — a freshly-allocated equal-valued error will NOT
    match. Tests must share one error instance across mock calls.
- **Chat-not-found retry contract** (all 9 covered wrappers): on `400
  "Chat not found"` (code exact, message case-insensitive), the wrapper
  triggers a LoadChats bootstrap and retries the call exactly **once**;
  other errors pass through untouched; the `%w` context string of every
  wrapper is preserved.
- **Warmup bootstrap semantics**: singleflight (mutex + cond — concurrent
  misses wait for the in-flight bootstrap), rate-limited to one bootstrap per
  `warmMinInterval` (30 s) so a genuinely-nonexistent chat cannot storm
  LoadChats; rate-limited triggers still return `true` (the single retry is
  cheap). LoadChats loops until TDLib errors (official tdjson semantics:
  "chat list is empty" terminates the loop), bounded by an iteration cap.
- **Mock surface**: `mocks/client_adapter.go` stubs the embedded interface;
  wrapper logic (warmup, retry) is the unit under test and must NOT be
  generated into mocks. App layers (`internal/app/handler`,
  `internal/app/facade`) define their own partial `telegramRepo` interfaces
  with their own mocks — Repo-level wrapper changes never ripple into
  app-layer tests.

### 4. Validation & Error Matrix

| Condition | Behavior |
|---|---|
| TDLib returns `400` + "Chat not found" (any case) | one LoadChats bootstrap (dedup + rate-limit), single retry, then final result |
| TDLib returns other 400 / 429 / transport errors | passthrough, no warmup, no retry |
| Bootstrap LoadChats fails | log Warn, retry caller still runs (partial warmup may cover the miss) |
| Retry also misses | return wrapped error with original `%w` context |
| TDLib message text drifts | matcher returns false → silent passthrough (no panic, no false positive) |

### 5. Good/Base/Bad Cases

- Good: `err != nil && r.warmOnChatNotFound(err, "GetChat", req.ChatId)` —
  retry once, preserve context string.
- Base: non-covered methods (EditMessageText, DeleteMessages, TranslateText,
  cmd/stand management calls) stay plain passthrough wrappers — they are
  caller-validated chat IDs, not cold-DB reads.
- Bad: warming up at startup (structurally blind to ruleset hot-reload and
  to the facade-servers-before-TDLib race); adding a wrapper without the
  `.Maybe()`/shared-instance discipline in tests; matching the error message
  with `==` instead of `EqualFold` (TDLib text drift).

### 6. Tests Required

- `internal/infra/telegram/warmup_test.go` (mockery `ClientAdapter` +
  `testing/synctest` for virtual time): miss→retry-ok, miss→retry-fails
  (shared error instance for `ErrorIs`), non-matching passthrough, concurrent
  misses → exactly one bootstrap, rate-limit window, per-wrapper table-driven
  coverage of all 9 methods.
- `go vet ./...`, `go test ./internal/infra/telegram/...`,
  `go test -short ./internal/...`, `gofmt -l .` clean.

### 7. Wrong vs Correct

#### Wrong

```go
var e *client.ResponseError          // As never matches: buildResponseError returns a VALUE
if errors.As(err, &e) && e.Code == 400 { ... }

assert.ErrorIs(t, err, chatNotFoundErr())  // fresh *Error allocation: pointer compare fails
```

#### Correct

```go
var e client.ResponseError
if errors.As(err, &e) && e.Err != nil &&
    e.Err.Code == 400 && strings.EqualFold(e.Err.Message, "Chat not found") { ... }

notFound := chatNotFoundErr()          // ONE instance for all mock calls
m.EXPECT().GetMessage(mock.Anything).Times(2).
    RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
        return nil, notFound
    })
assert.ErrorIs(t, err, notFound)
```

---

## Environment gotcha: this repo cannot build go-tdlib on Windows

go-tdlib v0.7.6 is **cgo-only** (`tdjson_static.go`/`tdjson_dynamic.go` carry
linux/darwin build tags; the C headers are TDLib's). A Windows checkout with
`CGO_ENABLED=0` fails on `undefined: JsonClient` / `Response` / `Type` — in
the dependency, not in project code. `go vet` / `go test` / `go build` for
any package importing `internal/infra/telegram` must run on Linux or in the
Docker image (the Dockerfile builds TDLib from source). On Windows, only
`gofmt` and static reading of the module cache
(`go env GOMODCACHE` → `github.com/zelenin/go-tdlib@v0.7.6/client/`) are
available for verification.
