# Design: TDLib chat warmup — wait-for-ready with lazy subscription

Expert-chatroom-converged design (6 rounds, 2026-09-10). All TDLib facts
verified against production pin 22d49d5 (local clone `D:\tmp-go-cache\td-src`)
and go-tdlib v0.7.6 module cache.

## 1. Verified source facts (the design's load-bearing evidence)

| # | Fact | Evidence |
|---|------|----------|
| F1 | `updateNewChat` is emitted from `add_new_dialog` exactly when the dialog enters `dialogs_` map AND the negative cache is erased — the edge precisely aligns with "subsequent pull calls succeed" | MessagesManager.cpp:35211-35223 (erase at 35218, send at 35223); TDLib treats object-visible-before-update as internal ERROR (19719-19729) |
| F2 | Negative cache (`failed_to_load_dialogs_`) short-circuits ALL pull calls (no db query) until the dialog is brought back | get_dialog_force:36251-36276; sole erase point 35218 |
| F3 | Load sources for the negative-cache erase are exactly two: LoadChats server pagination (on_get_dialogs:14779-14781 → add_dialog → add_new_dialog) and incoming messages (on_get_message:13933 → add_dialog_for_new_message:35006 → add_dialog → add_new_dialog); db-hit path (35024-35028) also routes through add_new_dialog — **the edge is the unified exit of every load path** | MessagesManager.cpp as cited |
| F4 | `GetChat` on a cold DB fails identically to `GetChatHistory` for non-bot accounts (no server fetch; immediate 400) — a separate membership oracle collapses into retrying the call itself | Requests.cpp:317-339 (GetChatRequest, set_tries(3)); load_dialog:15725-15789 (non-bot: immediate `400 Chat not found` at 15783) |
| F5 | Cold-start LoadChats emits ~1 updateNewChat per dialog (~200 production); all filtered by `isRelevantUpdate` before `r.updates` — cannot flood the cap-100 channel | isRelevantUpdate whitelist repo.go:410-421 |
| F6 | go-tdlib v0.7.6 receiver() backpressure: `listener.Updates <- typ` is an unbuffered blocking send (client.go:89); one slow consumer stalls ALL response dispatch → 60 s catchTimeout | client.go:72-101, listener.go (unbuffered `Updates chan Type`) |

## 2. Architecture

### 2.1 Core: per-chat readiness entry with close-persistent certificate

```go
// warmup.go (new file section)
type readyEntry struct {
    ready chan struct{} // closed exactly once = "dialog materialized in this session"
}

// sessionReadiness — certificates valid for ONE TDLib session. Swapped
// atomically at the clientAdapter overwrite (repo.go:324); see R-risk-2.
type sessionReadiness struct {
    mu    sync.Mutex
    ready map[int64]*readyEntry
}
```

`Repo` gains:

```go
readiness atomic.Pointer[sessionReadiness] // swapped per TDLib session
```

**Certificate semantics** (Kleppmann, round 5): the map caches "materialized
in this TDLib session" certificates, not truth. A closed channel is a
permanent level signal — later waiters observe it O(1); missed edges cost
nothing because close persists. Session boundary = the clientAdapter
overwrite (repo.go:324): the table is swapped atomically in the same
critical section that installs the new client (see R-risk-2 for the
converged invalidation mechanism).

### 2.2 State machine: withReady (replaces the single immediate retry)

```
withReady(ctx, chatID int64, fn func() error) error:
  entry := getOrInsert(chatID)          // insert BEFORE drive: the edge cannot slip past the subscription
  select {
  case <-entry.ready:                   // certificate holder: O(1) fast path
  default:                              // first wait: probe once via fn
  }

  if err := fn(); err == nil {          // hot path (also the first probe)
      closeReady(entry); return nil
  }
  if !isChatNotFound(err) { return err }   // non-matching → passthrough

  // subscribe first (2.3), then drive (2.4)
  deadline := time.After(cfg.WarmupDeadline)   // 5s default, single clock from entry
  ticker  := time.NewTicker(500 * time.Millisecond)
  drives  := 0
  for {
      select {
      case <-entry.ready:               // edge (or tick-observed level): retry fn
      case <-ticker.C:                  // tick: drive again (rate-limited inside), then retry
          warmChatsOnce(); drives++
      case <-deadline:
          close-and-delete entry (failed close in same critical section)
          return &ChatNotReadyError{ChatID: chatID, Drives: drives, LastErr: err}
      }
      if err = fn(); err == nil { closeReady(entry); return nil }
      if !isChatNotFound(err) { delete(entry); return err }
  }
```

Invariants:
- Correctness rests on exactly one signal: `fn()` succeeding (idempotent
  level). The edge is an accelerator; a lost edge costs one tick (≤500 ms),
  never correctness (Pike, round 5: "通道负责快,电平负责对").
