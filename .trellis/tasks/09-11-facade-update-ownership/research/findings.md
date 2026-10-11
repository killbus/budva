# Evidence ledger

## Current disposition - 2026-10-11

- Independent executed-evidence reconciliation passed with no blocking finding. Fresh metadata/full raw logs for M1/M2/M3/G match exact local commits, required behavioral failures, ordinary teardown, and restored uncached vet/internal-suite success. See research/native-red-green.md for the independent verdict and assertion-reach limits. Earlier dated pending-validation entries below are historical.
- HEAD remains a314b00 on fix/facade-update-ownership. Restored G remains eb90ad5; baseline-to-G diff is only temporary test flags. Production source/tests and main source/workflow fingerprints are unchanged; validation worktree is clean.
- A fresh all-state task-head PR query returned [] on 2026-10-11. The initial sandbox request returned HTTP 401; the same read-only query succeeded through normal approved escalation using local authentication. No alternate API route, credentials change, PR creation, or remote write occurred. An empty branch query is not a direct-merge audit.
- Spec-sync judgment: existing backend Telegram/TDLib and quality/CI guidelines already capture the construction contract, required controls, and account/lifecycle evidence boundaries. No new spec rule or runtime change is needed for this reconciliation.
- Step 4 is complete; task remains in_progress. The user approved the six-file local evidence-record commit on 2026-10-11. Push, PR/checks/review/merge authority, and an authorized isolated-account actual-facade probe remain separate gates. Production incident closure is not claimed.

## Authority and scope

- User accepted the zero-business-subscriber facade invariant and requested design plus review through Trellis.
- `.trellis/workflow.md` separates task creation/design approval from implementation approval. The user subsequently authorized the reviewed implementation with “开始推进” on 2026-09-11.
- Existing unrelated dirty files are user-owned and will be preserved. The bootstrap-guidelines task is not this task.

## Prior verified facts to re-check at the design seam

- `telegram.New` creates an updates channel of capacity 100 unconditionally. `Updates()` returns that channel, not a registered subscription.
- Facade has no production consumer of that channel. Engine constructs a separate Repo and starts `handler.Run`; it cannot drain facade memory.
- Facade exposes request APIs; current gRPC/GraphQL schemas have no business-update subscription endpoint.
- Repo internal send-result dispatch and warmup notification precede the sole business publication in `listenUpdates`.
- The user-provided production dump identifies blocking publication as the first edge of a cascade into the SDK receiver and global receive pump. Dump timings are incident evidence from the handoff, not newly reproduced here.
- The previous discussion identified independent engine-queue and SDK close-race risks; they are not automatically closed by a facade-only fix.
- A no-op metric comment is not proof that production metrics are disabled; provider initialization and deployment configuration matter.

## Current-source checks

- Repo publication is still a bare `r.updates <- typ`; internal send-result dispatch and `UpdateNewChat` readiness/convergence handling run before it (`repo.go:383-401`).
- Construction callers include not only facade/engine but `cmd/stand/main.go:107` and `internal/test/support/live_stack.go:98`; any default change must account for all four.
- Package tests and `doc.go` also use `New`. The implementation needs an exhaustive constructor/Repo-literal inventory, not a two-binary edit.
- The historical handoff corrects beta4 to beta1. Its further claim that metrics are definitely no-op rests on a comment and is not accepted as current deployment evidence.
- Read backend TDLib, quality, and Docker/CI specs completely. Native go-tdlib verification needs Linux/CI; tests must have falsifiable red paths and virtual-time synchronization.
- Read cross-layer and code-reuse guides. Constructor responsibilities and update routing belong at their existing boundaries; avoid a new event framework.
- Independent research started after each agent supplied a successful active-goal receipt: internal obligations (Parnas) and concurrency boundaries (Hoare).

## Assembly and verification checks

- First-principles cross-check: README.md describes Budva as a message-forwarding product, but its Run/Architecture sections explicitly assign forwarding to engine, HTTP/gRPC service to facade, and fixture management to stand. The Docker section describes independent facade/engine TDLib clients and session volumes. Product-wide forwarding does not imply a business-update consumer in every process; another process cannot drain this Repo channel. Package docs for app/facade and app/handler agree with that split.
- cmd/stand starts a Repo plus auth/terminal consumers, but no business-update consumer. Its request-only shape needs the same disabled outlet as facade.
- LiveStack owns a real business consumer (processUpdates, including temporary/permanent ID mappings and edit/delete handling), so it must retain an enabled outlet.
- Existing TestNew_InitializesChannels asserts an unconditional updates channel. Replace it with explicit construction-mode tests rather than silently weakening the assertion.
- There is no existing listenUpdates test. The new regression must drive the real loop, not just a helper disconnected from production. The concurrency research confirms a concrete SDK Listener literal with a non-nil channel is sufficient for the controlled adapter seam; the main reviewer checked the pinned listener source. It is not registered with the real SDK receiver and therefore cannot prove native fan-out safety.
- Current CI runs go vet ./... and go test ./internal/...; it does not run cmd tests, LiveStack account tests, a race detector, or live Telegram probes. Do not describe those as already covered.

