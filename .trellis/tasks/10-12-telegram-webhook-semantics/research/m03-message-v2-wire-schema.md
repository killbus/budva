# M03 Message-only v2 Wire Schema

Status: frozen for RED/GREEN implementation on 2026-10-13.

This record fixes the exact Message-only v2 wire contract approved in M02. It
does not authorize Attachment, resource access, delivery-state, relation,
album-completeness or raw TDLib contracts.

## Endpoint Opt-in

Each push target may select its webhook schema with:

```yaml
pushTargets:
  external:
    url: https://push.example.test/events
    schemaVersion: 2
```

Rules:

- an absent or zero `schemaVersion` selects v1;
- explicit `schemaVersion: 1` selects v1;
- explicit `schemaVersion: 2` selects Message-only v2;
- any other value is invalid and rejects the ruleset;
- different endpoints in one immutable generation may select different
  versions;
- a v2 projection or serialization failure affects only that endpoint and
  never falls back to v1.

## Top-level Shape

The v2 JSON object uses snake_case field names:

```json
{
  "schema_version": 2,
  "event": "new",
  "occurred_at": "2026-10-13T08:00:00Z",
  "message": {}
}
```

`schema_version`, `event` and `occurred_at` are always present.

`event` retains the existing canonical values `new`, `edited` and `deleted`.
`occurred_at` is the local observation time already captured by the source
emission. It is not Telegram sent time, edit time or delivery time.

`message` is always present. For `new` and `edited`, it is a current snapshot.
For `deleted`, it contains only identity and delete-update facts.

## Message Snapshot

The `new` and `edited` message object is:

```json
{
  "chat_id": -100123,
  "message_id": 456,
  "sender": {
    "type": "messageSenderUser",
    "user_id": 42
  },
  "sent_at": "2026-10-13T07:59:00Z",
  "edited_at": "2026-10-13T07:59:30Z",
  "content": {
    "type": "messagePhoto",
    "formatted_text": {
      "text": "caption",
      "entities": [
        {
          "offset": 0,
          "length": 7,
          "type": "textEntityTypeBold"
        }
      ]
    },
    "show_caption_above_media": true,
    "has_spoiler": false
  },
  "reply_to": {
    "type": "messageReplyToMessage",
    "chat_id": -100123,
    "message_id": 400
  },
  "message_thread_id": 99,
  "saved_messages_topic_id": 0,
  "media_album_id": 1234,
  "via_bot_user_id": 7,
  "sender_business_bot_user_id": 8,
  "author_signature": "editor",
  "has_sensitive_content": false,
  "restriction_reason": "regional",
  "forward": {}
}
```

Required snapshot fields:

- `chat_id`;
- `message_id`;
- `sent_at`;
- `content`.

Optional snapshot fields are omitted when the source does not supply a
meaningful value:

- `sender`;
- `edited_at`;
- `reply_to`;
- `message_thread_id`;
- `saved_messages_topic_id`;
- `media_album_id`;
- `via_bot_user_id`;
- `sender_business_bot_user_id`;
- `author_signature`;
- `restriction_reason`;
- `forward`.

`has_sensitive_content` is retained as a source boolean and is always emitted
for a snapshot. Zero-valued optional identifiers are omitted.

TDLib Unix timestamps are projected as UTC RFC 3339 timestamps. A zero source
timestamp is absent rather than rendered as the Unix epoch.

## Sender

Sender is a bounded tagged object:

```json
{"type":"messageSenderUser","user_id":42}
```

or:

```json
{"type":"messageSenderChat","chat_id":-100123}
```

A nil or unsupported sender is omitted. Raw polymorphic TDLib objects are
never serialized.

## Content

Every snapshot has a content object with a required `type`.

For a known pinned TDLib content type, `type` is its
`MessageContentType()` discriminator. No nested TDLib media, file, thumbnail,
cover, download-state or resource object is emitted.

For text-bearing content, `formatted_text` contains the message text or
caption:

```json
{
  "type": "messageText",
  "formatted_text": {
    "text": "hello",
    "entities": []
  }
}
```

`formatted_text` is omitted when the source content has no text or caption.
When present, `text` and `entities` are both emitted. Entity `offset` and
`length` preserve TDLib UTF-16 code-unit values without conversion.

Each entity is:

```json
{
  "offset": 0,
  "length": 5,
  "type": "textEntityTypeBold"
}
```

The entity `type` is the bounded `TextEntityTypeType()` discriminator. Fields
specific to TDLib entity variants, including URLs, language names, user IDs,
custom emoji IDs and media timestamps, are not part of Message-only v2.

The following safe message-level content flags are emitted only for content
types that supply them:

- `show_caption_above_media` for animation, photo and video;
- `has_spoiler` for animation, photo and video.

Unknown runtime content is represented without raw data:

```json
{
  "type": "unknown",
  "source_type": "futureMessageType"
}
```

