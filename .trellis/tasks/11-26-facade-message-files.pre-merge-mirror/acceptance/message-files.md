# Message file list/select/download acceptance guide

This guide is for an implementation-stage build containing `ListMessageFiles`
at this task head. It does not validate historical image `dev-run23-2af222b`,
which predates the endpoint and must return `Unimplemented`.

## Prerequisites

1. A running facade built from a future published build containing this task
   source, or a newly built local source image. The current orchestrator
   native Linux environment is for test execution only and has not been
   released or published.
2. Authorized TDLib account/session and accessible target chat.
3. Obtain actual TDLib IDs as today, for example through the existing
   `GetMessageLinkInfo` RPC.
4. A gRPC client that supports unary calls and the existing
   `ServerStreaming` `DownloadFile` events.

## List message files

Unary RPC: `pb.FacadeGRPC/ListMessageFiles`

Example request:

```json
{
  "chat_id": "-1001234567890",
  "message_id": "1048576"
}
```

The JSON int64 fields are string-rendered. Inspect the response:

- `chat_id` / `message_id` echo the request.
- `media_album_id` is the snapshot value; it does not expand sibling album
  messages.
- `content_supported` distinguishes recognized media from text/unsupported
  content.
- Each `MessageMediaItem` has an `item_index`, `kind`, `state`, and ordered
  `MessageFileDescriptor.files`.
- For paid media, locked previews have empty `files` and
  `MESSAGE_MEDIA_STATE_LOCKED`; unsupported slots also have no file IDs.
- `default_file_id` equals the existing GetMessageFile choice when present in
  the emitted entries; otherwise it is `0`.

For a photo, choose any `MESSAGE_FILE_ROLE_PHOTO_SIZE` descriptor. Do not
assume the largest one is the default; compare `default_file_id`. For a video,
`MESSAGE_FILE_ROLE_MAIN` and `MESSAGE_FILE_ROLE_ALTERNATIVE_VIDEO` are both
ordinary downloadable choices, while a paired `MESSAGE_FILE_ROLE_HLS_PLAYLIST`
exposes only its playlist file.

## Download a selected file

Use the unchanged streaming RPC:
`pb.FacadeGRPC/DownloadFile`.

```json
{
  "file_id": "1279"
}
```

Replace `1279` with a positive ID selected from the listing. Keep the listing
record so bytes can be identified by `(item_index, role, source_index)` and
metadata. The stream still emits:

1. `metadata` containing the requested file ID and its actual known size.
2. ordered `chunk` events.
3. one terminal `complete` event.

Validate that:

- the selected descriptor ID equals the download request ID;
- metadata file ID equals that selected ID;
- chunks concatenate to the metadata/complete size;
- the final status is OK and `Recv()` returns `io.EOF` cleanly;
- for an alternative-video/HLS pair, selecting the alternative video must
  download its ordinary file, not the HLS playlist bytes.

A synthetic isolated-account flow must separately validate the nondefault ID,
payload identity, and ordered completion. Live Telegram content is separate
runtime evidence and must be identified by source revision, image digest (when
published), account/session context, and the actual message/file IDs.

## Known limits

- Listing is a `GetMessage` snapshot only; later edits or session changes can
  make a listed ID fail normally.
- An emitted ID does not guarantee future access or that the download has
  completed.
- HLS playlists are not rewritten or expanded into segments.
- The listing does not unlock or purchase paid media.
- An old image returns `Unimplemented`; there is no fallback.
