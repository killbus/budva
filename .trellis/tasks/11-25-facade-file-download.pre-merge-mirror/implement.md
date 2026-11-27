# Implement — progressive gRPC file download

Active task: `.trellis/tasks/11-25-facade-file-download`.
Read `review/approval.md`: design SHA256
`ff408bb8ae45f9155468a271ac285ac2bf5578f53d08bf010d1010e2d475147d`.
Implementation/review baseline is the planning-only commit containing this
plan, supplied by SHA in the dispatch. Public API baseline is fbc22cc.
Historical prompts/briefs/reports are superseded for this phase.

## Ownership and execution

Use the configured Implementer profile with the task-active first line and
its ownership rules. Use trellis-implement when available; the project-local
Trellis hooks/scripts are absent in this checkout, so if no such agent/tool
is available, the configured Implementer performs the scoped edits directly
and records that fallback. No permission to move the design ruler is implied.

PRD, design, plan, approval, task/progress records, .trellis/spec and historical
review reports are read-only for the Implementer. Editable production/test
paths are exactly design §12. Writable implementation evidence is
`review/streaming-responses.json`; problems go to `review/spec-gap.md` or
`review/follow-ups.md`. The new acceptance guide may be written at
`review/streaming-acceptance.md`. No commit/push/merge; orchestrator owns git.

## Phase A — runnable protocol and production chain

1. Verify lock hash and worktree/baseline. Read design §1-§13, current code,
   relevant spec and `research/progressive-download-facts.md`.
2. Replace only this ticket's two unary methods and response messages with
   exact design §5 server streams; preserve all 15 fbc22cc RPCs/messages and
   api/proto symlink. Regenerate bindings with the pinned tools below.
3. Add the three live raw adapter methods (GetFile, prefix size, ReadFilePart)
   with exact pinned signatures and FloodWaitFromError conventions. Keep
   DownloadFile and existing receiver/NoBusinessUpdates ownership unchanged.
4. Implement app DTOs/callbacks and media resolution per §6; shared core per
   §7: Synchronous:false, state before prefix, capped reads, callback
   backpressure, correct unknown/empty EOF, cancellation without global
   upstream cancellation. No full-file staging or detached goroutines.
5. Implement new transport handlers, validation, mapping, send-error and
   context precedence per §8; preserve unrelated unary behavior.
6. Synchronize all three mocks; replace this ticket's obsolete tests enough
   that all callers compile against the new signatures. Add focused app,
   transport and infra coverage alongside the production chain. Do not
   weaken/delete unrelated assertions. Inspect actual fixture fields.
7. Check gofmt, generation identity, old-main contract and scope. Write
   partial-progress evidence before returning; no claim of completion yet.

## Phase B — falsifiable acceptance and local closeout

1. Complete all design §9 coverage. A1 uses real app + transport + generated
   client over bufconn, both RPCs, completion barrier and nonempty client
   payload before release. Fake synchronous start must wait for completion,
   so the §10 mutant fails at the intended pre-completion assertion.
2. Use synctest for timers/deadlines, no real sleeps; deterministic prefix
   evolution, exact bytes/bounds/backpressure, cleanup, same-file independent
   readers, completion race, unknown/empty state, error/send/cancel semantics.
3. Add scripts/verify-facade-streaming.sh and one ci-test call after existing
   full Go tests (§10). Unique mutation, red reason check, EXIT restoration,
   original-byte verification and final green; compile failures aren't proof.
   Preserve build/pin/cache/triggers/permissions and full vet/test commands.
4. Add cgo-free acceptance client per §11: exclusive .part creation, bounded
   sequential writes, incremental digest, valid completion + clean EOF before
   final rename; first payload diagnostics and honest sampled-state wording.
   Write runnable message/file examples in review/streaming-acceptance.md.
5. Update TEST-MATRIX from actual cases; verify row sum. Final gofmt, exact
   scratch regeneration, API compatibility and diff scope checks.
6. Write review/streaming-responses.json round 1 with changes/spec clauses,
   acceptance coverage and exact validations performed vs pending. Preserve
   historical review/responses.json. No PASS/FAIL self-verdict.

## Environment and verification

Worktree: `D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download`.
Use PowerShell without login profile. Tools already installed:

- `D:\Repositories\budva\.trellis\.runtime\protoc\bin\bin\protoc.exe`
  reports libprotoc 34.2.
- `C:\Users\ying\go\bin\protoc-gen-go.exe`: 1.36.11.
- `C:\Users\ying\go\bin\protoc-gen-go-grpc.exe`: 1.5.1.
- SDK cache: `C:\Users\ying\go\pkg\mod\github.com\zelenin\go-tdlib@v0.7.6`.

Canonical generation (repo root; plugins on PATH):
`protoc -I . --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/transport/grpc/pb/facade.proto`.
Compare final bindings with a scratch generation; generated diff is allowed
to replace the unreleased methods/messages, not required to be additions-only.

Use apply_patch for edits; never bypass denied worktree writes by writing in
main then copying without proper escalation. No credentials in artifacts.
Scratch native/proto tools must not be staged.

Windows cannot compile TDLib-dependent code. Do not run those Go builds/tests
or claim static checks as execution. The orchestrator runs the reviewed head
through existing Linux CI, including executed A1 red/green, before publishing
the acceptance image. A standalone cgo-free client build is separate evidence
and does not validate production TDLib or live Telegram streaming.

The code reviewer uses `review/streaming-code-brief.md` and writes only
`review/streaming-findings.json`; revised-design evidence lives in the planning
baseline, so implementation diff must not touch its ruler. A design issue
requires the documented revision/gate path, not a hidden fix in source.
