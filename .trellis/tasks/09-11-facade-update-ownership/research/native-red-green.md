# Native behavioral red/green validation

Status: all three behavioral reds and restored final green are verified. Independent executed-evidence reconciliation passed on 2026-10-11 with no blocking finding; PR/merge authority and actual-facade production-closure acceptance remain separate. Task remains in_progress; the production incident is not closed.

## Scope and source identity

- Date: 2026-09-12. The user approved isolated temporary branch/push/CI validation, excluding PR creation, merge, publishing, deployment, and live-account use.
- Production task branch: `fix/facade-update-ownership`, unchanged baseline `a314b0002e46b5a3ffeeef22c503a486a5293b13`. Its inherited dirty work is excluded from all validation commits.
- Temporary branch: `validation/facade-update-ownership-20260912`. Worktree: `D:/Repositories/budva/.trellis/.runtime/validation/facade-update-ownership-20260912`. Created clean from the baseline; no local/remote ref existed at preflight.
- Only `internal/infra/telegram/repo.go` and `.github/workflows/ci-test.yml` belong in temporary validation commits. Test code and assembly choices stay unchanged. The final source must match the baseline Git blob exactly.
- Main-worktree SHA256 before validation: repo.go `3D488415968C4D5F002B03EADC7C1D8934316FB747C31DFC9FEA347C605C22FB`; ci-test.yml `22E18F478B3071C77002953DA7127490B5EC59795B380FD4938C2ABC72A55E08`. These are working-file hashes, distinct from Git blob IDs and commit SHA.

## Runner and acceptance

Use the existing `ci-test` workflow on Linux/TDLib, pinning each run to its recorded head SHA. Keep Go 1.25.9, TDLib 22d49d5, existing cache policy, permissions, triggers, and scope unchanged. Temporary test invocation is `go test -count=1 -timeout=2m ./internal/...`; vet remains `go vet ./...`. Branch pushes alone do not trigger this workflow; main explicitly dispatches each run.

Each red must compile and reach its intended behavioral failure; a constructor-only failure, infrastructure failure, package timeout, or cleanup panic is not sufficient. The final restored source must run whole-repository vet and the complete internal suite uncached. Normal assertion failure plus package exit before the timeout is the relevant failed-path teardown receipt; it is not proof of native SDK close/race safety.

| Stage | Isolated mutation relative to baseline | Required observation | Result |
| --- | --- | --- | --- |
| M1 | Unconditional capacity-100 allocation and publication, migrated constructor API retained | T2 actually stalls the 4096-event burst; inspect assertion and ordinary exit | Valid behavioral red; run 34631556246; package exited in 0.066s |
| M2 | Nil-outlet guard before internal dispatch | Disabled loop receives input but misses the internal tail effect; do not describe an early prerequisite failure as a pending-send timeout | Valid behavioral red; run 34632417620; package exited in 0.103s |
| M3 | Subscribed publication uses select/default-drop | T4 attempts publication 101 while 100 slots are full and fails exact delivery count | Valid behavioral red; 104 expected/103 received; run 34633303904; package exited in 0.087s |
| G | Restore repo.go to the baseline blob | Whole-repository vet and uncached complete internal suite pass | Passed; run 34634421836; 21 packages passed, 14 had no test files; Telegram package 0.089s |

## Run receipts

### M1 — unconditional allocation/publication

- Commit: `c4df60fa4c00475ff57c678c478e7833295979ab`. Reviewed scope: two files, 2 insertions/7 deletions; repo.go alone restores allocation and removes the nil-outlet guard. The required typed constructor, invalid-mode rejection, listener, internal effects, filtering, and migrated callers remain intact.
- Run: https://github.com/killbus/budva/actions/runs/34631556246 ; job: https://github.com/killbus/budva/actions/runs/34631556246/job/103369339255 . `workflow_dispatch`, exact head SHA matched, completed/failure.
- `go vet ./...`: success, 2026-09-11T18:08:28Z–18:08:30Z. `go test -count=1 -timeout=2m ./internal/...`: executed and failed, 18:08:30Z–18:08:49Z. TDLib restore/unpack succeeded; cold build skipped.
- Actual T2 diagnostic: `internal/infra/telegram/updates_test.go:180: no progress within virtual deadline: 4096 updates and tail input` in `TestListenUpdates_NoBusinessConsumerBurst`. This is the required stalled-progress red, independent of the additional T1 allocation/accessor failures.
- T3 success/failure and readiness cases likewise failed their initial burst prerequisite at `internal/infra/telegram/updates_test.go:211` and `internal/infra/telegram/updates_test.go:238`; no claim that those runs reached their later waiter assertions.
- Teardown receipt: `FAIL github.com/pure-golang/budva-claude/internal/infra/telegram 0.066s`, subsequent internal packages completed, final process exit 1. No package timeout, runtime panic, or deadlock diagnostic appeared. Combined with the unchanged synchronous fixture cleanup, this verifies bounded teardown for this mutation, not SDK close/race safety.
- Static receipts: delegated gofmt list and diff --check passed; main reviewed full two-file diff and verified the exact staged path set plus staged diff --check before committing.

