# Telegram / TDLib Integration Guidelines

> Conventions for `internal/infra/telegram` — the go-tdlib (v0.7.6) adapter
> shared by facade and engine.

Content below comes from the chat-warmup tasks (Sept 2026: warmup-lazy-loadchats,
then warmup-wait-ready) and their investigation of go-tdlib's actual behavior.

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
// method — that is the established wrapper pattern. The 9 cold-DB wrappers
// route through withChatReady → withReady:
func (r *Repo) GetChatHistory(req *client.GetChatHistoryRequest) (*client.Messages, error)

// internal/infra/telegram/warmup.go — wait-for-ready state machine
func (r *Repo) withReady(ctx context.Context, chatID int64, fn func() error) error

// Typed error on deadline expiry; transport maps it to gRPC Unavailable.
type ChatNotReadyError struct {
    ChatID  int64
    Drives  int   // LoadChats bootstrap drives within the window
    LastErr error // last TDLib error (400 Chat not found)
}
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
- **Wait-for-ready contract** (all 9 covered wrappers — SendMessage,
  SendMessageAlbum, ForwardMessages, GetMessage, GetMessages, GetChatHistory,
  GetMessageLink, GetMessageLinkInfo, GetChat): on `400 "Chat not found"`
  (code exact, message case-insensitive) the wrapper enters `withReady`:
  in a bounded window (config `TELEGRAM_WARMUP_DEADLINE`, default 5s; single
  clock from window entry) it waits for the chat to materialize, woken by
  the ticker (`TELEGRAM_WARMUP_TICKER`, default 500ms — each tick drives the
  rate-limited LoadChats bootstrap) and/or the `updateNewChat` edge, and
  retries the call. Non-matching errors pass through untouched; every
  wrapper's `%w` context string is preserved.
- **Why `updateNewChat` is the readiness edge** (verified against TDLib pin
  22d49d5): `add_new_dialog` erases the negative cache and sends
  `updateNewChat` atomically at the moment the dialog enters the map — both
  load paths (LoadChats server pagination, incoming messages) route through
  it. The retry itself remains the only oracle; the edge is an accelerator
  (a lost edge costs one tick, never correctness).
- **Certificate semantics**: a per-chat ready entry is a close-persistent
  channel in a `sessionReadiness` table behind `atomic.Pointer`. Closed =
  permanent level signal (later waiters observe it O(1)); the table is swapped
  in the SAME critical section that overwrites `clientAdapter` in
  `runAuthLoop` (the actual session boundary — NOT `AuthorizationStateReady`,
  which lags a full handshake and can be lost). Safety premise (contract
  clause): `entry.ready` is closed ONLY by the holder's own fn success; the
  `updateNewChat` edge dispatch acts on the current table only. Every wake
  re-lookups from the CURRENT table (release = closed AND in current table).
- **Deadline semantics**: single clock from `withReady` entry (includes the
  synchronous bootstrap time inside the first tick). On expiry the entry is
  closed-and-deleted (failed close) and `*ChatNotReadyError{ChatID, Drives,
  LastErr}` is returned — `Unwrap` yields the last TDLib error.
- **gRPC mapping** (`internal/transport/grpc/transport.go`):
  `errors.As(*telegram.ChatNotReadyError)` → `codes.Unavailable` +
  `errdetails.RetryInfo{RetryDelay: 2s}` (single helper `mapFacadeError`,
  used by all 10 handlers); all other errors remain `codes.Internal`.
- **Warmup bootstrap semantics**: singleflight (mutex + cond — concurrent
  misses wait for the in-flight bootstrap), rate-limited to one bootstrap per
  `warmMinInterval` (30 s) so a genuinely-nonexistent chat cannot storm
  LoadChats. LoadChats loops until TDLib errors (official tdjson semantics:
  "chat list is empty" terminates the loop — logged at **Info**, not Warn),
  bounded by an iteration cap.
- **Observability** (otel, registered via global meter — no-op until an
  exporter is configured): `budva.telegram.warmup.misses`,
  `.readiness{outcome=success|timeout}`, `.timeouts` (alert: rate > 2%),
  `.loadchats_errors`, `.session_swaps`, `.invalidated`,
  `.convergence_ms` (histogram), `budva.telegram.updates_backlog` (gauge).
