# Design — progressive facade gRPC file download

Revision baseline: `518bfcc3b167512f089403d1709fb5bd38a6c2e0`.
Public compatibility baseline: `fbc22cc` (15 existing RPCs).
Status: proposed; independent streaming design gate and new hash lock required.
This replaces the path-only design preserved in `review/path-only/design.md`.
The corrected `prd.md` and `review/streaming-scope.md` authorize the replacement.
Numbered sections are normative implementation/review anchors.

## 1. Intent, sources and decisions

R1/R2 require file bytes at the remote client while TDLib is still downloading.
Start TDLib asynchronously, read each available contiguous range, and send it
immediately through a server stream. Neither a server-local path nor streaming
after full-file staging meets this contract. TDLib's internal disk cache remains.

Verified dependency facts and source locations are in
`research/progressive-download-facts.md`: go-tdlib v0.7.6 and native TDLib
`22d49d5b87a4d5fc60a194dab02dd1d71529687f` are unchanged. In particular:

- `DownloadFile` with `Synchronous=false` returns after starting the download.
- `GetFileDownloadedPrefixSize` returns `*client.FileDownloadedPrefixSize`;
  its `Size int64` is the contiguous readable length from the requested offset.
- `ReadFilePart` returns `*client.FilePart` with `Data []byte`. Count must be
  positive and bounded; zero would read everything available.
- `DownloadedSize` and physical cache length are not safe readable boundaries.
- Native `FileManager.cpp` resolves temporary/full paths and retries the rename
  race internally. SecureEncrypted partial data is unreadable until verified.
- SDK calls do not take context; an in-flight Send can last until its SDK
  catch timeout (default 60 seconds). RPC cancellation cannot interrupt it.

[ASSUMPTION] Use these request APIs with polling rather than file handles or
update subscriptions. The native read API handles cache-path transitions; polling
keeps `NoBusinessUpdates` and the shared receiver unchanged (R4-R6). This trades
extra SDK/JSON calls for simple ownership; it is not a throughput guarantee.

[ASSUMPTION] Fixed 256 KiB maximum payload, 250 ms empty-prefix poll interval,
priority 16, offset 0 and unlimited TDLib download (`Limit=0`). These constants
implement R2/R5 without adding tuning/resume APIs. Timers wait only when there
are no readable bytes; available data is never deliberately delayed.

## 2. Scope and non-goals

Replace the two unmerged/unreleased path-only methods in place (R7). Preserve
all 15 RPCs and message fields that exist at `fbc22cc`, including old `Message`
and upload path fields. Do not add a duplicate legacy download API.

No upload, GraphQL, engine/ruleset, photo variant parameter, resume/range API,
download manager, HTTP endpoint, second TDLib session, new cache service,
dependency upgrade, release/latest tag, or zero-disk promise. No SDK update
listener/queue/drain or repository-wide session/close refactor. No merge.

## 3. Module responsibilities

| Module | Primary responsibility |
|---|---|
| `internal/transport/grpc` and its `pb` | Wire contract, request validation, sequential frame sending and gRPC error mapping |
| `internal/app/facade` | Media resolution and progressive byte orchestration |
| `internal/infra/telegram` | Raw TDLib adapter calls with existing error translation |
| `cmd/facade-file-download-client` | Remote acceptance consumer writing and validating a local file |
| `scripts/verify-facade-streaming.sh` and `ci-test.yml` | Executable Linux regression evidence using real installed TDLib |

The existing `cmd/facade/main.go` wiring remains sufficient: one Repo,
`facade.New(repo)`, transport construction and generated service registration.

## 4. Dependencies and ownership

Production calls flow transport -> app -> infra -> pinned TDLib. App may use
the repository's existing raw client types and new app-owned stream DTOs; it
must not import protobuf, gRPC or transport. Infra must not import app/pb.
The acceptance client imports only generated pb, grpc and standard library,
so it has no TDLib/cgo dependency. Test fixtures cannot change public production
constructors or expose internal state just for tests.

