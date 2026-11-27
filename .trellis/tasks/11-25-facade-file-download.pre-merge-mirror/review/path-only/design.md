# Design — facade gRPC file download (GetMessageFile + DownloadFile)

Base commit: `fbc22cc` (main). This design is the single ruler for implementation
and review; every section is addressable as `design.md §x`.

## 1. Context and problem

The facade gRPC surface exposes no way to fetch a file received in Telegram.
TDLib v0.7.6 (pinned; `go.mod:17`) already provides the primitives:

- `(*client.Client).DownloadFile(*client.DownloadFileRequest) (*client.File, error)`
  (`client/function.go:11924`). `DownloadFileRequest`
  (`client/function.go:11909`): `FileId int32`, `Priority int32`, `Offset int64`,
  `Limit int64`, `Synchronous bool`. Doc: with `Synchronous=true` the response
  is returned only after the download succeeded, failed, or was canceled.
Revision 2 fact note: the design previously listed `GetFile` and
`GetFileMimeType` as download prerequisites. `GetFileMimeTypeRequest`
(`client/function.go:6366-6369`) has only `FileName string` — "The name of the
file or path to the file"; the previously written `{FileId: file.Id}` shape
does not exist, and the API only guesses MIME by file extension (empty on
failure). This design therefore does not add or call that RPC; MIME comes from
the resolved message content (§6.5). `GetFile` is also omitted because
`GetMessageFile` already receives the message content and needs no `getFile`
RPC. Only `DownloadFile` has a live consumer in this ticket.

`client.File` (`client/type.go:4277`): `Id int32`, `Size int64`,
`ExpectedSize int64`, `Local *LocalFile`, `Remote *RemoteFile`.
`client.LocalFile` (`client/type.go:4209`): `Path string`,
`CanBeDownloaded bool`, `IsDownloadingActive bool`, `IsDownloadingCompleted bool`,
`DownloadOffset int64`, `DownloadedPrefixSize int64`.

Message content → file mapping (`client/type.go`, go-tdlib v0.7.6):

| content type         | field           | file object                          |
|----------------------|-----------------|--------------------------------------|
| `MessagePhoto`       | `Photo *Photo`  | `Photo.Sizes []*PhotoSize`, `PhotoSize.Photo *File` (`type.go:4420-4425`) |
| `MessageVideo`       | `Video *Video`  | `Video.Video *File` (`type.go:5470`) |
| `MessageDocument`    | `Document *Document` | `Document.Document *File` (`type.go:5328`) |
| `MessageAudio`       | `Audio *Audio`  | `Audio.Audio *File` (`type.go:5297`) |
| `MessageVoiceNote`   | `VoiceNote *VoiceNote` | `VoiceNote.Voice *File` (`type.go:5565`) |
| `MessageAnimation`   | `Animation *Animation` | `Animation.Animation *File` (`type.go:5258`) |
| `MessageVideoNote`   | `VideoNote *VideoNote` | `VideoNote.Video *File` (`type.go:5505`) |
| `MessageSticker`     | `Sticker *Sticker` | `Sticker.Sticker *File` (`type.go:5394`) |

Note: TDLib's schema names the wrapper field and the inner `*File` field alike
(`Video.Video *File`, not `Video.File`), and the voice-note content type is
`MessageVoiceNote` (there is no `MessageVoice`). The `file object` column names
the wrapper field and the `*File` field inside it, verbatim for go-tdlib
v0.7.6.

[ASSUMPTION] Photo variant selection: TDLib serves `Photo.Sizes` with
`photoSize.type` ∈ {small, medium, large, xlarge} and clients conventionally
take the largest. This design selects the variant with the maximum
`PhotoSize.Photo.Size` (ties → last in slice order). Rationale: size-based is
mechanically falsifiable and does not rely on string ordering of type names.
Rationale source: none in `prd.md`/`.trellis/spec/` → marked [ASSUMPTION] (D4).
Client-selectable variant selection is NOT part of this ticket: `GetMessageFile`
always resolves to the largest variant, and no request field, app-layer
parameter, or test covers any optional selection. This omission is deliberate
and declared in non-goal N8 (§2); an additive proto field for variant selection
can be introduced later without breaking this contract.

