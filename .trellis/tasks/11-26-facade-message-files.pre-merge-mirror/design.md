# Facade message file enumeration

## 1. Decision and sources

Add a unary `ListMessageFiles(chat_id, message_id)` that describes one message, then reuse `DownloadFile(file_id)` for progressive byte transfer (PRD R1-R3). Preserve GetMessageFile as a convenience selecting its existing main file (R5). A media group is one logical photo/video/etc.; files within it are representations or auxiliaries, not additional message IDs.

Baseline: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`. SDK facts and production anchors are in `research/tdlib-message-files.md`; normative constraints are PRD R1-R6 and `.trellis/spec/backend/telegram-tdlib-guidelines.md` (raw adapters, chat readiness, no business subscriber) and `quality-guidelines.md` (test/red-path discipline).

**[ASSUMPTION S1]** The exact additive schema, enum values, order, snapshot/error/metadata policies in sections 4-8 are proposed API decisions implementing those requirements; they are not existing Telegram or owner contracts. Coverage follows the explicitly proposed scope in the PRD. These assumptions are visible for owner review before implementation.

## 2. Scope and non-goals

Enumerate exactly the direct file references in section 6 for the eight baseline media types plus MessagePaidMedia. No recursive traversal, sibling-album expansion, history search, embedded link/reply/forward/story/game/invoice media, paid-media unlocking, inline minithumbnail bytes, HLS playback/rewriting, new stream format, batching/ZIP, persistent file tokens, SDK upgrade, download engine changes, or server-local paths (R1-R6 and PRD non-goals).

HLS entries expose the playlist file itself. Downloading it does not fetch or rewrite its referenced segments. The alternative's VIDEO entry is the ordinary file choice. Listing is neither a downloadability guarantee nor an access bypass.

## 3. Responsibilities and dependency direction

| Module | Single primary responsibility |
| --- | --- |
| internal/app/facade/message_files.go (new) | Resolve a message snapshot into semantic media/file DTOs |
| internal/transport/grpc/message_files.go (new) | Validate/map the unary wire contract to that app operation |
| internal/infra/telegram (unchanged) | Supply raw GetMessage with existing readiness/error behavior |
| internal/transport/grpc/pb | Generated/public wire contract |

Production path remains `cmd/facade -> grpc.Transport -> facade.Service -> telegram.Repo.GetMessage -> clientAdapter`. Transport may import app DTOs; app may use raw TDLib types through its existing partial repository interface. App/infra must not import protobuf or transport; infra must not know enumeration DTOs. No direct SDK/file I/O from transport; no new repository method, listener, worker, channel, or business-update subscription. DTOs follow the already established app-owned streaming DTO precedent.

## 4. Additive wire contract

**[ASSUMPTION S2]** Append the following declarations to `internal/transport/grpc/pb/facade.proto`. Add exactly one RPC; preserve all 17 baseline RPCs and existing fields verbatim. Names/numbers below are normative for implementation; no second proto source.

```proto
rpc ListMessageFiles(ListMessageFilesRequest) returns (ListMessageFilesResponse);

message ListMessageFilesRequest {
  int64 chat_id = 1;
  int64 message_id = 2;
}

message ListMessageFilesResponse {
  int64 chat_id = 1;
  int64 message_id = 2;
  int64 media_album_id = 3;
  bool content_supported = 4;
  repeated MessageMediaItem items = 5;
  int64 default_file_id = 6;
}

message MessageMediaItem {
  int32 item_index = 1;
  MessageMediaKind kind = 2;
  MessageMediaState state = 3;
  repeated MessageFileDescriptor files = 4;
}

message MessageFileDescriptor {
  int64 file_id = 1;
  MessageFileRole role = 2;
  int32 source_index = 3;
  string file_name = 4;
  string mime_type = 5;
  int64 size = 6;
  bool size_known = 7;
  int32 width = 8;
  int32 height = 9;
  string codec = 10;
  string photo_size_type = 11;
}

