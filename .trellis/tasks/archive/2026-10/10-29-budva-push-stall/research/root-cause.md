# Root cause — budva push stall (10-29)

Final record for task `10-29-budva-push-stall`. Baseline: `main` @ `d8830f0`.
Per owner instruction (2026-10-30) this record is self-contained and forward:
task, symptom, method, root cause, evidence, fix. All paths are repo-relative.

## Task

Diagnose why the budva engine stopped pushing messages from its configured
source chats to the configured webhook target, and deliver the
minimal fix. **Producer-side only**: the target is a black box
(accepted / rejected / unreachable); no target-side change is made from this
task.

## Symptom (as observed)

After some uptime the engine **silently stops delivering**:

- New messages from the active source chats — including a manual NEW-text
  test — no longer reach the configured endpoint: its gateway receives
  **zero** inbound requests while stalled.
- The engine process stays alive (no crash; `restart: always` does not
  auto-recover) and the log stays clean — no `Push delivery failed`, no
  `Push submission rejected`.
- A manual `docker compose restart engine` **immediately restores delivery**;
  the stop recurs after uptime.

A restart changes neither the ruleset, the image, nor the endpoint
configuration, yet it cures — so the fault is **runtime state inside the
budva process**, not config/build/target.

## Method

The engine has no pprof endpoint; the decisive probe is a **Go SIGQUIT
goroutine dump taken while stalled**:

```
docker exec <engine> kill -QUIT 1     # or: docker kill --signal=QUIT <engine>
```

Go prints every goroutine's stack and exits; `restart: always` brings the
process back (delivery resumes — re-confirming the restart-cures behavior).
Reading the dump: find goroutines parked on `sync.(*Mutex).Lock` /
`RWMutex.Lock` (deadlock) or on a full-channel `chan send` (backpressure),
and identify which component holds the contested lock. The raw dump is saved
at **`kill.log`** (repo root).

## Root cause

**A reentrant `sync.RWMutex` self-deadlock in the single-serial task queue**
(`internal/infra/queue/repo.go`).

1. `processQueue` (`internal/infra/queue/repo.go:100-112`) holds the queue
   write lock for the **entire** execution of the dequeued task
   (`Lock` → dequeue → `executeTask(fn)`; unlock only after `fn` returns).
   `Add` takes the same write lock (`internal/infra/queue/repo.go:39`). Go's
   `sync.RWMutex` is not reentrant. (The test-only `ProcessAll`/`ProcessBatch`
   in the same file, `internal/infra/queue/repo.go:64-98`, already use the
   correct lock → dequeue → **unlock** → execute pattern; `processQueue` is
   the variant that kept the lock across execution.)
2. The queued media-album task (`internal/app/handler/service.go:573`) calls
   `Repo.Add` **from inside itself**: `internal/app/handler/service.go:638`
   (per-destination forward) and **`internal/app/handler/service.go:644`**
   (stats — unconditional, so it fires even under a webhook-only rule with
   empty `rule.To`). The worker re-acquires its own lock → parks forever.
   **Trigger:** any media album that reaches `processMediaAlbum` through a
   matching configured rule — the call sites are both inside the per-rule
   loop after `rule.From == chatID` matches
   (`internal/app/handler/service.go:243`), at
   `internal/app/handler/service.go:270` (textless members) and
   `internal/app/handler/service.go:285` (captioned members); the
   unconditional statistics `Add`
   (`internal/app/handler/service.go:644`) means an empty `rule.To` does not
   avoid it.
3. Cascade: the next `Add` from the sole update-loop consumer
   (`handler.Run` → `processMessage`, `internal/app/handler/service.go:398`)
   blocks on the same lock → the cap-100 update channel fills →
   `listenUpdates` blocks at `internal/infra/telegram/repo.go:431` → the
   go-tdlib receiver chain backpressures → **the whole engine freezes: no
   push, no log, all updates dead.**