## 2. Non-goals (normative)

- N1: No gRPC streaming RPCs (no byte transfer over the wire). The contract is
  path-based, matching `SendMessageAlbum`/`NewMessage.file_path`.
- N2: No progress reporting, no `updateFile` subscription, no push/webhook
  notifications about downloads.
- N3: No upload-side behavior change (`SendMessage`, `SendMessageAlbum`,
  `inputMessageByFileExt` untouched).
- N4: No GraphQL surface change (`schema.graphqls` stays `status`-only).
- N5: No engine (push/sink/ruleset) change.
- N6: No download management RPCs (cancel/pause/list) in this ticket.
- N7: The dead `Message.file_path` field is NOT removed (additive-only rule,
  §3) and NOT populated (no media metadata change to the message DTO).
- N8: No client-selectable photo variant selection in this ticket.
  `GetMessageFile` resolves photos to the largest variant only (the fixed
  default of §1/§6); the request carries no variant field. A variant-selection
  proto field can be introduced later as a purely additive change, which keeps
  this omission forward-compatible.

## 3. Module boundaries and responsibilities (D1)

| module | responsibility in this ticket |
|---|---|
| `internal/transport/grpc` (`pb/facade.proto`, `transport.go`) | gRPC contract (2 additive RPCs), raw-TDLib→proto mapping, gRPC error mapping via `mapFacadeError` |
| `internal/app/facade` (`service.go`, `errors.go`) | message→file resolution, largest-photo-variant selection (default-only, N8), synchronous download orchestration, typed errors |
| `internal/infra/telegram` (`client_adapter.go`, `repo.go`) | 1:1 passthrough wrapper over `*client.Client` (`DownloadFile`), no business logic |
| `cmd/facade` | no change (wiring already passes the whole `telegram.Repo` into `facade.New`, `cmd/facade/main.go:80,94,110-111`) |
| everything else (`internal/service/**`, `internal/app/handler`, engine, ruleset, GraphQL) | untouched |

## 4. Dependency direction (D2)

- `transport/grpc` → `app/facade` → `infra/telegram` → `zelenin/go-tdlib/client`.
  No new edges, no reverse edges, no cycles.
- `app/facade` must NOT import `pb` (it returns raw `*client.File` / `*client.Message`,
  same as every existing service method).
- `infra/telegram` must NOT import `app/facade` or `pb`.
- New `clientAdapter` methods mirror go-tdlib signatures 1:1 (no ctx, no
  domain types) per the package doc at `client_adapter.go:9-15`.

## 5. gRPC contract (additive only) (D5/D6)

Added to `internal/transport/grpc/pb/facade.proto` (the canonical contract file).
NOTE: `api/proto/facade.proto` is a git SYMLINK (mode 120000) to that file - never edit it separately; on this Windows checkout it surfaces as a one-line path text and is expected UNCHANGED in the diff.

```proto
rpc GetMessageFile (GetMessageFileRequest) returns (MessageFileResponse);
rpc DownloadFile (DownloadFileRequest) returns (FileResponse);

message GetMessageFileRequest {
  int64 chat_id = 1;
  int64 message_id = 2;
}

message MessageFileResponse {
  string file_path = 1;
  int64 size = 2;
  int64 file_id = 3;
  string mime_type = 4;
}

message DownloadFileRequest {
  int64 file_id = 1;
}

message FileResponse {
  string file_path = 1;
  int64 size = 2;
}
```

- Field numbers/names: [ASSUMPTION] (proto3 additive convention; A4 requires
  pure addition — existing 16 RPCs, all existing messages and field numbers
  untouched; regeneration must be byte-identical for the unchanged parts).
- `GetMessageFileRequest` intentionally carries ONLY `chat_id` and `message_id`:
  there is no variant-selection field, matching non-goal N8 (largest variant
  only in this ticket).
