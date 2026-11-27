# PRD — progressive facade file download over gRPC

## User intent and correction

The user requires a remote gRPC client to receive Telegram file bytes while
TDLib is still downloading the file. The user explicitly rejected both
returning only a facade-local path and waiting for TDLib to download the
entire file before starting a gRPC stream. Neither meets this task.

The former path-based PRD incorrectly inferred a shared filesystem from
the upload API. It is superseded. Its artifacts are retained under
`review/path-only/`; existing green tests and review verdicts prove only
that historical implementation. PR #16 is open and unmerged. Its acceptance
image run 45640542028 was cancelled before publishing any image tag.

## Requirements

- R1: Both message-based and file-ID-based downloads deliver bytes using
  server-streaming RPCs on FacadeGRPC. Message requests still identify
  chat_id/message_id; direct requests identify a TDLib file_id. All eight
  currently handled media types remain supported; photos select the largest
  variant. The client does not need the facade filesystem or docker cp.
- R2: Start a nonblocking TDLib download. Once a contiguous byte range is
  available, transmit it before full-file completion. Full-file staging,
  a completion gate before the first chunk, and whole-file memory buffering
  are forbidden. Internal TDLib cache files are allowed; zero disk use is
  not a requirement. Correctly handle an already-cached complete file.
- R3: Specify stream metadata (including file ID and best-effort MIME),
  ordered bounded byte chunks, and unambiguous successful completion.
  Unknown or estimated size must not cause a truncated success or waiting
  forever. Consumers only finalize their output after successful RPC end;
  a failure after any chunks leaves a partial download, not a valid file.
- R4: Honor RPC cancellation/deadlines and transport send errors. Stop
  unnecessary reads/production, release per-request resources, and define
  ownership of upstream TDLib cancellation. Concurrent requests for the
  same file must not corrupt data or cancel each other's download. Document
  the pinned SDK's non-context-aware call limits honestly.
- R5: Bound memory and queued work independently of file size. Slow clients
  apply backpressure to gRPC chunk production; do not block the shared TDLib
  update receiver or create an unconsumed business-update outlet. No second
  TDLib session, dependency upgrade, or new cache service.
- R6: Preserve transport -> app -> infra direction. Transport owns protocol
  mapping and stream sending; app owns download orchestration; infra owns
  TDLib adapter operations. Only add adapter methods with live consumers.
- R7: Preserve all 15 RPCs and existing message contracts from main baseline
  fbc22cc. The two new, unmerged/unreleased path-only RPC contracts may be
  replaced with streaming contracts; do not keep a path-only duplicate just
  for compatibility with an artifact that was never accepted or released.
- R8: Keep meaningful semantic errors for bad IDs, missing media, upstream
  failure, cancellation/deadline, and incomplete/corrupt reads. Do not
  convert mid-stream failure into a clean EOF or leak server paths as the
  client download mechanism.
- R9: Use Trellis plus the configured Paseo Design Reviewer, Implementer,
  and independent Reviewer loop. Re-lock a revised design before coding.
  Then verify Linux CI and publish a dispatch-test acceptance image from
  the reviewed source. Merge requires separate owner authorization.

## Acceptance criteria

- A1 (decisive progressive test): hold a fake upstream below completion with
  a nonempty contiguous prefix, call the real server-streaming transport,
  and observe a nonempty byte chunk at the client before allowing upstream
  completion. A metadata-only first response does not satisfy this test.
  Replacing the implementation with synchronous/full-file staging must
  cause the test to fail. Use synchronization barriers, not timing luck.
- A2: Extend available prefixes in multiple steps; client bytes must match
  the source exactly, in order, without gaps, duplicates, sparse/unwritten
  bytes, or reading beyond the reported safe prefix. Check the completed
  cached path and bounded per-chunk allocation.
- A3: Cancel during a prefix wait and during streaming, exercise a send
  failure/deadline, and verify resource cleanup. Two simultaneous requests
  for one file: cancelling one must not abort the other.
- A4: Empty/unknown size, completion with final bytes, upstream inactive or
  failed state, and short/read failures have deterministic end semantics.
  An incomplete/error stream never signals success.
- A5: Message media/MIME resolution and largest-photo choice remain covered;
  invalid file IDs and missing media preserve semantic error mappings.
- A6: Protocol generation matches the pinned toolchain; old-main APIs are
  unchanged. Existing unrelated tests are not removed/weakened. Tests for
  this ticket's obsolete synchronous path contract must be replaced with
  assertions for R1-R8, with the scope change explicitly recorded.
- A7: Linux go vet ./... and go test ./internal/... pass for the reviewed
  streaming commit. Demonstrate an executed red/green for A1 on a supported
  Linux channel. Windows static inspection cannot substitute for execution.
- A8: Publish amd64+arm64 dispatch-test only after both image smokes succeed;
  provide source SHA, digest/version, and a runnable acceptance client that
  writes a local file progressively. Live acceptance checks first payload
  before upstream completion and final byte integrity; image startup alone
  is insufficient.

## Non-goals

No upload, engine/ruleset, GraphQL, photo-variant-selection, release-tag, or
latest-tag changes. No download manager UI, pause/list RPC, resume/range API,
new HTTP file endpoint, whole-file-buffer fallback, or zero-disk guarantee.
Do not broaden into fixing all SDK session/close races. Any targeted shared
state needed for concurrent streaming must have an explicit lifetime and
cleanup contract in the design.

## Validation environment

Windows cannot compile the pinned cgo go-tdlib dependency. Existing Linux
ci-test workflow is authoritative; TDLib cache was successfully populated
by run 45630679946. Existing docker-publish workflow provides the acceptance
image. No Windows Go build/vet/test or test-only replacement TDLib library
may be reported as production validation.