Two further latent instances of the same reentry (same precondition, same
fix): edit-retry `internal/app/handler/service.go:723` (outer task enqueued
at `internal/app/handler/service.go:306`) and delete-retry
`internal/app/handler/service.go:863` (outer task enqueued at
`internal/app/handler/service.go:325`).
(`OnMessageSendSucceeded`, `internal/app/handler/service.go:331`, `Add`s a
task whose body never `Add`s again → not a reentry.)

## Evidence — `kill.log`

| g | blocked | decisive frames (both lock sites on the same mutex `0x40002f3e70`) | reading |
|---|---------|-----|---------|
| **39** | `sync.Mutex.Lock, 158 min` | `queue.(*Repo).processQueue` (`internal/infra/queue/repo.go:111`) → `executeTask` (:120) → `handler.(*Service).processMediaAlbum.func1` (**`internal/app/handler/service.go:644`**) → `queue.(*Repo).Add` (`internal/infra/queue/repo.go:39`) → `RWMutex.Lock` | **THE WEDGE** — the single queue worker deadlocked re-acquiring its own lock. Blocked longest = first event in the cascade. |
| **44** | `sync.Mutex.Lock, 144 min` | `handler.(*Service).Run` (`internal/app/handler/service.go:173`) → `handleUpdate` (`internal/app/handler/service.go:181`) → `OnNewMessage` (`internal/app/handler/service.go:291`) → `processMessage` (**`internal/app/handler/service.go:398`**) → `queue.(*Repo).Add` (`internal/infra/queue/repo.go:39`) → same `RWMutex.Lock` | A NEW-message update blocked at the per-message stats `Add` — consistent with the manual NEW-text test; the dump does not identify the specific message (no message/chat id in a stack). This is the **sole `Updates()` consumer** — from here the update loop is stopped. |
| **29** | `chan send, 144 min` | `telegram.(*Repo).listenUpdates` (`internal/infra/telegram/repo.go:431`, `r.updates <- typ`) | Publisher blocked on the **full cap-100 channel** — its consumer (g44) is wedged. |
| **48** | `chan send, 126 min` | go-tdlib `client.(*Client).receiver` (`client.go:89`) | Backpressure one level upstream. |
| **47** | `chan send, 62 min` | go-tdlib `(*tdlib).receiver` (`tdlib.go:72`) | The C++→Go bridge still **actively delivering** — the session is alive and producing; backpressure, not a dead connection. |
| **41** | `chan receive, 144 min` | `push.(*Generation).runWorker` (`internal/service/push/dispatcher.go:216`) | Push worker **idle** — no work arrives (update loop wedged upstream of `Submit`). |
| **40** | `select, 184 min` | `auth.(*Service).run` (`internal/app/auth/service.go:55`) | Auth goroutine alive in its select loop. (The stack alone does not show the auth-state value; together with g29/47/48 actively propagating updates, it is inconsistent with a dead session.) |

The blocked-duration order (158 > 144 = 144 > 126 > 62) matches the cascade
order exactly.

**Why the observations follow.** The push path (dispatcher/sink) is
independent of the task queue: `SubmitV2`
(`internal/app/handler/service.go:352`) precedes the blocking stats `Add`
(`internal/app/handler/service.go:398`). What the wedge kills is therefore
**processing itself**: from the moment Run blocks, every subsequent update
sits in the filling channel and never reaches `Submit` — hence zero inbound
requests, zero delivery WARN, and an idle push worker. The album closure
pushes its members (independent push path,
`internal/app/handler/service.go:597-629`) **before** the nested `Add` that
wedges it — so the triggering album's own push may be the last successful
delivery before the stop.

## Fix (implemented on `diag/budva-push-stall`)

Release the lock **before** executing the task — mirroring the already-correct
`ProcessAll`/`ProcessBatch` in the same file:

```go
// internal/infra/queue/repo.go:100 (as implemented)
func (r *Repo) processQueue() {
	r.mu.Lock()
	front := r.queue.Front()
	if front == nil {
		r.mu.Unlock()
		return
	}
	fn := front.Value.(func())
	r.queue.Remove(front)
	r.mu.Unlock()   // release BEFORE running the task (was: held until fn returned)
	r.executeTask(fn)
}
```

