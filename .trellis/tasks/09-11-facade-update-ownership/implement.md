# Implementation plan: facade update ownership

Status: source/static/structure/spec review, the authorized checkpoint/push, initial Linux CI/Docker build, and required native behavioral red/red/red/restored-green execution are complete. Independent executed-evidence reconciliation passed on 2026-10-11 with no blocking finding. The user approved the six-file local evidence-record commit on 2026-10-11. Push, PR/checks/review/merge authority, and pre-production-closure actual-facade acceptance remain separate; the task is still in_progress and the production incident is not closed. The 2026-10-11 all-state query found no task-head PR. See research/native-red-green.md, research/findings.md, and research/progress.md.

## Gate 0 — reviewed planning and user approval

- [x] Complete source research and independent design review, including the adversarial reviewer.
- [x] Resolve findings in prd.md/design.md and record their disposition in research/review.md; both independent reports approve with no design blocker.
- [x] Curate implement.jsonl and check.jsonl; task.py validate passed with nine real entries in each manifest.
- [x] User confirms the bounded scope and authorizes implementation (“开始推进”, 2026-09-11).
- [x] Snapshot dirty state; reuse the already-existing fix/facade-update-ownership branch at reviewed main baseline 84a9df2, preserving unrelated changes.
- [x] Run task.py start only after the above gates (in_progress, 2026-09-11).

## Step 1 — contract migration and red-capable fixture

- [x] Read the context manifests and all planning artifacts before code edits.
- [x] Inventory every telegram.New/New/Repo literal in production, support, and package tests; compare the actual call-site modes with the design construction table. Generic constructor tests and omitted-argument compile failures are not assembly proof.
- [x] Introduce the required typed construction mode, allocation postconditions, and fail-fast disabled Updates accessor contract.
- [x] Migrate facade and stand to NoBusinessUpdates; engine and LiveStack to BusinessUpdates; update tests and usage examples intentionally per case.
- [x] Build one reusable controlled-listener fixture that drives production listenUpdates. Use the pinned-source non-nil Listener literal and let the loop own Close.
- [x] Validate producer shutdown plus deterministic success/failure cleanup in Linux. M1/M2/M3 reached intended assertions and ordinary package exits; restored G passed. This validates the tested fixture paths, not SDK close/race safety.
- [x] Read research/review-validation.md before writing the fixture: install cleanup early, distinguish input/effect/exit barriers, wait for actual pending-send/readiness registration, and preserve verdicts before failure-only rescue drainage.
- [x] Replace the unconditional channel-initialization assertion with explicit mode tests and add T2-T4. The existing internal scope passed initially and after restoration; native behavioral discrimination is recorded separately.
- [x] Record behavioral red paths under the new signature: M1 blocked burst progress, M2 missing internal tail effect, M3 missing subscribed event count. No compiler or constructor-only failure substitutes for these reds.

Checkpoint: construction is explicit at every call site; recorded native controls detect the original publication stall, missing internal tail processing, and subscribed drop-on-full. Assertion and cleanup limits are preserved in the evidence report.

## Step 2 — remove only the unused business publication edge

- [x] Preserve the existing listener acquisition/lifetime and internal dispatch ordering.
- [x] Guard business publication after internal processing when no outlet exists.
- [x] Preserve capacity 100, relevance filtering, ordered blocking send, and event contents in BusinessUpdates mode.
- [x] Do not add drop/default, drain workers, extra staging buffers, dynamic registration, new close paths, or engine queue changes.
- [x] Log the static mode once; keep backlog semantics scoped to the actual business queue.
- [x] Update package docs and the relevant TDLib spec with the contract and explicit evidence boundaries. Existing spec sync was rechecked; native adapter-seam validation is now recorded, while SDK/actual-account acceptance remains separate.

Checkpoint: source implements removal of the unused publication edge while preserving internal processing and enabled delivery. Behavioral acceptance remains a separate gate.

## Step 3 — local/static and Linux validation

Windows preflight (does not prove native behavior):

~~~text
git diff --check
gofmt -l <changed Go files>
~~~

Review constructor mode choices, imports, strict mocks, and fixture cleanup before requesting CI.

- [x] Re-run gofmt over all nine deliverables in read-only list mode, tracked task-scoped diff checking, and a separate whitespace scan for the untracked regression file. All passed; see research/review-takeover.md.
- [x] Confirm and subsequently obtain the authorized initial native CI run for the committed patch. ci-test and docker-publish report success at a314b0002e46b5a3ffeeef22c503a486a5293b13; see research/findings.md. No workflow change was needed.

Existing Linux CI gates (currently no workflow changes required):

~~~text
go vet ./...
go test ./internal/...
~~~

