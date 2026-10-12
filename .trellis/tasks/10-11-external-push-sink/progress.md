# External Push Implementation Evidence

## Current Status: Correction Linux-Verified, Publication Pending (2026-10-12)

This section supersedes earlier completion claims in the historical report.
The reopened gaps were copy-dependent webhook-only edits and missing album new
events. P10/P11 corrective source changes and AC10/AC11 regressions now pass
against real Linux TDLib. P12 isolated Linux verification is complete; GitHub
CI and replacement test publication remain pending separate authorization.
No new image has been published, and this task remains in progress.

After the user's commit-plan confirmation, corrective code, regressions and
spec sync were committed locally as
`4243ddf8cffc70ef50420bd196f56f83ced2cbea` on `feat/external-push-sink`.
The second approved commit records task/evidence updates. Push, GitHub CI,
replacement image, PR/merge and production deployment remain unperformed.

User confirmed preserving full Telegram message intent as the product goal.
Linked planning task `10-12-telegram-webhook-semantics` owns versioned rich
content, entities, relationships and usable secure media access. Published
`dev-run12-d58909f` remains a text-event test build, not full message support.

### P10-P12 correction evidence

- Source edits route through all captured source rules, not copied-message
  references. Full supported text/caption, including an empty caption, is sent
  once on the first Telegram attempt. New-message filters are not re-evaluated.
- Album submission uses the captured dispatcher after existing collection;
  per-member `FiltersOK` gates only push. Duplicate bucket members are suppressed
  only for push, and unsupported text is skipped. Telegram batching, forwarding,
  copy mappings, limiter and statistics remain on their original path.
- Album new facts capture first-ingress bucket time before collection; delayed
  old captions cannot supersede later edits/deletes under downstream LWW. A
  two-member synctest regression queues later edits/deletes before collection
  execution and binds the shared timestamp. Album age comes from virtual elapsed
  time; collection and limiter assertions forbid execution before the 3s deadline.
- Tests cover source-only new/edit/delete, caption clearing, album empty captions,
  unsupported content, collection duplicates, captured snapshots and album
  forward/copy invariance under disabled/full/retired/success/HTTP-failure/timeout
  dispatch. Original Telegram tests are not modified.
- Known-red mechanisms: the original copy guard fails the required no-copy
  formatted-text expectation and edit-event count; missing album hooks fail
  required per-member text/filter expectations and event counts; altered Telegram
  members/mapping/scheduling cause strict mock or trace failures. These are
  inspected strict-mock paths. The subsequent authorized Linux run also records
  five behavioral mutation reds, followed by restored-source greens below.
- Windows Go 1.25.9: `go test -count=1 ./internal/service/push/...` passes the
  35 push tests (the event subpackage has no test files);
  `go vet ./internal/service/push/...` passes. Dependencies
  are cached (`GOPROXY=off`), build cache is workspace `.tmp-go-cache`.
- Handler test compilation is blocked in the existing go-tdlib v0.7.6 dependency:
  `client/client.go:11:18: undefined: JsonClient` and additional undefined
  `Response`, `Type`, `meta`, `JsonInt64` symbols on Windows with CGO disabled.
  This is an environment failure, not a behavioral red. No stubs or mock TDLib
  library are substituted to claim acceptance.
- Remote SSH validation was initially declined, then explicitly authorized on
  2026-10-12. Tests ran in a new isolated correction directory with the existing
  real TDLib shared library and pinned C ABI header, not production state.
- Linux Go 1.25.9 with `CGO_ENABLED=1` and `-tags libtdjson`: handler `TestPush`
  passes 14 top-level tests (45 including subtests); `./internal/...` passes
  22 tested packages and 509 top-level tests (853 including subtests).
- Race passes for push, handler and ruleset: 3 tested packages, 140 top-level
  tests (211 including subtests). Whole-repository vet/build and the engine
  binary build all exit 0. No test failures or race reports occur.
- Five isolated mutations each compile and fail the named behavioral test:
  no-copy edit guard, copy-only recipient routing, omitted album hook, delayed
  album timestamp and skipped album quiet-period wait. Source is restored after
  each mutation; all 157 source hashes match again before and after green runs.
- This is isolated Linux runtime evidence, NOT a new GitHub CI run or image.
  Reproduction commands, source/library hashes, counts and raw log references
  are in `research/linux-correction-verification.md` and
  `linux-correction-evidence/`. Replacement publication remains a separate gate.
- No commit, push, PR, merge, deployment or task archive occurred during the
  validation turn; the subsequently authorized local commit is recorded above.
- Trellis read-only check found no source-routing defect. Its album timing
  test finding is addressed with elapsed-time mocks and deadline assertions;
  these static review results are now complemented by the Linux runtime tests.
  Final local `gofmt`, task/JSONL parsing and `git diff --check` pass.
- Spec sync records elapsed-domain-time timer mocks and source-fact routing/
  delayed album timestamp boundaries in the backend quality and TDLib guides.
  Mixed P01-P12 step records are preserved verbatim in research; the context
  JSONL files now contain only spec/research file-and-reason entries.
- Final read-only evidence review finds no new factual/status issue; the scoped
  structure audit finds no above-threshold issue. Empty vet/build logs alone do
  not certify exit status; their success is backed by recorded command results.
  Final full-scope package query identifies this as a single-repo backend task.

## State

P01-P09 implementation and verification are complete; ready for review.
No commit, push, PR, merge, deployment or task archive was performed.
The inherited workspace remains on main (task branch == base branch); resolve
the task-branch workflow before any separately authorized publication.

## Delivered

- Transport-independent canonical event contract and HTTPS webhook sink.
- Optional raw-body HMAC, final-2xx success, classified failures, bounded
  redirects, no downgrade, and no transport-level POST replay/retry.
