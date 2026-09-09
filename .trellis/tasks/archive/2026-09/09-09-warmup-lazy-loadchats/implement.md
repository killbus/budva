# Implement: TDLib chat warmup — lazy LoadChats-on-miss

## Ordered checklist

### 1. `internal/infra/telegram/warmup.go` (new)
- [ ] `isChatNotFound(err error) bool` — `errors.As` into `*client.ResponseError`, match `Code == 400 && strings.EqualFold(Message, "Chat not found")`.
- [ ] Warmup state fields on `*Repo`: `warmMu sync.Mutex`, `warmInFlight bool`, `warmCond *sync.Cond` (or channel-based signal), `lastWarm time.Time`.
- [ ] `warmChatsOnce()` — guarded bootstrap: loop `LoadChats(Limit: 100)` until error, iteration bound 10; in-flight dedup (late callers wait for completion); min-interval guard `warmMinInterval = 30 * time.Second`; log trigger at Info, load error at Warn.
- [ ] `(r *Repo) warmOnChatNotFound(err error, method string, chatID int64) bool` — true if `isChatNotFound(err)`; triggers (or waits on) `warmChatsOnce`; returns true so the caller performs its single retry. Rate-limited triggers also return true (retry is cheap; the guard only prevents LoadChats storms).

### 2. `internal/infra/telegram/client_adapter.go` — wire 9 wrappers
Each wrapper gains the miss-retry shape (pattern below shown once; apply identically, preserving the existing `%w` context strings):
- [ ] `ForwardMessages` — retry extracts chat ID from `req.ChatId`
- [ ] `SendMessage` — `req.ChatId`
- [ ] `SendMessageAlbum` — `req.ChatId`
- [ ] `GetMessage` — `req.ChatId`
- [ ] `GetMessages` — `req.ChatId`
- [ ] `GetChatHistory` — `req.ChatId`
- [ ] `GetMessageLink` — `req.ChatId`
- [ ] `GetMessageLinkInfo` — no chat ID in request (parses a URL); pass `0` for logging
- [ ] `GetChat` — `req.ChatId`

```go
msg, err := r.clientAdapter.GetMessage(req)
if err != nil && r.warmOnChatNotFound(err, "GetMessage", req.ChatId) {
    msg, err = r.clientAdapter.GetMessage(req)
}
if err != nil {
    return nil, fmt.Errorf("get message: %w", err)
}
return msg, nil
```

Note: `SendMessageAlbum`'s existing wrapper is the only one without an early-return error shape (it currently returns via `fmt.Errorf` immediately) — keep its context string `send message album: %w` when reshaping.

### 3. Unit tests `internal/infra/telegram/warmup_test.go`
- [ ] miss → LoadChats ×1 → retry ok
- [ ] miss → LoadChats ×1 → retry fails → wrapped error
- [ ] non-matching errors (400 other message / 429) → no LoadChats, passthrough
- [ ] concurrent misses (10 goroutines) → LoadChats ×1, all succeed
- [ ] second miss within interval → no second LoadChats, retry still attempted
- [ ] table-driven per-method coverage: all 9 wrappers exhibit miss→retry (ForwardMessages, SendMessage, SendMessageAlbum, GetMessage, GetMessages, GetChatHistory, GetMessageLink, GetMessageLinkInfo, GetChat)
- [ ] min-interval uses a package-level variable or injected clock so tests can shrink it (no real sleeps beyond milliseconds)

### 4. Validation gates
- [ ] `go vet ./...`
- [ ] `go test ./internal/infra/telegram/...`
- [ ] `go test -short ./internal/...` (ensure no regressions elsewhere; wrappers are used across packages — check handler/facade tests still green since mock call-count expectations may change if any existing test stubs these Repo methods via the embedded interface)
- [ ] `gofmt -l .` clean

### 5. Review gates
- [ ] Confirm no caller changes: `git diff --stat` touches only `internal/infra/telegram/*`
- [ ] Confirm `mocks/client_adapter.go` untouched (mocks stub the inner interface; wrapper logic is the unit under test)

### 6. Manual smoke (optional but recommended — this is the original incident)
- [ ] Point `.env` at a scratch `TELEGRAM_DATABASE_DIR`, run `go run ./cmd/facade`, authorize, then gRPC `GetChatHistory` on a member chat → messages, not `Chat not found`.
- [ ] Watch logs for `TDLib chat warmup triggered` on first miss.

## Rollback points

- After step 1+2: one-shot revert of `warmup.go` + `client_adapter.go` restores passthrough behavior. No state, no config, no callers touched.
- Tests in step 3 are additive; reverting the source requires reverting the test file with it (single commit granularity).

## Out of scope reminders (do NOT drift into)

- No FLOOD_WAIT retry for ForwardMessages (orthogonal; separate task).
- No ruleset-activation validation lint (Phase 2).
- No gRPC/GraphQL/terminal API additions.
- No changes to `cmd/`, `internal/app/handler/`, `internal/app/facade/`.
