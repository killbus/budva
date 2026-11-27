# Independent code review — progressive streaming

Active task: `.trellis/tasks/11-25-facade-file-download`.
Use the configured Architecture-Focused Reviewer profile. Its normative
inputs are current design.md, review/approval.md and relevant spec; PRD is
scope/acceptance. Review baseline is the planning-only commit supplied by SHA
by the orchestrator, subject `docs(task): lock progressive facade download
design (11-25)`. fbc22cc is only the public compatibility baseline.

Owner explicitly rejected path-only and full-download-then-stream behavior.
The PRD/design revisions are authorized planning changes already independently
reviewed (streaming gate round 1). They must not be changed during coding.
Do not apply the superseded review/brief.md or historical prompts.

Output ONLY to review/streaming-findings.json (fresh round 1); read implementer
evidence in review/streaming-responses.json. Preserve historical reports.
Use normal profile prechecks against the NEW planning baseline, not 518bfcc
or fbc22cc; verify the exact design hash from approval.md and report if drift.

Trace actual entry and dependencies from cmd/facade registration to handler,
app progressive core and Repo adapter. Check real layer ownership and direct
production reachability before any finding. Streaming callback DTOs belong
to app, proto mappings/Send belong to transport, raw SDK methods belong to
infra. No receiver takeover, new queue, producer goroutine, file cache manager
or test-only production seam. Cancellation policy is consumer-only.

Verify §5 frame ordering and §7 safe-prefix/terminal algorithm, SDK call
limits, context/send-error precedence and main RPC preservation. Compare
removed/replaced tests individually: this ticket's obsolete path assertions
must change, unrelated tests must remain intact. New A1 must observe nonempty
client payload while the upstream barrier is still closed. A fake that
pre-completes or synthesizes EOF from estimated size cannot witness the design.

CI verification scope is a small temporary mutation script and one existing
job call; preserve build/cache/pins/triggers. Mutation compile errors are not
red proof. Runtime evidence may still be pending at code review; do not call
static review Linux validation or sampled state real Telegram timing proof.
Acceptance client and image publication follow §11, with no merge permission.

Profile convergence/reachability/scope rules apply. Only concrete in-scope
architectural violations are blocking; don't use styling/test totals as
architecture evidence. Report all round-1 blockers together and avoid
inventing requirements beyond the locked design.
