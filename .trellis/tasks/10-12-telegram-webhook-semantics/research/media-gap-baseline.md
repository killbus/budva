# Media Gap: Repository Baseline

Observed 2026-10-12 at `6d7daa750fb708b86c83b12f5d5c19bc10f93eb6` on
`feat/external-push-sink`, amended on 2026-10-14 after a read-only native-file
behavior investigation. This is scoped source inspection, not actual
media-delivery acceptance.

## Evidence

- `internal/service/push/event/envelope.go:9` fixes `SchemaVersion = 1`;
  `internal/service/push/event/envelope.go:20` contains only schema/event/chat/
  message/time/text fields. There are no entity, relationship or media fields.
- `internal/service/message/service.go:23` defines
  `func (s *Service) GetFormattedText(msg *client.Message) *client.FormattedText`.
  It extracts text or captions from text/photo/video/document/audio/animation/
  voice-note messages; other types return nil. Returning caption text is not
  delivering media bytes, variants, dimensions, duration or entity offsets.
- `internal/app/handler/service.go:208` defines
  `func (s *Service) OnNewMessage(ctx context.Context, msg *client.Message)`.
  Its system-message branch at `internal/app/handler/service.go:220` returns
  before push; nil formatted text at `internal/app/handler/service.go:235` also
  returns. `internal/app/handler/service.go:246` retains
  `if !rule.SendCopy && !msg.CanBeSaved {` for new routing. Reusing this path
  unchanged cannot establish independent rich/textless/protected push coverage.
- `internal/app/handler/service.go:538` retains the original album collector;
  `internal/app/handler/service.go:541` captures shared first-ingress time.
  It emits per-member v1 caption facts, not album relationship/usable media data.
- `internal/infra/telegram/client_adapter.go` lists the repository's actual
  partial client surface. No `GetFile`, `DownloadFile` or
  `CancelDownloadFile` method is exposed there today.
- Pinned go-tdlib v0.7.6 does provide `File`, `LocalFile`, `RemoteFile`,
  `UpdateFile`, `GetFile`, `DownloadFile` and `CancelDownloadFile`. The gap is
  therefore not absence from Telegram/TDLib; it is that Budva has not wired
  those native facts and actions through its adapter and facade paths.
- `internal/infra/telegram/repo.go` currently rejects `UpdateFile` in its
  relevant-update filter, so Budva does not consume the native pushed file
  state machine.
- `/app/engine` is the passive webhook producer. `cmd/facade` is the existing
  external active-interaction entry point. The current gRPC service exposes
  message and chat actions but no dedicated file lookup/download RPC in the
  inspected contract.
- Pinned generated dependency:
  `C:/Users/ying/go/pkg/mod/github.com/zelenin/go-tdlib@v0.7.6/client/type.go:2589`
  defines MessageContent; text/entity/file/message structures and media variants
  provide the inventory source. Examples include photo, sticker, video-note,
  location, poll, paid-media and unsupported content. This is not a support list;
  enumerate all actual content types and map fields during M01.

## Existing Correction And Asynchronous CI

P10/P11 source edit/album-text correction has isolated real-TDLib internal/race/
vet/build evidence in the linked task. Corrective code `4243ddf` and evidence
`6d7daa7` have been pushed. `ci-test.yml` run `38212912036` was observed `queued`
at `2026-10-12T06:31:08Z`, head SHA
`6d7daa750fb708b86c83b12f5d5c19bc10f93eb6`:
https://github.com/killbus/budva/actions/runs/38212912036
No completion result, new image or rich-media runtime claim follows from that.

## What Remains Unknown

The Message-to-media-to-File relationship already exists in Telegram's source
model. Do not replace it with `resource_ref`, a Budva identity mapping, an
Attachment table or a Relation authority.

The remaining question is deliberately narrower: whether a later Message
version should expose the minimum Telegram-native media/file facts required for
the consumer to call an active facade operation, and which existing facade
surface should be extended to perform that operation in the correct
account/session scope. This does not imply automatic download, cache, hosting,
permanent URLs, object storage or a generic resource lifecycle.
