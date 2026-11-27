# Verified baseline and SDK facts

## Baseline

Source: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`, feature worktree `feat/facade-file-download`; PR16 is not merged into main. The dispatch-test image built from this source supplies the two progressive RPCs, but no enumeration RPC.

- `cmd/facade/main.go:80,94,111`: NoBusinessUpdates repository -> facade service -> registered gRPC transport.
- `internal/app/facade/file_download.go:41,70,128`: GetMessageFile, main-file resolver, progressive orchestrator. Photo maximum uses File.Size and >= (last tie); videos use Video.Video only. Paid media is unsupported by this resolver.
- `internal/app/facade/service.go`: partial telegramRepo already exposes GetMessage and all four live download operations.
- `internal/infra/telegram/client_adapter.go:138`: GetMessage uses existing chat-readiness wrapper. Enumeration requires no new SDK method.
- `internal/transport/grpc/file_download.go:110`: validates nonzero chat ID and positive message ID; DownloadFile validates [1, MaxInt32].
- `internal/transport/grpc/transport.go:39`: shared error mapper supports typed missing-message/flood-wait/readiness errors, deadline and generic Internal. Canceled is handled separately in the streaming mapper, not the global mapper.
- `internal/transport/grpc/pb/facade.proto:24,25,205`: existing stream declarations and request messages. `api/proto/facade.proto` is a tracked symlink, not another source proto.

## Pinned SDK

Read from the local Go module cache `github.com/zelenin/go-tdlib@v0.7.6/client/type.go`. These are schema facts, not product scope requirements.

| Type / anchor | Actual file-bearing fields and metadata |
| --- | --- |
| File:4277 | Id int32; Size int64 (0 unknown); ExpectedSize is approximate; Local/Remote must not be returned as paths |
| PhotoSize:4420 | Photo *File, Type, Width, Height; ProgressiveSizes are prefix offsets in ONE file |
| Minithumbnail:4451 | Inline JPEG Data bytes, no TDLib file ID |
| Thumbnail:4653 | File *File, Format, Width, Height |
| Animation:5239 | Animation *File; Thumbnail; FileName, MimeType, Width, Height |
| Audio:5278 | Audio *File; AlbumCoverThumbnail; ExternalAlbumCovers []*Thumbnail; FileName, MimeType |
| Document:5317 | Document *File; Thumbnail; FileName, MimeType |
| Photo:5348 | Sizes []*PhotoSize |
| Sticker:5375 | Sticker *File; Thumbnail; FullType; Width, Height; no MimeType |
| StickerFullTypeRegular:5007 | PremiumAnimation *File |
| Video:5449 | Video *File; Thumbnail; FileName, MimeType, Width, Height |
| VideoNote:5490 | Video *File; Thumbnail; Length is square dimension; no MimeType |
| VoiceNote:5554 | Voice *File; MimeType; no thumbnail |
| AlternativeVideo:5876 | Video *File and HlsFile *File, Width, Height, Codec; no MimeType/FileName |
| MessageVideo:23986 | Video *Video; AlternativeVideos []*AlternativeVideo |
| MessagePaidMedia:23866 | Media []PaidMedia |
| PaidMediaPhoto:21479 | Photo *Photo |
| PaidMediaVideo:21506 | Video *Video (no AlternativeVideos field) |
| PaidMediaPreview:21446 | Width, Height, Duration, Minithumbnail only; hidden until paid, no file ID |
| PaidMediaUnsupported:21533 | No media/file fields |
| Message:12671 / MediaAlbumId:12734 | Separate messages may share a MediaAlbumId; Content is one MessageContent |

Download IDs address TDLib's current file namespace; they are not durable cross-account/session identities. A listing is only a snapshot of references exposed to the current account. File metadata cannot promise future access or availability.

## Owner acceptance evidence motivating the follow-up

Owner reported metadata file_id=1279, size=63455070, then chunks and complete.size=63455070. Read-only inspection of `E:\Desktop\response2.json` found offset=0, a 262144-byte decoded payload, upstream_complete=false, and ZIP magic. This supports actual progressive bytes in that sample; it does not independently establish whole-file checksum or final RPC status. It is not enumeration acceptance evidence.

## Workflow constraints

The referenced sibling level85 AGENTS.md, task.py, and trellis-brainstorm are unavailable in this checkout. Use the existing Trellis document layout manually and the configured Paseo Design Reviewer/Reviser profiles. No hook/script execution is claimed. Current phase is planning; no source implementation is authorized by this ticket yet.