- Non-Chat-not-found errors mid-wait exit immediately (do not mask real
  errors with not-ready semantics).
- The deadline clock starts at withReady entry and includes the synchronous
  bootstrap time inside the first tick handler (single clock, round 6: 4/4;
  two-phase timing rejected as false precision).
- Release condition (round 7): certificate closed AND the entry found in
  the CURRENT session table — every wake re-lookups from the current table
  before proceeding (R-risk-2).

### 2.3 Edge subscription: non-blocking dispatch in listenUpdates

In `listenUpdates` (repo.go:352), BEFORE the `isRelevantUpdate` whitelist:

```go
case *client.UpdateNewChat:
    r.signalChatReady(u.Chat.Id)   // idempotent close; NEVER blocks
    continue                        // still not forwarded to r.updates
```

`signalChatReady` looks up the entry (read-locked map), and on hit does
`close(entry.ready)` guarded by sync.Once-equivalent (close only if not
already closed — use a `closed bool` in the entry under readyMu, or
`select { case <-ch: default: close(ch) }` under the map lock).
**No waiter channel sends ever happen** — close-only signaling structurally
severs the receiver() backpressure loop (F6): the dispatch path in
listenUpdates stays O(1) with zero blocking sends.

Flood arithmetic (F5 + SRE round 5): ~200 updateNewChat on cold start are
absorbed by map lookups + one close each — no r.updates traffic, no pump
stall.

### 2.4 Drive: warmChatsOnce unchanged

Existing singleflight (mutex+cond) + 30 s `warmMinInterval` stay as-is
(correct and tested). Ordering guarantee: subscription (2.3 map entry)
exists before the first `warmChatsOnce()` call inside withReady, so an edge
fired by the bootstrap's own pagination lands in a registered waiter.

Note the bootstrap also runs for OTHER concurrent misses (singleflight
dedups); their withReady loops tick-retry against the same shared map.

### 2.5 Typed error and gRPC mapping

```go
// domain or telegram package
type ChatNotReadyError struct {
    ChatID  int64
    Drives  int         // LoadChats drives performed within the window
    LastErr error       // last TDLib error (400 Chat not found)
}
func (e *ChatNotReadyError) Error() string
func (e *ChatNotReadyError) Unwrap() error { return e.LastErr }
```

gRPC transport (transport.go): errors.As(*telegram.ChatNotReadyError) →
`status.New(codes.Unavailable, ...)` + `WithDetails(&errdetails.RetryInfo{RetryDelay: 2s})`
(google.golang.org/grpc/status; errdetails from
google.golang.org/genproto/googleapis/rpc/errdetails — verify availability in
go.sum; if absent, attach retry delay via metadata `grpc-retry-delay` or
plain Unavailable without details, client still retries). All other errors
remain `codes.Internal` (AC6 keeps current behavior for them).

### 2.6 Wrapper integration (all 9, D1)

Every covered wrapper (client_adapter.go:63-237) changes from
`warmOnChatNotFound + single retry` to:

```go
func (r *Repo) GetChatHistory(req *client.GetChatHistoryRequest) (*client.Messages, error) {
    var msgs *client.Messages
    err := r.withReady(context.Background(), req.ChatId, func() error {
        var e error
        msgs, e = r.clientAdapter.GetChatHistory(req)
        return e
    })
    if err != nil {
        return nil, fmt.Errorf("get chat history: %w", err)
    }
    return msgs, nil
}
```

- ctx: the wrappers currently have no ctx (raw go-tdlib signatures). Use a
  small internal context with the warmup deadline; the caller-facing ctx
  propagation is a larger refactor (out of scope). The withReady select
  still honors the deadline channel; a ctx cancellation path is added via
  `context.WithTimeout` inside withReady so a hung fn cannot outlive the
  window.
- `warmOnChatNotFound` is retired (its dedup/rate-limit logic lives on in
  warmChatsOnce; its logging moves into withReady with the REAL chat id).
- GetMessageLinkInfo has no chat_id (URL-parsed): chatID=0 → map key 0
  (harmless shared entry; document). Log carries 0 — same as today, no
  regression.

### 2.7 Observability (R3 + R4)

