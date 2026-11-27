# Independent streaming design gate

Active task: `.trellis/tasks/11-25-facade-file-download`.
Revision baseline: `518bfcc3b167512f089403d1709fb5bd38a6c2e0`.
Published public API baseline: `fbc22cc` (15 pre-existing RPCs).

The owner explicitly requires first file bytes while TDLib is still
downloading. Path-only and full-file-then-stream implementations were
rejected. `prd.md` is revised accordingly; `review/streaming-scope.md`
records authorization. Historical reviews in `review/path-only/` are not
the ruler for this design. The previous design approval is superseded.

Apply the configured Design Reviewer D1-D8 criteria independently to the
new design.md and corrected PRD. Write only
`review/streaming-design-findings.json`, with round 1 for this fresh gate
(round 0 was the orchestrator's requirement mismatch, not an independent
review). Preserve historical design-findings.json and code findings.

Focus on concrete implementability and falsifiable R1-R9/A1-A8 coverage:

- Exact streamed proto, app/transport signatures, message/file resolution,
  safe-prefix observation and capped reads. Verify real pinned SDK field
  names and return types, not guessed ones. Local native sources are in
  main `.trellis/.runtime/tdlib-22d49d5-streaming/`; research facts include
  full SHA and source anchors.
- Payload before completion, not a metadata-only initial message; no
  synchronous download or waiting for completed state before first read.
- Ordered cursor, bounded reads, size-unknown behavior, terminal data
  arriving between a prefix query and completion observation, read errors,
  inactive incomplete state, and successful end only after final bytes.
- Context, SDK in-flight timeout limitation, slow-client backpressure,
  per-request resource lifetime and same-file cancellation ownership.
  Don't demand an ownership manager if a clear shared-cache policy suffices;
  if one exists, check acquire vs last-release/cancel races explicitly.
- Existing NoBusinessUpdates and shared SDK receiver remain live without
  a new unconsumed queue/listener. No reverse dependencies/test-only hooks.
- Cross-layer bufconn test uses the real facade and real transport with an
  upstream completion barrier. A nonempty client chunk must precede barrier
  release. A regression to completion-gating must fail on Linux. SDK mocks
  must not accidentally force the old synchronous behavior to look green.
- State telemetry is a sample, not a proof of the exact instant of remote
  receipt. Live verification must state that limitation and not mislabel
  cached/tiny-file completion as a streaming failure or simulated proof as
  a real Telegram result. App must send any available prefix without an
  artificial completion gate; native unreadable bytes cannot be streamed.
- Runnable acceptance client saves progressively and treats stream errors
  as incomplete output. All 15 old-main contracts preserved; this ticket's
  two unreleased RPCs may change in place under the corrected PRD.

No source/spec/PRD edits, no commit/push/merge, no Windows cgo build tests.
Static design PASS is not runtime validation. Return concise gate summary
and the report path; avoid printing large source excerpts.