## Reviewed choices (implementation now authorized)

- Use an explicit construction-time role/mode rather than a dynamic subscriber registry or background drain. A required typed mode forces all current constructors to make a choice; source review and the actual-binary probe must still establish that each choice is correct.
- Keep the production receive loop and all internal dispatch before the optional business publication guard. Mode is immutable for Repo lifetime.
- Preserve the subscribed path capacity, type filter, order, and blocking semantics in this bounded repair. Engine overload is a separate risk, not solved by this task.
- Distinguish a disabled outlet from a dropped subscribed event; do not add a false drop counter or claim absence of a backlog proves health.

## Independent review outcome

- Both validation and anti-audit reports approve the bounded design with no blocking design findings. V2 incorporates their two low-severity corrections and acceptance clarifications; see research/review.md.
- Corrected the draft attribution: stand does not call SendMessageAndWait; LiveStack is the verified caller. Keeping private send-result dispatch is a shared-Repo compatibility decision.
- T4 must attempt the 101st relevant publication while capacity 100 remains full. T2/T3 require observed internal effects, actual waiter registration, and pre-ticker readiness after the burst; failure cleanup must not mask the verdict.
- All four scoped research/review goals have completed and their agents are closed. No runtime implementation, native test, CI, or live-account probe occurred.

## Execution resumption

- Sandbox startup still fails, but explicitly approved outside-sandbox commands now work. The inspected installed apply_patch wrapper invokes `codex.exe --codex-run-as-apply-patch`; use that same editor through normal approval without changing ACLs or project tooling.
- Current branch is `fix/facade-update-ownership` at `84a9df228944a72876e08cf4555c69c423440078`, not main as in the historical handoff. Both 9-entry manifests revalidated successfully; task.py set-branch and start succeeded.
- The CLI patch entrypoint can partially update earlier files before a later hunk fails. Verify every target after a patch error.

## Implemented contract (supersedes the baseline source observations above)

- New requires an immutable typed UpdateMode. Invalid values panic; disabled construction allocates no business outlet, and disabled Updates access panics as a programming error. Enabled access returns the same capacity-100 outlet; it is not registration or broadcast.
- All four assembly choices were verified by source inspection and constructor inventory: facade/stand disabled, engine/LiveStack enabled. No external subscriber, drain goroutine, dynamic registry, drop-on-full policy, or SDK lifecycle change was introduced.
- The production listener still dispatches private send results and NewChat readiness/convergence before skipping absent business publication. Enabled filtering, contents, order, and blocking semantics are retained.
- Tests drive the actual Repo loop with an inactive SDK Listener fixture, not the native receiver. They cover a 4096-event no-consumer burst, real send waiters and pre-ticker readiness after the burst, attempted 101st publication while full, exact subscribed delivery, and normal receive-wait cancellation. Behavioral/mutation execution is pending.
- A listener-bypass negative control must return at function entry before GetListener. Do not close synthetic input underneath producers or add an unjoinable nil-channel-send mutation. Cleanup must preserve the failed verdict before rescue-draining the actual private outlet.
- Formatter success is established by the runtime worker's exit-0 receipt. Native compilation, liveness, mutation discrimination, SDK fan-out, and actual-binary behavior are separate unexecuted gates.

## Takeover verification (2026-09-11)

- Revalidated the current task and all constructor choices; the nine-file source implementation remains inherited, with no runtime edit during takeover. The source snapshot and review disposition are recorded in research/review-takeover.md.
- Re-ran nine-file gofmt list mode, task-scoped tracked diff checking, and a separate trailing-whitespace scan for the untracked update-loop tests. All passed as static checks only.
- A fresh independent structure audit returned no finding above threshold. Rechecked the existing TDLib ownership spec and backend index; spec sync is present and consistent, contrary to older pending notes.
- No usable local native runner was established: Docker command absent, wsl --list --quiet returned installation/help text with exit 1, and wsl --status returned no output with exit 1. No installation or remote CI dispatch was attempted.
- Native test execution, actual red-control failures, restored final green, and isolated-account facade acceptance remain open. Formatting, source review, and inactive-listener tests cannot substitute for these gates.