[ASSUMPTION] Use an ordinary synchronous emit callback on the RPC handler
goroutine (R4/R5). One request has one cursor and at most one read/send in
progress. No producer goroutine, chunk queue, worker, iterator or subscription.
The callback returns only after `stream.Send` returns; no following read may
start before then. gRPC/SDK serialization uses a constant multiple of the
bounded chunk, independent of total file size; aggregate memory scales with
active requests and normal gRPC flow-control buffers.

[ASSUMPTION] TDLib owns the shared cache download, not an individual RPC.
Cancel/deadline/send failure stops that consumer's reads, waits and sends but
does NOT invoke `CancelDownloadFile`. The upstream transfer may continue to
completion even after the last reader leaves. This deliberately avoids
cancelling another reader or a pre-existing unrelated TDLib consumer. All
requests use identical offset/limit parameters and independent cursors; no
refcount registry, last-reader race, per-file lock or session-lifetime map.

## 5. Wire contract and frame lifecycle

[ASSUMPTION] Names/field numbers below are the minimal concrete R1/R3 wire
encoding; no published-main message is changed. Canonical file is
`internal/transport/grpc/pb/facade.proto`. The `api/proto/facade.proto` git
symlink must remain byte-unchanged.

```proto
rpc GetMessageFile (GetMessageFileRequest) returns (stream FileDownloadEvent);
rpc DownloadFile (DownloadFileRequest) returns (stream FileDownloadEvent);

message GetMessageFileRequest {
  int64 chat_id = 1;
  int64 message_id = 2;
}
message DownloadFileRequest {
  int64 file_id = 1;
}
message FileDownloadEvent {
  oneof event {
    FileDownloadMetadata metadata = 1;
    FileDownloadChunk chunk = 2;
    FileDownloadComplete complete = 3;
  }
}
message FileDownloadMetadata {
  int64 file_id = 1;
  string mime_type = 2;
  int64 size = 3;
  bool size_known = 4;
}
message FileDownloadChunk {
  int64 offset = 1;
  bytes data = 2;
  bool upstream_complete = 3;
}
message FileDownloadComplete {
  int64 size = 1;
}
```

Remove only this ticket's obsolete `MessageFileResponse` and `FileResponse`.
Both server handlers have the generated server-stream signature:
`(request *pb.<Request>, stream grpc.ServerStreamingServer[pb.FileDownloadEvent]) error`.

Successful sequence is exactly one metadata frame, zero or more nonempty
chunks, exactly one complete frame, then clean gRPC EOF. Chunk offsets begin
at 0 and increase by exactly the preceding payload length. No frame carries
a server path. Failure can occur before/after metadata or data; it returns a
non-OK status, never a success frame for an incomplete file. Receiving a
complete frame alone is insufficient: clients must also observe clean EOF.

Metadata is a startup snapshot, not an EOF oracle. `size_known` is true when
the starting File.Size is positive or the starting file is complete; size is
that nonnegative Size, otherwise 0 with size_known=false. ExpectedSize is not
sent or used for termination. Direct file-ID mode uses MIME `""`.

[ASSUMPTION] `chunk.upstream_complete` is the last GetFile completion state
sampled before that chunk's prefix query/read (§7). It supports live diagnostics,
not a claim about completion at the exact instant of client receipt. It must
never gate delivery. A1's completion barrier is the decisive timing proof.

Transport rejects direct file IDs outside 1..MaxInt32 before casting or app
calls. It also rejects message_id <= 0 or chat_id == 0; negative Telegram chat
IDs are valid. These new-method validations implement R8 and do not change
validation of old RPCs.

## 6. App API, message resolution and infra calls

[ASSUMPTION] App DTOs live in `internal/app/facade/file_download.go`:
`FileDownloadMetadata{FileID int32, MIMEType string, Size int64, SizeKnown bool}`,
`FileDownloadChunk{Offset int64, Data []byte, UpstreamComplete bool}`,
`FileDownloadComplete{Size int64}` and
`FileDownloadEvent{Metadata *FileDownloadMetadata, Chunk *FileDownloadChunk,
Complete *FileDownloadComplete}`. Exactly one pointer is non-nil per event.
The callback consumes data synchronously; app does not mutate/retain a sent
chunk. There is no shared event state.