### M2 — guard before internal processing

- Commit: `679d8c045d1cca890ff4defb720c7a84223de300`. The incremental commit contains repo.go only (+6/-1). Baseline comparison confirms M1 fully restored: only the guard relocation and its mutation comment differ (+5/-5); the temporary test invocation is unchanged.
- Run: https://github.com/killbus/budva/actions/runs/34632417620 ; job: https://github.com/killbus/budva/actions/runs/34632417620/job/103372199833 . `workflow_dispatch`, exact head SHA matched, completed/failure.
- `go vet ./...`: success, 2026-09-11T18:17:52Z–18:17:54Z. `go test -count=1 -timeout=2m ./internal/...`: executed and failed, 18:17:54Z–18:18:15Z. TDLib restore/unpack succeeded; cold build skipped.
- Actual T2 diagnostic: `internal/infra/telegram/updates_test.go:180: no progress within virtual deadline: tail NewChat readiness effect`. The prior input-count assertion succeeded, distinguishing missing internal processing from M1's blocked input.
- Both `TestListenUpdates_SendResultsAfterBurst` subcases and `TestListenUpdates_ReadinessAfterBurstBeforeTicker` failed the same initial-tail prerequisite at `internal/infra/telegram/updates_test.go:211` and `internal/infra/telegram/updates_test.go:238`. They did not reach pending-send/cold-readiness operations.
- Teardown receipt: `FAIL github.com/pure-golang/budva-claude/internal/infra/telegram 0.103s`, subsequent packages finished, final process exit 1. No timeout, runtime panic, or deadlock diagnostic appeared.
- Static receipts: delegated gofmt list/diff --check passed; main compared against the baseline and verified only repo.go dirty, then enforced that exact staged path and checked staged whitespace before committing.

### M3 — subscribed drop-on-full

- Commit: `260cac52b682d7e34e50d8be5b865d5fe24f9589`. Incremental commit contains repo.go only (+9/-6); baseline comparison proves M2 fully restored and only the final send differs (+4/-1: select/send/default). Tests, conditional allocation, internal dispatch, post-internal guard, and filtering remain unchanged.
- Run: https://github.com/killbus/budva/actions/runs/34633303904 ; job: https://github.com/killbus/budva/actions/runs/34633303904/job/103375119334 . `workflow_dispatch`, exact head SHA matched, completed/failure.
- `go vet ./...`: success, 2026-09-11T18:27:19Z–18:27:20Z. `go test -count=1 -timeout=2m ./internal/...`: executed and failed, 18:27:20Z–18:27:35Z. TDLib restore/unpack succeeded; cold build skipped.
- `TestListenUpdates_SubscribedBackpressurePreservesEvents` failed at `internal/infra/telegram/updates_test.go:360`: `should have 104 item(s), but has 103`; assertion message `exact count after processing the tail`. The pointer list printed by testify is omitted here. Event 101 was necessarily offered while the 100-slot outlet was full.
- The fatal count assertion precedes later content/identity and dual-delivery assertions; those were not reached in this red. Exactly 103 is this run's observation, not a universal scheduler-independent prediction (the allowed mutation outcome is at most 103).
- Teardown receipt: `FAIL github.com/pure-golang/budva-claude/internal/infra/telegram 0.087s`, final process exit 1, no timeout, runtime panic, or deadlock diagnostic.
- Static receipts: delegated gofmt list/diff --check and both comparisons passed; main reviewed baseline diff/status, required the exact one-file staged set, and passed staged diff --check before commit/push/dispatch.

### G — restored production source

- Commit: `eb90ad511083c7e507f351a3829988074f05e0fb`, `test: restore Telegram update implementation for final validation`. This commit and its successful run already existed when recovery resumed; no repeated mutation, commit, push, or dispatch was needed.
- Run: https://github.com/killbus/budva/actions/runs/34634421836 ; job: https://github.com/killbus/budva/actions/runs/34634421836/job/103378747313 . `workflow_dispatch`, exact head SHA matched in run metadata and raw checkout logs, completed/success.
- `go vet ./...`: executed successfully, 2026-09-11T18:39:14Z–18:39:16Z. `go test -count=1 -timeout=2m ./internal/...`: executed successfully, 18:39:16Z–18:39:37Z. Neither validation step was skipped. Raw log confirms Go 1.25.9 linux/amd64 and restored cache `tdlib-22d49d5-amd64`; TDLib cold build/save were skipped.
- Complete test-command output contains 21 `ok` packages and 14 `[no test files]` packages, with no `(cached)` results. Telegram receipt: `ok github.com/pure-golang/budva-claude/internal/infra/telegram 0.089s`. The job reached ordinary post-job cleanup; no timeout, runtime panic, or deadlock diagnostic. Package summaries are not per-test counts or account-test execution.
- Source identity: both baseline and final `internal/infra/telegram/repo.go` Git blobs are `5da9b9a3caf3fe6d1d0319aab9a2bb351a3e4c5e` (receipt b52d70). The full baseline-to-final committed diff contains only the temporary CI test flags (receipt 5495de); production source, test code, constructor choices, dependency pins, and other workflow settings match the patch.
- Validation worktree is clean on its tracking branch (receipt 23ffdf). Main HEAD remains `a314b0002e46b5a3ffeeef22c503a486a5293b13` (receipt 4910e6). Main repo.go and ci-test.yml SHA256 values still match the pre-validation values above (receipt 137c83); inherited task notes and unrelated WIP remain unstaged.
- Recovery receipts: history e26715; run list d7ccad; job/step metadata 52bf4d; complete raw log d1b61e. The CLI labels raw log steps `UNKNOWN STEP`; actual `Run go vet` / `Run go test` markers and timestamps were matched with named job-step metadata. An initial name-based log filter returned no lines; the already-fetched complete log was inspected without repeating the remote request.