## Existing native CI channel confirmed (2026-09-11 continuation)

- Read the exact ci-test workflow and Dockerfile. Both retain TDLib pin 22d49d5; ci-test uses ubuntu-latest, Go 1.25.9 from go.mod, the existing installed-library cache, go vet ./..., and go test ./internal/.... No workflow change is needed for the first native run of the patch.
- Approved read-only GitHub queries confirm killbus/budva, default main, and active workflow ID 354474075 at .github/workflows/ci-test.yml. The latest run 34462381435 passed for baseline 84a9df228944a72876e08cf4555c69c423440078, not for this uncommitted patch.
- A push to fix/facade-update-ownership alone will not trigger CI; the workflow supports pushes to main, PRs to main, and explicit workflow_dispatch. Commit, push, and dispatch authorization are distinct from the observed GitHub access capability.
- All nine Go hashes match the takeover review. Prepared one task-only checkpoint commit plan with 31 explicit candidate paths and 20 excluded dirty path/directory entries in research/native-ci-handoff.md. This explicitly requests inclusion of inherited task work rather than treating it as new session edits.
- Only remote metadata was read. Native patch tests, required negative controls/restored final green, and actual-facade isolated-account acceptance are still unexecuted. Do not use baseline CI success to close this task.

## Post-build merge-readiness recheck

- Local HEAD is a314b0002e46b5a3ffeeef22c503a486a5293b13, fix: make Telegram business update ownership explicit: 31 files changed, 1947 insertions, 60 deletions. Status shows the task branch tracking origin with no ahead/behind marker and only the 20 previously excluded dirty entries. This supersedes the earlier uncommitted/checkpoint-pending state.
- GitHub run metadata reports both branch workflows completed successfully at that exact SHA on fix/facade-update-ownership:
  - ci-test, workflow_dispatch, run 34597416184, created 2026-09-11T12:08:38Z: https://github.com/killbus/budva/actions/runs/34597416184
  - docker-publish, workflow_dispatch, run 34606650762, created 2026-09-11T13:50:47Z: https://github.com/killbus/budva/actions/runs/34606650762
- Re-read the unchanged workflows. ci-test runs go vet ./... then go test ./internal/.... docker-publish builds and health/startup-smokes linux/amd64 and linux/arm64 before publishing the dispatch-test manifest; dispatch does not publish a release version or latest. This recheck obtained run-level conclusions, not fresh step logs, package counts, or an immutable image digest. The dispatch-test tag is mutable and is not a pinned rollout receipt.
- Initial unmutated native green is now established, unlike the older baseline-only notes. Required negative controls are still unrecorded: restore unconditional allocation/publication (T2 blocked progress), bypass internal processing (T2/T3 lost effects), and enabled drop-on-full (T4 missing event), with bounded failed-path cleanup. Final whole-suite green after restoring all mutants remains required by prd.md AC5 and design.md section 7.2.
- design.md section 7.3 places the isolated-account actual-facade burst/request/readiness probe before production closure. It is not automatically a separate pre-merge requirement; keep it open as a rollout/incident-closure gate. Docker dummy-credential health smoke does not satisfy it.
- PR-list approval was rejected because the reviewer's context exceeded its model window. No retry or alternate PR/API query was made. PR existence, mergeability, current base, required checks, and review state remain unverified; branch workflow success does not establish them.
- Recommendation: do not merge yet because required behavioral red/green evidence is incomplete. After completing it, recheck the PR and obtain explicit merge authorization. No mutation run, new workflow dispatch, account probe, PR creation, merge, deployment, or task completion was performed in this recheck.

## CI step verification after retry

