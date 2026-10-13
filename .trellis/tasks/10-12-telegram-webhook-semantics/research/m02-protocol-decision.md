# M02 Protocol Decision

Decision date: 2026-10-13.
Status: approved by the user on 2026-10-13; M03 is active.

## Decision Summary

M02 defines a versioned Message protocol only. Current product evidence does
not justify independent Attachment or MessageAttachmentRelation contracts.

This is a deliberate deferral, not a claim that resources do not exist. M01
shows that Telegram message content can identify direct media and resource
representations. It does not show that Budva consumers need to independently
identify, address, manage or receive those resources.

Consequently:

- Message v2 contains only Message facts.
- Attachment and MessageAttachmentRelation are not part of the proposed v2
  contract.
- No download, hosting, cache, token, URL, byte-access, readiness or resource
  lifecycle contract is approved.
- A later resource-access requirement must return through a separate product
  and design review.
- Delivery remains bounded, at-most-once, unordered and without replay.

## Cross-Audit Result

The DBS-chatroom cross-audit used three independent frames:

- domain modeling: do not derive an entity from TDLib fields, database
  normalization or technical transport;
- distributed messaging: do not let notification identity or packaging imply
  ordering, replay, completeness, exactly-once behavior or consumer atomicity;
- evolutionary contracts: approve the smallest reversible v2 boundary and
  defer infrastructure choices until a product requirement exists.

All three frames converge on the same decision:

1. Message v2 is justified.
2. Independent Attachment and Relation contracts are not currently justified.
3. Authenticated fetch, blob/token lifecycle, preparation workers, queues,
   storage budgets and resource outcome states must not be part of this M02
   approval candidate.

## Canonical Message V2 Facts

The canonical record owns:

- schema version and notification kind;
- Telegram chat and message identity;
- the sender representation supplied by the source;
- source sent time and, when supplied, source edit time;
- a stable canonical content discriminant;
- formatted text or caption plus entity offsets and types;
- shallow reply, topic and media-group context;
- edit facts;
- deletion facts supplied by the delete update; and
- sanitized forwarding provenance where the source supplies it.

The record does not own:

- Attachment entities or nested TDLib media/file objects;
- resource variants, bytes, URLs, file IDs, paths or credentials;
- relationship collections, counts, ordering or completeness;
- album membership completeness;
- queue, attempt, timeout or receiver state; or
- download, caching, storage, preparation or availability state.

The exact field spelling remains a schema-detail task. Every field must map to
one of the approved facts above and to M01 source evidence.

## Identity

Telegram Message identity is `(chat_id, message_id)`.

TDLib does not provide one native identity shared by new-message,
message-content and delete updates. M02 therefore does not require a generated
notification ID.

If a later operational requirement needs a generated ID for tracing or an
idempotency hint, that ID is protocol metadata. It does not become a Telegram
Message fact and does not create ordering, replay, uniqueness across retries or
exactly-once delivery.

Delivery-attempt identity, if introduced, belongs only to Delivery.

## New, Edit And Delete

Consumers must be able to interpret every delivered notification without
assuming that a predecessor was delivered.

- `new` carries the complete current Message snapshot available from its source
  input.
- `edit` carries the complete current Message snapshot available from the edit
  processing input rather than a patch that requires the prior `new`.
- `delete` carries the deletion fact and the identities and source facts
  actually supplied by the delete update. Facts unavailable from that update
  are absent, not synthesized, recovered from an assumed predecessor or encoded
  as misleading zero values.

This self-description does not reconstruct missing history. Under at-most-once
delivery, a consumer first seeing an edit or delete learns only the facts in
that notification.

## Album, Reply And Topic Context

An album member is an ordinary Message with source `media_album_id` context.
Message v2 makes no count, index, membership-completeness or finality promise.

Reply and topic context remains shallow. It identifies source context without
embedding another Message or requiring the referenced Message to have been
delivered.

## Unknown Content

Every pinned known TDLib MessageContent type has an explicit M01 policy.

If runtime input has an unrecognized content discriminant, v2 emits a canonical
unknown-content Message only when message identity and the safe common facts
can still be serialized. The record preserves a bounded source discriminator
when available, but never emits raw TDLib JSON or nested implementation data.

If the common Message facts cannot be formed safely, that endpoint submission
fails as a serialization failure.

## Serialization Failure

A serialization failure is isolated to the affected endpoint submission. It:

- does not fall back to v1;
- does not emit raw TDLib JSON;
- does not silently emit text-only success;
- does not change Telegram forwarding, copying or deletion behavior; and
- is reported through bounded stage/reason observability owned by Delivery.

Because current delivery is at-most-once and has no replay, failure reporting
must not imply that the notification will later arrive.

## Version And Compatibility

- Existing endpoints remain on v1 by default.
- V2 requires explicit per-endpoint opt-in.
- V1 and v2 coexist; v2 does not reinterpret existing v1 fields.
- Unsupported or invalid v2 construction never silently downgrades to v1.
- Removing v2 opt-in returns the endpoint to existing v1 behavior.

This is a parallel-version migration, not an in-place schema extension with an
assumption of field-level compatibility.

## Packaging

The proposed unit is one Message fact notification per observed message
operation.

No shared multi-fact package is needed because Attachment and Relation
contracts are deferred. If later product evidence establishes additional facts
that consumers must interpret from one source observation, packaging may be
reviewed separately.

A shared HTTP payload would only share serialization, queue admission and one
delivery attempt. It would not provide consumer transaction atomicity,
completeness, replay or end-to-end exactly-once behavior.

## Scenario Checks

### Photo

A photo update establishes that the Message has canonical photo content.
Message v2 may preserve caption and message-level source semantics. It does not
embed photo sizes, TDLib File objects, download state or an Attachment entity.

### Document

A document update establishes canonical document content and its message-level
semantics. File access and storage are not implied.

### Album

Three updates sharing `media_album_id` are three Message notifications. Loss of
one notification does not cause the other two to claim an incomplete attachment
set; each remains a valid Message fact.

### Edit Without New

The first notification observed by a consumer may be `edit`. It carries the
current snapshot available to the producer and can be interpreted without a
stored predecessor.

### Delete Without New

The first notification observed may be `delete`. It carries the source-supplied
deletion fact and identities. It does not pretend to know deleted content that
the delete input did not supply.

### Serialization Failure

An unformable v2 Message is not replaced with a v1 or caption-only success. The
v2 endpoint submission fails in isolation, while unrelated Telegram behavior
and other endpoint processing remain governed by their existing contracts.

## Deferred Product Decisions

The following require new evidence and a separate review:

- whether consumers need an independently identifiable Attachment;
- which stable resource facts an Attachment would own;
- whether any association needs independent identity, role, slot or order;
- whether consumers need actual bytes;
- media authorization and source-policy rules;
- download, hosting, caching, expiry and cleanup;
- resource statuses and failure reasons;
- worker, queue, disk, size, time and concurrency budgets; and
- packaging or scheduling of any future non-Message facts.

## Approval Gate

This record completed the M02 design candidate. The user explicitly approved it
on 2026-10-13, authorizing M03 schema verification and runtime implementation,
but not commit, push, publication or deployment.

M03 proceeds by:

1. reviewing exact v2 field names and fixtures against the M01 matrix;
2. verifying source availability for new, edit and delete; and
3. the implementation and validation plan is updated for Message-only v2.
