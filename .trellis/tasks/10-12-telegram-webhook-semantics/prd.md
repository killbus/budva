# Telegram Message Semantics for Webhook

## Goal

Define a versioned canonical Webhook protocol that preserves Telegram message
facts without conflating Message with Attachment resources, transport behavior
or cross-message aggregates.

The protocol must preserve the meaning available from supported TDLib message
updates while keeping the existing v1 text envelope compatible by default.
Whether and how an independent Attachment resource is delivered remains a
separate product decision; this PRD does not assume that Budva downloads,
caches, hosts or serves media bytes.

The absence of an Attachment contract does not erase Telegram-native media
facts. `/app/engine` is the passive Message webhook path; facade is the external
active-interaction boundary. If a later Message version exposes a native file
identifier for an approved facade operation, that is a direct source fact and
must not be wrapped in `resource_ref` or backed by a Budva relation mapping.

## Status And Approval Boundary

Status: active. M01 content mapping and M02 design are complete. M03 completed
on 2026-10-14 with focused Linux CI evidence. M04 is resolved as a no-code
product decision gate; M05 is the next delivery milestone.

This PRD defines the approved product boundary and records the completed M03
implementation scope. It does not approve media transport, a resource identity
layer, numerical resource budgets, merge to `main`, publication or deployment.

## Confirmed Semantic Boundary

Message expresses only message facts. Depending on the source update, these
may include:

- message and chat identity;
- sender identity represented by the source;
- Telegram source timestamps;
- the closed TDLib content type and its message-level semantics;
- formatted text or caption and entities;
- reply, topic and media-group context;
- edit state; and
- deletion facts supplied by the source.

The following do not belong to Message:

- Attachment entities or resource representations;
- relationship collections, counts or completeness declarations;
- download, cache, storage, access or preparation state;
- Webhook queue, attempt, timeout, failure or ordering state; and
- album completeness.

Attachment is a separate concept only where the approved product scope needs
an independently identifiable media or file resource.
`MessageAttachmentRelation` is optional. It exists only if business semantics
require an independently addressable association, role, slot or order. Domain
separation does not by itself require three tables, three packages or three
asynchronous Webhook events.

Webhook is the existing transport mechanism. A failed, delayed, lost or
out-of-order delivery does not change a Message fact and must not be projected
back into Message state.

A Telegram media album is a cross-Message aggregate identified through source
media-group context. Its membership or completeness, if the product needs to
model either, belongs to a separate aggregate boundary.

The rationale, retracted assumptions and evidence limits are recorded in
`research/message-attachment-boundaries.md`.

## Repository Baseline

The current v1 envelope is text-only. It does not preserve all formatted-text
entities, content types, reply context, media-group context, edits or deletes
with one canonical message representation.

The current protocol has no unified event ID. TDLib new-message,
message-content and delete updates also do not provide one common native event
identity. A locally generated notification ID would therefore be protocol
identity, not a Telegram message fact, and must not be introduced without an
explicit M02 decision.

The current Webhook mechanism is bounded, at-most-once and unordered. Those
properties constrain delivery guarantees but do not redefine Message.

See `research/media-gap-baseline.md` for repository evidence and
`research/content-type-matrix.md` for the pinned go-tdlib v0.7.6 inventory.

## Product Requirements

- Define a stable, versioned canonical representation of Message facts rather
  than exporting raw TDLib objects.
- Preserve supported message identity, source timestamps, content type,
  formatted text or caption entities, reply context, topic context,
  media-group context, edit state and deletion facts.
- Inventory all pinned TDLib `MessageContent` implementations. For every type,
  identify the message fact represented, any direct resource reference, and
  whether support is included, deferred or explicitly excluded.
- Treat photo sizes, thumbnails, covers and primary files according to their
  verified source semantics. Do not automatically count each representation
  as a separate business Attachment.
- Keep Message, optional Attachment, optional
  `MessageAttachmentRelation`, Webhook transport behavior and Album aggregation
  semantically separate.
- Do not place Attachment entities, nested media objects, resource access
  fields, relationship collections/counts/completeness or file lifecycle state
  in Message fixtures or the Message contract.