Log fixes:
- warmup miss log: real `req.ChatId` (incident showed 0 — root cause TBD in
  implementation; if HEAD code already passes req.ChatId, the 0 came from
  the deployed binary path or another wrapper; verify by reading the exact
  log line in the incident against each wrapper's chatID argument).
- LoadChats natural termination (404/"chat list is empty"): Warn → Info
  (official list-complete semantics, warmup.go:132).

Metrics (stdlib, no new deps — mirror existing service/transform pattern):
- `warmup_outcome_total{outcome=edge|ticker|timeout, phase=bootstrap|pagination}`
  (phase label on timeout only; replaces two-phase timing).
- `warmup_convergence_seconds` histogram: first updateNewChat → last
  updateNewChat per cold start (set in signalChatReady: first close starts
  it, subsequent closes update last).
- `warmup_loadchats_errors_total`.
- `updates_backlog_depth` gauge (cap of r.updates) — the whitelist-flood ×
  slow-consumer sentinel (SRE round 5: the REAL risk window, decoupled from
  dialog floods).
- Alert: warmup timeout rate > 2% (drift detector).

### 2.8 Config

```go
// TelegramConfig
WarmupDeadline time.Duration `envconfig:"TELEGRAM_WARMUP_DEADLINE" default:"5s"`
WarmupTicker   time.Duration `envconfig:"TELEGRAM_WARMUP_TICKER" default:"500ms"` // tests may shorten
```

## 3. Compatibility / migration

- Mock surface unchanged (mocks/client_adapter.go). Wrapper logic is the
  unit under test (spec rule: no warmup logic in mocks).
- App layers (handler, facade) define their own partial interfaces — no
  ripple (verified: their mocks already stub these methods).
- The old behavior "one bootstrap + one immediate retry" is strictly
  subsumed: the first fn call after the drive IS that retry; the difference
  is the bounded wait loop around it.
- Errors returned by the 9 wrappers keep their `%w` context strings
  (AC5); the typed error wraps the last TDLib error, so `errors.Is` on a
  shared instance still works for existing tests.

## 4. Trade-offs (recorded, with the round that decided them)

- close-persistent channel vs subscription map with per-waiter chans:
  close-persistent wins (round 5: monotonic certificate; sever F6 deadlock
  structurally; Kleppmann lifecycle contract).
- Static deadline vs adaptive: static 5s (round 3: 4/4; round 6: 4/4).
- Level-only (Hickey round 5) vs edge+level: edge+level with close-as-level
  — the disagreement dissolved because a closed channel IS a level signal;
  edge = the close act, level = its persistence.
- All 9 wrappers vs read-only: all 9 (D1, user decision — engine
  pure-output self-healing needs the write path).
- Two-phase timing: rejected (round 6: 4/4; phase label on the timeout
  metric instead).

## 5. Rollback

Single-file-ish change (warmup.go + client_adapter.go + repo.go hook +
transport.go mapping + config). Revert commit restores the v0.1.1 behavior
(bootstrap + single retry) since withReady degrades to it when the wait
loop is removed. Config default 5s can be tuned to ~0 behavior-wise (not a
supported mode; rollback is the git revert).

## 6. Risks

- R-risk-1: TDLib error text drift breaks `isChatNotFound` → silent
  passthrough (existing accepted risk, spec'd; unchanged).
- R-risk-2 (RESOLVED, chatroom round 7 — session-boundary invalidation):
  `runAuthLoop` overwrites `r.clientAdapter` (repo.go:324) without
  recreating the Repo — the ready map survives across TDLib sessions and
  stale closed certificates would false-positive on the new session's cold
  DB. Converged mechanism (4/4 for table swap, 4/4 against per-entry
  generation/pointer compares):
  - **Session table**: the ready map is wrapped in an immutable
    `sessionReadiness` struct behind an atomic pointer;
    getOrInsert/signal/close all go through the current table.
  - **Swap point = the clientAdapter overwrite line itself** (repo.go:324
    critical section), NOT AuthorizationStateReady — the adapter takes
    effect at :324, a full handshake before Ready fires; swapping at Ready
    leaves a seconds-wide window where the old table blesses calls into the
    new cold client, and a lost/delayed Ready leaks stale certificates for
    the whole session. Swap table first, then overwrite the adapter, in the
    same critical section (Pike round 7).
  - **Re-lookup on wake**: waiters capture the table pointer at lookup and,
    on every wake, re-lookup from the CURRENT table; the release condition
    is "certificate closed AND entry found in the current table". Stale
    waiters fall back to fn-fail → tick → fresh lookup — no false-ready.
  - **Safety premise (contract clause, must stay explicit)**:
    entry.ready is closed ONLY by the holder's own fn success; the
    updateNewChat edge dispatch acts on the current table only. If a future
    change lets the edge close entries across tables, the no-false-positive
    guarantee silently dies (Kleppmann round 7).
  - Observability: session-generation counter + invalidated-certificate
    count logged at swap (SRE round 7); Ready event may carry the log, but
    no longer owns invalidation.
  - CI regression test: issue certificate under client A → swap adapter
    (triggering table swap) → assert old certificate misses and the waiter
    re-waits (SRE round 7; guards against a future refactor reusing the
    map).
- R-risk-3: GetMessageLinkInfo chat_id=0 shared entry — multiple distinct
  URL chats share one readiness key. Acceptable: correctness unaffected
  (fn is still the oracle), only wait granularity degrades. Document.
