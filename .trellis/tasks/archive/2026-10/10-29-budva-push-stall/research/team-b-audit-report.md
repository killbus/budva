# Team B Independent Follow-up Audit Report

## 1. Audit identity and scope

- **Audit party:** Team B, acting as an independent reviewer
- **Audit date:** 2026-10-30
- **Audited artifact:** `.trellis/tasks/10-29-budva-push-stall/research/root-cause.md`
- **Runtime evidence:** `kill.log`
- **Code baseline:** repository `main` at `d8830f0`
- **Audit mode:** independent technical review; no implementation change, test execution, Git commit, or branch operation was performed

Team B independently rechecked the updated final incident record against the
stall-time goroutine dump, repository source, and task metadata. This report
supersedes the earlier Team B audit of the removed
`research/h2-new-text-path.md`.

## 2. Executive verdict

**Team B confirms the updated root-cause conclusion.**

The diagnosis is supported by direct agreement between runtime and source
evidence:

1. `internal/infra/queue/repo.go:100-112` holds `Repo.mu` while invoking the
   dequeued task.
2. The media-album task enqueued at
   `internal/app/handler/service.go:573` calls `Repo.Add` again at
   `internal/app/handler/service.go:644`.
3. The same goroutine attempts to reacquire the same non-reentrant
   `sync.RWMutex` and self-deadlocks.
4. The sole update consumer subsequently blocks on that lock; the cap-100
   update channel fills and backpressure propagates into the TDLib receiver
   chain.
5. New updates no longer reach the push submitter, while the process remains
   alive and the push worker remains idle.

The proposed minimal fix, releasing the queue lock before `executeTask`, is
correctly scoped. It removes the confirmed deadlock precondition while
preserving serial execution by the production worker.

**Audit disposition: PASS.**

No remaining documentation issue invalidates or qualifies the root cause,
evidence chain, or proposed fix direction. The two editorial findings from
the previous follow-up audit have now been applied to the audited record.

## 3. Previous Team B findings: closure assessment

### TB-P1-01: goroutine 44 message identity

**Closed.** The updated record states that goroutine 44 is a NEW-message
update consistent with the manual NEW-text test and explicitly notes that the
dump cannot identify the specific message.

### TB-P1-02: goroutine 40 authentication-state evidence

**Closed.** The updated record no longer treats the auth goroutine stack as
proof of a `Ready` state. It correctly uses goroutines 29, 47, and 48 as the
stronger evidence that updates were propagating and blocked by downstream
backpressure.

### TB-P2-01: obsolete hypotheses mixed into the final record

**Closed.** The former diagnostic ladder was removed. Its replacement,
`research/root-cause.md`, is a forward, self-contained record organized
around task, symptom, method, root cause, evidence, fix, validation, and
residual hardening.

### TB-P2-02: ambiguous abbreviated source paths

**Closed.** The audited root-cause record now uses complete
repository-relative paths for the previously abbreviated queue and handler
citations.

### Previously omitted delete-retry reentry site

**Closed.** The updated record includes the outer task at
`internal/app/handler/service.go:325` and the nested retry `Add` at
`internal/app/handler/service.go:863`.

## 4. Independently confirmed evidence

### 4.1 Production queue executes while holding the mutex

`internal/infra/queue/repo.go:100-112` acquires the write lock, defers its
release, removes the front item, and calls `executeTask` before the deferred
unlock can run.

By contrast, `ProcessAll` and `ProcessBatch` in
`internal/infra/queue/repo.go:64-98` release the mutex before executing the
task. The proposed fix therefore matches an established lock boundary in the
same component.

### 4.2 The album closure re-enters the queue

`internal/app/handler/service.go:573` enqueues the outer album closure. Once
that closure has collected and popped the album, it can enqueue
per-destination forwarding at `internal/app/handler/service.go:638` and
unconditionally calls `Repo.Add` for statistics at
`internal/app/handler/service.go:644`.

The statistics `Add` is reached even when `rule.To` is empty. This explains
why the deployed webhook-only configuration is exposed.

### 4.3 Stall-time dump shows the exact self-deadlock

Goroutine 39 in `kill.log` contains the decisive chain:

```text
queue.(*Repo).processQueue
  -> queue.(*Repo).executeTask
    -> handler.(*Service).processMediaAlbum.func1
      -> queue.(*Repo).Add
        -> sync.(*RWMutex).Lock
```

This directly shows that the worker is executing the album closure and is
blocked trying to acquire the queue lock again.

### 4.4 The downstream cascade matches the diagnosis