Public service methods:

```go
GetMessageFile(ctx context.Context, chatID, messageID int64,
    emit func(FileDownloadEvent) error) error
DownloadFile(ctx context.Context, fileID int32,
    emit func(FileDownloadEvent) error) error
```

Both call one private progressive core with file ID, MIME and emit. Message
mode checks context, calls existing GetMessage, checks context again, resolves
media, then enters the core. A nil message/content/wrapper/file or unsupported
content yields `MessageFileNotFoundError`; preserve existing message-query
errors. Typed nil contents must not panic.

| Content | File inside content wrapper | MIME |
|---|---|---|
| MessagePhoto | Photo.Sizes[].Photo | image/jpeg |
| MessageVideo | Video.Video | Video.MimeType |
| MessageDocument | Document.Document | Document.MimeType |
| MessageAudio | Audio.Audio | Audio.MimeType |
| MessageVoiceNote | VoiceNote.Voice | VoiceNote.MimeType |
| MessageAnimation | Animation.Animation | Animation.MimeType |
| MessageVideoNote | VideoNote.Video | empty |
| MessageSticker | Sticker.Sticker | empty |

Field spellings are verified against v0.7.6; see the research and historical
G3 correction. [ASSUMPTION] Retain the fixed largest-photo definition: maximum
PhotoSize.Photo.Size, ties choose the last valid slice entry; skip nil entries.
No valid entry means missing media. [ASSUMPTION] Photo MIME is image/jpeg;
other missing MIME fields yield empty without an extra SDK lookup.

The app telegramRepo, infra clientAdapter and Repo expose precisely these
live file primitives, using the SDK signatures unchanged:

```go
DownloadFile(*client.DownloadFileRequest) (*client.File, error)
GetFile(*client.GetFileRequest) (*client.File, error)
GetFileDownloadedPrefixSize(*client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error)
ReadFilePart(*client.ReadFilePartRequest) (*client.FilePart, error)
```

DownloadFile already exists at the revision baseline; add the other three.
No GetFileMimeType or CancelDownloadFile. Follow existing clientAdapter
passthrough conventions and `FloodWaitFromError`, with no `withChatReady`:
file-only operations are not chat-scoped. This implements R6 and the
live-consumer rule in `telegram-tdlib-guidelines.md`. Sync all three affected
mock interfaces without unrelated generator churn.

## 7. Progressive algorithm and terminal oracle

Each SDK operation and each emit has a context check immediately before and
after it. If cancellation occurs during a call, return ctx.Err after it
returns, without using returned bytes or issuing another call. Error handling
is §8. SDK calls execute directly, without detached timeout goroutines.

1. Start `DownloadFile{FileId, Priority:16, Offset:0, Limit:0,
   Synchronous:false}`. Error, nil file/local, negative size or nonpositive
   returned file ID is failure. Use the returned file ID for following calls
   (TDLib can canonicalize IDs). Emit metadata immediately, set cursor=0.
   A complete cached start follows the same loop; do not require a path.
2. Get current file state via `GetFile{FileId}`. Validate non-nil File/Local
   and nonnegative Size. Then query
   `GetFileDownloadedPrefixSize{FileId, Offset:cursor}`. This order is
   mandatory: the completion observation precedes the prefix observation.
   Nil/negative prefix or offset+prefix overflow is a download failure.
3. When prefix.Size > 0, choose `count=min(prefix.Size, 256 KiB)` and call
   `ReadFilePart{FileId, Offset:cursor, Count:count}` without waiting for
   completion. Require non-nil part and `len(Data)==count`; nil, empty,
   short or oversized results are failure, not EOF. Emit one chunk using
   the current cursor and step-2 completion sample. Advance cursor only
   after successful emit. Continue at step 2 without an artificial wait.
