# Independent review brief (P3)

## Base

- base_commit: `609f181`
- worktree: `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc`
- branch: `feat/bot-menu-grpc`
- task path: `.trellis/tasks/11-04-bot-menu-grpc`
- normative: `prd.md`, `design.md` (§4–§8, §10 are the contract), `.trellis/spec/**`

## Production entry (anchors already confirmed by orchestrator)

- `cmd/facade/main.go:94` — `facadeService := facade.New(telegramRepo)`
- `cmd/facade/main.go:108-112` — `agrpc.NewDefault(..., func(s *grpc.Server) { grpcTransport := tgrpc.New(facadeService); pb.RegisterFacadeGRPCServer(s, grpcTransport) })`
  → every new RPC reaches `*internal/transport/grpc.Transport` → `*internal/app/facade.Service` → `internal/infra/telegram.Repo` → go-tdlib `client`.
- No auth interceptor on that server (baseline fact, noted in design §10).

## Verified facts (orchestrator ran these — do not spend time re-deriving)

- `go build/vet/test -p 2 -tags libtdjson` on `internal/transport/grpc`, `internal/app/facade`,
  `internal/infra/telegram`, `cmd/facade` — all green. Baseline was green before the change.
- Existing 10 RPC tests untouched (no deletions/weakenings in the diff of `transport_test.go`
  / `service_test.go`; implementer did not modify them).
- Proto regeneration is reproducible: protoc 34.2 (`v7.34.2`) + protoc-gen-go v1.36.11 +
  protoc-gen-go-grpc v1.5.1; unchanged `.proto` regenerates byte-identically apart from the
  version comment line.
- go-tdlib v0.7.6 facts: `GetCallbackQueryAnswer` doc says "Returns an error with code 502 if
  the bot fails to answer the query before the query timeout expires"
  (`client/function.go:7458`); `UserFullInfo.BotInfo{ShortDescription, Description,
  MenuButton *BotMenuButton{Text,Url}, Commands []*BotCommand{Command,Description}}`
  (`type.go:9410`, `:9532`, `:6249`, `:6199`); `ReplyMarkupInlineKeyboard.Rows
  [][]*InlineKeyboardButton` (`type.go:16094`); `InlineKeyboardButton{Text, Type}` with
  `InlineKeyboardButtonTypeUser` (`type.go:15903`); `CallbackQueryPayloadData{Data []byte}`
  (`type.go:35770`); `GetChatHistoryRequest` documents `from_message_id = 0` = last message
  (`function.go:2515`); `SendBotStartMessageRequest{BotUserId, ChatId, Parameter}`
  (`function.go:3875`); TDLib errors are `client.ResponseError{Err *client.Error{Code, Message}}`
  returned **by value**, matched via `errors.As` on the value type
  (`client/tdlib.go:175-189`, see `.trellis/spec/backend/telegram-tdlib-guidelines.md`).
- `parseFloodWait` (`internal/infra/telegram/repo.go:299`) matches `retry after N` and returns
  `N s + 500 ms` (`:309`).
- spec: app layers define their own **partial** `telegramRepo` interfaces with own mocks;
  Repo-level wrapper changes never ripple into them (`telegram-tdlib-guidelines.md:108-110`).
  `mocks/client_adapter.go` stubs the embedded interface (tests-only).

## Blocking criteria (P3)

1. **Additivity / compatibility** — 5 new RPCs only; the 10 existing RPCs and every existing
   `Message`/`NewMessage`/request/response field are unchanged (proto field numbers & names too);
   no removed/renamed field; no changed semantics on existing paths.
2. **Non-goals honored** (design §2) — no reply fetching/polling/stream RPC; no Web App
   execution; no bot-account CRUD / Bot API / bot token; no generic `Execute(opcode,payload)`;
   no second TDLib session; no new Go module; no new required env; no ruleset change.
3. **No new runtime dependency / env / config** — `go.mod`/`go.sum` unchanged; no new required
   env; no ruleset/schema change; `proto` Taskfile task is dev tooling only.
4. **Reuse, no reinvention** — every TDLib fact goes through go-tdlib types/methods; no
   hand-rolled JSON/MTProto/Bot-API HTTP; flood-wait reuses `parseFloodWait` (no second regex);
   no TDLib client constructed outside `repo.go`.
5. **Layering** (design §6) — `transport/grpc` must not call TDLib directly; it sees facade
   values + the typed errors design §7 assigns to infra (compare baseline
   `transport.go` → `telegram.ChatNotReadyError`). `app/facade` stays a thin wrapper (no TDLib
   mapping/DTO logic beyond the request construction design §4 lists). `infra` owns TDLib
   specifics no test-only branches, no mock-shaped openings in production code.
