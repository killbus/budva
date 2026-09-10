# TDLib chat warmup: wait-for-ready with lazy subscription

## Goal

On a cold TDLib database, the first pull call for a chat (e.g. `GetChatHistory`)
fails with `400 Chat not found`. The current fix (v0.1.1, commit 070e2fe)
bootstraps `LoadChats` and retries **immediately, once** — but production
 showed TDLib's async server-side dialog pagination is still incomplete when
the retry fires (~2.5 s after bootstrap start), so the retry also fails with
400 and only a later external request succeeds.

Replace the single immediate retry with a **wait-for-ready** mechanism:
on a chat-not-found miss, wait (bounded) until TDLib has actually loaded the
dialog, retrying the original call only when the dialog is ready. The design
was converged through a 5-round expert review (chatroom, 2026-09-10) grounded
in TDLib source (pin 22d49d5); see design.md for the verified facts.

## Background (production incident, 2026-09-10)

- Fresh TDLib DB, first external `GetChatHistory` request: 400 Chat not found;
  warmup log shows `chat_id: 0` (observability bug); LoadChats bootstrap ran;
  immediate retry failed; gRPC duration 2.625 s; second request seconds later
  succeeded (50 messages).
- v0.1.1 (070e2fe) warmup code is byte-identical to HEAD — no drift; the
  `chat_id: 0` in logs is inconsistent with HEAD code (`client_adapter.go:217`
  passes `req.ChatId`) **unless** the request entered with an unset chat id —
  root cause of the log value to be confirmed during implementation (the gRPC
  request in the incident carried chat_id = -1001164706519, so the mismatch
  needs a code-path explanation: candidates are log-emission ordering, or the
  error surfacing from a wrapper other than GetChatHistory).

## Verified TDLib source facts (design constraints, full evidence in design.md)

1. `updateNewChat` is emitted from `add_new_dialog` (MessagesManager.cpp
   35211-35223) exactly when the dialog enters the in-memory map AND the
   failed-to-load negative cache is erased — i.e. the edge precisely aligns
   with "subsequent pull calls succeed".
2. The negative cache short-circuits ALL pull calls (36260-36263) until the
   dialog is brought back by LoadChats pagination or an incoming message
   (on_get_message → add_dialog_for_new_message:13933 → add_new_dialog — same
   edge). "Probe once, then wait" cannot succeed while the negative cache
   holds the chat; the miss itself must drive LoadChats.
3. `GetChat` on a cold DB fails identically to `GetChatHistory` (no server
   fetch on the non-bot path) — a separate "membership oracle" call collapses
   into retrying `fn` itself.
4. Cold-start LoadChats emits ~1 updateNewChat per dialog (~200 for the
   production account); all of them are filtered out by `isRelevantUpdate`
   before `r.updates` (repo.go:367-371) and cannot flood it.

## Requirements

### R1 — wait-for-ready instead of single immediate retry

- On `400 Chat not found` in any of the 9 covered wrappers
  (client_adapter.go: SendMessage, SendMessageAlbum, ForwardMessages,
  GetMessage, GetMessages, GetChatHistory, GetMessageLink,
  GetMessageLinkInfo, GetChat):
  - establish a lazy per-chat readiness subscription **before** triggering
    the LoadChats bootstrap (subscription must precede the drive, or the
    edge can fire before the waiter exists);
  - drive `warmChatsOnce()` (existing singleflight + 30 s rate limit,
    unchanged);
  - wait for readiness via a `select` over: the ready channel, a retry
    ticker, and the deadline; on each wake-up, retry the original call
    (the call itself is the only readiness oracle — see fact 3);
  - on success, persist readiness (close the ready channel — a closed
    channel is a permanent level signal for later waiters);
  - on deadline expiry, return a typed "chat not ready" error that wraps the
    last TDLib error and reports how many LoadChats drives were made.

- Readiness subscription hooks into `listenUpdates` **before** the
  `isRelevantUpdate` whitelist (repo.go:367) and uses a **non-blocking**
  send/close (never block the TDLib receiver pump — go-tdlib v0.7.6
  receiver() backpressure topology, client.go:72-101).

- The ready channel for a chat must be **monotonic within the TDLib session**:
  once closed, stays closed (matches TDLib semantics — a dialog that entered
  the map does not leave it during the session).

### R2 — typed error at the gRPC boundary

- The new typed error must surface as gRPC `Unavailable` (with `RetryInfo` /
  backoff metadata) from `internal/transport/grpc/transport.go`, not
  `codes.Internal` as today. Fast-fail before any internal retry storm;
  clients can retry per standard gRPC semantics.

