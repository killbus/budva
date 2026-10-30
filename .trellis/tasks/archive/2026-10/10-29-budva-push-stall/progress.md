# Budva push stall — progress / final record

## Status: DONE (2026-10-30) — merged, published, runtime-accepted

**Merged.** Squash **`415d90c`** on `main` (PR **#7**,
`https://github.com/killbus/budva/pull/7`) — a single commit
`fix: release task-queue lock before executing tasks (10-29 push stall) (#7)`,
per the main-history convention (one squash commit per PR, cf. `(#4)`, `(#5)`).
The 9 branch iteration commits were collapsed to the logical units
`fix` / `test` / `docs` in the clean-up step (tree-identical tip) before the
PR was opened.

**Published.** Tag **`v0.3.1`** on `415d90c` → `docker-publish` run
**41005031378 success** (amd64 + arm64 build, per-arch smoke, merge) →
`ghcr.io/killbus/budva:v0.3.1` and **`:latest`** published. The box's
`:latest` image is now the fixed build.

**Runtime acceptance (PRD) PASSED (2026-10-30, user-confirmed on the box with
the new `:latest`):** an **album + a plain text** into a source chat both
reached the configured endpoint **without** an engine restart — the stall does
not recur on the fixed build.

**CI GREEN.** `ci-test.yml` run **40995828178** on code tip `89f40b8` completed
**success**: `go vet ./...` clean and `go test ./internal/...` green —
including `ok internal/app/handler 3.166s` (the TDLib/CGO tree provisioned; the
new `TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` executed and passed, not
merely source-traced) and `ok internal/infra/queue 3.007s` (all 16 tests, the
two deadlock regressions included). The pre-merge PR check (run 41000710235 on
the cleaned tip) was also green.

**Root cause:** a reentrant `sync.RWMutex` self-deadlock in the single-serial
task queue (`internal/infra/queue/repo.go`). `processQueue` (`repo.go:100-112`)
held the queue write lock for the **entire** execution of the dequeued task;
the queued media-album task (`internal/app/handler/service.go:573`) calls
`Repo.Add` **from inside itself** (`service.go:638/:644` — the stats `Add` is
unconditional, so it fires even under a webhook-only rule) → the worker
deadlocked on its own lock → the sole update-loop consumer blocked at the next
`Add` (`service.go:398`) → the cap-100 update channel filled →
`listenUpdates` blocked (`internal/infra/telegram/repo.go:431`) → the go-tdlib
chain backpressured → **the whole engine froze (no push, no log)**.

**Fix:** `processQueue` releases the lock **before** `executeTask` (mirrors the
same file's `ProcessAll`/`ProcessBatch`). All regression tests added (see
`research/root-cause.md` → Validation).

- **Trigger:** any media **album** that reaches `processMediaAlbum` through
  a matching configured rule (call sites `internal/app/handler/service.go:270,
  285`, both after the `rule.From` match at `:243`).
- **Reset (before the fix):** process restart (clears lock + channels) —
  matched the observed "stops after uptime, manual restart cures, recurs".
- **Proof:** SIGQUIT dump taken **while stalled** — `kill.log` (repo root);
  goroutine 39 is the wedge, full cascade in `research/root-cause.md`.
- **Audit:** `research/team-b-audit-report.md` — initial audit **confirmed**
  the root cause; 2026-10-30 follow-up: **PASS** (two editorial findings
  applied); implementation follow-up (report §9): production fix **PASS**,
  queue tests **PASS**; its two findings **both resolved**:
  - **TB-IMPL-P2-01** (handler test left the worker goroutine + 1s ticker
    alive via a never-cancelled `context.Background`) → fixed (bounded ctx
    cancelled in `t.Cleanup`).
  - **TB-IMPL-P3-01** (stale task metadata) → resolved by the metadata
    refresh (task.json `commit`/`pr_url` = merged squash, both manifests
    populated, prd acceptance wording corrected).

## Test status

- `go test -race ./internal/infra/queue/...` → **ok**, all 16 tests, race-clean
  (the two new deadlock regression tests + 14 pre-existing).
- `internal/app/handler` → **executed in CI** (run 40995828178, TDLib tree
  provisioned): `ok ... internal/app/handler 3.166s`. The new
  `TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` **runs and passes in CI**
  (it could not be compiled locally — the handler package imports the CGO
  `go-tdlib/client`, which needs the ~15GB TDLib tree CI provisions).

## Residual hardening (not this incident; tracked as separate follow-ups)

No per-task timeout; no re-auth on session death; no dead-man's-switch on last
successful delivery. See `research/root-cause.md` → Residual hardening.

## Scope boundary

Producer-side only. The target is a black box (accepted / rejected /
unreachable); no target-side change from this task.

## Files

- `prd.md` — task definition + acceptance
- `research/root-cause.md` — task, symptom, method, root cause, evidence, fix
- `research/team-b-audit-report.md` — independent audit (confirms root cause)
- `design.md` — producer-side signal map (source-verified)
- `task.json` — metadata
- `kill.log` (repo root) — the decisive SIGQUIT dump
