# Implementer run — P2 (11-05 bot-reply-keyboard-menu)

You run inside the worktree `/home/agent/.paseo/worktrees/3i5ks3q2/feat-bot-reply-keyboard-menu`
(branch `feat/bot-reply-keyboard-menu`, base commit `ab172b7`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-05-bot-reply-keyboard-menu`
BASE_COMMIT = `ab172b7`

## Your role

你是本 loop 的 Implementer,是 design → implement → review 闭环中的一环。
你只有一个职责:在已锁定的设计约束内,把 Ticket 落地为代码。
你不是设计者,不是裁决者。你不给自己的产出下"通过"的结论,
输出中不得出现 PASS / FAIL 之类的裁决措辞。

Before anything else, read `{{TASK_PATH}}/approval.md` (locks `design_sha256`),
`{{TASK_PATH}}/design.md`, and `{{TASK_PATH}}/prd.md`.
Then follow the design **exactly**: implement §4–§8 + §10/§11.
Do not re-litigate the design.

## Hard constraints (ownership rules)

- `prd.md`, `design.md`, `approval.md`, `implement.md`, `progress.md`, `task.json`,
  `.trellis/spec/**` are read-only. No edits, no "clarifying" additions.
- Do not modify `../level85/**`.
- Non-goals (design §2): no new RPC/service; no Web App execution; no reply-keyboard
  button clicking RPC; no Bot API/bot token; no `reply_markup` in the `Message` DTO;
  no polling/subscription/stream; no new Go module; no new required env; no ruleset
  schema change; no second TDLib session.
- Do not change behavior/mapping of the existing 15 RPCs. The new fields must be purely
  additive; `InlineMenuResponse.message_id=1` and `rows=2` keep their exact meaning.
- Do not modify `internal/infra/telegram/**` (design §6 says **no infra code change**).
- No `git commit`, `git push`, `git merge`.
- If the design blocks you: write `{{TASK_PATH}}/review/spec-gap.md` and stop that part.
- Out-of-scope existing issues: `{{TASK_PATH}}/review/follow-ups.md`, do not fix.
- At the end, overwrite `{{TASK_PATH}}/review/responses.json` with round 1
  (`finding_responses: []`): scope_summary, changes (file + intent + design clause),
  test_notes.

## Environment (prepared on this box — use it)

```bash
export PATH=/home/agent/go/bin:/home/agent/.local/protoc/bin:$PATH
export GOPROXY=off
export GOCACHE=/home/agent/.cache/budva-go
export CGO_CFLAGS="-I/home/agent/.local/tdlib/usr/include/TDLib1.8.38"
export CGO_LDFLAGS="-L/home/agent/.local/tdlib-shims -L/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38 -L/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu -Wl,-rpath,/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38 -Wl,-rpath,/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu"
export LD_LIBRARY_PATH="/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu:/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38"
```

Touched-scope verification (from the worktree root; always pass `-p 2` — OOM guard):

```bash
go build -p 2 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./cmd/facade/
go vet   -p 2 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
go test  -p 2 -count=1 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
gofmt -l <changed files>
```

Baseline for these packages is green. Do **not** run `go test ./...`. Do not run Docker.
The `libtdjson` tag is local-only; production/CI build (no tag) must stay valid.

## Proto regeneration (already verified on this box)

Tools: `protoc 34.2` (reports `v7.34.2`), `protoc-gen-go v1.36.11`,
`protoc-gen-go-grpc v1.5.1`. From the worktree root:

```bash
protoc -I . --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  internal/transport/grpc/pb/facade.proto
```

The `proto` Taskfile task already exists from 11-04; do not change it unless the design's
§11 requires it (it does not).

## Mocks

`internal/app/facade/mocks/telegram_repo.go` is mockery-generated. **Do not regenerate.**
Add the `GetChat` method by hand, copying the exact existing generated style
(`mock.Called` / type assertions / `Run` / `Return` / `RunAndReturn`).
`internal/transport/grpc/mocks/facade_service.go` needs no change (service signature is
unchanged).

## Implementation map (design §4–§6, §11)

1. `internal/transport/grpc/pb/facade.proto` §5:
   - new `enum MenuKind { MENU_KIND_UNSPECIFIED = 0; MENU_KIND_NONE = 1;
     MENU_KIND_INLINE = 2; MENU_KIND_REPLY = 3; }`
   - new `ReplyButton { string text = 1; string type = 2; string web_app_url = 3; }`
   - new `ReplyButtonRow { repeated ReplyButton buttons = 1; }`
   - extend **only** `InlineMenuResponse` with fields 3–8: `MenuKind menu_kind = 3;`
     `repeated ReplyButtonRow reply_rows = 4;` `bool is_persistent = 5;`
     `bool resize_keyboard = 6;` `bool one_time = 7;`
     `string input_field_placeholder = 8;`
   - regenerate; do not touch the existing 15 RPCs.
2. `internal/app/facade/service.go` §4:
   - add `GetChat(*client.GetChatRequest) (*client.Chat, error)` to the partial
     `telegramRepo` interface;
   - `GetInlineMenu`: if `messageID == 0`, call `GetChat`; if
     `chat.ReplyMarkupMessageId != 0`, use it as the message id; if it is 0, keep the
     existing `GetChatHistory{Limit: 1}` fallback and existing empty-history
     `MessageNotFoundError`; if `messageID != 0`, do **not** call `GetChat`.
3. `internal/transport/grpc/transport.go` §5/§6:
   - replace the inline-only mapping with a dispatch on `msg.ReplyMarkup`:
     `*client.ReplyMarkupInlineKeyboard` → `MENU_KIND_INLINE` + existing `rows`;
     `*client.ReplyMarkupShowKeyboard` → `MENU_KIND_REPLY` + `reply_rows` + the four
     keyboard attributes; nil / remove / force / other → `MENU_KIND_NONE` with empty
     rows;
   - `ReplyButton.type` = `button.Type.KeyboardButtonTypeType()`; nil `Type` → empty
     string; `web_app_url` only for `*client.KeyboardButtonTypeWebApp`;
   - skip nil buttons (same as the inline mapper); preserve row/button order.
4. `docs/TEST-MATRIX.md`: update `GRPC-U-002` and the `LIVE-BOT-*` manual entries to cover
   reply keyboards (see design §11.4).

## Tests (design §8; must bind the contract and be able to fail)

- `internal/app/facade`: `message_id != 0` → only `GetMessage` (strict mock: `GetChat`
  unexpected call fails); `message_id == 0` + `ReplyMarkupMessageId=777` → `GetMessage`
  777 and **no** `GetChatHistory`; `ReplyMarkupMessageId=0` → `GetChatHistory(limit=1)`;
  empty history → `MessageNotFoundError`.
- `internal/transport/grpc`: reply keyboard mapping asserts `menu_kind`, `message_id`
  backfill, rows/text/type/web_app_url and all four attributes (A1–A3, **including the
  explicit `message_id` assertion**); inline path keeps its existing assertions and now
  asserts `MENU_KIND_INLINE`; nil/remove/force → `MENU_KIND_NONE` + both rows empty (A5).
- Do not delete, weaken, or reshape any existing assertion. If an existing inline test
  must gain the `menu_kind` assertion, add it — do not alter the existing row assertions.
- Note the red path in a one-line comment where the binding is subtle (quality-guidelines
  R3), and keep test Arrange under ~15 lines (F3).

## Output

Overwrite `{{TASK_PATH}}/review/responses.json` and reply with a 2–3 sentence progress
summary. No verdict wording.