Removes the self-deadlock precondition; serial execution is preserved (only
the single worker calls `executeTask` in the production loop); fixes all
three reentry sites at once.

**Validation (implemented).** Regression tests added, mapped to the plan:
- `internal/infra/queue/repo_test.go::TestRepo_ProcessQueue_nested_Add_
  does_not_deadlock` — plan (1)+(2): a running task re-enters `Add`;
  `processQueue` must return (timeout-guarded) and the nested task must run
  FIFO on the next call. Covers all three reentry sites through the shared
  queue. **PASS, race-clean.**
- `internal/infra/queue/repo_test.go::TestRepo_StartContext_nested_and_
  external_tasks_complete` — plan (1), production worker path: a running task
  appends a follow-up and an external `Add` (the update loop) arrives; both
  must drain. **PASS, race-clean.**
- `internal/app/handler/album_wedge_regression_test.go::
  TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` — plan (4)+(6), incident
  shape on the **real** `StartContext` worker: a webhook-only (empty `rule.To`)
  album reaches `processMediaAlbum`, appends its statistics task, and the
  worker must drain it; the next NEW text's `Add` (service.go:398, goroutine
  44 in the dump) must not block and must reach the submitter. Driven through
  `StartContext` (not `ProcessAll`) precisely because `ProcessAll` already
  releases the lock and would not reproduce the bug. **Verified by source
  trace; not compiled here** — the handler package imports the CGO
  `go-tdlib/client` and needs the ~15GB TDLib tree CI provisions, which this
  sandbox lacks (pre-existing limit affecting all handler tests).
- Panic recovery (plan 3) is already covered by the pre-existing
  `TestRepo_ProcessQueue_recovers_from_panic` and is preserved.

Local run: `go test -race ./internal/infra/queue/...` → **ok** (all 16 tests,
race-clean). CI run (provisions the TDLib tree, which this sandbox lacks)
`ci-test.yml` 40995828178 on the code tip `89f40b8` → **success**:
`ok internal/app/handler 3.166s` (the incident-shape test executed and
passed) and `ok internal/infra/queue 3.007s`; `go vet ./...` clean.

**Runtime acceptance (PRD, PASSED 2026-10-30):** on the box running the fixed
`v0.3.1` / `:latest` build, an **album followed by a plain text** into a source
chat both reached the configured endpoint **without** an engine restart, with
the user's confirmation on the endpoint side. The stall does not recur.

## Residual hardening (not this incident's cause; separate follow-ups)

- **No per-task timeout.** A hung RPC inside a task would wedge the serial
  worker (and thus the update loop) the same way. The wedge stack contains
  no RPC, so this is not this incident — it remains a latent exposure.
- **No re-auth on session death.** `internal/app/auth/service.go:60,66-67`
  swallows `Closing`/`Closed` with no log, no listener, no re-auth anywhere
  in the production tree; a mid-session TDlib death would be equally silent
  and terminal. Refuted as this incident's cause by g29/47/48.
- **No observability.** The success path is log-silent
  (`internal/service/push/dispatcher.go:230-233`, metric only) and the
  metrics backend is unconfigured in this deployment — there is no
  dead-man's-switch on last successful delivery. This is the PRD
  closeout answer ("the missing signal that should have surfaced this
  early"): enable metrics export and alert on `last_success_unix` not
  advancing.
- **1s tick granularity** (`internal/infra/queue/repo.go:51-62`): nested
  work is picked up on a later tick (≤1s) — pre-existing behavior, recorded
  so it is not misread as a regression of the fix.

## Provenance

- Raw dump: `kill.log` (repo root), taken on the box while stalled.
- Independent audit: `research/team-b-audit-report.md` — initial audit:
  root cause **confirmed** (its P1 wording corrections, path
  normalization, and the delete-retry site are applied here); 2026-10-30
  follow-up audit: verdict **PASS** with two remaining editorial findings
  (citation normalization, trigger wording), both applied in this revision.
- This record intentionally does not retain the pre-dump diagnostic ladder
  (superseded), per owner instruction 2026-10-30.