- [x] Ensure all new regression tests live in the existing internal test scope.
- [x] Record initial unmutated branch CI green for the existing whole-repository vet and internal-suite workflow. This is not restored-final-green evidence after negative controls.
- [x] Record red and green evidence for unconditional outlet restoration, disabled internal dispatch, and subscribed drop-on-full mutations. M3 fails the exact count (104 expected, 103 received) after the attempted 101st publication while full. All source mutations are removed and final source identity is verified.
- [x] Require actual T2 blocked-progress red and ordinary teardown on every required red. Restored green exercises T3 real waiters/pre-ticker readiness and T4 exact delivery/no extras. M2 stops at the initial tail prerequisite, and M3 at the fatal count assertion; neither red independently proves later assertion sensitivity.
- [x] Re-run the whole internal suite on the restored final revision: ci-test 34634421836 executed `go vet ./...` and `go test -count=1 -timeout=2m ./internal/...` successfully. No tests or production source differ from the original patch.
- [x] Explain which constructor/command paths are compiled or inspected but not executed by CI. No LiveStack account execution is claimed; see research/review-takeover.md.
- [x] No permanent race-detector/workflow expansion is implied. If evidence requires one, propose the minimal additional gate explicitly.

Checkpoint: CI logs identify the exact revision, commands, failure reasons, and final green result. Push/dispatch/PR actions require their normal separate authorization; none occurs in the planning turn.

## Step 4 — independent implementation check and spec sync

- [x] Dispatch the Trellis check agent with Active task plus the curated manifests and all three artifacts. No nested implement/check dispatch from child agents.
- [x] Check all affected layers by source review: Telegram Repo, facade/engine/stand assembly, LiveStack, tests, and docs.
- [x] Include an adversarial question: would disabling all listening, silently disabling engine, or dropping subscribed updates also pass this evidence? The saved review records both test oracles and their limits.
- [x] Run a structure audit if new helpers/files were introduced; do not add a generalized event framework for the fixture. Fresh Dirac audit returned no findings above threshold.
- [x] Review new learnings for the required spec update before the commit plan. Ownership and evidence-boundary guidance already exists and matches the implementation.
- [x] Record no blocking source findings and preserve residual-risk wording; see research/review-takeover.md and the saved adversarial review.
- [x] Repeat final full-scope native validation after all negative controls are restored; exact source equality and successful command/package receipts are recorded.
- [x] Independently reconciled M1/M2/M3/restored-G with AC5/design 7.2 on 2026-10-11. Fresh existing GitHub metadata/raw logs and local commit/blob comparisons confirm intended behavioral failures, ordinary failed-path exits, and restored uncached internal-suite/vet success. No blocking finding; only the adapter-seam evidence-reconciliation gate closes. M2/M3 assertion-reach and actual-account/SDK/lifecycle limitations remain explicit in research/native-red-green.md.

## Step 5 — controlled runtime acceptance and delivery

- [ ] Obtain explicit authorization and an isolated account/session for a real facade idle-burst/request probe. No production-chat mutations by default.
- [ ] Before the probe, record the relevant update load, real client-backed request and expected data, request count, latency/error bounds, cold/waiting precondition, and exact revision.
- [ ] Capture expected successful responses to valid requests against known test data and internal readiness behavior after the burst in the actual facade binary. Neither /live nor fast request-validation errors pass.
- [x] State that actual-facade end-to-end acceptance is pending: native CI is available for adapter-seam tests, but no authorized isolated-account probe was executed. The production incident is not closed.
- [x] Draft a task-only checkpoint commit plan with explicit inherited-work classification and all unrelated dirty paths excluded; see research/native-ci-handoff.md.
- [x] Receive one-shot confirmation and create the approved 31-file checkpoint a314b0002e46b5a3ffeeef22c503a486a5293b13. The later push was separately authorized; unrelated dirty work was excluded and acceptance was not waived.
- [x] Establish initial branch CI and Docker build success before considering merge.
- [x] Complete required behavioral red/green execution evidence in the isolated validation branch, without modifying the production patch or adding permanent CI flags.
- [x] Receive one-shot approval for the six-file local evidence-record commit on 2026-10-11; exact scope is recorded in research/progress.md. No push or PR is included.
- [ ] Obtain authorization for evidence-record push and PR creation, verify the actual PR target/checks/reviews, and obtain separate squash/merge authorization. The actual-facade probe remains required before production closure; deployment requires its own approved workflow.
- [x] Record accepted limitations: engine overload, SDK listener/responses close races, and full shutdown/session safety were not repaired.

## Stop / return-to-planning conditions

- A real facade responsibility requires business delivery through the removed outlet.
- A required internal event cannot remain correct with the existing listener or test seam.
- The patch needs new drop/retry/persistence semantics, a dependency fork, or a pump redesign.
- Review finds a new lifecycle race caused by the proposed change rather than an unchanged known residual risk.

Do not solve these by silently expanding the patch. Update the artifacts and request the missing decision/authority.
