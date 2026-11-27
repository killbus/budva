# Message file enumeration and selection

## Intent and authorization

The owner successfully exercised the progressive GetMessageFile stream, then asked whether one message can have several file IDs. After discussing enumeration followed by explicit file selection, the owner requested: "是的，所以我们要进入设计流程" (enter the design process). This ticket currently authorizes planning and independent design review only.

The preceding ticket provides progressive transfer of one selected file. It does not describe all files attached to a message. This follow-up must let a remote caller discover choices before downloading bytes.

## Requirements

- **R1 Discovery:** Given one chat ID and message ID, expose the message's supported file choices with IDs and enough metadata to distinguish their purpose. Discovery must not start or wait for file downloads or return server-local paths.
- **R2 Selection:** A caller can choose a returned file ID and use the existing DownloadFile stream. One stream continues to represent one file, with progressive transfer rather than full-file staging.
- **R3 Structure:** Distinguish multiple media items from multiple representations or auxiliary files of one item. Preserve the association between a video alternative and its HLS descriptor. A Telegram album comprises separate message IDs; listing one message must not silently expand the album.
- **R4 Honest coverage:** Define the exact supported message/file families, unknown metadata, duplicate IDs, inaccessible paid media, and excluded nested media. Do not represent a locked preview or inline thumbnail bytes as a downloadable file ID. Existence in a snapshot is not a guarantee of subsequent download success.
- **R5 Compatibility:** Add enumeration without changing either published streaming RPC or any older RPC. GetMessageFile keeps its current deterministic main-file choice. DownloadFile keeps its streaming/error/cancellation behavior.
- **R6 Boundaries:** Reuse the existing GetMessage repository operation and application/transport separation. No new listener, background download, update queue, generic recursive SDK traversal, or TDLib dependency upgrade.

## Proposed scope for design review

These are **[ASSUMPTION] proposed defaults**, not additional owner quotations: cover the eight currently supported media content types (photo, video, document, audio, voice note, animation, video note, sticker), their explicitly identified direct file references, and paid-media photo/video items already visible to the current account. Include photo sizes, video alternatives and HLS files, file-backed thumbnails/covers, and regular-sticker premium animation. Expose locked/unsupported paid slots without purchasing or unlocking. Exact schema and metadata policy belong in design.md.

## Non-goals

Album expansion, history scanning, message searching, media purchasing/unlocking, recursive link-preview/reply/forward/story/game/invoice extraction, inline minithumbnail export, HLS playback or playlist rewriting, ZIP/batch downloads, persistent file identities, download-session tokens, changing the default GetMessageFile selection, and implementation/publication in the current design phase.

## Acceptance criteria for later implementation

- **A1 Metadata-only:** A real app/transport listing path calls GetMessage once; strict mocks fail any DownloadFile, GetFile, prefix-size, or ReadFilePart call. A photo with three sizes lists all three positive file IDs before any download.
- **A2 Media versus variants:** Exact fixtures cover every supported content type and each included file field, including video alternatives paired with HLS. Two paid photo/video items remain separate groups. Nil and unsupported paid slots preserve their original positions; locked previews have no file entries.
- **A3 Explicit selection:** List through the registered gRPC server, select a non-default photo size or alternative-video file, then DownloadFile that ID through the real app/transport path. Verify payload identity, ordered completion, and first payload before upstream completion.
- **A4 Compatibility:** The 17 RPCs at baseline 2af222b, their field numbers and streaming flags remain intact. Regressions preserve GetMessageFile's largest-by-file-size photo choice (last on ties), primary-video choice, and unsupported paid-media behavior. The existing two-RPC progressive mutation checks still pass.
- **A5 Snapshot semantics:** Assert deterministic group/file order, repeated-ID associations, omitted invalid file references, metadata unknowns, and default selection. A message with an album ID returns that ID but causes no sibling-message/history lookup. Text/unsupported content and supported-but-empty content have distinct documented responses.
- **A6 Errors:** Reject invalid request IDs before app calls. Test cancellation/deadline, missing messages, flood wait, readiness timeout, and upstream failure according to the explicit mapping. Never convert access failures to successful empty listings.
- **A7 Deliverability:** Regenerated bindings match the source proto; no proto symlink replacement. Linux vet/full internal tests pass on the implementation source. An acceptance guide supplies list/select/download examples, identifies the source/image used, and separates synthetic validation from live Telegram results. New image publication requires its own implementation-stage execution; the current dispatch-test image does not contain this endpoint.
