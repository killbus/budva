# Independent review brief (P3) — 11-05 bot-reply-keyboard-menu

## Base

- base_commit: `ab172b7`
- worktree: `/home/agent/.paseo/worktrees/3i5ks3q2/feat-bot-reply-keyboard-menu`
- branch: `feat/bot-reply-keyboard-menu`
- task path: `.trellis/tasks/11-05-bot-reply-keyboard-menu`
- normative: `prd.md`, `design.md` (§2–§8, §10 are the contract), `approval.md`
  (`design_sha256`), `.trellis/spec/**`
- object under review: `git diff ab172b7`

## Production entry (anchors)

- `cmd/facade/main.go:94` — `facadeService := facade.New(telegramRepo)`
- `cmd/facade/main.go:108-112` — gRPC server registers `tgrpc.New(facadeService)`;
  every RPC reaches `internal/transport/grpc.Transport` → `internal/app/facade.Service`
  → `internal/infra/telegram.Repo` → go-tdlib `client`.
- No auth interceptor (baseline fact).

## Verified facts (orchestrator; spot-check if needed)

- TDLib pinned by `Dockerfile:15` = `22d49d5b87a4d5fc60a194dab02dd1d71529687f`.
  `chat.reply_markup_message_id` doc (`td_api.tl:2197`) is the authoritative pointer to
  the message whose reply markup the client renders; `message.reply_markup` may be null
  (`td_api.tl:1813`); `MessagesManager.cpp:14372/14394` maintain/restore it.
- go-tdlib v0.7.6: `Chat.ReplyMarkupMessageId` (`client/type.go:14956`),
  `ReplyMarkupShowKeyboard` (`:16057-16071`), `KeyboardButton` (`:15622`),
  `KeyboardButtonTypeType()` values (`:950-956`), `KeyboardButtonTypeWebApp.Url`
  (`:15595`).
- Baseline: `service.go:191-206` (old `message_id=0` → chat history), `transport.go:413-458`
  (inline-only mapping), `facade.proto:156-159` (`InlineMenuResponse` fields 1–2),
  `client_adapter.go:290-301` (`Repo.GetChat`, no `FloodWaitFromError`).
- Local verification available via the env block in `review/implementer-prompt.md`;
  always `-p 2`; do not run `go test ./...`; no Docker.

## Blocking criteria

1. **Additivity/compatibility** — `facade.proto` only adds `MenuKind`, `ReplyButton`,
   `ReplyButtonRow`, and `InlineMenuResponse` fields 3–8. Existing 15 RPCs and every
   existing message field number/name/semantic unchanged; `message_id=1` and `rows=2`
   keep their meaning. Generated bindings match the `.proto` and are reproducible with
   the documented protoc line.
2. **Non-goals honored** (design §2) — no new RPC/service; no Web App execution; no
   button-click RPC; no Bot API/bot token; no `reply_markup` added to the `Message` DTO;
   no polling/subscription/stream; no new Go module/env; no ruleset change; no second
   TDLib session.
3. **Layering** (design §6) — `transport/grpc` maps raw TDLib values to proto only;
   `app/facade` performs message selection and request construction; **no change under
   `internal/infra/telegram/**`**. No reverse dependency, no cross-layer leak, no
   test-only branch in production code.
4. **Message-selection contract** (design §4) — `message_id != 0` → `GetMessage` only,
   no `GetChat`; `message_id == 0` → `GetChat` → `chat.reply_markup_message_id`; if 0 →
   existing `GetChatHistory{Limit:1}` fallback; empty history still
   `MessageNotFoundError`.
5. **Mapping contract** (design §5/§6) — inline → `MENU_KIND_INLINE` + unchanged `rows`;
   showKeyboard → `MENU_KIND_REPLY` + `reply_rows` + `is_persistent`/`resize_keyboard`/
   `one_time`/`input_field_placeholder`; nil/remove/force/other → `MENU_KIND_NONE` with
   empty rows; `ReplyButton.type` = `KeyboardButtonTypeType()`; `web_app_url` only for
   `KeyboardButtonTypeWebApp`; nil buttons skipped; row/button order preserved.
6. **Test evidence** — new assertions actually bind criteria A1–A8 (including the
   explicit `message_id` backfill and the "no `GetChat` when id given" strict-mock fact);
   assertions can fail; no existing assertion deleted, weakened, or reshaped.
7. **Scope** — only design §11 files: proto + generated, `app/facade/service.go` (+ its
   mock's new `GetChat`), `transport/grpc/transport.go`, their tests,
   `docs/TEST-MATRIX.md`. No unrelated refactor.
8. **Docs** — `docs/TEST-MATRIX.md` manual entries reflect the reply-keyboard path.

## Output

Write `{{TASK_PATH}}/review/findings.json` in your role schema (verdict, round,
system_context with file:line anchors, spec_conformance_summary, test_assertion_review,
blocking_findings with reachability_proof + violated_clause + minimal_fix_direction +
verification, resolved_previous, withdrawn, follow_ups, discarded_count,
escalation_reason), then reply with the same JSON followed by `done=true|false`.
You may not modify implementation files.