4. When prefix.Size == 0 and the step-2 state is complete, require equality
   of cursor and File.Size if Size > 0. If Size is 0 (unknown/empty), the
   complete-state prefix query returning 0 at cursor is the EOF oracle:
   native Full locations return actual remaining bytes, unlike estimated
   size. Thus an empty complete file succeeds at cursor=0, and an initially
   unknown file is drained fully. Emit complete{Size:cursor}; success is
   only the callback success and nil method return.
5. When prefix.Size == 0 and the state is incomplete but active, wait via
   a context-aware timer for 250 ms, release/stop the timer on every exit,
   then repeat step 2. Size/ExpectedSize/progress counters never end the loop.
6. When prefix.Size == 0 and the state is incomplete and inactive, fail.
   [ASSUMPTION] Treat stopped upstream with no readable remainder as
   failed rather than silently restarting or waiting forever. If a stopped
   file still has a prefix, step 3 may deliver it before the terminal error;
   it must not become a complete stream.

For a complete step-2 state with positive Size, also reject cursor beyond
Size or a prefix larger than Size-cursor before reading: contradictory
authoritative data must not yield success. For incomplete states, ignore
Size as a read cap; safe prefix alone bounds reads.

Completion races: if state is sampled incomplete and completion happens
before/after the prefix query, this iteration may read or wait; the next
iteration observes completion before querying its remaining prefix. Never
reuse a zero prefix obtained before a completion observation as EOF evidence.
Final bytes are drained before completion. The native ReadFilePart handles
cache-path rename retries; an unrecoverable read error fails this request,
without implementing an independent file retry/cache manager.

## 8. Errors, cancellation and transport

Retain MessageFileNotFoundError and FileDownloadFailedError with Unwrap.
SDK/read/state failures use FileDownloadFailedError with file ID and cause;
missing media uses MessageFileNotFoundError. Context errors and callback
errors are returned directly, not wrapped as download failures.

The two handlers use a shared transport-local streaming helper if needed.
It maps one app event to one pb event and calls Send sequentially. It records
the first Send error; when app returns, that transport error takes precedence
and is returned unchanged. No subsequent Send is attempted. This preserves
transport status without making app import gRPC.

For app failures on these two RPCs, check errors.Is for Canceled and
DeadlineExceeded before delegating to existing mapFacadeError; don't change
unrelated unary error behavior. Missing media/existing typed missing message
maps to NotFound, flood waits retain ResourceExhausted + RetryInfo, other
upstream/download failure maps to Internal. [ASSUMPTION] Do not classify raw
TDLib 400 errors by guessing strings: invalid client ranges are InvalidArgument,
but a valid-range unknown file reported by TDLib remains an upstream failure.
Use a stable file-ID/operation error message for download failures in the new
transport so raw native cache paths are not exposed; preserve underlying
causes internally for errors.Is/As. No clean EOF on failure after data.

Cancellation during prefix wait wakes immediately; cancellation during Send
follows grpc's context; an in-flight SDK call can only finish/timeout before
cleanup. Do not promise an upper bound shorter than SDK catch timeout.
No application goroutine, file handle or subscription remains after return;
the continuing shared cache download is the explicit §4 ownership policy.

## 9. Tests and falsifiable acceptance mapping

Replace only tests added by this ticket that enforce synchronous/path-only
behavior. Preserve tests and assertions unrelated to downloads. Follow
quality-guidelines: strict mocks, shared fixtures, event-based barriers and
testing/synctest for timers/deadlines; no real sleeps. Test imports/callback
fixtures must use actual v0.7.6 types. Red-path explanations go in review
evidence; no need for production testing hooks.