- `file_id` is `int64` in proto but `int32` in TDLib (`client.File.Id`):
  the app layer validates range `[0, MaxInt32]` and maps overflow to
  `InvalidArgument` (transport) — `FileId=0` is also rejected as
  `InvalidArgument` (transport-level request validation, same style as the
  existing `SendMessage`/`SendMessageAlbum` empty checks in `transport.go`).
- `mime_type` comes from the resolved message content (§6.5), not an extra
  MIME-guessing RPC; a missing or empty content field maps to the empty
  string, never an error for `GetMessageFile`.

## 6. App-layer contract (D6)

`internal/app/facade/service.go`:

- `GetMessageFile(ctx, chatID, messageID) (*client.File, string /*mime*/, error)`:
  1. `s.telegramRepo.GetMessage(&client.GetMessageRequest{ChatId, MessageId})`
     (existing interface method — no new dep for this).
  2. Resolve the file per the §2 table:
     - `*client.MessagePhoto` → select the fixed largest variant from
       `Photo.Sizes[].Photo` by max `Photo.Size` (ties → last in slice order;
       §1). There is no client-selectable variant parameter (N8): the
       selection is always this default. nil `Sizes` or all-nil entries → error.
     - other media types → the file object named in the §2 table's `file object`
       column.
     - nil file object (e.g. `Photo.Sizes` entry with nil `Photo`) → error.
     - non-media content (text, contact, location, …) →
       `*facade.MessageFileNotFoundError` (new typed error, §7).
  3. Download synchronously:
     `s.telegramRepo.DownloadFile(&client.DownloadFileRequest{FileId: file.Id, Priority: 16, Synchronous: true})`.
     [ASSUMPTION] `Priority: 16` (mid of the 1-32 range).
  4. Success oracle (normative): the returned `*client.File` counts as
     downloaded **iff** `f != nil && f.Local != nil &&
     f.Local.IsDownloadingCompleted && f.Local.Path != ""`. Any other shape
     (incl. error return) → `*facade.FileDownloadFailedError` (new, §7)
     wrapping the raw cause. The oracle is mechanical and red-path-testable.
  5. `mime_type`: read directly from the already-resolved message content
     (zero extra RPC). Source mapping:

     | content type       | mime source |
     |--------------------|-------------|
     | `MessagePhoto`     | `"image/jpeg"` [ASSUMPTION] |
     | `MessageVideo`     | `content.Video.MimeType` |
     | `MessageDocument`  | `content.Document.MimeType` |
     | `MessageAudio`     | `content.Audio.MimeType` |
     | `MessageAnimation` | `content.Animation.MimeType` |
     | `MessageVoiceNote` | `content.VoiceNote.MimeType` |
     | `MessageSticker`   | `""` |
     | `MessageVideoNote` | `""` |

     A nil or empty source field returns `""` (§5). [ASSUMPTION]
     `MessagePhoto` is always JPEG because its `PhotoSize` variants have no
     MIME field and photo variants are JPEG.

- `DownloadFile(ctx, fileID int32) (*client.File, error)`:
  passthrough of step 3+4 above for a caller-supplied `file_id`
  (the `GetMessageFile` response `file_id` is the intended input).
  Success oracle identical (§6.4).

- Interface growth: the partial `telegramRepo` interface (`service.go:18-36`)
  gains only `DownloadFile` (same 1:1 go-tdlib signature as `clientAdapter`).
  The `facadeService` interface in
  `transport.go:90+` gains `GetMessageFile` and `DownloadFile`.

- `Priority`/`Offset`/`Limit` are fixed at the app layer (no proto knobs —
  YAGNI, and the non-goal N2 excludes progress-style controls).

## 7. Errors (D4/D6)

New typed errors in `internal/app/facade/errors.go` (style of
`MessageNotFoundError`/`TargetNotBotError`, `errors.go:1-25`):

- `MessageFileNotFoundError{ChatID, MessageID int64}` — message exists but its
  content carries no downloadable file (or its file object is nil).
  gRPC mapping: `codes.NotFound` (add a `facade.MessageFileNotFoundError`
  branch in `mapFacadeError`, `transport.go:39`, before the generic
  `Internal` fallthrough).