- Goroutine 44 is the sole handler update consumer and is blocked in another
  `Repo.Add`, reached from a NEW-message update.
- Goroutine 29 is blocked sending into the full business update channel at
  `internal/infra/telegram/repo.go:431`.
- Goroutines 48 and 47 are blocked progressively farther upstream in the
  TDLib receiver chain.
- Goroutine 41, the push worker, is idle on channel receive.

The blocked-duration ordering is consistent with the album worker wedging
first, followed by the handler, business update publisher, and upstream
receiver stages.

### 4.5 Secondary instances are valid

The same bug class exists in:

- edit retry: outer task at `internal/app/handler/service.go:306`, nested
  `Add` at `internal/app/handler/service.go:723`;
- delete retry: outer task at `internal/app/handler/service.go:325`, nested
  `Add` at `internal/app/handler/service.go:863`.

These are latent instances of the same lock-reentry precondition. They are
not claimed as the trigger shown in the dump.

## 5. Follow-up findings: closure assessment

### TB-FU-P2-01: repository-relative citations

**Closed.** `research/root-cause.md` now expands the previously abbreviated
queue and handler references, including the production lock boundary, album
reentry sites, edit/delete retry sites, and evidence-table frames.

### TB-FU-P2-02: trigger wording

**Closed.** The trigger is now explicitly limited to a media album that
reaches `processMediaAlbum` through a matching configured rule. The record
also identifies the relevant per-rule call sites and explains why the
unconditional statistics `Add` leaves webhook-only rules with an empty
`rule.To` exposed.

## 6. Fix and validation assessment

Team B agrees with the minimal fix:

```text
lock
read and remove the front task
unlock
execute the task
```

This change:

- removes the confirmed self-deadlock precondition;
- matches `ProcessAll` and `ProcessBatch`;
- keeps production execution serial because the production loop has one
  worker invoking `executeTask`;
- safely permits a running task to append follow-up work;
- covers the confirmed album path and both retry paths through the shared
  queue behavior.

The validation list in `research/root-cause.md` is appropriate. Diagnosis
acceptance is complete, but implementation acceptance remains pending until
the fix is implemented and the fixed build demonstrates an album followed by
a plain-text message reaching the configured endpoint without restarting the
engine.

## 7. Final Team B determination

- **Root cause:** confirmed.
- **Runtime evidence:** sufficient and directly aligned with the source.
- **Cascade explanation:** confirmed.
- **Minimal fix direction:** correct.
- **Previous Team B P1 findings:** resolved.
- **Previous Team B structural finding:** resolved.
- **Previous Team B follow-up findings:** both resolved.
- **Remaining documentation findings:** none.
- **Implementation status:** not implemented at the time of this audit.
- **Runtime acceptance status:** pending implementation and deployment.
- **Overall verdict:** the updated diagnosis is technically correct and is
  accepted by Team B as the final root-cause record.

## 8. Independence statement

This report is explicitly authored by **Team B** as an independent follow-up
audit. Team B did not author the updated root-cause narrative, did not modify
the implementation, and did not perform a Git commit or branch operation.
The verdict is based on an independent comparison of `research/root-cause.md`,
`kill.log`, repository source, and the current task metadata.

## 9. Team B implementation follow-up audit

### 9.1 Scope and supersession

Team B performed a second independent review after the implementation was
landed on `diag/budva-push-stall`. This section supersedes only the earlier
implementation-status statements in sections 6 and 7; the root-cause audit
and its evidence findings remain valid.

Audited commits:

- `b11ddad` — `fix: release queue lock before executing task`
- `b0a5ccf` — `test: lock the queue self-deadlock regression (10-29 push stall)`
- `8743405` — `docs: record the 10-29 push-stall diagnosis, evidence, and fix`

Baseline:

- `main`, `origin/main`, and `v0.3.0` at `d8830f0`

Audited implementation and test artifacts:

- `internal/infra/queue/repo.go`
- `internal/infra/queue/repo_test.go`
- `internal/app/handler/album_wedge_regression_test.go`
- `.trellis/tasks/10-29-budva-push-stall/prd.md`
- `.trellis/tasks/10-29-budva-push-stall/progress.md`
- `.trellis/tasks/10-29-budva-push-stall/task.json`

### 9.2 Findings

#### TB-IMPL-P2-01: handler regression test leaves a permanent worker goroutine

**Open.**

`TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` starts the real queue worker
with `context.Background()`:

```go
ctx := context.Background()
require.NoError(t, q.StartContext(ctx))
```