| PRD | Required executable evidence |
|---|---|
| A1 | `TestProgressiveDownloadBeforeCompletion` in transport package: real facade Service + real Transport + generated gRPC client on bufconn; controllable fake repo holds completion behind a barrier; client receives nonempty chunk before test releases barrier. Run subcases for both RPCs. Fake asynchronous start returns immediately; if Synchronous=true it waits for completion. Assert the async request too. A full-file/completion-gated implementation must fail on the pre-release payload assertion. Metadata never releases the barrier. |
| A2 | App tests progressively expose several prefixes from fixed source bytes, assert request Offset/Count never exceed safe prefix, ordered exact bytes, maximum chunk cap, and cached-file behavior. Block emit and assert no subsequent SDK/read activity until release (backpressure). |
| A3 | Synctest cancel in empty-prefix wait, cancellation after a chunk, and deadline; callback/transport Send failure stops reads; cancelled in-flight fake call releases before method exits and no following operation occurs. Two real Service requests share a concurrency-safe fake file: cancel one, advance the prefix, other completes with exact bytes; no upstream cancellation method exists. |
| A4 | Unknown initial size, complete empty file, positive final size mismatch, completion between state and prefix samples (zero and final-data cases), inactive incomplete, nil/bad SDK returns, SDK read error, short/oversized part. No failed case may emit complete or return successful RPC EOF. |
| A5 | All eight media/MIME mappings, nontrivial largest-photo and tie, nil content/wrappers/files, no media, missing message, direct range and message-ID validation, flood/context/upstream mappings. |
| A6 | Pinned scratch regeneration equals tracked bindings; compare old-main proto descriptors/contracts to fbc22cc, preserving all 15 RPCs. Review removed assertions individually against this authorized replacement. |
| A7 | Linux full vet/internal tests plus executed A1 red/green as §10. Static strict-mock reasoning alone is insufficient for this specific gate. |
| A8 | §11 acceptance client; simulated full-stack evidence is labelled as such. Publish only after source CI/review and both architecture smokes; record SHA/digest/version and user-run real Telegram evidence separately. |

Bufconn test deadlock guard uses context deadline inside synctest, not wall-clock
timing luck; always release fake waiters/cancel/close server+connection in cleanup.
Use a buffered listener and explicit channels so completion is controlled by
the receipt of nonempty data, not mock call counts. A timeout/deadlock is a test
failure with the barrier still unreleased, never permission to release it early.
The fake does not use real Telegram credentials or add TDLib listeners.

## 10. Linux validation and generation

Windows cannot build packages importing pinned cgo TDLib. Local gofmt,
field/signature inspection and proto generation are useful but not runtime
proof. Use protoc 34.2, protoc-gen-go 1.36.11, protoc-gen-go-grpc 1.5.1 and
source_relative outputs for canonical facade.proto. Scratch regenerate the
final proto and require byte identity. Generated changes need not be additions
only: the two unreleased unary methods/messages are intentionally replaced.

[ASSUMPTION] Add a small `scripts/verify-facade-streaming.sh` and invoke it
after the existing green `go test ./internal/...` in ci-test.yml (R9/A7).
It reuses the job's installed real TDLib, avoiding a new runner/image/workflow.
Keep current pin/cache/CMake override/parallelism/triggers/permissions and
existing full vet/test commands unchanged. Script sequence:

1. Run only `TestProgressiveDownloadBeforeCompletion` with `-count=1` against
   original source, requiring success.
2. Copy the relevant app file to a temporary backup, register an EXIT trap
   that restores it, and change exactly the unique download starter literal
   `Synchronous: false` to `Synchronous: true`. Fail if not exactly one match.
   This is a temporary workspace mutation, never committed production logic.
3. Run the same test; require nonzero exit AND the named progressive test
   failure. Compile/import/setup failure, zero tests or a harness timeout do
   not count as red proof. Capture/print enough output for CI evidence.
4. Restore and verify byte identity, rerun the original test and require green.

The fixture must diagnose the failed pre-completion payload observation when
the synchronous mutant is used (not merely return a fake setup error). The
assertion of false startup mode is additional evidence. If an implementation
uses a different concrete starter spelling, update this narrow mutation anchor
before lock/implementation rather than shipping an ineffective script.

This narrow test-runner addition is part of the corrected scope, not a general
CI refactor. Do not publish a deliberately broken commit just to obtain red.
Record test commands/output/run URL/exact head; old run 45630679946 proves
only the superseded path-only code.

## 11. Runnable acceptance and publication