- `FileDownloadFailedError{FileID int32; Err error}` — download did not reach
  the §6.4 success oracle (TDLib error, incomplete local copy, canceled).
  gRPC mapping: `codes.Internal`. [ASSUMPTION] no dedicated code — TDLib's
  synchronous `downloadFile` failures are not uniformly classifiable by code
  (e.g. 502 applies to callback answers, not files); `Internal` matches the
  existing "everything else" behavior of `mapFacadeError`.

Existing `MessageNotFoundError` (message itself missing) keeps its
`NotFound` mapping; the two new errors are distinct and tested distinctly.

## 8. Infra-layer contract (D6)

`internal/infra/telegram/client_adapter.go`:

- `clientAdapter` interface gains:
  - `DownloadFile(*client.DownloadFileRequest) (*client.File, error)`
- `Repo` implements them as **bare passthroughs** (no `withChatReady`):
  download/file-state operations are not chat-scoped
  ("400 Chat not found" cannot occur for a file id), matching the existing
  chat-less wrappers `GetMe`/`GetUser`/`GetOption`
  (`client_adapter.go:367-437`). `Repo.DownloadFile` wraps errors with
  `FloodWaitFromError` for uniformity with the other wrappers (it is a no-op
  for non-flood errors). [ASSUMPTION] the `FloodWaitFromError` wrap is
  defensive only; flood-wait on downloadFile is not expected by TDLib.

## 9. Tests (D6 — every A is mechanically falsifiable)

- `internal/app/facade/service_test.go` (mockery `TelegramRepo` mock, strict):
  - A1 per media type, covering the 8 content types of the §2 table
    (photo/video/document/audio/voiceNote/animation/videoNote/sticker):
    `GetMessage` returns the content; `DownloadFile` is called with
    `FileId` = the resolved id and `Synchronous=true`; response path/size come
    from the returned `File` (oracle shape: `Local.IsDownloadingCompleted=true`,
    non-empty `Path`). Each fixture's content is built with the file object
    named in the §2 table's `file object` column, so the per-type resolution is
    exercised against the dependency's actual fields. Photo: 3-size variant
    with a non-obvious largest (e.g. sizes 100/300/200) → asserts max-by-Size
    selection, not slice order. No test exercises client-selectable variant
    selection — it is out of scope (N8); only the fixed largest-variant
    default is asserted.
  - A1 MIME source: document asserts `mime_type == fixture.Document.MimeType`;
    video/audio/animation/voiceNote assert the corresponding content MIME
    field; sticker and videoNote assert `""`; photo asserts `"image/jpeg"`.
    A nil or empty content MIME field also asserts `""` with download success.
  - A2: text message → `*MessageFileNotFoundError` (assert type + fields);
    photo with `Sizes=[]` → same error; photo entry with nil `Photo` → same.
  - A3: `DownloadFile` with oracle shape (completed, path set) → success;
    `Local.IsDownloadingCompleted=false` → `*FileDownloadFailedError`;
    error from repo → `*FileDownloadFailedError` wrapping it.
  - Strict-mock discipline (quality-guidelines F1/F3): unexpected
    `GetChatHistory`/`GetMessage` calls fail the test; red-path noted in a
    one-line comment where the binding is subtle (quality-guidelines R3).
- `internal/transport/grpc/transport_test.go`:
  - A4 covered structurally: no existing RPC behavior change (existing tests
    untouched and green); new RPC validation: `DownloadFile` with `file_id=0`
    or `file_id > MaxInt32` → `InvalidArgument`, repo not called.
  - Error mapping: service returns `MessageFileNotFoundError` → `NotFound`;
    `FileDownloadFailedError` → `Internal`; plain error → `Internal`.
  - Success mapping: raw `*client.File` → `MessageFileResponse`
    (file_path/size/file_id/mime_type) and `FileResponse` (file_path/size).
- Mocks: `internal/app/facade/mocks/telegram_repo.go` and
  `internal/transport/grpc/mocks/facade_service.go` are mockery-generated.
  Regenerate if mockery v3 + protoc toolchain are available locally; otherwise
  hand-add the new methods copying the exact generated style (as 11-05 did for
  `GetChat`). The generated file's diff must be limited to the new methods.