- The independent CI-step read initially failed before process creation with the known Windows sandbox ACL error. Its escalation then returned: `Automatic approval review failed: rate limit exceeded: Your requests to gpt-6-astra for gpt-6-astra in eastus have exceeded rate limit.` This was an approval-service failure, not a GitHub HTTP response; the command had not executed.
- After reporting that failure and receiving another retry request, the same read-only command was approved through the normal escalation path and exited 0 (receipt 5232e0): `gh run view 34597416184 --repo killbus/budva --json headSha,status,conclusion,event,jobs,url`. No alternate execution channel or new workflow run was used.
- The response confirms run 34597416184 is completed/success, event workflow_dispatch, at a314b0002e46b5a3ffeeef22c503a486a5293b13. Job `test` (103256329010) is completed/success: https://github.com/killbus/budva/actions/runs/34597416184/job/103256329010
- Step 9, `go vet`, executed successfully from 2026-09-11T12:09:10Z to 2026-09-11T12:09:12Z. Step 10, `go test`, executed successfully from 2026-09-11T12:09:12Z to 2026-09-11T12:09:18Z. Neither validation step was skipped. Restore/Unpack TDLib succeeded; only the cold-cache Build TDLib and Save TDLib steps were skipped.
- This adds step-level execution evidence to the previously inspected workflow commands and initial native green. It does not supply raw test logs, per-test counts, required mutation red paths, restored final-suite green, or actual-account acceptance. It proves this read recovered, not that future requests cannot be rate-limited.
- The earlier rejected PR query was not retried. PR state remains unverified, the Trellis task remains in_progress, and the existing merge/production-closure gates remain open. No new dispatch, commit, push, PR creation, merge, or deployment occurred.

## PR query recovery

- Following the renewed retry request, the previously failed read-only PR query was submitted through normal approval, without an alternate channel: `gh pr list --repo killbus/budva --head fix/facade-update-ownership --state all --json number,title,state,baseRefName,headRefName,headRefOid,isDraft,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,url`.
- Approved execution exited 0 and returned `[]` (receipt 58db7a). No PR in any state matched that head branch in killbus/budva at query time. This supersedes the earlier unavailable-PR-query state; there is no matching PR whose mergeability, reviews, or required checks can be assessed yet.
- This query does not prove that the commit has never reached main by a direct merge or another branch. No merge-base audit or remote mutation was performed. It is a successful empty result, not another failed request to retry.
- Mandatory behavioral negative controls and restored full-suite green remain pending. Complete that evidence before recommending merge; obtain authorization to create a PR, then inspect its target/checks/reviews and obtain merge authorization. The live-account probe remains a separate pre-production-closure gate.
- Both interrupted read-only queries have now completed successfully. Recovery of those queries is not acceptance or completion of the Trellis implementation task, which remains in_progress. No commit, push, workflow dispatch, PR creation, merge, account operation, or deployment was performed.

## 2026-09-12 — requested-session recovery

- Recovered the user-requested session 01a08fe7-0469-7762-8b43-a183c4e9b9a2 and the existing task artifacts. HEAD is still a314b0002e46b5a3ffeeef22c503a486a5293b13 on fix/facade-update-ownership. Prior CI and Docker successes remain the recorded initial green evidence; no new test run occurred during recovery.
- The previous session's two read-only query failures and record-update interruption had already recovered. Repeated automatic retry messages add no acceptance evidence and are not a reason to repeat completed requests.
- A fresh `gh pr list --repo killbus/budva --head fix/facade-update-ownership --state all --json number,title,state,baseRefName,headRefName,headRefOid,isDraft,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,url` exited 0 with `[]` (receipt 199e6e). No matching PR exists at this query time; this is not a direct-merge audit.
- Current command discovery found wsl, go, and gh, but no docker command. `wsl --list --quiet` exited 1 with installation/help output (receipt abbaf7), not an installed-distribution list. A usable local Linux/TDLib runner remains unestablished; no environment setup was attempted.
- The existing ci-test workflow supports workflow_dispatch at a selected ref, without mutation inputs, and runs `go vet ./...` plus `go test ./internal/...`. Repeating it at the unchanged patch only produces another unmutated green; behavioral controls require separately authorized validation refs/CI execution. No permanent workflow expansion, remote write, account use, merge, or deployment was performed.
- Required remaining pre-merge evidence is still unconditional allocation/publication blocked-progress red, listener/internal-dispatch bypass missing-effect red, subscribed drop-on-full missing-events red, bounded teardown, and restored full-suite/vet green. Actual-facade isolated-account acceptance remains a separate pre-production-closure gate.

## 2026-09-12 — remote validation authorized

