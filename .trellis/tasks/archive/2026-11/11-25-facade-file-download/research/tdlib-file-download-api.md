# Research — TDLib/go-tdlib v0.7.6 file-download API + baseline anchors

Verified against `go-tdlib v0.7.6` (module cache, `go.mod:17`) and baseline
`fbc22cc`. All line numbers are from the pinned dependency unless prefixed
`repo:`.

## 1. go-tdlib client methods (the capability already exists)

- `func (client *Client) GetFile(req *GetFileRequest) (*File, error)` —
  `function.go:1614`. `GetFileRequest{FileId int32}`.
- `func (client *Client) DownloadFile(req *DownloadFileRequest) (*File, error)` —
  `function.go:11924`. Request (`function.go:11909-11923`):
  - `FileId int32` — "Identifier of the file to download"
  - `Priority int32` — 1-32; higher = earlier; ties → later request first
  - `Offset int64` — start position
  - `Limit int64` — bytes to download; 0 = no limit
  - `Synchronous bool` — "Pass true to return response only after the file
    download has succeeded, has failed, has been canceled, or a new
    downloadFile request with different offset/limit parameters was sent;
    pass false to return file state immediately"
  - doc: "Downloads a file from the cloud. Download progress and completion
    of the download will be notified through updateFile updates"
- `func (client *Client) GetFileMimeType(req *GetFileMimeTypeRequest) (*Text, error)` —
  `function.go:6394`.
- Management API exists but is a non-goal (N6): `CancelDownloadFile`
  (`function.go:11985`), `AddFileToDownloads` (`:12256`),
  `ToggleDownloadIsPaused` (`:12287`), `SearchFileDownloads` (`:12409`),
  `GetFileDownloadedPrefixSize` (`:11956`).

## 2. Types

- `File` (`type.go:4277-4289`): `Id int32`, `Size int64` (0 if unknown),
  `ExpectedSize int64`, `Local *LocalFile`, `Remote *RemoteFile`.