enum MessageMediaKind {
  MESSAGE_MEDIA_KIND_UNSPECIFIED = 0;
  MESSAGE_MEDIA_KIND_PHOTO = 1;
  MESSAGE_MEDIA_KIND_VIDEO = 2;
  MESSAGE_MEDIA_KIND_DOCUMENT = 3;
  MESSAGE_MEDIA_KIND_AUDIO = 4;
  MESSAGE_MEDIA_KIND_VOICE_NOTE = 5;
  MESSAGE_MEDIA_KIND_ANIMATION = 6;
  MESSAGE_MEDIA_KIND_VIDEO_NOTE = 7;
  MESSAGE_MEDIA_KIND_STICKER = 8;
  MESSAGE_MEDIA_KIND_PREVIEW = 9;
  MESSAGE_MEDIA_KIND_UNSUPPORTED = 10;
}

enum MessageMediaState {
  MESSAGE_MEDIA_STATE_UNSPECIFIED = 0;
  MESSAGE_MEDIA_STATE_FILES_PRESENT = 1;
  MESSAGE_MEDIA_STATE_LOCKED = 2;
  MESSAGE_MEDIA_STATE_UNSUPPORTED = 3;
  MESSAGE_MEDIA_STATE_NO_FILES = 4;
}

enum MessageFileRole {
  MESSAGE_FILE_ROLE_UNSPECIFIED = 0;
  MESSAGE_FILE_ROLE_MAIN = 1;
  MESSAGE_FILE_ROLE_PHOTO_SIZE = 2;
  MESSAGE_FILE_ROLE_ALTERNATIVE_VIDEO = 3;
  MESSAGE_FILE_ROLE_HLS_PLAYLIST = 4;
  MESSAGE_FILE_ROLE_THUMBNAIL = 5;
  MESSAGE_FILE_ROLE_ALBUM_COVER = 6;
  MESSAGE_FILE_ROLE_EXTERNAL_ALBUM_COVER = 7;
  MESSAGE_FILE_ROLE_PREMIUM_ANIMATION = 8;
}
```

Enum UNSPECIFIED values are zero-value wire placeholders and are never emitted by this server. Clients must tolerate future values. Chat IDs may be negative, but not zero; message IDs must be positive, in TDLib's ID format, exactly as existing GetMessageFile. Proto JSON renders int64 fields as strings.

## 5. Snapshot, grouping, order and metadata

**[ASSUMPTION S3]** Listing is one GetMessage snapshot, with no cache or new file availability calls. Echo the requested chat/message IDs; copy `int64(msg.MediaAlbumId)` (0 means no album). Do not fetch album siblings.

- Ordinary supported content produces one item with item_index=0, even when file references are absent. Its kind follows the content type. State is FILES_PRESENT if at least one valid file reference is emitted, otherwise NO_FILES.
- MessagePaidMedia produces one item per original Media slot in order, item_index equal to that slice index. Photo/video use the same extraction rules as ordinary photo/video (paid video has no alternatives). A nonnil PaidMediaPreview is PREVIEW/LOCKED with files empty. PaidMediaUnsupported, nil/typed-nil paid slots, or an unrecognized paid subtype are UNSUPPORTED/UNSUPPORTED with files empty. Recognized nonnil paid photo/video with nil nested data is its proper kind/NO_FILES. No paid slots are dropped or unlocked.
- content_supported=true for those nine recognized top-level content types, including typed-nil wrappers. A typed-nil ordinary wrapper yields its kind/NO_FILES; typed-nil MessagePaidMedia has no items. Text, nil content and any other top-level type yield content_supported=false, items empty. This is successful inspection, not a missing-message error.
- Emit file entries in section 6 order. A nil File or Id<=0 is omitted. Nil array elements are skipped for files, but source_index retains the original source slice index. Scalar references use source_index=0. Arrays are never sorted.
- **Preserve duplicate file IDs** in different roles, source slots or media items. Identity of an occurrence is (item_index, role, source_index), not file_id. For ALTERNATIVE_VIDEO/HLS_PLAYLIST, equal source_index within the same item pairs the two references. There is at most one occurrence for each such tuple. These indices are snapshot positions, not persistent IDs.
- default_file_id is the positive ID returned by the existing `resolveMessageFile(msg)` only if present in the emitted entries; otherwise 0. Use that existing pure resolver without altering it. Paid/unsupported content has default_file_id=0. Do not invent a default for multi-item paid media. A duplicate default ID may occur more than once; the field identifies the file, not one occurrence.
- file_id uses the positive raw TDLib int32 ID widened to int64; feed it to existing DownloadFile in the same facade account/session namespace. List again after message edits/session changes; later download can fail normally. No reserved download, capability token, or immutable snapshot is promised.

**[ASSUMPTION S4]** `size=File.Size,size_known=true` only when Size>0; otherwise size=0,size_known=false. Do not use ExpectedSize or local downloaded bytes as actual size. An unknown or empty file size remains unknown in this metadata-only endpoint. Empty strings mean absent/unknown, width/height 0 mean unknown. Copy known source dimensions, clamped at 0; do not infer a filename, extension, MIME, or codec from another representation or path.

FileName/MimeType come from the specific main wrapper where present. Photo-size MIME is image/jpeg, matching the baseline resolver's assumption; copy PhotoSize.Type to photo_size_type. VideoNote/sticker MIME is empty. Thumbnail/cover, alternative-video/HLS, and premium-animation filename/MIME are empty because this contract does not guess them; they remain selectable by role, dimensions, codec and ID. `codec` is copied only from the corresponding AlternativeVideo for both paired entries. `photo_size_type` is empty except on PHOTO_SIZE. All unspecified scalar fields are zero/empty. Never copy File.Local.Path, Remote data, minithumbnail bytes, captions, or payment metadata.

## 6. Exact extraction table

Read actual pinned fields (research table), not an assumed universal `.File`. MAIN denotes the primary file within the media item, not necessarily the whole message's default.

| Content / item kind | File occurrences in emitted order | Metadata sources beyond File.Size/Id |
| --- | --- | --- |
| MessagePhoto / PHOTO | Photo.Sizes[index].Photo as PHOTO_SIZE | each size Width/Height/Type, JPEG MIME |
| MessageVideo / VIDEO | Video.Video MAIN; Video.Thumbnail.File THUMBNAIL; then each AlternativeVideos[index]: Video ALTERNATIVE_VIDEO followed by HlsFile HLS_PLAYLIST | main Video.FileName/MimeType/Width/Height; thumbnail dimensions; alternative dimensions/Codec for both paired entries |
| MessageDocument / DOCUMENT | Document.Document MAIN; Document.Thumbnail.File THUMBNAIL | main FileName/MimeType; thumbnail dimensions |
| MessageAudio / AUDIO | Audio.Audio MAIN; Audio.AlbumCoverThumbnail.File ALBUM_COVER; Audio.ExternalAlbumCovers[index].File EXTERNAL_ALBUM_COVER | main FileName/MimeType; each cover dimensions |
| MessageVoiceNote / VOICE_NOTE | VoiceNote.Voice MAIN | MimeType only |
| MessageAnimation / ANIMATION | Animation.Animation MAIN; Animation.Thumbnail.File THUMBNAIL | main FileName/MimeType/Width/Height; thumbnail dimensions |
| MessageVideoNote / VIDEO_NOTE | VideoNote.Video MAIN; VideoNote.Thumbnail.File THUMBNAIL | main Width=Height=Length; thumbnail dimensions |
| MessageSticker / STICKER | Sticker.Sticker MAIN; Sticker.Thumbnail.File THUMBNAIL; if FullType is nonnil StickerFullTypeRegular, its PremiumAnimation PREMIUM_ANIMATION | main sticker Width/Height; thumbnail dimensions; premium-animation dimensions unknown |
| MessagePaidMedia / per-slot PHOTO or VIDEO | PaidMediaPhoto.Photo as photo row; PaidMediaVideo.Video as primary-video + thumbnail part only | same policies as above |

Skip nil nested wrappers without panic, preserving the item rules of section 5. For MessageVideo, a missing primary Video wrapper does not suppress valid AlternativeVideos. Inline Minithumbnail and PhotoSize.ProgressiveSizes are never extra entries. No file-list truncation or pagination is introduced: it describes only one finite TDLib message object and excludes embedded payload bytes (R1/R3).

## 7. Application contract and errors

**[ASSUMPTION S5]** Add `func (s *Service) ListMessageFiles(ctx context.Context, chatID, messageID int64) (*MessageFiles, error)` in the new app file. MessageFiles contains ChatID, MessageID, MediaAlbumID int64, ContentSupported bool, Items []MessageMediaItem, DefaultFileID int32. MessageMediaItem has ItemIndex int32, Kind MessageMediaKind, State MessageMediaState, Files []MessageFileDescriptor. Descriptor uses FileID int32, Role MessageFileRole, SourceIndex int32, FileName/MIMEType/Codec/PhotoSizeType string, Size int64, SizeKnown bool, Width/Height int32. Three app-owned int32 enum types have named constants corresponding to section 4; no protobuf imports. Transport performs explicit enum and field mapping.

1. Check ctx.Err before repository call; if set, return it and make no call.
2. Call existing telegramRepo.GetMessage exactly once with the requested IDs. Its internal readiness retries remain its own concern.
3. Check ctx.Err after the SDK call; a canceled/deadline context wins over its result. The synchronous SDK call is not interruptible by this context; do not add a detached goroutine or promise prompt abort of in-flight SDK work.
4. Return any repository error unchanged. If message is nil with no error, return existing MessageNotFoundError (use its baseline constructor). Do not classify every TDLib 400 as missing.
5. Extract DTOs using sections 5-6 without further repository calls; check context once more before returning the completed snapshot.

**[ASSUMPTION S6]** New unary handler validates request IDs before calling the app; invalid IDs -> InvalidArgument. For this handler only, errors.Is(context.Canceled/DeadlineExceeded) maps to Canceled/DeadlineExceeded. Otherwise delegate to existing mapFacadeError: typed MessageNotFoundError -> NotFound; FloodWaitError -> ResourceExhausted + RetryInfo; ChatNotReadyError -> Unavailable + existing RetryInfo; other upstream errors -> Internal. Raw TDLib errors keep baseline classification, including unclassified 400/404; no new broad error translation. Do not modify the global mapper or reuse streaming event helpers. Nil-message NotFound differs from a valid text message's empty listing. No errors are converted to empty success.

## 8. Compatibility and user flow

All current RPCs, event types, transport error policies and stream machinery remain unchanged (R5). List uses the existing resolver solely for default_file_id; it does not route GetMessageFile through enumeration or alter that stream's supported content set.

1. Obtain actual TDLib chat/message IDs as today (e.g. GetMessageLinkInfo).
2. Call ListMessageFiles with `{ "chat_id": "-1001234567890", "message_id": "1048576" }` (illustrative IDs).
3. Choose an item's file occurrence. Photo variants share an item, paid media elements have separate items, thumbnail roles are visibly auxiliary, and a locked preview offers no file ID.
4. Call existing DownloadFile with `{ "file_id": "1279" }` (illustrative returned ID). Validate the metadata/chunk/complete stream as before and write decoded bytes on the caller's machine.

**[ASSUMPTION S7]** No new download request parameter or MIME override. DownloadFile's metadata MIME may remain empty even if ListMessageFiles supplied MIME: the listing describes message context, the existing ID-based stream does not acquire that context. The caller may keep the listing descriptor beside its downloaded file. default_file_id=0 means no existing GetMessageFile default, not no selectable files. A new client against an old image receives Unimplemented for ListMessageFiles; no automatic lossy fallback.

## 9. Verification and acceptance mapping

Use table-driven fixtures built with the actual pinned SDK fields. Expected IDs/roles/grouping come from hand-specified fixture values, not the production extractor. Keep app tests on real Service and strict repository mocks; transport unit tests may mock app, while integration uses real app and registered pb server.

| PRD acceptance | Falsifiable checks / landing point |
| --- | --- |
| A1 | App photo fixture with three distinct sizes; one GetMessage expectation and zero other allowed calls. Register the real transport in bufconn for metadata-only listing; strict unexpected calls bind the no-download rule. |
| A2 | Table for every section 6 source, exact count/ordered roles/IDs/source_index; paired HLS/video, premium animation, internal/external covers; paid photo+video+preview+unsupported+nil and typed-nil slots. Verify all group positions and empty-file states. |
| A3 | Real bufconn List -> choose a nondefault photo/alternative ID -> DownloadFile. Mock request must use that ID and Synchronous=false; payload must match that fixture, not the default. Fake upstream completion is released only after client receives the first chunk. Require complete + clean EOF and decoded total size. No actual Telegram dependency in this integration test. |
| A4 | Compare proto descriptors to baseline: exactly one extra unary RPC, no altered legacy fields/stream flags. Existing photo tie/default video/unsupported paid GetMessageFile fixtures and progressive green-red-green script stay intact; add regressions only if not already covered. |
| A5 | Fixtures for duplicate IDs across items/roles/source slots; nil/Id<=0 omission; size 0 with ExpectedSize>0 stays unknown; nil/missing MIME/dimensions; photo Type, alternative Codec; default ID agrees with baseline resolver; text vs supported-empty; album metadata with strict no-history calls. Repeating enumeration of the same immutable fixture yields the same ordering. |
| A6 | Invalid IDs fail before app invocation; canceled-before-call means no repo call; cancellation after controlled SDK return wins over a result/error; deadline and errors follow section 7. Check RetryInfo, shared raw ResponseError instance passthrough, nil message NotFound, and text OK. |
| A7 | Pinned proto regeneration hash check, source symlink unchanged, Linux go vet/full internal tests, existing streaming mutation script, published-source acceptance guide. Runtime/image/live evidence is recorded only after actual execution. |

Per quality-guidelines F1/F2, strict mocks are the red mechanism for forbidden calls, selected ID and calls/arguments; literal expected descriptor assertions bind each included field/ordering/state. For mapping/omission assertions without a strict-call failure path, perform one temporary omission/flattening mutation of the relevant enumeration output and record the exact failing test, then restore source. Existing streaming mutation remains the progressive-regression gate. Use synctest/barriers for cancellation and progressive ordering; no real sleeps. Do not add production test hooks or change infrastructure to simplify mocks.

## 10. Controlled implementation file set

- `internal/transport/grpc/pb/facade.proto`, generated `facade.pb.go`, `facade_grpc.pb.go`: append new contract only.
- New `internal/app/facade/message_files.go` and `message_files_test.go`: DTOs/extraction/service and tests. Existing `file_download.go` remains read-only dependency for its resolver.
- New `internal/transport/grpc/message_files.go` and `message_files_test.go`: unary handler/mapping/unit and bufconn tests.
- `internal/transport/grpc/transport.go`: add only the new method to facadeService; `mocks/facade_service.go`: sync one method.
- `docs/TEST-MATRIX.md`: actual executed coverage/counts, not estimated totals.
- This task's review/evidence/acceptance documents.

No new app/infra repository method or corresponding mocks; no changes to old task rulers, .trellis/spec, API symlink, download client/stream engine, CI workflow, Dockerfile, engine, ruleset, GraphQL, auth or configuration. If the implementation needs an unlisted source file, raise a concrete design gap before expanding scope. No broad refactor.

## 11. Delivery and rollback

This ticket is a separate follow-up to 11-25, based on its published streaming source. Current deliverable is reviewed planning documents only. Preserve PR16/source/image identity and historical task records. A design PASS means the ruler is implementable, not runtime acceptance or approval to merge.

After the owner selects the implementation step, record approved design/PRD hashes in `approval.md`, choose the integration baseline/branch disposition explicitly, and run Implementer -> independent Reviewer profiles. Do not create another branch or commit during planning. Before a later publication, Linux validation must run against the implementation head and a new acceptance image must carry that head/version; `dev-run23-2af222b` has no ListMessageFiles. Existing pipeline can build the image; publication is not part of this design execution.

Rollback removes/reverts the additive enumeration implementation before release or redeploys the previous immutable image; clients then get Unimplemented for the added method. Existing progressive downloads continue using their original contracts. Once consumers adopt the new RPC, removal is an explicit API rollback decision, not a silent compatibility claim.