- The user approved the proposed temporary validation branch/push/CI work, explicitly limited to verification without PR creation, merge, or publishing. No permission to use a live account or deploy was inferred.
- Read-only preflight found only the main workspace at the patch SHA, no local or remote validation/facade-update-ownership-20260912 ref, and confirmed the chosen .trellis/.runtime worktree path is ignored. The main workspace dirty inventory is unchanged.
- Validation will use four scoped commits/runs on that temporary branch: unconditional allocation/publication, early absent-outlet guard, subscribed drop-on-full, then restored production source. Only repo.go and the temporary CI test command are included; Go/TDLib pins, cache layout, trigger policy, and test scope remain unchanged.
- The temporary CI command will be `go test -count=1 -timeout=2m ./internal/...`; whole-repository `go vet ./...` remains unchanged. Each red must reach the intended behavioral assertion and terminate normally; a compile error, timeout, or constructor-only failure does not establish the required behavioral proof.

## 2026-09-12 — first two native behavioral controls

- M1 ci-test 34631556246 at c4df60fa4c00475ff57c678c478e7833295979ab: whole-repository vet succeeded; the uncached internal suite failed on actual T2 burst input progress, with the Telegram package exiting normally in 0.066s. Constructor failures were additional, not the accepted liveness proof.
- M2 ci-test 34632417620 at 679d8c045d1cca890ff4defb720c7a84223de300: vet succeeded; input handoff completed but the tail NewChat internal effect was absent. T2/T3 failed that prerequisite and the package exited normally in 0.103s. Neither red had a package timeout, runtime panic, or deadlock diagnostic.
- Independent protocol review found no blocker or required extra control for M1→M2→M3→G. Its boundaries are preserved: M2 does not reach later T3 waiters; M3 will fail on count before later identity assertions; fixture joins plus ordinary process exit are not native SDK shutdown/race proof.
- Detailed immutable commits, run/job URLs, diagnostic locations, and teardown receipts are in research/native-red-green.md. M3 and restored final green remain pending.

## 2026-09-12 — restored-green recovery

- The final restore commit already exists: `eb90ad511083c7e507f351a3829988074f05e0fb`, `test: restore Telegram update implementation for final validation`. Read-only history succeeded (receipt e26715); no new commit or dispatch was needed.
- The baseline-to-final committed diff contains only `.github/workflows/ci-test.yml`, replacing `go test ./internal/...` with `go test -count=1 -timeout=2m ./internal/...` (receipt 5495de). No production-source or test-code difference remains in that comparison.
- Existing run https://github.com/killbus/budva/actions/runs/34634421836 is completed/success at that exact SHA, event workflow_dispatch (receipts d7ccad and 52bf4d). Job 103378747313 reports successful vet at 2026-09-11T18:39:14Z–18:39:16Z and successful test at 18:39:16Z–18:39:37Z; neither was skipped. Complete raw log d1b61e confirms checkout, actual commands, Go 1.25.9 linux/amd64, TDLib cache pin, 21 passing packages plus 14 with no test files, and no cached test results. Telegram passed in 0.089s and the job exited normally.
- Baseline and final repo.go Git blobs both equal `5da9b9a3caf3fe6d1d0319aab9a2bb351a3e4c5e` (receipt b52d70). Main HEAD is unchanged at `a314b0002e46b5a3ffeeef22c503a486a5293b13` (4910e6), the validation worktree is clean (23ffdf), and both main working-file SHA256 values match their pre-validation fingerprints (137c83).
- Together with the recorded M1/M2/M3 behavioral assertion failures and ordinary failed-path exits, restored G completes the required native red/green execution. Detailed receipts and assertion limits are in research/native-red-green.md. Final independent evidence reconciliation is in progress; source review, original spec sync, and the earlier structure audit were not reopened.
- The raw-log step label was `UNKNOWN STEP`, not evidence that validation was skipped. After the first step-name filter returned no matches, the same stored complete log was inspected by actual command markers and timestamps; no new remote request was necessary.
- Main status still preserves the same task-record edits and unrelated WIP (receipt 8f0694). Session catchup exited 0 without a report (receipt 1c3920). No source edit, staging, commit, push, new workflow run, PR, merge, publishing, deployment, or live-account use occurred during this recovery.
- Latest recorded all-state PR query found no matching task-head PR; this recovery has not created one or audited a current merge candidate. Pending evidence-record commit/push, PR creation/checks/review, merge, and isolated-account rollout acceptance each retain their authority boundaries. Task status remains in_progress.

## Resources

- `README.md` (Run, Docker, and Architecture sections)
- `.trellis/workspace/killbus/research-pipeline-freeze.md`
- `.trellis/workspace/killbus/pipeline-freeze-brief-team-b.md`
- `.trellis/spec/backend/index.md`
- `internal/infra/telegram/repo.go`
- `cmd/facade/main.go`, `cmd/engine/main.go`
- `internal/app/facade/service.go`, `internal/app/handler/service.go`