- `docs/TEST-MATRIX.md`: add rows under App / Facade (`SVC-U-004` section) and
  Transport / gRPC (`GRPC-U-001` section) with the new test counts; update the
  summary table if it lists per-package totals.

## 10. Proto regeneration (process contract)

Tools (must match the headers of the committed generated files):
`protoc` v7.34.2 (reporting `34.x`), `protoc-gen-go v1.36.11`,
`protoc-gen-go-grpc v1.5.1` (already pinned: `Taskfile.yml` `proto` task;
generated headers `facade.pb.go:2-4`, `facade_grpc.pb.go:2-3`).
Command (repo root):

```
protoc -I . --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  internal/transport/grpc/pb/facade.proto
```

Byte-identity rule: regenerating the UNCHANGED baseline proto must reproduce
the committed `facade.pb.go`/`facade_grpc.pb.go` exactly (proves toolchain
parity before touching the proto). After the additive edit, the diff of the
generated files must be additions only.
## 11. File set (scope — only these)

1. `internal/transport/grpc/pb/facade.proto` (+ 2 RPCs, +4 messages, §5)
2. `internal/transport/grpc/pb/facade.pb.go`, `facade_grpc.pb.go` (regenerated)
3. `internal/app/facade/service.go` (2 methods + 1 interface method)
4. `internal/app/facade/errors.go` (2 typed errors)
5. `internal/app/facade/mocks/telegram_repo.go` (new mock methods only)
6. `internal/app/facade/service_test.go` (new tests)
7. `internal/infra/telegram/client_adapter.go` (1 adapter method + 1 Repo passthrough)
8. `internal/infra/telegram/repo_test.go` (passthrough tests, if the file's
   pattern covers chat-less wrappers; else skip — design accepts either,
   the contract is in client_adapter.go)
9. `internal/transport/grpc/transport.go` (2 RPC impls + 2 `mapFacadeError` branches
    + 2 `facadeService` interface methods)
10. `internal/transport/grpc/mocks/facade_service.go` (new mock methods only)
11. `internal/transport/grpc/transport_test.go` (new tests)
12. `docs/TEST-MATRIX.md` (rows + summary)

Out of scope: anything not listed (no `service/album`, no engine, no GraphQL,
no Dockerfile/CI changes, no `inputMessageByFileExt` changes, no
client-selectable photo variant selection (N8)).

## 12. Compatibility and rollout

- Wire compatibility: proto3 additive (new RPCs + new messages). Existing
  clients and the engine are unaffected; the server accepts all old calls
  unchanged. No rollout/rollback beyond a normal branch merge (additive code;
  rollback = revert the merge).
- Behavior compatibility: `GetMessage` (used by `GetMessageFile`) is already a
  live path; download runs on the facade's single TDLib session
  (`cmd/facade/main.go:80-94`) — same session that serves all other RPCs, no
  second session (mirrors the "no second TDLib session" non-goal of 11-05).

## 13. Verification (normative for P2/P3 gates)

- Local (Windows, this box): `gofmt -l` on changed files; proto
  byte-identity per §10; `go vet` on the non-cgo packages is NOT possible
  (go-tdlib needs linux/darwin cgo) — do not attempt `go build`/`go test`
  locally except packages that do not import go-tdlib (none of the touched
  ones qualify; skip local build entirely).
- CI (authoritative): branch push to `origin` triggers `ci-test.yml`
  (`go vet ./...` + `go test ./internal/...`, Linux, real TDLib static libs,
  `.github/workflows/ci-test.yml`). Green run on the branch tip is the gate.
  CI has a cached TDLib artifact (`tdlib-22d49d5-amd64`) so runs are minutes,
  not the 20-25 min cold build.
- Red-path requirement (quality-guidelines F2): before the final commit, the
  implementer must prove at least one new test can fail (e.g. temporarily
  break the photo variant selection or the success oracle, observe the red,
  restore) and record it in `responses.json`.