[ASSUMPTION] Provide `cmd/facade-file-download-client/main.go`, run from this
source checkout with `CGO_ENABLED=0 go run ./cmd/facade-file-download-client`.
No extra Docker binary/layer is required. Flags: `-addr` (host:port), `-out`
(required destination), `-file-id` OR both `-chat-id`/`-message-id`, `-timeout`
(default 10m), optional `-expected-sha256` (64 hex digits). Use the facade's
existing plaintext gRPC endpoint; document trusted network/port-forward setup
in the acceptance guide. No transport/auth deployment change in this task.

Use context plus signal cancellation, call the matching generated stream,
write chunks directly to `<out>.part`, and compute SHA256 incrementally.
Reject duplicate/missing metadata, nonpositive file ID, unknown/empty event,
empty/oversized/nonsequential chunk, duplicate completion, data after complete,
missing complete before EOF, complete size mismatch and non-OK RPC status.
Only close successfully, verify optional expected digest and rename to out
after clean EOF; preserve partial file on error. Refuse pre-existing out or
part (exclusive creation); never overwrite acceptance artifacts.

Report metadata, first nonempty payload latency, bytes saved, first chunk's
upstream_complete sample, final byte count and digest. The guide supplies both
file-ID and message examples and explains the sample's timing limitation.
Use a large, uncached ordinary Telegram document with independently known
SHA256 for live acceptance; tiny/cached/encrypted unreadable cases cannot
prove early delivery. A1 proves deterministic causal ordering in the real
app/transport; startup smokes or sampled false state alone do not prove real
Telegram timing. If credentials/sample are unavailable, report live acceptance
pending and provide the ready command, never invent results.

After independent code review and current Linux green, dispatch existing
docker-publish on the reviewed feature head. Both amd64 and arm64 smokes must
pass before its existing digest/manifest publication creates dispatch-test.
Record manifest digest, source SHA and dev-run version; verify image revision
matches reviewed code. Cancelled run 45640542028 did not publish this feature.
Merge still requires owner authorization after acceptance.

## 12. File set and ruler lock

Source scope:

- `internal/app/facade/service.go`, new `file_download.go`, `errors.go`,
  `service_test.go`, new `file_download_test.go`, `mocks/telegram_repo.go`.
- `internal/infra/telegram/client_adapter.go`, `mocks/client_adapter.go`,
  `repo_test.go` for exact passthrough/error evidence.
- `internal/transport/grpc/transport.go`, optional `file_download.go`,
  `transport_test.go`, new `file_download_test.go`,
  `mocks/facade_service.go`, canonical proto and its two generated Go files.
- `cmd/facade-file-download-client/main.go`,
  `scripts/verify-facade-streaming.sh`, `.github/workflows/ci-test.yml`,
  `docs/TEST-MATRIX.md`.
- This task's planning, review, research, progress and acceptance records.

New tests may use the listed existing test files instead of adding both
new files; this is organization only, not permission to broaden behavior.
No Dockerfile/docker-publish workflow/spec edits are required. `.runtime`
tools/native sources/credentials must never be staged.

Planning artifacts are owner-authorized revisions. After independent design
PASS, record the new design_sha256 in review/approval.md, align implement.md,
and establish a planning-only commit as the implementation review baseline.
Implementer treats PRD/design/implement/spec as read-only; discrepancies go
to review/spec-gap.md. Fresh streaming reports use streaming-prefixed names
to preserve historical evidence. No self-awarded gate.

## 13. Risks and completion gates

Polling and SDK base64/JSON copies cost CPU/latency; the bounded read size
limits application memory but is not a throughput SLA. Polling loads TDLib
proportionally to active requests. Cancellation can leave upstream cache
work running (§4) and cannot interrupt an in-flight SDK request (§1).
Cache deletion/session shutdown can make a read fail midstream; callers
retain a partial output. No new recovery manager is implied.

Required order: independent design gate -> hash/plan baseline -> profile
Implementer -> independent profile Reviewer -> executed Linux A1 red/green
and full CI -> corrected dispatch-test -> live owner acceptance. Static
review and old CI cannot satisfy a later gate. Do not merge or describe
the progressive implementation/image as delivered until the relevant
runtime/publication evidence exists.
