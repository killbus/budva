# Implementer run — P2 (bot-menu-grpc)

You run inside the worktree `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc`
(branch `feat/bot-menu-grpc`, base commit `609f181`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-04-bot-menu-grpc`
BASE_COMMIT = `609f181`

Before anything else, read `{{TASK_PATH}}/approval.md`, `{{TASK_PATH}}/design.md`
(locked; `design_sha256` in approval.md), and `{{TASK_PATH}}/prd.md`.
Then follow your role **exactly**: implement design §4–§6 + §8 tests + §9/§11
engineering items. Do not re-litigate the design.

## Hard constraints (repeat of ownership rules)

- `prd.md`, `design.md`, `approval.md`, `implement.md`, `task.json`, `.trellis/spec/**`
  are read-only. No edits, no "clarifying" additions.
- Do not modify `../level85/**`.
- Non-goals: no reply fetching/polling/stream RPC; no Web App execution; no bot-account
  CRUD / Bot API; no generic `Execute(opcode, payload)`; no new Go module; no new
  required env var; no ruleset schema change; no second TDLib session; no bot token.
- Do not change the behavior/mapping of the existing 10 RPCs. New error semantics apply
  only to the 5 new RPCs' call paths.
- No `git commit`, `git push`, `git merge`.
- If design blocks you: write `{{TASK_PATH}}/review/spec-gap.md` and stop that part.
- Out-of-scope existing issues: `{{TASK_PATH}}/review/follow-ups.md`, do not fix.
- At the end, overwrite `{{TASK_PATH}}/review/responses.json` with round 1
  (`finding_responses: []`), listing files + intent + the design clause each serves.

## Environment (already prepared for this box — use it)

Local compilation is possible via the Debian TDLib **shared** library extracted under
`/home/agent/.local`. Always export this exact block, and always pass `-p 2` (OOM guard):

```bash
export PATH=/home/agent/go/bin:/home/agent/.local/protoc/bin:$PATH
export GOPROXY=off
export GOCACHE=/home/agent/.cache/budva-go
export CGO_CFLAGS="-I/home/agent/.local/tdlib/usr/include/TDLib1.8.38"
export CGO_LDFLAGS="-L/home/agent/.local/tdlib-shims -L/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38 -L/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu -Wl,-rpath,/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38 -Wl,-rpath,/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu"
export LD_LIBRARY_PATH="/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu:/home/agent/.local/tdlib/usr/lib/x86_64-linux-gnu/TDLib1.8.38"
```

Touched-scope verification (from the worktree root):

```bash
go build -p 2 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/ ./cmd/facade/
go vet   -p 2 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
go test  -p 2 -count=1 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
gofmt -l <changed files>
```

Baseline (pre-change) for the last two packages is green on this box. Do **not** run
`go test ./...` (heavy, no value here). Do not attempt Docker builds. The `libtdjson`
tag is local-only; production/CI builds without it must remain valid — never put the
tag in code/build files.

## Proto regeneration (design §9; verified working on this box)

The repo has no proto task. Tools are installed user-locally:
`protoc 34.2` (reports `v7.34.2`, same 34.x line as the committed `v7.34.1`),
`protoc-gen-go v1.36.11`, `protoc-gen-go-grpc v1.5.1`.

Regeneration command (run from repo root; body output is byte-identical to the
committed files when the `.proto` is unchanged — only the `protoc v…` comment line
changes from 7.34.1 to 7.34.2):

```bash
protoc -I . --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  internal/transport/grpc/pb/facade.proto
```

Add a `proto` task to `Taskfile.yml` (design §11.1) that installs the two plugins at
the pinned versions (`go install …@v1.36.11`, `…@v1.5.1`) and runs the command above,
with a comment that the protoc binary (34.x) must be on PATH. Keep the existing
`mock` task untouched.

## Mocks

`internal/transport/grpc/mocks/facade_service.go` and
`internal/app/facade/mocks/telegram_repo.go` are mockery-generated. Do **not** run
`go run github.com/vektra/mockery/v3@latest` (unpinned; rewrites unrelated mocks).
Add only the new methods by hand, copying the exact existing generated style
(`mock.Called`/`mock.MethodCalled`, type assertions, `RunAndReturn`, constructor
signature). Mention this in `responses.json`.

## Implementation map (design §4–§6, §10, §11)

1. `internal/infra/telegram/client_adapter.go`: `SearchPublicChat`, `GetUser`,
   `GetUserFullInfo`, `SendBotStartMessage` (+ compile-time assertions if the file's
   style has them). `GetMessage`, `GetChatHistory`, `SendMessage`,
   `GetCallbackQueryAnswer` already exist.
2. `internal/infra/telegram`: flood-wait typed error + exported helper per §7,
   reusing package-private `parseFloodWait` (`repo.go:299`; `+500ms` at `:309`).
   Apply it at the 5 new RPC TDLib call sites only. Keep every other mapping unchanged.
3. Semantic typed errors for the new paths (transport cannot parse text): not-found and
   not-a-bot (→ NotFound / FailedPrecondition), callback 502 (→ DeadlineExceeded),
   other callback 4xx (→ FailedPrecondition). Put them in `internal/infra/telegram`
   next to `ChatNotReadyError` so `transport` keeps its current dependency direction;
   `app/facade` stays a thin wrapper.
4. `internal/app/facade/service.go`: add the 5 methods (raw TDLib types or simple DTOs);
   extend the package-private `telegramRepo` interface with exactly what they use.
5. `internal/transport/grpc/transport.go`: extend `facadeService`, add 5 handlers,
   raw→proto mapping, extend `mapFacadeError` only for the new typed errors. Validate
   empty `callback_data` / empty `command` with `InvalidArgument`.
6. `internal/transport/grpc/pb/facade.proto`: add exactly the §5 surface (5 RPCs,
   messages, `InlineButton` oneof incl. `other_type`), then regenerate.
7. Docs: `docs/TEST-MATRIX.md` manual acceptance item and the `doc.go` package docs per
   the file's existing convention. No README rewrites beyond what §11.5 asks.

## Tests to add (design §8 — encode the contract, do not weaken existing assertions)

- infra: `FloodWaitFromError` on an error containing `retry after 7` yields
  `FloodWaitError.Delay == 7.5s`; a non-flood error passes through unchanged.
- transport mapping: inline keyboard → `InlineMenuResponse` (rows/order preserved;
  callback_data/url/web_app/other_type); `UserFullInfo` → `BotInfoResponse`;
  `CallbackQueryAnswer` → `CallbackAnswerResponse`.
- transport error mapping: `FloodWaitError` → `ResourceExhausted` **and**
  `errdetails.RetryInfo.RetryDelay == 7.5s` (status details, not just the code);
  callback 502 typed error → `DeadlineExceeded`; not-found → `NotFound`;
  not-a-bot → `FailedPrecondition`; empty data/command → `InvalidArgument`.
- app/facade: each wrapper builds the expected TDLib request through the strict mock.
- Keep the existing 10-RPC tests untouched (and green).

## Output

- Implement; run the scoped checks above until green (record exact commands in
  `responses.json.test_notes`, and state that they are not architecture proof).
- Write `{{TASK_PATH}}/review/responses.json` (round 1 schema from your role).
- Reply with a 3–6 line progress summary only (no verdict words).
