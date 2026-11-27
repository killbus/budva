# Independent review brief (P3) — 11-25 facade-file-download

## Base

- base_commit: `fbc22cc`
- worktree: `D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download`
- branch: `feat/facade-file-download`
- task path: `.trellis/tasks/11-25-facade-file-download`
- normative: `prd.md`, `design.md` (§2-§9 are the contract), `approval.md`
  (`design_sha256`), `.trellis/spec/**`
- object under review: `git diff fbc22cc`

## Production entry (anchors)

- `cmd/facade/main.go:94` — `facadeService := facade.New(telegramRepo)`
- `cmd/facade/main.go:110-111` — gRPC server registers
  `tgrpc.New(facadeService)`; every RPC reaches
  `internal/transport/grpc.Transport` -> `internal/app/facade.Service` ->
  `internal/infra/telegram.Repo` -> go-tdlib `client`.
- No auth interceptor (baseline fact).

## Blocking criteria

1. **Additivity/compatibility** (prd A4, design §5/§10) — `facade.proto`
   diff is additions only: 2 new RPCs on the existing `FacadeGRPC` service,
   4 new messages (`GetMessageFileRequest`, `MessageFileResponse`,
   `DownloadFileRequest`, `FileResponse`) with the §5 field numbers. All 16
   existing RPCs and every existing message field number/name/semantic
   unchanged; `api/proto/facade.proto` (symlink) untouched. Generated
   bindings match the `.proto` and regenerate byte-identically with the
   documented toolchain (libprotoc 34.2 / protoc-gen-go v1.36.11 /
   protoc-gen-go-grpc v1.5.1).
2. **Non-goals honored** (design §2 N1-N8) — no streaming RPCs; no
   progress/updateFile push; no upload-side change
   (`SendMessage`/`SendMessageAlbum`/`inputMessageByFileExt` untouched);
   no GraphQL schema change; no engine/ruleset change; no
   download-management RPCs; dead `Message.file_path` field neither removed
   nor populated.
3. **Layering** (design §3/§4) — `transport/grpc` maps raw TDLib values to
   proto only (no business logic in RPC handlers beyond request validation +
   mapping + `mapFacadeError`); `app/facade` owns resolution/variant
   selection/download orchestration and returns raw `*client.File`
   (never pb); `infra/telegram` additions are 1:1 passthroughs (no
   `withChatReady`, matching the chat-less `GetMe`/`GetUser`/`GetOption`
   pattern at `client_adapter.go:367-437`); no reverse dependency (app does
   not import pb; infra does not import app/pb); no test-only branch in
   production code.
4. **Resolution contract** (design §6) — `GetMessageFile`: `GetMessage`
   first; photo -> max `PhotoSize.Photo.Size` variant (ties -> last in
   slice order); video/document/audio/voice/animation/videoNote/sticker per
   the §2 table; nil/absent file object or non-media content ->
   `*facade.MessageFileNotFoundError` (never an empty-path success).
5. **Download contract** (design §6) — `DownloadFile` request built with
   `Synchronous: true` and `Priority: 16`; success oracle is exactly
   `f != nil && f.Local != nil && f.Local.IsDownloadingCompleted &&
   f.Local.Path != ""`; any other shape or error ->
   `*facade.FileDownloadFailedError` wrapping the cause; `mime_type` is
   best-effort: mime from resolved content fields (§6.5), missing/empty ->
   `""`, never fails the RPC.
   transport-level `file_id` validation: `0` or `> math.MaxInt32` ->
   `codes.InvalidArgument` without calling the service.
6. **Error mapping** (design §7) — `*facade.MessageFileNotFoundError` ->
   `codes.NotFound`; `*facade.FileDownloadFailedError` -> `codes.Internal`;
   both branches before the generic Internal fallthrough; existing
   `MessageNotFoundError` (NotFound), `FloodWaitError`, `ChatNotReadyError`,
   etc. mappings unchanged.
7. **Test evidence** (prd A5, design §9) — new tests actually bind A1-A4:
   per-media-type resolution (photo largest-variant fixture NOT trivially
   first/last), oracle true/false shapes, error-type assertions (not just
   non-nil), strict-mock expectations (unexpected calls fail), no existing
   assertion deleted/weakened/reshaped; red-path evidence present in
   `responses.json` test_notes.
8. **Scope** (design §11) — only the 12 listed files changed (proto,
   generated pb x2, app service/errors/mocks/tests, infra adapter,
   transport + mocks + tests, TEST-MATRIX) plus exactly ONE documented
   forced companion: internal/infra/telegram/mocks/client_adapter.go,
   whose diff must be additions-only (the DownloadFile mock method;
   required because the clientAdapter interface gained that method and
   the package must compile — flagged in progress.md, not a scope
   expansion). No unrelated refactor, no other file outside the set.
