# External Push Implementation Evidence

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