- `LocalFile` (`type.go:4209-4226`): `Path string` ("may be empty"),
  `CanBeDownloaded bool`, `CanBeDeleted bool`, `IsDownloadingActive bool`,
  `IsDownloadingCompleted bool` ("True, if the local copy is fully
  available"), `DownloadOffset int64`, `DownloadedPrefixSize int64`,
  `TotalSize int64` (progress only).
- Success oracle (design §6.4): `Local != nil &&
  Local.IsDownloadingCompleted && Local.Path != ""`.

## 3. Message content → downloadable file

`type.go` (v0.7.6):

| content | line | media field | file |
|---|---|---|---|
| `MessagePhoto` | 23922 | `Photo *Photo` (23925) | `Photo.Sizes []*PhotoSize` (5370); `PhotoSize{Type string, Photo *File, Width, Height int32, ProgressiveSizes []int32}` (4420-4432) |
| `MessageVideo` | 23986 | `Video *Video` (23989) | `Video.Video *File` (`type.go:5470`) |
| `MessageDocument` | 23837 | `Document *Document` (23840) | `Document.Document *File` (5328); `Document.FileName/MimeType/Thumbnail` (5317-5329) |
| `MessageAudio` | 23808 | `Audio *Audio` (23811) | `Audio.Audio *File` (`type.go:5297`) |
| `MessageVoiceNote` | 24054 | `VoiceNote *VoiceNote` (24057) | `VoiceNote.Voice *File` (`type.go:5565`) |
| `MessageAnimation` | 23773 | `Animation *Animation` (23776) | `Animation.Animation *File` (`type.go:5258`) |
| `MessageVideoNote` | 24023 | `VideoNote *VideoNote` (24026) | `VideoNote.Video *File` (`type.go:5505`) |
| `MessageSticker` | 23957 | `Sticker *Sticker` (23960) | `Sticker.Sticker *File` (`type.go:5394`) |

Caption fields (`*FormattedText`) exist on photo/video/document/audio/
animation — same pattern `clientMessageToProto` already switches on
(`repo:internal/transport/grpc/transport.go:349-374`).

Note: `Message` also has `Content interface{}` (TDLib `MessageContent`
oneof). Non-media contents (MessageText, MessageContact, MessageLocation,
MessageDice, MessagePoll, MessageChatCreate, …) carry no file.

## 4. Baseline code anchors (repo @ fbc22cc)

- `cmd/facade/main.go:80` — `telegramRepo := telegram.New(...)`;
  `:94` — `facadeService := facade.New(telegramRepo)` (whole Repo, so new
  Repo methods are visible to the facade service automatically — no wiring
  change); `:110-111` — gRPC transport registration.
- `internal/app/facade/service.go:18-36` — partial `telegramRepo` interface
  (20 methods; `GetMessage` already in it at `:21`).
- `internal/app/facade/service.go:47-50` — `GetMessage` service method
  (1:1 passthrough, the template for the new methods).
- `internal/app/facade/errors.go:1-25` — typed-error style
  (`MessageNotFoundError`, `TargetNotBotError` + `New*` constructors).
- `internal/infra/telegram/client_adapter.go:9-68` — `clientAdapter`
  interface (1:1 go-tdlib signatures, no ctx/domain types; "Method is added
  here only when it has a live consumer" doc rule — this ticket is the
  consumer); `:60-63` — static assertion
  `var _ clientAdapter = (*Repo)(nil)`.
- `internal/infra/telegram/client_adapter.go:134-157` — chat-scoped wrapper
  pattern (`withChatReady` + FloodWait wrap); `:367-437` — chat-less bare
  passthroughs (`GetMe`, `GetUser`, `GetOption`) — the pattern for the new
  file wrappers (file ops are not chat-scoped).
- `internal/transport/grpc/transport.go:39-84` — `mapFacadeError`
  (typed-error → gRPC-code mapping; new branches slot before the `Internal`
  fallthrough at `:83`); `:90-113` — `facadeService` interface.
- `internal/transport/grpc/pb/facade.proto` — 16 RPCs (lines 8-23),
  `Message` (27-33, dead `file_path` field 5), `NewMessage` (35-39).
  Generated: `facade.pb.go` header `protoc v7.34.2 / protoc-gen-go
  v1.36.11`; `facade_grpc.pb.go` header `protoc-gen-go-grpc v1.5.1`.
- `api/proto/facade.proto` is a SEPARATE tracked file (git mode 120000
  indicates symlink in the index; in this Windows checkout it materializes as
  a one-line text file `../../internal/transport/grpc/pb/facade.proto`).
  Both tree paths must carry identical contract content after the edit.
- Mocks (mockery-generated, `.mockery.yml`): `internal/app/facade/mocks/
  telegram_repo.go` (`type TelegramRepo` :27, methods alphabetical, e.g.
  `GetChat` :1024, `GetMessage` :342); `internal/transport/grpc/mocks/
  facade_service.go` (`type FacadeService` :30, methods e.g. `GetMessage`
  :254, `SendBotCommand` :1035). 11-05 precedent: hand-adding a mock method
  in exact generated style is acceptable when regeneration is unavailable.
- `docs/TEST-MATRIX.md`: App / Facade section `:179-187` (`SVC-U-004` row
  `:185`: "14 tests"); Transport / gRPC section `:231-240` (`GRPC-U-001`
  `:237`: "37 tests"); summary table `:406`.

## 5. Verification channel facts

- Windows: `go build ./internal/app/facade/` fails at go-tdlib cgo
  (build tags `linux || darwin` only; verified 2026-11-25, error `undefined:
  JsonClient` etc.). No Docker, no WSL distribution on this box.
- `debian` SSH host (previous loop's Linux box, 192.168.216.3): connection
  timed out 2026-11-25 — NOT available for this loop.
- CI: `.github/workflows/ci-test.yml` — `go vet ./...` + `go test
  ./internal/...` on Linux; TDLib static libs cached under
  `tdlib-22d49d5-amd64` (pin matches Dockerfile:15); runs minutes on cache
  hit. Trigger matrix: `push: branches [main]`, `pull_request: branches
  [main]`, `workflow_dispatch`. A direct feature-branch push does NOT trigger
  this workflow; a PR to main (or workflow_dispatch) does. PRACTICAL
  IMPLICATION: the orchestrator/owner creates the PR (gh CLI is
  unauthenticated on this box; git push to origin works) or triggers
  workflow_dispatch once a token exists. Loop agents push the branch only.
- git: `git push origin <branch>` works (credential manager, verified
  2026-11-25 with ls-remote). `gh` CLI unauthenticated (no token in env, no
  hosts.yml).

## 6. TDLib download semantics notes

- `downloadFile` with `synchronous=true` returns `file` whose
  `local.is_downloading_completed` is true on success; on failure TDLib
  rejects the promise — the go-tdlib wrapper surfaces that as
  `(*File, error)` with `error != nil`. Our oracle (design §6.4) handles
  both shapes.
- The file lands in the TDLib `files_path` (from `SetTdlibParameters`,
  `repo:internal/infra/telegram/repo.go:349+` — the facade's own storage
  files dir). The returned `local.path` is absolute on the facade host — the
  path-based contract (N1) depends on the client sharing that filesystem
  (same assumption `SendMessageAlbum.file_path` already makes).
- No `updateFile` subscription needed for the synchronous call — the
  response IS the completion signal (non-goal N2 holds).