6. **Error mapping = design §7** — username not found → NotFound; non-bot →
   FailedPrecondition; no keyboard → OK with empty rows; stale data / other callback 4xx →
   FailedPrecondition; bot 502 → **DeadlineExceeded** (not Internal); flood → ResourceExhausted
   + RetryInfo with `Delay` = `parseFloodWait` result; empty data/command → InvalidArgument.
7. **Test assertions encode the contract** — the §7 flood-wait mechanical test exists
   (`retry after 7` → 7.5 s at infra, `ResourceExhausted` + `RetryInfo.RetryDelay == 7.5 s` at
   gRPC) and the mapping/validation tests actually pin §4/§5/§7 behavior. Existing assertions
   not deleted/weakened/reshaped.
8. **Scope** — only design §11 files: `facade.proto` + regenerated bindings,
   `infra/telegram/client_adapter.go` (+ typed-error file), `app/facade/service.go` (+ typed-error
   file), `transport/grpc/transport.go`, the three mock files (new methods only), tests,
   `Taskfile.yml proto` task, `docs/TEST-MATRIX.md` entry, `doc.go` notes. No unrelated refactor.

## Orchestrator's own observations — confirm or dismiss with evidence (do not treat as rulings)

- **O1** Design §4③ puts message lookup in `facade.Service.GetInlineMenu`
  (`GetChatHistory` when 0), consistent with `app` shelling to the repo. **But** `transport.go:29-32`
  declares `callbackAnswerTimeout = 10 * time.Second` and *never uses it* — no handler wires
  it. Is the §7 callback-timeout contract implemented at all, and does the unused constant
  violate something the diff touches (dead code / §5's "锁定" claim)?
- **O2** `internal/infra/telegram/errors.go` `FloodWaitFromError` has a **string fallback**:
  any error whose text contains `retry after N` becomes a `FloodWaitError`, even without code
  429 / `ResponseError`. Is that a transport/parser concern leaking into infra, and can it
  misfire on a wrapped non-flood TDLib error or a locally-generated message? What does the
  §7 mechanical test actually prove?
- **O3** `NewCallbackAnswerError` emits `Code` 0 for a non-`ResponseError`; `mapFacadeError`
  then leaves it as `Internal`. Is that a silent fall-through of the §7 contract?
- **O4** Layering: `transport.go` now imports `internal/app/facade` and does
  `errors.As(facade.MessageNotFoundError)` / `errors.As(facade.TargetNotBotError)`. Baseline
  transport already imported `internal/infra/telegram`, so this is not a new *layer* — but
  design §6 lists facade errors as infra; is the vertical direction (transport → app) an
  established or forbidden edge in `.trellis/spec/` or the graph?
- **O5** design §4⑤ says command → `"/command"`, while the handler sends `command` as given.
  Is there normalization (leading slash) at either layer, and is the missing one a contract
  gap or simply caller-trusted input (no validation required in §7)?
- **O6** `ResolveBot` uses `GetUser` on the *private-chat user id* only after
  `ChatTypePrivate`; a public **supergroup/channel** bot-chat is rejected as
  `TargetNotBotError`. Is that precisely "not a bot" per §7 (FailedPrecondition), or mislabeled
  input (InvalidArgument)?
- **O7** `GetUserFullInfo.BotInfo == nil` (e.g. a non-bot id passed) returns an empty
  `BotInfoResponse` with OK (`botInfoToProto`). Is that "非 bot id 由 TDLib 错误自述" —
  i.e.: does TDLib error, or does the RPC silently return OK for a non-bot id?
- **O8** `ClickCallbackButton` on §7 "消息没有 inline 键盘 → OK（空）" is not applicable
  (click needs a message + data); on empty `callback_data` the handler returns InvalidArgument —
  consistent. ✔ No action unless you disagree.
- **O9** `SendBotCommand` validation runs on `TrimSpace(command)` and drops empties, but a
  `command` of `"/start"` without `bot_user_id` reaches `SendBotStartMessage(0, …)`. Is that an
  InvalidArgument gap or caller-trusted (design §5 has no "required" annotation)?
- **O10** docs: `docs/TEST-MATRIX.md` manual entry and the three `doc.go` notes — do they
  materially contradict design §2 (reply invisibility), §10 (bot chat not in ruleset), or §7?

## Prior rounds

`review/findings.json` does not exist → **round = 1**. `previous_findings = []`.

## Output

Write `{{TASK_PATH}}/review/findings.json` in your role schema and reply with the same JSON
followed by `done=true|false`. Do not modify implementation files.