`Repo.StartContext` launches `run(ctx)` in a goroutine, and `run` exits only
when `ctx.Done()` is closed. Because a background context is never cancelled
and `Repo.Close` is currently a no-op, the test leaves the worker goroutine
and its ticker alive until the test process exits.

This does not invalidate the deadlock assertion or the production fix, but it
is a test-lifecycle defect. It can fail goroutine-leak checks and leaves a
worker associated with completed mocks running after the test returns.

**Required cleanup:** use `context.WithCancel`, register `cancel` with
`t.Cleanup` or `defer`, and preferably allow the worker to observe
cancellation before the test completes if the package adopts strict leak
checking.

#### TB-IMPL-P3-01: task metadata is stale after implementation commits

**Open.**

- `.trellis/tasks/10-29-budva-push-stall/task.json` still has `"commit": null`
  despite the three landed task commits.
- `implement.jsonl` and `check.jsonl` remain empty despite implementation and
  validation work.
- `prd.md` still says acceptance is pending “the fix commit and deployment”,
  although the fix commit now exists. Only CI, deployment, endpoint-side
  confirmation, and runtime acceptance remain pending.

These inconsistencies do not affect runtime correctness, but they weaken the
task handoff and audit trail.

### 9.3 Implementation correctness

The production change in `internal/infra/queue/repo.go` is correct and
minimal:

1. acquire `Repo.mu`;
2. read and remove the front task;
3. release `Repo.mu`;
4. invoke `executeTask(fn)`.

Team B confirms:

- the empty-queue branch explicitly unlocks before returning;
- the non-empty branch unlocks before executing user-supplied task logic;
- a running task can therefore call `Repo.Add` without attempting to
  reacquire a lock already held by the same worker;
- the production loop still has one worker, so task execution remains
  serial;
- the change is limited to the confirmed shared queue defect and therefore
  covers the album, edit-retry, and delete-retry reentry sites without
  duplicating fixes in handler code;
- the implementation commit contains only the queue lock-boundary change.

No correctness regression was found in the production change.

### 9.4 Test assessment

The two queue tests cover the essential regression contract:

- `TestRepo_ProcessQueue_nested_Add_does_not_deadlock` directly proves that a
  dequeued task can re-enter `Add`, return, leave the nested task queued, and
  execute it next.
- `TestRepo_StartContext_nested_and_external_tasks_complete` exercises the
  real ticker-backed production worker, verifies that an external `Add` does
  not remain blocked, and verifies that both nested and external work
  eventually execute.

The handler regression test correctly uses `StartContext` rather than
`ProcessAll`; `ProcessAll` already released the lock before task execution and
could not reproduce the old production defect. Its combined submission-count
and `q.Len() == 0` predicates avoid treating the brief dequeue-before-execute
window as success. The thread-safe submitter also protects concurrent slice
access.

The handler test was not compiled or executed during this Team B review
because this sandbox does not contain the approximately 15 GB TDLib/CGO tree
required by the handler package. It remains a CI validation obligation and
must not be reported as passed.

Team B attempted an independent queue race run with:

```text
GOMAXPROCS=2 go test -race -p=1 ./internal/infra/queue/...
```

The command could not start because the current audit environment has no
`go` executable (`env: 'go': No such file or directory`). Therefore Team B
cannot independently attest to a fresh local test pass. The project record's
reported 16-test race-clean result remains a team-provided result, not an
independently reproduced Team B result.

### 9.5 Runtime and acceptance boundary

The implementation removes the source-level self-deadlock precondition, but
full task acceptance is still pending:

1. CI must provision TDLib and execute the handler regression test.
2. A fixed build must be deployed.
3. A matching source album followed by plain text must reach the configured
   endpoint without restarting the engine.
4. The user must confirm the target-side delivery result.

Team B does not claim deployment or production runtime acceptance.

### 9.6 Final implementation verdict

**Overall implementation verdict: PASS WITH REQUIRED TEST CLEANUP.**

- **Production fix:** PASS; correct, minimal, and aligned with the confirmed
  root cause.
- **Queue regression coverage:** PASS by source review; the team's recorded
  race result was not independently rerun because Go is absent.
- **Handler regression coverage:** structurally appropriate, but not executed
  in this environment.
- **Required correction:** cancel the handler test's worker context to avoid a
  permanent goroutine/ticker leak.
- **Documentation hygiene:** update stale task metadata and acceptance
  wording.
- **CI/deployment/runtime acceptance:** still pending.

