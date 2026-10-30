# Budva push stall — progress / handoff

## Status: FIX IMPLEMENTED + CI GREEN (2026-10-30) — PR + tag + runtime acceptance remain

**CI GREEN.** `ci-test.yml` run **40995828178** on code tip `89f40b8`
completed **success**: `go vet ./...` clean and `go test ./internal/...`
green — including `ok internal/app/handler 3.166s` (the TDLib/CGO tree
provisioned; the new
`TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` now executed and passed,
not merely source-traced) and `ok internal/infra/queue 3.007s` (all 16
tests, the two deadlock regressions included). The docs-only commits on top
(`c0a1fd5`, `3dc9bb4`) change no code.

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

**Fix (implemented):** `processQueue` now releases the lock **before**
`executeTask` (mirrors the same file's `ProcessAll`/`ProcessBatch`). All
regression tests added (see `research/root-cause.md` → Validation).

- **Trigger:** any media **album** that reaches `processMediaAlbum` through
  a matching configured rule (call sites `internal/app/handler/service.go:270,
  285`, both after the `rule.From` match at `:243`).
- **Reset:** process restart (clears lock + channels) — matches the observed
  "stops after uptime, manual restart cures, recurs".
- **Proof:** SIGQUIT dump taken **while stalled** — `kill.log` (repo root);
  goroutine 39 is the wedge, full cascade in `research/root-cause.md`.
- **Audit:** `research/team-b-audit-report.md` — initial audit **confirmed**
  the root cause; 2026-10-30 follow-up: **PASS** (two editorial findings
  applied); a second **implementation follow-up** (report §9) audited the
  three landed commits: production fix **PASS**, queue tests **PASS**, and
  raised two findings, **both now resolved**:
  - **TB-IMPL-P2-01** (handler test left the worker goroutine + 1s ticker
    alive via a never-cancelled `context.Background`) → fixed in
    `16fcc92` (bounded ctx cancelled in `t.Cleanup`).
  - **TB-IMPL-P3-01** (stale task metadata: `commit` null, empty
    `implement.jsonl`/`check.jsonl`, prd "fix commit" wording) → resolved by
    this metadata refresh (task.json `commit` = branch tip, both manifests
    populated, prd acceptance wording corrected).

## Test status

- `go test -race ./internal/infra/queue/...` → **ok**, all 16 tests, race-clean
  (the two new deadlock regression tests + 14 pre-existing).
- `internal/app/handler` → **executed in CI** (run 40995828178, TDLib tree
  provisioned): `ok ... internal/app/handler 3.166s`. The new
  `TestAlbumWebhookOnly_workerDrainsAfterAlbumTask` now **runs and passes in
  CI** (it could not be compiled locally — the handler package imports the CGO
  `go-tdlib/client`, which needs the ~15GB TDLib tree CI provisions).

## Next steps

1. ~~CI run (provisions TDLib) to execute the handler regression test.~~
   **DONE** — run **40995828178** on `89f40b8` is green (see Status).
2. Open the PR (squash-merge per `commit-hygiene.md` — the branch commits
   collapse to the logical units `fix` / `test` / `docs`), then a `v*` tag
   push to refresh the `:latest` image the box runs
   (`[[github-push-dispatch-mechanics]]`).
3. Acceptance (PRD): on the fixed build, an **album + plain text** into a
   source chat both reach the configured endpoint **without** a restart;
   user confirmation on the endpoint side.

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
