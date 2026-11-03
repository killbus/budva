# 10-30 webhook-delivery-latency — final state (closeout, 2026-11-03)

## Outcome

**Problem 1 (HEADLINE) — FIXED and ACCEPTED on the box (2026-11-03).**
Problem 2 (endpoint HOL) — DOWNGRADED to "potential, unobserved"; NOT pursued
(no owner-observed per-message queueing delay ever seen). Task closed.

## Implementation (PR #11 → main `ecea160`)

Branch `feat/10-30-openchat-acquisition` (commit `b2eaedc`, 9 files, +1034),
squash-merged to main as `ecea160` (tree-identical; tag
`pre-squash/10-30-b2eaedc` holds the pre-squash commit locally).

- `internal/domain/chat_class.go` — `IsChannelOrSupergroup`: classify by id
  MAGNITUDE (single 1e12 boundary; stored source keys are always negative,
  sign carries no type).
- `internal/infra/telegram/source_opener.go` — `SourceOpener`: single
  lifecycle-ctx worker (no reentrancy — the 10-29 lesson), plain
  desired/opened sets + `sessionReady` flag (not ref-counting);
  `ApplySet(ids)` idempotent entry (desired = channel/sg subset);
  `OnSessionSwap` marks ready + clears `opened` + signals (no snapshot
  re-apply → no lost-update vs a concurrent hot-reload); reconcile = closes
  then opens with GetChat→OpenChat order and bounded in-edge retry
  (`openRetryPasses=4`, 2s backoff); per-open pace-floor stagger (local, not
  `internal/service/limiter` — different axis).
- `internal/infra/telegram/repo.go` — `RegisterSessionSwap` + `fireSessionSwap`
  fired in the `runAuthLoop` swap critical section (trigger = adapter-swap,
  NOT `AuthStateReady` which lags and can be lost); nil-callback → WARN
  (SB2 canary: a missed registration must not be silent).
- `internal/infra/telegram/client_adapter.go` — `OpenChat`/`CloseChat` on the
  `clientAdapter` interface + thin `*Repo` wrappers; mock hand-extended.
- `cmd/engine/main.go` — register callback BEFORE `Start` (SB2); worker under
  engine ctx (SB4); `ApplySet(rulesetSourceIDs(rs))` BEFORE
  `pushManager.Replace` in `loadSnapshot` (SB3: backfill starts ASAP;
  invariant "eventually open + backfilled").
- `internal/config/config.go` — `TELEGRAM_SOURCE_OPEN_PACE` (default 300ms).

Edges (only two, per Gate A — the re-auth edge is a GHOST: one-shot
`runAuthLoop` + `CleanUp` deletes the DB, so re-auth = restart):
startup adapter-swap (full open) + hot-reload diff (added→open /
removed→close). Private/basic-group sources are NOT opened (global stream).

## Verification

- **CI (assertable half):** `ci-test` GREEN on `b2eaedc` (run 41556729958,
  job `test` — real TDlib compiled; `go vet ./...` + `go test ./internal/...`
  pass, incl. the 10 new tests: classifier boundary table + 9 SourceOpener
  tests [open/classify/order, hot-reload diff, session-swap full re-open,
  stagger pace-floor, bounded in-edge retry, GetChat-failure sequence,
  ApplySet-before-swap convergence, close non-fatal, cancelled-ctx
  convergence]). Local: gofmt clean, vet EXIT 0 on all touched packages,
  tests green 3/3 under `-race` in isolation. (The full telegram package's
  `-race` run is flaky in the local link-stub env — proven present at base
  `bb0519b` without this code; CI runs plain `go test ./internal/...`, so
  unaffected.)
- **Box (the half CI cannot assert): ACCEPTED 2026-11-03 by the owner.**

## Box acceptance evidence (2026-11-03, v0.4.0 deployed)

- **17** `source opened (open-once-keep)` lines in the ENGINE log
  (`source_opener.go:257`), staggered across the open batch — all 17
  channel/supergroup sources opened on Budva's own session. No
  session-swap WARN in the engine log (SB2 canary clean).
- **Core symptom gone:** random time-of-day tests, official client NOT
  holding any of the channels open → inbound latency **<10 s** throughout.
  The reported symptom (≥10-min zero-request window coupled to the client's
  tab, plus the open→burst catch-up) no longer reproduces.
- **N>10 no-starvation** exercised for free: 17 open sources exceed the
  engine's 10-concurrent diff-poll cap; no source starved, no source
  permanently silent (engine FIFO rotation, per-page fairness — the pinned-
  source behavior predicted at design time, now observed live).
- Residual owner-optional checks (not blocking closeout): cold-restart
  re-open (engine restart should re-emit the 17 opens, no intervention) and
  quiet-channel re-arm persistence across long idle.

## Release

`v0.4.0` (annotated tag on `ecea160`) → `docker-publish` run 41563299621
green (amd64 + arm64, each smoked before push; manifest list tagged).
`ghcr.io/killbus/budva:v0.4.0` + `:latest` verified on GHCR (v0.3.1 kept).

## Ops note (log triage)

The SB2 WARN "session swap fired but no session-swap callback registered"
appears in **facade** (and `stand`) logs once per auth — BY DESIGN: those
binaries don't own the ruleset and are not acquirers. The WARN is the
ENGINE canary; it is structurally impossible in the engine
(`RegisterSessionSwap` precedes `Start`, `cmd/engine/main.go:105→107`).
Do not treat facade occurrences as a fault. Optional follow-up (not done):
facade/stand could register an explicit no-op to silence the expected WARN.

## Scope boundary

Producer-side only. The webhook receiver is a black box; no target-side
change. Problem 2 remains a separate, unobserved, non-gating leg (its 4
contract decisions are that leg's blockers only if ever pursued).

## Files

- `prd.md` — task definition + acceptance
- `research/facts-and-model.md` — facts, root cause, three-layer evidence
- `research/design-and-position.md` — converged design + problem-2 leg
- `research/design-dimensions.md` — Gate A/B verdicts (10 dimensions)
- `research/precedent-openchat-lifecycle.md` — Stage 0b precedent survey
- `design.md` — Stage 3 converged design record
- `research/team-b-*.md` — verbatim ground-fact corroboration (not design
  approval)
- `task.json` — metadata (status: completed)
- `progress.md` — this closeout