This implementation follow-up is explicitly authored by **Team B** as an
independent audit. Team B changed only this audit report and did not modify
the production implementation or regression tests.

## 10. Team B remediation re-audit

### 10.1 Scope

Team B re-audited the remediation commits that followed section 9:

- `16fcc92` — `test: stop the handler regression worker on test cleanup`
- `89f40b8` — `docs: refresh task metadata after implementation (TB-IMPL-P3-01)`

The review covered the exact commit diffs, the current handler regression test,
task metadata, implementation/check manifests, PRD, progress handoff, and the
resulting repository state. No production implementation file changed in these
two commits.

### 10.2 Findings

#### TB-RE-P2-01: cancellation is signalled but worker termination is not observed

**Open; TB-IMPL-P2-01 is improved but not fully closed.**

`16fcc92` correctly replaces `context.Background()` with a cancellable context
and registers `t.Cleanup(cancel)`. This removes the permanently non-cancellable
worker condition.

However, `cancel()` only closes `ctx.Done()`; it does not wait for
`Repo.run(ctx)` to return. `Repo.StartContext` exposes no completion handle and
`Repo.Close` remains a no-op. Consequently, when test cleanup returns, the
worker goroutine may still be scheduled in `select` or between observing the
cancellation and executing its deferred `ticker.Stop()`.

The current change therefore does not prove the stronger claim recorded in
`check.jsonl` that the test "leaks no goroutine/ticker after the test". It also
does not satisfy section 9's preferred strict-leak-checking form, which called
for allowing the worker to observe cancellation before test completion.

**Required closure:** add an observable worker-termination mechanism, then
cancel and wait for termination during cleanup. This can be a queue lifecycle
change (`Close`/done channel) or a test-visible completion mechanism that does
not alter production semantics. Until then, classify the original finding as
partially remediated rather than resolved.

#### TB-RE-P2-02: the authoritative Team B report still says both findings are open

**Open.**

`89f40b8` updates `progress.md`, `task.json`, and `check.jsonl` to say both Team
B findings are resolved, but it commits section 9 of this report unchanged.
The authoritative audit text still states:

- `TB-IMPL-P2-01`: **Open**;
- `TB-IMPL-P3-01`: **Open**;
- overall verdict: **PASS WITH REQUIRED TEST CLEANUP**.

Therefore the task records claim closure that the cited Team B report did not
grant. A team implementation commit cannot retroactively turn an independent
auditor's open finding into a closed finding merely by referencing the report.
This section is the first Team B re-audit of those remediations.

#### TB-RE-P3-01: task commit metadata is described as the branch tip but is not

**Open.**

`task.json` sets `commit` to `16fcc92835c293dd7b28100484045e4b37ee777e`,
while the branch tip after the metadata commit is `89f40b8`. The commit message
and `progress.md` describe the value as the "branch tip", which is factually
incorrect after `89f40b8` was created.

If the field is intended to identify the last code/test remediation commit,
that meaning should be stated explicitly. If it is intended to identify the
current task delivery tip, it must be updated by the workflow that finalizes
the task rather than described as self-referential branch-tip metadata inside
the same commit.

### 10.3 Confirmed improvements

Team B confirms the following parts of the update are correct:

- `16fcc92` gives the handler test a cancellable context and is a material
  improvement over the permanently live background context.
- `prd.md` no longer incorrectly says the fix commit is pending.
- `implement.jsonl` and `check.jsonl` are populated with relevant review
  inputs and gates.
- `task.json` no longer has a null commit field.
- the task remains `in_progress`, correctly preserving the boundary that CI,
  deployment, endpoint confirmation, and production runtime acceptance are
  still pending.
- `git diff --check 8743405..HEAD` reports no whitespace errors.

### 10.4 Validation boundary

No fresh Go test was run in this re-audit because the current environment still
has no `go` executable. The handler package also still requires the external
TDLib/CGO tree. Team B therefore does not independently attest that either the
queue race suite or handler regression test passes after these remediations.

### 10.5 Re-audit verdict

**Overall remediation verdict: CHANGES REQUESTED.**

- The permanent background-context leak was improved, but deterministic worker
  shutdown is not demonstrated.
- The metadata refresh contains useful corrections, but it overstates audit
  closure and mislabels `16fcc92` as the branch tip.
- The production deadlock fix remains accepted and unchanged.
- CI, deployment, and album-plus-text runtime acceptance remain pending.

This re-audit is explicitly authored by **Team B**. Team B changed only this
independent audit report and did not modify production code, tests, or task
metadata.