- Independent endpoint buffers, non-blocking submission, leased generation
  retirement, complete atomic rules/dispatcher snapshots and shutdown drain.
- Configuration/reference/header/timeout validation before worker startup.
  Manager startup is infallible after validation, so no partially started
  candidate failure path exists; invalid replacement retains the old snapshot.
- OTel outcome counters, depth/duration and actual last-success metrics.
- Production new/edit/delete hooks; Telegram retries do not repeat push.
- Shared strict-mock Telegram invariance harness across disabled, full,
  retired, successful and every classified delivery failure outcome.
- Sensitive configuration and downstream loss/LWW/history contract docs.

## Final Verification

All commands completed with exit 0 against the final runtime source:

```text
Windows: go test ./internal/service/push/... (35 tests passed)
Windows: go vet ./internal/service/push/...
Windows: git diff --check
Linux: go test -tags libtdjson ./internal/... -count=1 (22 tested packages)
Linux: go test -race -tags libtdjson ./internal/service/push/... ./internal/app/handler ./internal/infra/ruleset -count=1
Linux: go vet -tags libtdjson ./...
Linux: go build -tags libtdjson ./cmd/engine
```

Linux final race package durations: push 1.131s, handler 2.201s, ruleset 1.121s.
Whole-repository vet and engine build logs are empty on success. Existing
`internal/app/handler/service_test.go` was not modified.

Real TDLib validation runs in `/home/ying/budva-push-validation-20261011`,
with isolated source, Go 1.25.9 toolchain/cache and dependencies. No production
configuration was uploaded and no system package/runtime was replaced.
The shared library comes from public `ghcr.io/killbus/budva:v0.2.0` (1.8.40),
not a stub. The original short source pin `22d49d5` failed with HTTP 404/422;
public C ABI headers use full SHA `c15d3f5a5de6e3ba5839822c451152e5e18bb700`.
Official CMake GenerateExportHeader generated the required export macros;
no replacement TDLib library was built.

```text
libtdjson.so SHA256:
11fa9154f9b0146b4c8e7abccb316b3117de28d570ccae8faa277c55189ddc98
td_json_client.h SHA256:
cdf6190b0264417c69c108d192d1f3381f3a1315ddb309798dbefc5036be1eb3
```

Remote logs: `internal-final.log`, `race-final.log`, `vet-final.log`,
`build-final.log`; setup and command wrapper are recorded beside this file.
This is isolated Linux runtime verification, not a CI run or live deployment.
Prometheus live scraping was not exercised; SDK collection tests verify the
instruments, and production wiring uses the existing global OTel provider.

## RED Evidence

- P01/P03/P04/P05/P06 contract/API tests first failed on missing symbols.
- Duplicate names previously produced two deliveries; worker timeout omission
  caused a synctest deadlock before deadline enforcement.
- Plaintext downgrade and static Content-Type override tests failed before
  security corrections; duplicate delete IDs produced two push events.
- Final Linux mutation removed the captured submitter: production hook tests
  failed; bypassed validation: invalid timeout/header tests failed.
- Final Linux mutation skipped old-generation retirement: concurrent manager
  test failed on late old-snapshot acceptance; restored source afterward.
- Replay inspection failed (`Should be false`) with replayable POST requests.
  Transport now sees GetBody=nil while the client retains bounded 307/308
  redirect support. A mutation clearing the client's GetBody failed with
  `webhook delivery failed: class=redirect status=307`.
- The race detector found a test-only shared publication pointer race;
  changed the test to atomic.Pointer, then reran final race successfully.

`linux_mutation.py` restores each isolated file in finally and confirms
behavioral failures rather than compilation failures.

## Scope And Review

Self-review checked the three emit points, fixed snapshot routing, no Telegram
queue/limiter/mapping/statistics changes, no retries/breaker/raw TDLib payload,
HTTPS/HMAC error redaction, and lease-safe bounded drain. Configuration errors
retain the prior publication; manager Close serializes replacement. There was
no independent agent review.

The existing separate media-album new-message path remains unchanged and does
not emit push in v1, matching the three prescribed hook locations. Edits route
through existing copy mappings; permanent deletes route through source rules
even without mappings. These boundaries are explicit in the package README.
Delivery remains in-memory and lossy, with no predecessor/order guarantee.

No implementation or verification step remains open.

## Test Image Publication

User subsequently authorized creating `feat/external-push-sink`, committing
task files, pushing the branch and publishing the test image through the
existing GitHub `docker-publish` workflow. Use the established GHCR
`ghcr.io/killbus/budva:dispatch-test` channel; do not publish a release tag,
update latest, merge main, or deploy production. This section supersedes the
earlier no-publication state; final run/image evidence will follow publication.

Publication completed on 2026-10-12 (UTC):

- Source branch: `feat/external-push-sink`.
- Source commit: `d58909f4c576209958393201a295629d9f6dfba8`.
- Linux CI: https://github.com/killbus/budva/actions/runs/38195811065 (success).
- Docker: https://github.com/killbus/budva/actions/runs/38195815275 (success).
- Both linux/amd64 and linux/arm64 passed version, facade-health and
  engine-startup smoke checks before manifest publication.
- Tag: `ghcr.io/killbus/budva:dispatch-test`.
- Embedded version: `dev-run12-d58909f`.
- Immutable manifest: `ghcr.io/killbus/budva@sha256:5f9b41c31eb6a3003c245bad263d1b273524761abad4a562b1727d606ee39037`.
- GHCR manifest was independently read back; its revision annotation matches
  the source commit and its digest matches the publication log.
- No release/latest tag, PR, main merge or production deployment performed.
  Live Telegram-to-user-webhook testing remains the user's test phase.