### R3 — observability fixes (bundled)

- Warmup miss log must report the real `chat_id` of the failing request
  (incident showed `chat_id: 0`).
- LoadChats loop natural-termination log (`404 / "chat list is empty"`) is
  official TDLib list-complete semantics — downgrade from Warn to Info
  (warmup.go:132).

### R4 — pre-launch observability (SRE minimal set, from expert review)

- withReady outcome distribution counter: edge-hit / ticker-hit / timeout
  (timeout rate is the single hard alert; >2% threshold as drift detector;
  timeout metric carries a phase label distinguishing bootstrap-slow from
  pagination-slow instead of two-phase timing).
- Cold-start convergence latency: time from first updateNewChat to the last
  one (compare against the ~200-dialog expectation).
- LoadChats pagination error count.
- `r.updates` backlog depth (cap 100) — the sentinel for the real
  backpressure risk window (whitelist-update flood × slow handleUpdate
  consumer), which is decoupled from dialog-load floods.

### R5 — contract updates

- Update `.trellis/spec/backend/telegram-tdlib-guidelines.md` chat-not-found
  retry contract section to the wait-for-ready semantics.

## Constraints

- go-tdlib v0.7.6 pinned; single shared listener (pendingSends pattern,
  repo.go:52-56) — no per-call GetListener()/Close() (v0.7.6 Listener.Close()
  race).
- The repo cannot build go-tdlib on Windows (cgo; see spec): go vet / go test
  for telegram packages run on Linux/Docker only.
- Existing tests use mockery mocks + testing/synctest virtual time; new tests
  must follow the same discipline (shared error instances for
  `errors.Is`; `.Maybe()` discipline in wrapper tables).
- `warmChatsOnce` singleflight + 30 s `warmMinInterval` semantics stay
  unchanged (they are correct and tested).

## Acceptance Criteria

- [ ] AC1: On a mocked cold DB (first call → 400, ready edge fires, later
      calls succeed), a `GetChatHistory` call ultimately succeeds within one
      wait window without a second external request.
- [ ] AC2: On a mocked permanently-missing chat (edge never fires, fn keeps
      failing), the call fails with the typed not-ready error after the
      static deadline; the error is mappable to gRPC Unavailable.
- [ ] AC3: Subscription ordering — the ready channel is registered before
      `warmChatsOnce()` is invoked (unit-testable via mock ordering).
- [ ] AC4: Non-blocking dispatch — an updateNewChat delivery to the ready
      map never blocks (no unbuffered sends into waiter channels; close-only
      signaling), verified by test.
- [ ] AC5: All 9 covered wrappers keep their `%w` context strings and
      pass-through behavior for non-matching errors (existing table tests
      still pass, adapted).
- [ ] AC6: gRPC transport maps the typed error to `codes.Unavailable` with
      RetryInfo; other errors remain `codes.Internal` (transport_test.go
      extended).
- [ ] AC7: Warmup miss log carries the actual chat_id; LoadChats
      natural-termination log level is Info.
- [ ] AC8: Metrics/counters for R4 exist and are populated in the wait path.
- [ ] AC9: `go vet ./...`, `go test ./internal/infra/telegram/...`,
      `go test -short ./internal/...`, `gofmt -l .` clean (Linux).

## Out of scope

- Adaptive deadlines (killed in expert review, round 3: 4/4 for static).
- Per-chat miss counters / readiness coarse gates (killed: round 4).
- A separate GetChat membership oracle call (collapsed into retry, fact 3).
- Alternative subscription mechanisms (updateChatList etc.) — updateNewChat
  is the exact, TDLib-authority-backed edge.
- Production A/B for the verbosity-4 hypothesis (separate operational task;
  logs live on the server, not local).
- FLOOD_WAIT retry for ForwardMessages (separate orthogonal task, per
  warmup-convergence.md).

## Key decisions (resolved)

- D1 (scope, user-approved): withReady covers **all 9 covered wrappers** —
  read and write paths; engine pure-output channel self-healing depends on
  the write path being covered.
- D2 (deadline, chatroom round 6, 4/4): **5 s, single clock starting at
  withReady entry** (includes the synchronous ~2.5 s bootstrap). Delivered
  as a config item (envconfig `TELEGRAM_WARMUP_DEADLINE`, default 5s), not
  a code constant — account size drifts, tuning must not wait for a release.
  Retry ticker 500 ms (tick ≪ deadline; a missed edge is caught by the next
  tick within 0.5 s). Timeout-rate >2% is the drift-detector alert; two-phase
  timing explicitly rejected (false precision; pagination has no bounded
  upper latency — the liveness boundary belongs to the whole request).
