# Media Gap: Repository Baseline

Observed 2026-10-12 at `6d7daa750fb708b86c83b12f5d5c19bc10f93eb6` on
`feat/external-push-sink`. This is scoped source inspection, not an exhaustive
TDLib type audit or actual media-delivery acceptance.

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
  partial client surface. No GetFile/DownloadFile method is present; media access
  cannot be assumed available just because upstream TDLib exposes file APIs.
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

Full content-type policy, exact entity/relationship schema, supported media
variants, transport/hosting/auth, immutable ownership, all numerical budgets,
cleanup and migration are open. Keep these explicit in design review; do not
replace a media-delivery problem with opaque IDs or flattening to captions.