All three required behavioral reds and the restored whole-repository-vet/internal-suite green are recorded. This closes the native behavioral red/green gate only.

## Independent protocol check

The read-only Trellis check agent read the complete manifests/references and reviewed baseline source/tests against the original artifacts. It found no blocker to M1→M2→M3→G and required no additional mutation for the reviewed gate. This protocol approval is not a substitute for the executed receipts.

The reviewer confirmed two reporting limits: M2 detects the initial missing-tail effect before T3's later waiters; M3 should fail the fatal exact-count assertion (104 expected, at most 103 observed), so that red will not itself reach later content/identity/dual-delivery assertions. Final green exercises those paths but does not prove each one's independent mutation sensitivity. Failure cleanup has synchronous joins with no private timeout; the two-minute package timeout is a backstop, never an acceptable result.

## Evidence boundaries

The inactive Listener fixture executes production Repo logic but not live SDK fan-out, command startup, or actual facade client-backed requests. T3 green exercises real pending-send registration and a materialization-controlled cold readiness waiter after the burst; an M2 early-tail failure must not be mislabeled as reaching those later assertions. No claim is made about engine overload, SDK close races, full session/shutdown safety, or account-based production closure. Those gates and PR/merge authority remain separate.

## Independent executed-evidence reconciliation - 2026-10-11

The read-only Trellis reviewer `/root/evidence_check` independently inspected the context manifests, acceptance/design artifacts, production source/tests, local mutation history, and freshly retrieved metadata/full raw logs for all four existing runs. Verdict: PASS for bounded AC5/design 7.2 reconciliation, with no blocking finding. This is the executed-evidence review, distinct from the earlier protocol approval.

- M1: c4df60fa / run 34631556246 reached the T2 4096-event blocked-input verdict at updates_test.go:180; ordinary package failure in 0.066s. Constructor failures are not the accepted liveness proof.
- M2: 679d8c04 / run 34632417620 completed input handoff, then failed on the missing NewChat tail effect; T3 also stopped at that prerequisite. Ordinary package failure in 0.103s, not an executed later waiter assertion.
- M3: 260cac52 / run 34633303904 reached the exact-count failure at updates_test.go:360 (104 expected, 103 received), after the full-queue 101st handoff and quiescence. Ordinary package failure in 0.087s; later identity/content/dual-delivery assertions were not reached.
- G: eb90ad51 / run 34634421836 executed whole-repository vet and the uncached complete internal suite successfully. Both reviewers counted 21 passing packages, 14 without test files, and zero cached results; Telegram passed in 0.089s. All four logs match their checkout SHAs and actual command markers; none contains a package timeout, runtime panic, or deadlock diagnostic.
- Local Git comparisons confirm each control's intended isolated source mutation and restored source identity. The complete a314b00-to-eb90ad5 diff contains only temporary test flags; repo.go blob equality, unchanged main working-file fingerprints, and clean validation-worktree status were rechecked.
- Source inspection confirms all four assembly modes and the production launch at repo.go:370. Direct loop invocation in the fixture cannot detect removal of that launch. No executed constructor-omission compiler control is claimed; required-signature migration coverage is distinct from behavioral reds.
- The fixture's joined synthetic work and ordinary package exits support the tested cleanup paths only. Native fan-out, actual requests/account flow, SDK close races, engine overload, and full session/shutdown safety remain outside this proof. The existing isolated-account facade probe is still required before production closure.

Fresh read-only receipts are cached locally under `.trellis/.runtime/evidence/facade-20261011/`, with `<run-id>.json` and `<run-id>.log` for the four runs above. This directory is Git-ignored, not part of the proposed evidence commit; immutable run/job URLs and exact revisions remain recorded in this report. No new CI run, mutation, commit, push, PR, merge, account use, or deployment occurred during reconciliation.
