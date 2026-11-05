# Progress — 11-04 bot-menu-grpc

## Loop goal

Deliver bot **chat menu interaction** over gRPC per `design.md` (option A2, user account),
passing a design gate and an independent architecture review:

- 5 additive RPCs on `FacadeGRPC`: `ResolveBot`, `GetBotInfo`, `GetInlineMenu`,
  `ClickCallbackButton`, `SendBotCommand`;
- no bot token, no second session, no new dependency, no ruleset change;
- **no reply fetching/polling/stream** (owner non-goal);
- Web App menus out of scope;
- an independent checker reports no blocking finding.

## Controller

- mode: state-only; state file: this file
- single writer: one Implementer at a time
- checker: separate Paseo agent, sees only the diff + gates
- stop: all phases ✅, or the same failure signature 3× with no strategy change
- worktree: `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc` (branch `feat/bot-menu-grpc`)
- base commit: `609f181`

## Phase plan

```xml
<phases>
  <phase id="P1" owner="design-reviewer" files="review/design-findings.json">
    <goal>Design gate: is design.md lockable as the single ruler for implementation + review?</goal>
    <gate>verdict PASS with D1-D8 met and no blocking finding.</gate>
  </phase>
  <phase id="P2" owner="implementer" files="internal/**, proto, docs">
    <goal>Implement design.md §4-§6 within its non-goals.</goal>
    <gate>gofmt + focused build/tests on touched scope; existing 10 RPCs unchanged.</gate>
  </phase>
  <phase id="P3" owner="reviewer" files="review/findings.json">
    <goal>Independent architecture/contract review of `git diff 609f181`.</goal>
    <gate>verdict PASS with no blocking finding.</gate>
  </phase>
  <phase id="P4" owner="main" files="task artifacts + branch">
    <goal>Commit on the branch; prepare PR; owner gate for merge/publish.</goal>
    <gate>branch commit + CI evidence; merge only on explicit owner go-ahead.</gate>
  </phase>
</phases>
```

## Status

| Phase | Owner | Gate | Status |
| --- | --- | --- | --- |
| P0 setup | main | worktree + task artifacts copied, base 609f181 | ✅ |
| P1 design gate | design-reviewer | D1-D8 met, PASS | ✅ |
| P2 implement | implementer | scoped checks green | ✅ |
| P3 independent review | reviewer | no blocking finding | ✅ r1 FAIL (B1–B4) → r2 PASS |
| P4 closeout | main | branch commit + owner gate | ⏳ |

## Evidence

- P0: worktree `wks_a4023f8cada85504` at `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc`,
  branch `feat/bot-menu-grpc` off `main` @ `609f181`; task artifacts copied in.
- P1: Design Reviewer round 1 FAIL (G1: flood-wait layer/RetryInfo source gap);
  Design Reviser round 1 fixed §6/§7/§8 (infra `FloodWaitError` + exported helper reusing
  `parseFloodWait`, `N s + 500ms`, transport-only `errors.As` mapping, `retry after 7` → 7.5s
  test) and tagged the four assumptions. Round 2 PASS, D1–D8 met, no blocking finding.
  `approval.md` written; `design_sha256 =
  ad40f183a7b45f3f251cf732849e45fa97b6030076f06007bbb7540b5f06ea17`.
  Reviewer also caught a citation nit (`repo.go:311` → `:309`), corrected before lock.
- P2: implemented per design §4–§6 + §8 tests + §9/§11. Orchestrator re-ran scoped
  build/vet/test/gofmt (green); found and had the implementer fix one design-§7 miss
  (unused `callbackAnswerTimeout` — now wired as a server-side deadline, with a
  non-sleeping DeadlineExceeded contract test). P3 review started.
- P3 round 1: FAIL, 4 blocking findings — B1 username-not-found not mapped to NotFound;
  B2 flood conversion missing on GetInlineMenu/SendBotCommand repo paths; B3 the 10s
  callback deadline is not enforceable (go-tdlib blocks with its own 60s timeout; test
  proved only the mock); B4 GetBotInfo returns OK for a non-bot id. Dismissed: O2/O3/O4/
  O5/O6/O8/O9; non-blocking follow-ups recorded in findings.json. Implementer round 2
  dispatched with minimal fixes (typed BotNotFoundError, infra flood wrapping on the four
  shared wrappers, handler-side select with synctest test, nil BotInfo → FailedPrecondition).
- P3 round 2 fixes (implementer): B1 `BotNotFoundError` on SearchPublicChat (mapped NotFound);
  B2 flood conversion added to GetMessage/GetChatHistory/SendMessage/SendBotStartMessage
  (SendMessageAndWait semantics preserved; warmup 429 assertion intentionally strengthened);
  B3 handler goroutine+select enforces the 10s deadline, both branches prefer
  DeadlineExceeded/Canceled over a late result (race fix after orchestrator review);
  B4 nil BotInfo → TargetNotBotError → FailedPrecondition. Orchestrator re-ran scoped
  build/vet/test/gofmt — green; callback tests `-count=20` green.
- P3 round 2 result: **PASS**, round 2, blocking_findings = []; B1–B4 all resolved.
  Reviewer independently re-ran scoped tests/vet plus `-race` on the callback deadline test.
  Non-blocking round-1 follow-ups (O2/O3/O5/O9 hardening, doc claims) recorded in the
  round-1 findings copy; O10 doc claims were fixed. Merge/publish remains owner-gated.
- P2 environment: local TDLib is available for verification without building it — Debian
  `libtd-dev`/`libtdjson1.8.38`/`libsqlcipher1` extracted under `/home/agent/.local/tdlib`
  (headers + shared libs), linker shims in `/home/agent/.local/tdlib-shims`; build/test with
  `-tags libtdjson` + the env block in `review/implementer-prompt.md`. Baseline
  `go test ./internal/transport/grpc/ ./internal/infra/telegram/` is green. Proto toolchain
  installed user-locally (protoc 34.2 → `v7.34.2`, protoc-gen-go v1.36.11,
  protoc-gen-go-grpc v1.5.1); regenerating the unchanged proto yields byte-identical bodies.

## Failure log

| phase | signature | classification | action |
| --- | --- | --- | --- |
| — | — | — | — |

---

## Prior context (research phase, main checkout)

- Semantics confirmed by owner 2026-11-04: a **user account** interacts with a bot's menu
  inside that bot's private chat (option A2). No bot token / second session / Bot API.
- `不取信息 = 不轮询 chat 中的回复` — no reply-fetch RPC, no stream.
- Full research: `research/bot-menu-grpc.md`; design: `design.md`; notes: `prd.md`.
