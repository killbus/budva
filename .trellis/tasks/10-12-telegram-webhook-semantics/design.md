# Draft: Versioned Telegram Message Facts

Status: M02 approved on 2026-10-13; M03 completed on 2026-10-14. M04 is
resolved as a no-code product decision gate. M05 is the next delivery
milestone. Integration into `main`, image publication and deployment remain
unauthorized.

## Design Authority

Read these records in order:

1. `research/message-attachment-boundaries.md`
2. `research/content-type-matrix.md`
3. `research/m01-representative-fixtures.jsonl`
4. `research/m02-protocol-decision.md`

The amended boundary decision supersedes earlier candidates for authenticated
fetch, Attachment preparation, opaque resource handles, blob/token lifecycle,
resource status, independent Relation delivery and numerical resource budgets.
Telegram-native media facts remain part of the source Message model; Budva must
not recreate their relationship through `resource_ref` or a mapping store.

## Scope

The proposed v2 contract serializes canonical Telegram Message facts. It does
not introduce Attachment or MessageAttachmentRelation contracts because current
product and repository evidence does not establish a need for independently
addressable resource or association facts.

The approved M03 implementation scope is:

- a transport-independent canonical Message representation;
- new, edit and delete notification kinds;
- self-describing current facts without predecessor assumptions;
- explicit per-endpoint v2 opt-in while v1 remains the default;
- bounded unknown-content behavior;
- isolated serialization failure; and
- no change to Telegram destination behavior.

Media byte access, hosting, authentication, expiry, cleanup and capacity design
are outside this scope.

`/app/engine` remains the passive Message webhook path. Facade remains the
external active-interaction boundary. A future approved schema may project the
minimum Telegram-native file facts needed by a facade operation, but webhook
publication must not trigger downloading and no Budva resource identity or
Message-to-file relation may be invented.

## Domain Boundaries

Message owns message and chat identity, source sender representation, source
times, canonical content discriminant, formatted text or caption entities,
shallow reply/topic/media-group context, edit/delete facts and sanitized
forwarding provenance.

Message does not own resources, relations, delivery progress or album
completeness. The canonical content value must not embed raw TDLib objects.

Delivery owns route selection, queue admission, HTTP attempt, timeout and
failure observation. Delivery cannot create or change Telegram Message facts.

Album remains a possible cross-Message aggregate. V2 only preserves the
source-provided `media_album_id` on each member.

## Notification Semantics

Telegram Message identity is `(chat_id, message_id)`.

One notification represents one observed message operation:

- `new`: complete current facts available from the new-message input;
- `edit`: complete current facts available from edit processing, not a patch;
- `delete`: deletion facts and identities actually supplied by the delete
  update, without inventing unavailable prior content.

No generated notification ID is required. A future tracing ID, if justified,
would be protocol metadata and would not imply order, replay or exactly-once
delivery.

## Compatibility

V1 remains unchanged and selected by default. V2 is enabled explicitly per
endpoint and coexists with v1.

V2 construction never silently falls back to v1, raw TDLib JSON or text-only
success. Removing v2 opt-in restores existing v1 behavior.

## Unknown Content

Known pinned content follows the completed M01 mapping.

An unrecognized runtime discriminant may produce a canonical unknown-content
Message only when safe common Message facts remain serializable. A bounded
source discriminator may be retained, but raw TDLib JSON and implementation
objects are forbidden.

If a valid canonical Message cannot be formed, the endpoint submission fails.

## Failure Boundary

Serialization failure is isolated to the affected endpoint submission and is
reported through bounded Delivery observability. It must not change Telegram
copy, forward or delete behavior.

Current Webhook guarantees remain bounded, at-most-once, unordered and without
replay. Neither notification identity nor a shared payload may claim stronger
guarantees.

## Source And Runtime Design Checks

Before implementation, verify the actual source inputs for:

- sender representation and source timestamps;
- full current content on edit processing;
- delete identities, timestamps and deletion flags;
- reply, topic, media-group and forwarding fields;
- formatted-text entity offsets in UTF-16 code units; and
- every M01 included, deferred and excluded content policy.

Missing source facts must remain absent rather than synthesized from assumed
history.

## Deferred Resource Design

Attachment, MessageAttachmentRelation, byte access, authenticated fetch,
resource preparation, URLs/tokens, readiness states, storage, expiry, cleanup
and all size/time/worker/queue/disk budgets are deferred.

They may be designed later only if a product requirement establishes:

- an independently referenced resource;
- stable resource facts consumers need;
- an independently meaningful association role, slot or order; or
- actual byte-access requirements.

## Implementation Gate

The user explicitly approved this design and
`research/m02-protocol-decision.md` on 2026-10-13. M03 may proceed after review
of exact canonical field names, fixtures and source availability.

Do not run `task.py start`, modify runtime files, commit, push, publish an image
or deploy as part of this design completion.
