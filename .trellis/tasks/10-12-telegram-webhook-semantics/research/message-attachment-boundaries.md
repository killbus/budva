# Message, Attachment And Delivery Boundaries

Decision date: 2026-10-12.
Status: user-confirmed semantic boundary; protocol and runtime design remain
unapproved. This is the authoritative record of the DBS-chatroom correction,
not a new implementation plan or a claim of implemented behavior.

## Decision And Rationale

The user rejected embedding Attachment entities in Message and subsequently
rejected relationship-set completeness as meaningless for Message. Message is
a message, not an attachment collector or a view of consumer delivery progress.

The discussion initially confused source message content, resource readiness,
cross-message albums and consumer knowledge after event loss. Those are separate
concerns. Domain separation neither requires three database tables nor requires
three asynchronous webhook deliveries.

## Semantic Boundaries

| Concept | Owns | Does not own |
| --- | --- | --- |
| Message | Message identity, chat, sender, source time, canonical content type, text/entities, edits and deletion facts | Attachment entities, relation collections, file access or delivery progress |
| Attachment | Independent media/resource identity, metadata, file representations, download/cache/access and lifecycle | Message identity or whether a consumer received all events |
| MessageAttachmentRelation (optional) | An explicit association between message and attachment identities, with business-required role/slot/order | Message content, resource storage or delivery completeness |
| Delivery | Queue admission, routing, HTTP attempt, timeout, loss and lack of ordering/replay | Source message or attachment truth |
| Album | A cross-Message media-group aggregate and any aggregate membership reasoning | A single Message's attachment collection |

Relation becomes an independent entity only if its business semantics require
it. Do not infer that requirement from normalization or transport mechanics.

## Message Webhook Boundary

Message Webhook expresses only Message facts. It must not contain Attachment
entities, bytes, resource metadata, download URLs, TDLib file IDs, local paths,
credentials, relation collections/counts/completeness or attachment processing
states. A canonical content discriminant is a message fact; it is not a raw
TDLib content object containing nested media/file structures.

Attachment and any explicit Relation are separate facts. Facts established by
the same source update may be constructed together and may share a transport
package, provided they remain separate records outside Message. This does not
select a package shape, event taxonomy, database model or reliable delivery mode.

Attachment download failure must not turn Message into an unavailable message.
If an Attachment or Relation notification is lost, the consumer lacks that
fact; Message does not acquire an unknown attachment state.

## Retracted Assumptions

Do not restore any of the following as Message fields or equivalent wrappers:

- `relation_set_completeness`, `observed_relation_ids`;
- `none / known / unknown` as relationship-set declarations;
- `hasAttachments`, `attachmentCount`, complete relationship snapshots;
- `media` arrays, attachment preparation outcomes, storage or access details.

Also retract the assumption that an ordinary Telegram photo message needs later
relationship events to determine which media belongs to it. Waiting for file
download is not waiting for media ownership to become known. Photo sizes and
video thumbnails/covers/main files must not automatically become separate
business attachments merely because they contain multiple TDLib File objects.

## Telegram Interpretation And Evidence Limits

For recognized ordinary `MessagePhoto`, `MessageVideo` and `MessageDocument`
content, the TDLib update identifies the single Message's content type and
direct media at that event time. A later content edit is a new message state;
resource downloading/caching is a different lifecycle.

Telegram albums group multiple Messages by `media_album_id`; they are not one
Message whose attachment collection is gradually assembled. An individual
update does not prove the album's complete membership. Any such question belongs
to an independent Album boundary, not to Message.

These are the corrected modeling conclusions. Exact field mappings across all
74 pinned go-tdlib v0.7.6 MessageContent implementations were subsequently
verified by M01: 74 unique types and 201 exact top-level fields are represented
in `content-type-matrix.md`. This research proof is not runtime support.

Repository evidence already recorded in `media-gap-baseline.md` shows:

- `internal/service/push/event/envelope.go:9` keeps schema v1;
  `internal/service/push/event/envelope.go:20` defines a text-only Envelope.
- `internal/app/handler/service.go:321` constructs an Envelope and submits it
  to the captured push submitter; existing album handling is in
  `internal/app/handler/service.go:538`.
- `internal/service/push/dispatcher.go` handles bounded generation/routing and
  enqueue; it does not establish attachment facts.
- `internal/infra/telegram/client_adapter.go` does not expose file download APIs.

No attachment facade or relation authority is established by this evidence.

## Webhook Responsibilities

The minimal responsibilities are source observation, semantic translation,
event construction, queue/dispatch and HTTP delivery. Event construction encodes
facts supported by its inputs. Dispatcher and transport deliver those facts;
they do not create Message-Attachment relationships or resource truth.

Facade is an optional internal organization pattern, not a protocol role or an
assumed fact owner. Map responsibilities to the actual repository call chain
before assigning new components.

Keep v1 default behavior and field meanings unchanged. Any v2 must be explicit
opt-in. Existing bounded, at-most-once, unordered delivery without replay stays
unchanged; a shared package does not create end-to-end atomicity or completeness.

## Consequences For M01/M02

M01 completed the 74-type revalidation, including direct media, resource
representations, album meaning and representative boundary fixtures.

M02 subsequently concluded that current evidence supports a Message-only v2 and
does not justify independent Attachment or Relation contracts. See
`m02-protocol-decision.md`. Authenticated fetch, resource lifecycle and
numerical budgets are deferred rather than approved candidates.

M01 is complete and M02 awaits explicit approval. This recording action does not
start M03, run
`task.py start`, change runtime/spec behavior, commit, push, publish an image or
deploy. Broader protocol approval remains a separate gate.
