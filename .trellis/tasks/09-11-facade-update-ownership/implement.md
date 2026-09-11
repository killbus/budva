# Implementation plan: facade update ownership

Status: source implementation, static review, structure audit, and spec consistency are complete. Native red/green and actual-binary acceptance remain pending; the task is still in_progress. Checkmarks below distinguish source work from executed behavioral evidence. See research/review-takeover.md and research/progress.md.

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
- [ ] Validate producer shutdown plus deterministic success/failure cleanup in Linux; fixture implementation alone does not satisfy this gate.
- [x] Read research/review-validation.md before writing the fixture: install cleanup early, distinguish input/effect/exit barriers, wait for actual pending-send/readiness registration, and preserve verdicts before failure-only rescue drainage.
- [x] Replace the unconditional channel-initialization assertion with explicit mode tests and add T2-T4. These tests are implemented, not yet accepted as green.
- [ ] Record behavioral red paths under the new signature. An old tree that merely fails to compile is not evidence.

Checkpoint: construction is explicit at every call site; native negative controls must still demonstrate that the tests catch the original publication failure and missing internal events.

## Step 2 — remove only the unused business publication edge

- [x] Preserve the existing listener acquisition/lifetime and internal dispatch ordering.
- [x] Guard business publication after internal processing when no outlet exists.
- [x] Preserve capacity 100, relevance filtering, ordered blocking send, and event contents in BusinessUpdates mode.
- [x] Do not add drop/default, drain workers, extra staging buffers, dynamic registration, new close paths, or engine queue changes.
- [x] Log the static mode once; keep backlog semantics scoped to the actual business queue.
- [x] Update package docs and the relevant TDLib spec with the contract and explicit evidence boundaries. Existing spec sync was rechecked during takeover; native behavior is not yet verified.

Checkpoint: source implements removal of the unused publication edge while preserving internal processing and enabled delivery. Behavioral acceptance remains a separate gate.

## Step 3 — local/static and Linux validation

Windows preflight (does not prove native behavior):

~~~text
git diff --check
gofmt -l <changed Go files>
~~~

Review constructor mode choices, imports, strict mocks, and fixture cleanup before requesting CI.

- [x] Re-run gofmt over all nine deliverables in read-only list mode, tracked task-scoped diff checking, and a separate whitespace scan for the untracked regression file. All passed; see research/review-takeover.md.
- [x] Confirm the existing native CI channel without changing or triggering it. ci-test is active; the latest green covers only baseline 84a9df2. See research/native-ci-handoff.md for the validation checkpoint proposal.

Existing Linux CI gates (currently no workflow changes required):

~~~text
go vet ./...
go test ./internal/...
~~~

- [x] Ensure all new regression tests live in the existing internal test scope.
- [ ] Record red and green evidence for unconditional outlet restoration, disabled internal dispatch, and subscribed drop-on-full mutations. T4 must attempt a 101st relevant publication while the 100-slot outlet is full before resuming its reader. Remove every mutation before final validation.
- [ ] Require the T2 old-publication control to fail on actual blocked progress, not just T1 allocation. Exercise T3 after the burst with real waiters and a readiness-state oracle before polling; check T4 absence of extra events after its completion barrier. Verify teardown on each required red path.
- [ ] Re-run the whole internal suite on the final revision, not only selected new tests.
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
- [ ] Repeat final full-scope native validation after all negative controls are restored.

## Step 5 — controlled runtime acceptance and delivery

- [ ] Obtain explicit authorization and an isolated account/session for a real facade idle-burst/request probe. No production-chat mutations by default.
- [ ] Before the probe, record the relevant update load, real client-backed request and expected data, request count, latency/error bounds, cold/waiting precondition, and exact revision.
- [ ] Capture expected successful responses to valid requests against known test data and internal readiness behavior after the burst in the actual facade binary. Neither /live nor fast request-validation errors pass.
- [x] If the native environment is unavailable, state that end-to-end acceptance is pending. No usable local Linux/TDLib runner was established; the production incident is not closed.
- [x] Draft a task-only checkpoint commit plan with explicit inherited-work classification and all unrelated dirty paths excluded; see research/native-ci-handoff.md.
- [ ] Receive one-shot confirmation, then stage only the approved files and create the local checkpoint commit. Do not push or merge as an implicit part of committing; native acceptance is not waived.
- [ ] Green CI precedes any separately authorized squash/merge into main; deployment requires its own approved workflow.
- [x] Record accepted limitations: engine overload, SDK listener/responses close races, and full shutdown/session safety were not repaired.

## Stop / return-to-planning conditions

- A real facade responsibility requires business delivery through the removed outlet.
- A required internal event cannot remain correct with the existing listener or test seam.
- The patch needs new drop/retry/persistence semantics, a dependency fork, or a pump redesign.
- Review finds a new lifecycle race caused by the proposed change rather than an unchanged known residual risk.

Do not solve these by silently expanding the patch. Update the artifacts and request the missing decision/authority.