`source_type` is retained only when a non-empty bounded discriminator is
available. Its implementation must enforce a fixed maximum length and reject
control characters. The frozen maximum is 128 UTF-8 bytes.

Nil content is also represented as:

```json
{"type":"unknown"}
```

## Reply And Topic Context

Only `messageReplyToMessage` has a v2 projection:

```json
{
  "type": "messageReplyToMessage",
  "chat_id": -100123,
  "message_id": 400
}
```

Zero-valued `chat_id` or `message_id` is omitted. Quote content, nested origin
and nested message content are excluded. Other reply variants may retain only
their bounded `MessageReplyToType()` discriminator:

```json
{"type":"messageReplyToStory"}
```

`message_thread_id` and `saved_messages_topic_id` remain independent scalar
context. They do not imply a Relation entity or completeness.

`media_album_id` means only that independently emitted Messages share the same
Telegram media album identifier. It does not create an album collection.

## Forward Provenance

Forward provenance is shallow and sanitized:

```json
{
  "origin": {
    "type": "messageOriginChannel",
    "chat_id": -100500,
    "message_id": 20,
    "author_signature": "author"
  },
  "sent_at": "2026-10-12T10:00:00Z",
  "source": {
    "chat_id": -100500,
    "message_id": 20,
    "sender": {
      "type": "messageSenderChat",
      "chat_id": -100500
    },
    "sender_name": "hidden",
    "sent_at": "2026-10-12T10:00:00Z"
  },
  "public_service_announcement_type": "type"
}
```

Supported origin projections:

- `messageOriginUser`: `sender_user_id`;
- `messageOriginHiddenUser`: `sender_name`;
- `messageOriginChat`: `sender_chat_id`, optional `author_signature`;
- `messageOriginChannel`: `chat_id`, `message_id`, optional
  `author_signature`.

Unsupported or nil origin values are omitted. Zero timestamps and zero
identifiers are omitted. No raw forward or sender object is serialized.

## Delete Shape

Delete notifications contain only facts supplied by
`UpdateDeleteMessages`:

```json
{
  "schema_version": 2,
  "event": "deleted",
  "occurred_at": "2026-10-13T08:00:00Z",
  "message": {
    "chat_id": -100123,
    "message_id": 456,
    "delete": {
      "is_permanent": true,
      "from_cache": false
    }
  }
}
```

`chat_id`, `message_id`, `delete.is_permanent` and `delete.from_cache` are
always present for a delete notification. No sender, content, sent time, edit
time, reply, topic, album or forwarding facts are synthesized.

Every message ID in one TDLib delete update produces an independent
notification with the same two delete flags.

Preserving these webhook facts must not change existing Telegram destination
behavior. In particular, the application may continue to skip destination
deletion for a non-permanent update while still making the source delete facts
available to the v2 projection.

## Exact Representative Fixtures

### New text

```json
{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sender":{"type":"messageSenderUser","user_id":42},"sent_at":"2026-10-13T07:59:00Z","content":{"type":"messageText","formatted_text":{"text":"hello","entities":[{"offset":0,"length":5,"type":"textEntityTypeBold"}]}},"has_sensitive_content":false}}
```

### Edited photo caption

```json
{"schema_version":2,"event":"edited","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","edited_at":"2026-10-13T07:59:30Z","content":{"type":"messagePhoto","formatted_text":{"text":"caption","entities":[]},"show_caption_above_media":true,"has_spoiler":true},"media_album_id":1234,"has_sensitive_content":false}}
```

### Delete

```json
{"schema_version":2,"event":"deleted","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"delete":{"is_permanent":true,"from_cache":false}}}
```

### Unknown content

```json
{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","content":{"type":"unknown","source_type":"futureMessageType"},"has_sensitive_content":false}}
```

## RED Controls

Before runtime implementation, tests must fail for:

1. absent and explicit v1 `schemaVersion` retaining exact six-key v1 JSON;
2. explicit v2 parsing and unsupported-version rejection;
3. exact new, edited, deleted and unknown JSON fixtures above;
4. nil sender, reply, origin, entity type and content handling;
5. UTF-16 entity offsets and bounded discriminators;
6. no nested media, file, raw TDLib, URL, path, credential or delivery fields;
7. mixed v1/v2 endpoints receiving their selected representation;
8. v2 projection or serialization failure remaining endpoint-isolated with no
   v1 fallback;
9. preservation of both `is_permanent` and `from_cache`;
10. unchanged Telegram copy, forward and delete behavior.

## Implementation Boundary

The handler captures transport-independent source facts. Endpoint schema
selection occurs only at the independently configured delivery boundary.

The existing v1 `Envelope` contract remains source-compatible for current
callers and tests. M03 may add a canonical source-fact carrier and endpoint
projector, but it must not make handlers inspect endpoint configuration or
serialize webhook JSON.

M04 resource access remains deferred.
