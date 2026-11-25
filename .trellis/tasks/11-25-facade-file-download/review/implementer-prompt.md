# Implementer run — P2 (11-25 facade-file-download)

You run inside the worktree `D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download`
(branch `feat/facade-file-download`, base commit `fbc22cc`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-25-facade-file-download`
BASE_COMMIT = `fbc22cc`

## Your role

你是本 loop 的 Implementer,是 design → implement → review 闭环中的一环。
你只有一个职责:在已锁定的设计约束内,把 Ticket 落地为代码。
你不是设计者,不是裁决者。你不给自己的产出下"通过"的结论,
输出中不得出现 PASS / FAIL 之类的裁决措辞。

Before anything else, read `{{TASK_PATH}}/approval.md` (locks `design_sha256`),
`{{TASK_PATH}}/design.md`, and `{{TASK_PATH}}/prd.md`.
Then follow the design **exactly**: implement §5-§9 + §11.
Do not re-litigate the design.

## Hard constraints (ownership rules)

- `prd.md`, `design.md`, `approval.md`, `implement.md`, `progress.md`,
  `task.json`, `.trellis/spec/**` are read-only. No edits, no "clarifying"
  additions.
- Non-goals (design §2 N1-N7): no gRPC streaming RPCs; no progress/
  updateFile push; no upload-side change; no GraphQL change; no engine/
  ruleset change; no download-management RPCs; do not populate or remove the
  dead `Message.file_path` field.
- Do not change behavior/mapping of the existing 16 RPCs. The two new RPCs
  and their messages must be purely additive; every existing message field
  number/name/semantic unchanged.
- `api/proto/facade.proto` is a git SYMLINK — it must stay UNCHANGED in the
  diff (do not delete, replace with a regular file, or rewrite it).
- Do not modify anything outside the design §11 file set.
- No `git commit`, `git push`, `git merge` (the orchestrator owns git).
- If the design blocks you: write `{{TASK_PATH}}/review/spec-gap.md` (clause
  locator + problem + blocked work) and stop that part.
- Out-of-scope existing issues: append to
  `{{TASK_PATH}}/review/follow-ups.md`, do not fix.
- At the end, overwrite `{{TASK_PATH}}/review/responses.json` with round 1
  (`finding_responses: []`): scope_summary, changes (file + intent + design
  clause), test_notes (INCLUDING the red-path evidence, design §13).

## Environment (this box — Windows; use exactly this)

- go-tdlib (cgo, linux/darwin build tags only) CANNOT be built or tested
  here. Do NOT run `go build`, `go vet`, or `go test` on the touched
  packages — they will fail at the dependency's cgo layer and the failure is
  NOT a signal about your code. The authoritative vet/test is GitHub Actions
  `ci-test.yml` (Linux, real TDLib static libs), run by the orchestrator via
  PR to main. Your local gates are: gofmt + proto byte-identity + code
  reading (mock signatures, interface satisfaction by hand-checking).
- Tools (already installed by the orchestrator — put them on PATH first):
  ```powershell
  $env:PATH = "D:\Repositories\budva\.trellis\.runtime\protoc\bin\bin;C:\Users\ying\go\bin;$env:PATH"
  # protoc: libprotoc 34.2  (parity PROVEN at P0: baseline regen byte-identical)
  # protoc-gen-go v1.36.11, protoc-gen-go-grpc v1.5.1 in C:\Users\ying\go\bin
  ```
- The working tree is the ONLY place you may write code. Regenerating with
  `--go_out=.` fails with "Permission denied" from this agent sandbox —
  if you hit that, run protoc into a scratch dir under
  `D:\Repositories\budva\.trellis\.runtime\proto-out\` with
  `--go_out=<scratch> --go_opt=paths=source_relative` and COPY the two
  generated files over `internal/transport/grpc/pb/` yourself.
- gofmt check: `gofmt -l <changed .go files>` must print nothing.

## Proto regeneration (design §10)

From the worktree root (scratch-output variant if direct write is denied):

```
protoc -I . --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  internal/transport/grpc/pb/facade.proto
```

Parity was already proven at P0 (baseline regen byte-identical). After your
additive edit: the diff of `facade.pb.go` must be additions only (new
message types + getters + enum/oneof untouched), and `facade_grpc.pb.go`
must gain exactly the two new RPC methods. If regeneration produces
differences in pre-existing regions, STOP and write spec-gap.md.

## Mocks

`internal/app/facade/mocks/telegram_repo.go` and
`internal/transport/grpc/mocks/facade_service.go` are mockery-generated.
Do NOT regenerate with mockery (versions drift the whole file). Hand-add the
new methods copying the exact existing generated style (`_m mock.Called` /
type assertions / `Run` / `Return` / `RunAndReturn` / `_Call` struct),
keeping alphabetical method order where the file has one. The generated
file's diff must be limited to the new methods.

## Implementation map (design §5-§9)

1. `internal/transport/grpc/pb/facade.proto` (§5): add exactly the 2 RPCs
   + 4 messages from the §5 proto block; regenerate.
2. `internal/infra/telegram/client_adapter.go` (§8): `clientAdapter` +3
   methods (`GetFile`, `DownloadFile`, `GetFileMimeType`, 1:1 go-tdlib
   signatures); `Repo` implements them as bare passthroughs (NO
   `withChatReady` — file ops are not chat-scoped; follow `GetMe`/`GetUser`/
   `GetOption` at lines ~367-437). `Repo.DownloadFile` wraps its error with
   `FloodWaitFromError` for uniformity (no-op for non-flood errors).
3. `internal/app/facade/errors.go` (§7): `MessageFileNotFoundError{ChatID,
   MessageID int64}` + `NewMessageFileNotFoundError`, `FileDownloadFailedError
   {FileID int32; Err error}` + `NewFileDownloadFailedError` — same style as
   `MessageNotFoundError`/`TargetNotBotError`.
4. `internal/app/facade/service.go` (§6): partial `telegramRepo` interface
   +3 methods; `GetMessageFile(ctx, chatID, messageID) (*client.File, string,
   error)` — GetMessage -> resolution table (design §2: photo = max
   `PhotoSize.Photo.Size` variant, ties -> last in slice order; other media
   types per table; nil/absent file or non-media content ->
   `*MessageFileNotFoundError`) -> `DownloadFile{FileId, Priority: 16,
   Synchronous: true}` -> success oracle §6.4 (`Local != nil &&
   Local.IsDownloadingCompleted && Local.Path != ""`, else
   `*FileDownloadFailedError` wrapping the cause) -> best-effort
   `GetFileMimeType` (any error -> ""). `DownloadFile(ctx, fileID int32)
   (*client.File, error)` — same download+oracle core for a caller-supplied
   id.
5. `internal/app/facade/mocks/telegram_repo.go`: hand-add the 3 methods.
6. `internal/transport/grpc/transport.go` (§5/§7): `facadeService` interface
   +2 methods; `GetMessageFile` RPC impl (validate request: chat_id/message_id
   presence is implicit int64 — no validation needed beyond what the service
   enforces; map raw `*client.File` + mime -> `MessageFileResponse`);
   `DownloadFile` RPC impl (validate `file_id`: `0` or `> math.MaxInt32` ->
   `codes.InvalidArgument`, service not called; map `*client.File` ->
   `FileResponse`); `mapFacadeError` +2 branches BEFORE the Internal
   fallthrough: `*facade.MessageFileNotFoundError` -> `codes.NotFound`,
   `*facade.FileDownloadFailedError` -> `codes.Internal`.
7. `internal/transport/grpc/mocks/facade_service.go`: hand-add
   `GetMessageFile`/`DownloadFile`.
8. Tests (design §9):
   - `internal/app/facade/service_test.go`: A1 per media type
     (photo/video/document/audio/animation/voice/videoNote/sticker) with a
     photo fixture whose LARGEST size is NOT first or last in the slice
     (e.g. sizes 100/300/200 by Size) to bind max-by-Size (not order);
     assert `DownloadFile` called with the resolved `FileId` and
     `Synchronous: true` (mock args); success oracle shape
     (`Local{IsDownloadingCompleted: true, Path: "/x/y.bin"}`) -> returned
     path/size; mime: `GetFileMimeType` error -> "" with success. A2: text
     message -> `*MessageFileNotFoundError` (assert type + fields);
     photo `Sizes: []` -> same; photo entry nil `Photo` -> same. A3:
     `DownloadFile` oracle-false (`IsDownloadingCompleted: false`) ->
     `*FileDownloadFailedError`; repo error -> `*FileDownloadFailedError`
     wrapping it. Strict mocks: unexpected calls must fail the test
     (quality-guidelines F1); one-line red-path comment where the binding is
     subtle (R3); Arrange under ~15 lines (F3).
   - `internal/transport/grpc/transport_test.go`: `DownloadFile`
     validation: `file_id=0` -> `codes.InvalidArgument` and
     `file_id=2147483648` (MaxInt32+1) -> `codes.InvalidArgument`, service
     mock gets NO call (strict); success mapping: raw `*client.File` ->
     `MessageFileResponse{file_path,size,file_id,mime_type}` and
     `FileResponse{file_path,size}`; error mapping:
     `*facade.MessageFileNotFoundError` -> `NotFound`;
     `*facade.FileDownloadFailedError` -> `Internal`.
9. `docs/TEST-MATRIX.md`: add rows under App / Facade (`SVC-U-004`
   section) and Transport / gRPC (`GRPC-U-001` section) with the ACTUAL new
   test names/counts; update the summary table counts if it lists
   per-package totals.

## Red-path proof (design §13, quality-guidelines F2) — REQUIRED

Before finishing: temporarily break ONE binding (e.g. make the photo
selector take the FIRST size instead of the max, or flip the oracle to
accept `IsDownloadingCompleted: false`), and prove the specific new test
fails by reasoning about the assertion (you cannot run go test on this
box — state exactly which assertion trips and why, and note that the CI run
on the final code must be green). Record this in `responses.json`
`test_notes` as the red-path evidence. Then restore the correct code.

## Output

Overwrite `{{TASK_PATH}}/review/responses.json` and reply with a 2-3
sentence progress summary. No verdict wording.
