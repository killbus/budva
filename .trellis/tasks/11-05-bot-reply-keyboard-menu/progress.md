# Progress — 11-05 bot-reply-keyboard-menu

## Loop goal

让现有 `FacadeGRPC.GetInlineMenu` 增量表达 bot 私聊"输入框下方"的 reply keyboard
（`replyMarkupShowKeyboard`），并修正 `message_id=0` 的消息选择；通过 Design Reviewer
设计关卡与独立 Code Reviewer 审查。不新增 RPC、不执行 Web App、不取 bot 后续回复。

## Controller

- mode: state-only; state file: this file
- single writer: one Implementer at a time
- checker: separate Paseo agent, sees only the diff + gates
- worktree: `/home/agent/.paseo/worktrees/3i5ks3q2/feat-bot-reply-keyboard-menu`
  (branch `feat/bot-reply-keyboard-menu`)
- base commit: `ab172b7`
- paseo workspace: `wks_ca6ab7d39df75ed9`

## Phase plan

```xml
<phases>
  <phase id="P1" owner="design-reviewer" files="review/design-findings.json">
    <goal>Design gate: is design.md lockable as the single ruler for implementation + review?</goal>
    <gate>verdict PASS with D1-D8 met and no blocking finding.</gate>
  </phase>
  <phase id="P2" owner="implementer" files="proto, internal/**, docs">
    <goal>Implement design.md §4-§8, §10, §11 within its non-goals.</goal>
    <gate>gofmt + focused build/vet/test on touched scope; existing behavior unchanged.</gate>
  </phase>
  <phase id="P3" owner="reviewer" files="review/findings.json">
    <goal>Independent architecture/contract review of `git diff ab172b7`.</goal>
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
| P0 setup | main | worktree + task artifacts, base ab172b7 | ✅ |
| P1 design gate | design-reviewer | D1-D8 met, PASS | ✅ r1 PASS |
| P2 implement | implementer | scoped checks green | ✅ |
| P3 independent review | reviewer | no blocking finding | ⏳ r1 |
| P4 closeout | main | branch commit + owner gate | pending |

## Evidence

- P0: task `.trellis/tasks/11-05-bot-reply-keyboard-menu`; artifacts `prd.md`,
  `design.md`, `research/reply-keyboard-tdlib.md`, prompts in `review/`; worktree
  `/home/agent/.paseo/worktrees/3i5ks3q2/feat-bot-reply-keyboard-menu` on branch
  `feat/bot-reply-keyboard-menu` @ `ab172b7`.
- P1: Design Reviewer round 1 PASS (D1–D8 all met, no blocking finding), agent
  `30706945-2611-4157-8b38-036fd87de6c2`. `approval.md` written;
  `design_sha256 = 06ae508531c0af182ecca4c8e9b1da9c9a706f60147294c06a75a6eaa3fb4b2c`.
  Non-blocking assumptions recorded in `approval.md` (enum/field conventions, GetChat
  flood-wait limitation, missing-markup → NONE, explicit message_id assertion in A1 test).
- P2: implementation complete (agent `a80eac7a-31d0-4515-b716-914900ff25df`).
  Files: `facade.proto` (+23), `facade.pb.go` (regenerated), `app/facade/service.go`
  (+ interface `GetChat`, message selection), `app/facade/mocks/telegram_repo.go`
  (hand-added `GetChat`), `transport/grpc/transport.go` (inline/reply/none dispatch),
  tests (`service_bot_test.go`, `transport_bot_test.go`), `docs/TEST-MATRIX.md`.
  Orchestrator re-ran scoped verification: build + vet green; tests green
  (`transport/grpc`, `app/facade`, `infra/telegram`); `gofmt -l` empty for changed files;
  protoc regeneration byte-identical to the committed generated files
  (`facade.pb.go`, `facade_grpc.pb.go` both OK). Diff is additive; `facade_grpc.pb.go`
  unchanged (no new RPC). Implementer also proved red by temporarily removing the reply
  mapping (recorded in `responses.json`).
- Owner input (2026-11-05): screenshots prove the bottom menu is a reply keyboard
  (`input_field_placeholder = "Choose a menu…"`); bot message `181538914304` carries it;
  `GetInlineMenu` returns empty rows today.