- Do not expose raw TDLib objects, private local paths, credentials or other
  implementation-only data in an external protocol.
- Do not invent a Budva resource identity or persist a
  `resource_ref -> TDLib file_id` mapping merely to connect passive Message
  delivery to active facade operations.
- Make new, edited and deleted notifications self-describing within the facts
  their source update supplies. Consumers must tolerate a missing predecessor
  because the transport is at-most-once.
- Define Webhook-only eligibility independently of Telegram copy/forward state.
  Do not accidentally inherit forwarding guards or change existing Telegram
  behavior.
- Preserve v1 behavior by default. Any v2 protocol must be explicitly selected
  and must define compatibility, unknown-content and serialization-failure
  behavior before implementation.
- Determine Attachment product scope before choosing any byte-delivery
  mechanism. If actual resource access is later approved, specify it in a
  separate requirement and design review rather than presupposing download,
  hosting or caching in this PRD.
- Preserve the architectural direction: webhook publication never triggers a
  file download; a consumer that needs media actively calls the existing facade
  boundary using only a reviewed Telegram-native identifier and explicit
  account/session scope.

## Acceptance Criteria

- [x] AC1: A reviewed, repository-grounded matrix accounts for all 74 pinned
  TDLib `MessageContent` implementations without unexplained omissions.
- [ ] AC2: The approved Message contract contains only Message facts. Its
  representative fixtures contain no Attachment entity, relationship
  collection/count/completeness, resource access/preparation state or Webhook
  transport state.
- [ ] AC3: M02 explicitly decides whether Attachment and
  `MessageAttachmentRelation` are product requirements, what facts each owns,
  and whether multiple facts share one transport package. It does not infer
  these choices from normalization or transport mechanics.
- [ ] AC4: Album membership is modeled as cross-Message source context.
  No single Message contract claims album completeness.
- [ ] AC5: New, edited and deleted message fixtures preserve the facts available
  from their source updates, and consumers can interpret them when a predecessor
  notification was not delivered.
- [ ] AC6: v1 remains the default behavior. Any approved v2 path has explicit
  opt-in, unknown-content, compatibility and bounded serialization-failure
  behavior.
- [ ] AC7: Production-handler tests for the approved scope cover formatted
  text, captions, representative content types, replies, media-group context,
  edits, deletes and missing predecessors without changing Telegram behavior.
- [ ] AC8: Linux validation covers the approved implementation scope. Fixture
  evidence is distinguished from live-user delivery, and no merge, image
  publication or production deployment is implied.

## Open Product Decisions

- The exact Message fact mapping for each pinned TDLib content type.
- Whether future product evidence will justify an independently delivered
  Attachment contract. M02 currently defers it.
- Whether a later Message schema must expose minimum Telegram-native media/file
  facts to close the webhook-to-facade interaction.
- Which existing facade gRPC boundary should be extended for native file
  lookup/download/cancellation; the current inspected contract has no dedicated
  file RPC.
- If Attachment is later required, which resource identity and metadata are
  product facts. Actual byte access remains a separate decision.
- Whether a future business case will require `MessageAttachmentRelation`,
  including independent role, slot or order semantics. M02 currently defers it.
- The canonical treatment of edits, deletes, replies, topics, forwarding
  provenance and media-group membership.
- Exact canonical v2 field names and source availability for edit/delete.
- Whether a later operational requirement justifies a protocol-generated
  tracing identity. M02 does not require one.

The current decisions and remaining evidence gaps are recorded in
`research/m02-protocol-decision.md`. They are not implementation defaults and
require explicit approval before M03.

## First Deliverable And Approval Gate

First complete and review:

1. the repository-grounded 74-type message-fact and resource-reference matrix;
2. representative Message fixtures proving the confirmed boundary;
3. a decision record for optional Attachment and Relation scope;
4. the v1/v2 compatibility and unknown-content contract; and
5. a corrected architecture trace from TDLib update through event construction
   and existing Webhook transport.

Only after those artifacts are reviewed may `design.md` and `implement.md` be
treated as an approved implementation plan. Media transport, hosting,
authentication, expiry, cleanup, disk budgets and worker limits are out of
scope unless a later product decision explicitly requires actual resource
access.
