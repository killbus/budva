# Implement: TDLib chat warmup — wait-for-ready

Ordered checklist. Build/test commands run on Linux (repo cannot build
go-tdlib on Windows — see telegram-tdlib-guidelines.md environment gotcha);
from Windows use Docker or defer to CI.

## Pre-flight

- [ ] P1. Verify `google.golang.org/genproto` errdetails availability:
      `grep -r "rpc/errdetails" go.sum` (needed for RetryInfo in 2.5).
      If absent, add dep or fall back to plain Unavailable (design 2.5
      documents both).

## Implementation (ordered)

- [ ] I1. `internal/infra/telegram/warmup.go`:
      - `readyEntry` + `sessionReadiness` (mutable map + mutex, wrapped in
        an immutable struct) + `Repo.readiness atomic.Pointer[sessionReadiness]`
        (lazy init in initWarmState).
      - `getOrInsertReady(chatID)` / `signalChatReady(chatID)` /
        `closeReadyOnTimeout` — all through the CURRENT table.
      - `withReady(ctx, chatID, fn)` state machine per design 2.2 (select
        over ready/ticker/deadline; deadline from entry; drives counter;
        typed error on timeout; **every wake re-lookups from the current
        table** — release = closed AND in current table).
      - `ChatNotReadyError` (Error/Unwrap) — package `telegram`; transport
        errors.As it.
- [ ] I1b. `internal/infra/telegram/repo.go` runAuthLoop: at the
      clientAdapter overwrite (:324) — same critical section: swap in a
      fresh `sessionReadiness` table FIRST, then overwrite the adapter, then
      `close(r.clientDone)`. Log session-generation + invalidated-entry
      count (SRE observability).
- [ ] I2. `internal/config/config.go`: `WarmupDeadline` (default 5s),
      `WarmupTicker` (default 500ms) on TelegramConfig.
- [ ] I3. `internal/infra/telegram/repo.go` listenUpdates: add
      `*client.UpdateNewChat` case BEFORE `isRelevantUpdate` check →
      `signalChatReady(u.Chat.Id)`; continue (never forwarded). Adjust
      isRelevantUpdate docs (updateNewChat intentionally consumed here).
- [ ] I4. `internal/infra/telegram/client_adapter.go`: rewrite the 9 covered
      wrappers to withReady closure form (design 2.6). Delete
      `warmOnChatNotFound`. Keep `%w` context strings byte-identical.
- [ ] I5. Observability:
      - miss log with real chat_id inside withReady (one log per wait-loop
        entry, not per fn retry — avoid spam).
      - LoadChats natural-termination log Warn→Info (warmup.go:132 area,
        now inside warmChatsOnce).
      - counters/histogram per design 2.7 (stdlib; no new deps).
      - `updates_backlog_depth` gauge in repo.go listenUpdates (len(r.updates)
        sampled on each receive).
- [ ] I6. `internal/transport/grpc/transport.go`: errors.As →
      Unavailable (+RetryInfo per P1 outcome). Single helper function for
      the mapping, used by all 10 handlers.

## Tests (ordered, same discipline as existing: mockery + synctest)

- [ ] T1. warmup_test.go rework:
      - withReady fast path: already-closed entry → fn once.
      - cold miss → edge fires (simulate via signalChatReady from test) →
        fn retried → success; entry stays closed for a second waiter (O(1)
        hit, fn called once more — expected).
      - cold miss → no edge → ticker drives warmChatsOnce (mock counts) →
        deadline → ChatNotReadyError with Drives count + LastErr unwrap.
      - non-Chat-not-found error mid-wait → immediate passthrough, entry
        deleted (second call does not inherit stale entry).
      - ordering: subscription registered before LoadChats mock is invoked
        (mock Run hook asserts entry exists).
      - non-blocking: flood 300 signalChatReady calls → no deadlock (loop
        completes).
      - session swap regression (SRE round 7): certificate issued under
        client A → table swapped at adapter overwrite → assert old
        certificate misses (waiter re-waits via new table). Guards against
        a future refactor reusing the map.
      - keep isChatNotFound table tests as-is.
- [ ] T2. Wrapper table tests (TestAllWrappers_*): adapt to withReady
      behavior — miss → (edge or tick) → retry success; keep context-string
      assertions and shared-error-instance discipline (AC5).
- [ ] T3. transport_test.go: typed error → codes.Unavailable (+RetryInfo
      detail if P1 green); other errors still codes.Internal (AC6).
- [ ] T4. Metrics smoke: outcome counters incremented on edge/tick/timeout
      paths (AC8).
- [ ] T5. `internal/test/support/live_stack.go` callers compile (no signature
      change — wrappers keep raw go-tdlib signatures).

## Validation commands

```bash
gofmt -l .
go vet ./...
go test ./internal/infra/telegram/...
go test -short ./internal/...
go build ./...
```

(Linux/Docker only; the CI workflow covers this on push.)

## Risky files / rollback points

- `client_adapter.go` — touches all 9 wrappers; highest blast radius. Commit
  after I4 with tests T2 passing before touching transport.
- `repo.go` listenUpdates — update-pipeline hot path; keep the new case
  above the whitelist exactly; run the handler/service update tests.
- Rollback: git revert restores v0.1.1 semantics (design 5).

## Pre-start checklist

- [ ] implement.jsonl / check.jsonl curated (at least one real entry each).
- [ ] Spec update (Phase 3.3): telegram-tdlib-guidelines.md chat-not-found
      contract section → wait-for-ready semantics + new error matrix rows
      (typed error, Unavailable mapping, metrics contract) + session-boundary
      clause: certificates are per-TDLib-session; table swaps at the
      clientAdapter overwrite; safety premise = entry.ready closes only by
      holder's fn success, edge dispatch acts on current table only.
- [ ] User review of prd/design/implement, then task.py start.
