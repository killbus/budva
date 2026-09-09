# PRD: TDLib chat warmup — lazy LoadChats-on-miss + single retry

## Problem

TDLib does not load the full chat list after authorization (`authorizationStateReady` only pushes sparse `updateChatLastMessage`/`updateChatOrder`). Any **pull-based** call on a chat that is not yet in the local TDLib database fails with `400 Chat not found`.

budva has two affected surfaces:

1. **facade** (pull-only API gateway) — every gRPC/GraphQL read or write (`GetMessage`, `GetChatHistory`, `SendMessage`, ...) fails on a cold DB. **Observed in production**: user logged in, gRPC `GetMessage` returned `Internal: get message: 400 Chat not found`.
2. **engine** (push input / pull output) — receiving is unaffected (`updateNewMessage` carries the full Message; source matching is an in-memory map). The **output** side (`ForwardMessages`/`SendMessage` on `dstChatID`) fails on a cold DB, and forwards have **no retry**: the message is silently dropped with one error log line.

Four concrete failure forms, all the same root cause:

| Form | Trigger | Today's result |
|---|---|---|
| Cold start (facade) | fresh volume + any API call | 400 Chat not found (observed) |
| Cold start (engine) | fresh volume + first forward to a silent destination | message dropped, one log line |
| Ruleset hot-reload | new silent destination added via ruleset edit, no restart | message dropped — startup warmup cannot cover this by construction |
| Pure-output channel deadlock | destination whose only writer is engine itself (mirror/archive channel) | first forward fails → chat never enters DB → **every** forward fails forever; restart does not help (DB persists empty); no process-external action can break the cycle (facade has a separate DB) |

## Decision (from adversarial two-camp brainstorm, converged)

- **Startup-only warmup is rejected** by both camps: it is structurally blind to ruleset hot-reload, and the facade HTTP/gRPC servers start before TDLib is ready (race window even at boot).
- The fix is **lazy LoadChats-on-miss + single retry**, implemented as **Repo-level method overrides** in `internal/infra/telegram` (the layer both processes already share). Zero changes to callers: engine handler, facade service, and the check/other forward bypasses (`internal/app/handler/service.go:314,327`) all inherit automatically.
- Ruleset-activation validation lint (async per-destination `GetChat` probe, log-only) is **deferred to Phase 2** (separate decision, not in this task).
- Forward-time FLOOD_WAIT retry for `ForwardMessages` is **orthogonal** and out of scope.

## Requirements

1. When a covered Repo method fails with TDLib `400 Chat not found`, the Repo must:
   - trigger a LoadChats bootstrap (load-until-error loop, per official tdjson example semantics),
   - retry the original call exactly **once**,
   - return the final result (success or the original/retried error).
2. The bootstrap must be de-duplicated: concurrent misses trigger at most one in-flight LoadChats (singleflight), and re-triggers are rate-limited by a minimum interval so a genuinely-nonexistent chat does not cause a LoadChats storm.
3. Non-Chat-not-found errors must pass through unchanged — no warmup, no retry.
4. Covered methods (all pull-based or output calls reachable with a caller-supplied chat ID):
   - Output: `ForwardMessages`, `SendMessage`, `SendMessageAlbum`
   - Facade reads: `GetMessage`, `GetMessages`, `GetChatHistory`, `GetMessageLink`, `GetMessageLinkInfo`
   - Transform path: `GetChat`
5. Error matching must not silently break on TDLib message-text drift: match `client.ResponseError` with code 400 and "Chat not found" (case-insensitive). If a ResponseError cannot be matched, degrade to a log line (no warmup), never panic.
6. No caller-side changes: `cmd/facade/main.go`, `cmd/engine/main.go`, `internal/app/handler/service.go`, `internal/app/facade/service.go` are untouched by Phase 1.
7. Warmup side effects must be observable: a log line (info) when a LoadChats bootstrap is triggered, including which method/chat hit the miss.

## Non-goals

- Startup / ruleset-activation validation lint (Phase 2 candidate).
- FLOOD_WAIT retry for forwards (separate orthogonal task; pattern exists at `internal/infra/telegram/repo.go:157-189`).
- Any gRPC/GraphQL API surface additions (e.g. a warmup RPC or terminal command).
- Retrying the retried call more than once.

## Acceptance criteria

- [ ] Unit tests with the existing `internal/infra/telegram/mocks/client_adapter.go` verify: miss → LoadChats → retry-once → success; miss → LoadChats → retry fails → error returned; non-matching error → no LoadChats, no retry; concurrent misses → exactly one LoadChats.
- [ ] Rate limiting: a second miss within the minimum interval does not trigger a second LoadChats (test with injected clock or short interval).
- [ ] `go test ./internal/infra/telegram/...` and `go vet ./...` pass.
- [ ] All 9 covered methods route through the warmup wrapper (verified by test or by structural assertion).
- [ ] No changes under `cmd/`, `internal/app/handler/`, `internal/app/facade/`.
- [ ] Live verification (manual, this task's smoke): fresh TDLib database directory + facade gRPC `GetChatHistory` on a member chat returns messages instead of `Chat not found`.

## References

- Convergence record: `.trellis/workspace/killbus/warmup-convergence.md`
- Brainstorm channel: `brainstorm-chat-warmup` (trellis channel, project scope)
- Precedent in repo: FLOOD_WAIT retry composite op — `internal/infra/telegram/repo.go:157-189` (`SendMessageAndWait`)
- BDD warmup precedent: `internal/test/support/live_stack.go:128-131`
