# Message, Attachment And Delivery Boundaries

Decision date: 2026-10-12.
Status: user-confirmed semantic boundary, amended on 2026-10-14 after M03.
This is the authoritative record of the boundary correction, not a claim that
all Telegram media or file operations are implemented.

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
| Message | Message identity, chat, sender, source time, canonical content type, text/entities, direct Telegram content facts, edits and deletion facts | Invented Attachment identities, relation collections, file access or delivery progress |
| Attachment | Independent media/resource identity, metadata, file representations, download/cache/access and lifecycle | Message identity or whether a consumer received all events |
| MessageAttachmentRelation (optional) | An explicit association between message and attachment identities, with business-required role/slot/order | Message content, resource storage or delivery completeness |
| Delivery | Queue admission, routing, HTTP attempt, timeout, loss and lack of ordering/replay | Source message or attachment truth |
| Album | A cross-Message media-group aggregate and any aggregate membership reasoning | A single Message's attachment collection |

Relation becomes an independent entity only if its business semantics require
it. Do not infer that requirement from normalization or transport mechanics.

## Message Webhook Boundary

Message Webhook expresses only Message facts observed by `/app/engine`. It must
not contain invented Attachment entities, Budva resource handles, bytes,
download URLs, local paths, credentials, relation collections/counts/
completeness or attachment processing states. It also must not copy a raw TDLib
object indiscriminately.

This boundary does not prohibit a reviewed Message version from honestly
projecting Telegram-native media facts, including the native file identifier
needed to address that media through the facade. Such a field is source message
content, not a Budva Attachment identity or a separately persisted relation.
Its account/session scope and TDLib stability limits must remain explicit.

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

Also do not introduce `resource_ref`, an opaque Budva resource identity, or a
`resource_ref -> TDLib file_id` mapping merely to connect the webhook to active
Telegram operations. The Telegram message already contains the native
Message-to-media-to-File relationship. Re-wrapping it would invent lifecycle,
persistence, expiry and recovery obligations not established by the source.

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
- `internal/infra/telegram/client_adapter.go` does not currently expose TDLib
  file download APIs through the adapter surface inspected for this task.
- Pinned go-tdlib v0.7.6 provides `File`, `LocalFile`, `RemoteFile`,
  `UpdateFile`, `GetFile`, `DownloadFile` and `CancelDownloadFile`; their
  existence is upstream capability, not proof that Budva currently wires them.
- `internal/infra/telegram/repo.go` filters the update stream to message-related
  updates and does not currently forward `UpdateFile`.

No Attachment authority or new relation store is established by this evidence.
`/app/engine` is the passive Message webhook path. `cmd/facade` is the existing
external active-interaction boundary. Any future file lookup/download work must
extend or reuse that facade boundary and the Telegram-native identifier rather
than create a second resource identity layer.

## Webhook Responsibilities

The minimal responsibilities are source observation, semantic translation,
event construction, queue/dispatch and HTTP delivery. Event construction encodes
facts supported by its inputs. Dispatcher and transport deliver those facts;
they do not create Message-Attachment relationships or resource truth.

Facade is the external active-interaction boundary already present in the
repository. Message webhook publication remains passive. A consumer may decide
after receiving a Message that it needs the referenced Telegram media and then
invoke a reviewed facade operation in the appropriate account/session scope.
Webhook delivery itself must not download the file.

Keep v1 default behavior and field meanings unchanged. Any v2 must be explicit
opt-in. Existing bounded, at-most-once, unordered delivery without replay stays
unchanged; a shared package does not create end-to-end atomicity or completeness.

## Consequences For M01/M02

M01 completed the 74-type revalidation, including direct media, resource
representations, album meaning and representative boundary fixtures.

M02 subsequently concluded that current evidence supports a Message-only v2 and
does not justify independent Attachment or Relation contracts. See
`m02-protocol-decision.md`. M03 implemented that approved narrow schema without
nested media/file facts. Any later addition of native Telegram media facts and
facade file actions is a versioned follow-up, not permission to invent
Attachment, Relation, cache, hosting or resource-lifecycle infrastructure.

M01, M02 and M03 are complete. M04 is a product decision gate, not an
implementation stage. It may be replaced only by an explicitly approved narrow
follow-up for Telegram-native media facts plus facade active operations. This
record does not authorize merge to `main`, publication or deployment.