- **Mock surface**: `mocks/client_adapter.go` stubs the embedded interface;
  wrapper logic (warmup, wait) is the unit under test and must NOT be
  generated into mocks. App layers (`internal/app/handler`,
  `internal/app/facade`) define their own partial `telegramRepo` interfaces
  with their own mocks — Repo-level wrapper changes never ripple into
  app-layer tests.

### 4. Validation & Error Matrix

| Condition | Behavior |
|---|---|
| TDLib returns `400` + "Chat not found" (any case) | enter `withReady` window: ticker drives LoadChats (dedup + rate-limit), `updateNewChat` edge wakes early; retry until success or deadline |
| TDLib returns other 400 / 429 / transport errors | passthrough, no warmup, no wait |
| Certificate already closed (second waiter) | O(1) fast path: one fn call, no window |
| Session changes mid-wait (table swap at adapter overwrite) | stale certificate ignored; waiter re-anchors to the current table |
| Wait deadline expires | entry closed-and-deleted; `*ChatNotReadyError` → gRPC Unavailable + RetryInfo(2s) |
| Non-matching error mid-wait | immediate passthrough; entry deleted |
| Bootstrap LoadChats fails | log Info (error is often the official list-complete), retry caller still runs |
| TDLib message text drifts | matcher returns false → silent passthrough (no panic, no false positive) |

### 5. Good/Base/Bad Cases

- Good: new cold-DB pull wrapper routed through
  `withChatReady(req.ChatId, "context string", closure)` — the closure
  preserves the `%w` context string and the caller-facing raw signature.
- Base: non-covered methods (EditMessageText, DeleteMessages, TranslateText,
  cmd/stand management calls) stay plain passthrough wrappers — they are
  caller-validated chat IDs, not cold-DB reads.
- Bad: warming up at startup (structurally blind to ruleset hot-reload and
  to the facade-servers-before-TDLib race); retrying immediately without a
  wait window (the single-retry pre-wait-ready design lost the race against
  server-side pagination in production); adding a wrapper without the
  `.Maybe()`/shared-instance discipline in tests; matching the error message
  with `==` instead of `EqualFold` (TDLib text drift); letting the
  `updateNewChat` edge close entries across session tables (silently kills
  the no-false-positive guarantee).

### 6. Tests Required

- `internal/infra/telegram/warmup_test.go` (mockery `ClientAdapter` +
  `testing/synctest` for virtual time): hot path (fn closes certificate),
  fast path (closed entry → one fn), cold miss → edge release, cold miss →
  tick drives LoadChats → retry, deadline → `ChatNotReadyError` (Drives +
  `Unwrap`), mid-wait non-matching passthrough, ctx cancel, subscription
  registered before the first drive, signal flood non-blocking, session-swap
  regression (stale certificate not honored; fresh certification; mid-wait
  re-anchor), concurrent misses → exactly one bootstrap, rate-limit window,
  per-wrapper table-driven coverage of all 9 methods (shared error instance
  for `ErrorIs`).
- `internal/transport/grpc/transport_test.go`: `ChatNotReadyError` →
  Unavailable + RetryInfo detail; wrapped typed error still mapped; other
  errors stay Internal.
- `go vet ./...`, `go test ./internal/infra/telegram/...`,
  `go test -short ./internal/...`, `gofmt -l .` clean (Linux/CI only — see
  the environment gotcha below).

### 7. Wrong vs Correct

#### Wrong

```go
var e *client.ResponseError          // As never matches: buildResponseError returns a VALUE
if errors.As(err, &e) && e.Code == 400 { ... }

assert.ErrorIs(t, err, chatNotFoundErr())  // fresh *Error allocation: pointer compare fails

// Swapping the readiness table at AuthorizationStateReady — the adapter
// already points at the new client a full handshake earlier; stale
// certificates would bless calls into the new cold client.
case *client.AuthorizationStateReady:
    r.swapSessionTable()
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

// runAuthLoop — session boundary is the adapter overwrite, same critical section:
r.swapSessionTable()
r.clientAdapter = tdlibClient
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
