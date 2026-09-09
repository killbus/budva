# Design: TDLib chat warmup — lazy LoadChats-on-miss

## Boundary

One new file: `internal/infra/telegram/warmup.go` (+ its test `warmup_test.go`). The Repo struct already embeds `clientAdapter` (`repo.go:39`); same-name methods defined on `*Repo` naturally shadow the embedded interface — this is the established pattern (e.g. `SendMessage`/`ForwardMessages` wrappers in `client_adapter.go` already call `r.clientAdapter.X`).

**Wait — important design correction:** the current methods in `client_adapter.go` are the *wrappers* (they add the `fmt.Errorf("get message: %w", err)` context). The shadowing target is those wrappers, not the raw embedded interface. Two implementation shapes were considered:

### Option A (chosen): warmup inside the existing wrapper methods

Move the warmup logic into the existing per-method wrappers in `client_adapter.go` (they already exist for all 9 methods). Each wrapper becomes:

```go
func (r *Repo) GetMessage(req *client.GetMessageRequest) (*client.Message, error) {
    msg, err := r.clientAdapter.GetMessage(req)
    if err == nil {
        return msg, nil
    }
    if !r.warmOnChatNotFound(err, "GetMessage") {
        return nil, fmt.Errorf("get message: %w", err)
    }
    // retry once after bootstrap
    msg, err = r.clientAdapter.GetMessage(req)
    if err != nil {
        return nil, fmt.Errorf("get message: %w", err)
    }
    return msg, nil
}
```

- `warmOnChatNotFound` returns true if (a) err is `400 Chat not found` AND (b) a LoadChats bootstrap was actually attempted this call (not rate-limited away). Rate-limited re-triggers still retry once — the rate limit protects against LoadChats storms, not against the retry.
- Pros: no new dispatch layer, error-context wrapping (`"get message: %w"`) preserved at a single site, mock-based tests keep working (mocks stub `clientAdapter`, `Repo` wrapper logic is what we test).
- Cons: touches 9 wrapper bodies — mechanical but repetitive.

### Option B (rejected): generic decorator table

A `map[method]func` wrapper registry in warmup.go, constructors wiring each. Rejected: more indirection, breaks the file's current 1-wrapper-per-method readability, and the error-context strings live at each wrapper anyway.

## Contracts

### `isChatNotFound(err error) bool`

```go
var responseErr *client.ResponseError
if !errors.As(err, &responseErr) {
    return false
}
return responseErr.Code == 400 && strings.EqualFold(responseErr.Message, "Chat not found")
```

- Error chain is preserved: wrappers use `%w`, go-tdlib returns `*client.ResponseError` from TDLib responses.
- If TDLib message text drifts (go-tdlib type.go marks messages "subject to future changes"), matching fails → `false` → no warmup, plain passthrough. Degrades to today's behavior, never panics or misfires.

### `warmChatsOnce(ctx)` — the bootstrap

- Loop `LoadChats(Limit: N)` until it returns an error (official tdjson example semantics: TDLib errors with a "chat list is empty"-family error when the full list is loaded; a fixed large limit per call, loop bounded by e.g. 10 iterations to avoid infinite loops on exotic errors).
- `sync.Once`-style in-flight dedup: `sync.Mutex` + `inFlight bool` + `lastWarm time.Time`.
  - Concurrent callers while a warm is in flight: **wait for it to finish** (so their one retry sees the warmed DB), then return.
  - Min-interval guard (default 30s): if a warm completed less than the interval ago, do not re-run — the caller still retries once (a recent warm already loaded everything a LoadChats can load; persistent miss means "not a member / wrong ID").
- Log on trigger: `slog.Info("TDLib chat warmup triggered", method, chatId where extractable)`.
- Warm errors: log at Warn, still allow the caller's single retry (cheap, and a partial load may have fixed the miss).

### Concurrency / lifetime

- State lives on `*Repo` — one TDLib client per Repo, one Repo per process, so the guards need no cross-process coordination.
- `ctx`: the wrappers currently have no ctx parameter (they mirror go-tdlib signatures). The bootstrap uses `context.Background()`; the LoadChats loop is bounded by iteration count, not ctx deadline. This matches the existing repo layer's ctx posture (see repo.go Close/SendMessageAndWait notes).

## Data flow (engine forward, after change)

```
handler.forwardMessage
  → Repo.ForwardMessages(req)            [client_adapter.go, now warmup-aware]
    → clientAdapter.ForwardMessages      [TDLib]
    ← 400 Chat not found
  → isChatNotFound? yes
  → warmChatsOnce()                       [first miss in 30s window]
    → LoadChats loop until error
  → clientAdapter.ForwardMessages         [single retry]
    ← ok → return messages
```

The same shape covers facade reads and the transform-path `GetChat`.

## Compatibility / rollback

- No caller changes; no API surface changes; no config changes.
- Rollback = revert the one file pair (warmup.go + client_adapter.go edits) — behavior returns to today's passthrough.
- Risk register (from brainstorm, both camps):
  - Error-text drift (mitigated: code+message match, degrade-to-passthrough).
  - LoadChats in the caller's thread blocks a taskQueue worker for the warm duration (sub-second typically; misses are rare; acceptable — flagged in brainstorm by the minimalist camp as the price of the mechanism).
  - go-tdlib LoadChats re-call safety: official example loops it; legacy budva43 had a TODO claiming it can't be called twice — we loop with an iteration bound and log; if the loop misbehaves in practice it is visible in logs, not silent.

## Test strategy

Unit tests in `internal/infra/telegram/warmup_test.go` using `internal/infra/telegram/mocks/client_adapter.go` (mocks already generated for the full interface):

1. Miss → LoadChats called once → retry succeeds → result returned.
2. Miss → LoadChats called once → retry also fails → wrapped error returned.
3. Non-matching error (e.g. code 400 different message, or code 429) → LoadChats NOT called → error passthrough.
4. Concurrent N goroutines miss simultaneously → exactly one LoadChats → all retries succeed.
5. Second miss within min-interval → no new LoadChats → retry still attempted once.
6. Structural coverage: all 9 methods route through warmup (per-method miss test or table-driven).
