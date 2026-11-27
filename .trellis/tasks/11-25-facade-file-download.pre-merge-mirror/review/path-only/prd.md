# PRD — facade file download over gRPC

## Background

The facade's gRPC surface (`FacadeGRPC`, 16 RPCs) supports sending local files
(`SendMessageAlbum` + `NewMessage.file_path`) but exposes **no way to download a
file received in Telegram**. The proto `Message.file_path` field is dead:
`clientMessageToProto` never populates it. TDLib v0.7.6 (pinned dependency)
provides the full file-download API (`downloadFile`, `getFile`,
`getFileMimeType`, `cancelDownloadFile`); it is simply not wired through the
layers.

## User intent

A gRPC client must be able to ask the facade to fetch a media file from a
Telegram message and receive the local filesystem path of the downloaded copy,
consistent with the existing file exchange pattern (both sides operate on paths
on a shared/owned filesystem — that is how `SendMessageAlbum` already receives
`file_path`).

## Requirements

- R1: New gRPC RPC `GetMessageFile(GetMessageFileRequest) returns
  (MessageFileResponse)` on the existing `FacadeGRPC` service. The request
  identifies a message (`chat_id`, `message_id`). Photo messages resolve to
  the largest variant (client-selectable variant selection is out of scope,
  see Non-goals).
- R2: New gRPC RPC `DownloadFile(DownloadFileRequest) returns (FileResponse)`
  on the same service, taking a TDLib `file_id` and downloading it
  synchronously (blocks until the local copy is complete).
- R3: The app layer (`internal/app/facade`) owns message→file resolution and
  download triggering. The gRPC transport layer only maps raw TDLib values to
  proto. The infra layer (`internal/infra/telegram`) gains passthrough wrapper(s)
  following the existing `clientAdapter` → `Repo` pattern; the concrete wrapper
  set is design.md §8 (currently `DownloadFile` only — see design.md §1
  Revision 2 fact note for why `GetFile`/`GetFileMimeType` are not added).
- R4: Error mapping stays semantic: message with no downloadable file →
  `NotFound`; download failures → `Internal`. (The `DeadlineExceeded` 502
  mapping exists for callback answers only; `downloadFile` failures are not
  code-classified — the exact mapping is design §7, [ASSUMPTION]-marked.)

## Acceptance criteria

- A1: `GetMessageFile` resolves a media message (photo/video/document/audio/
  animation/voice/videoNote/sticker) to a downloadable `File`, downloads it
  synchronously, and returns the local `file_path` plus `size`. For photos the
  largest variant is returned by default; the response carries `file_id` so
  the client can call `DownloadFile` directly if it wants.
- A2: `GetMessageFile` on a non-media message returns a typed error mapped to
  gRPC `NotFound` (or `FailedPrecondition`); it never returns an empty path.
- A3: `DownloadFile(file_id)` blocks until the file is locally available and
  returns `file_path` + `size`; a failed/canceled download returns an error,
  never a partial path.
- A4: No existing RPC, proto message, or field number/semantic changes. The
  two new RPCs and their messages are purely additive.
- A5: New behavior is covered by unit tests in `internal/app/facade` and
  `internal/transport/grpc` that can fail (red-path proven), using the
  existing mockery-generated mocks. No existing test assertion is deleted or
  weakened.

## Non-goals

- No streaming of file bytes over gRPC (no client streaming / server
  streaming RPCs). The contract is path-based, matching `SendMessageAlbum`.
- No progress reporting / `updateFile` subscription / push updates.
- No upload-side changes (`SendMessage`/`SendMessageAlbum` behavior
  unchanged).
- No GraphQL exposure of file download (GraphQL schema stays `status`-only).
- No change to engine (push/sink) behavior or ruleset.
- No download cancellation / pause / list-downloads RPCs in this ticket.
- No client-selectable photo variant selection (largest variant only in this
  ticket; an additive proto field can be introduced later without breaking
  the contract).

## Verification channel

This repo is developed on Windows where `go-tdlib` (cgo, linux/darwin build
tags) cannot compile at all. `go vet ./...` + `go test ./internal/...` run in
GitHub Actions `ci-test.yml` (Linux, real TDLib static libs) — that is the
authoritative green. Local verification is limited to: `gofmt -l`, proto
regeneration byte-identity, and mock/interface-signature sanity. A branch push
triggers CI via a PR to main (a bare branch push does not: the workflow
triggers on push[main] + pull_request[main] + workflow_dispatch).
