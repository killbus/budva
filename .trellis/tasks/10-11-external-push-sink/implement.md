# Implementation Plan

P01-P09 produced the text-event test image. Acceptance review reopened this
task for P10-P12. Publication history is recorded in progress.md; no main merge
or production deployment is authorized.

- P01: Canonical envelope and Sink contract; JSON schema tests.
- P02: pushTargets / pushTo model and validation; invalid configurations fail before startup.
- P03: Bounded HTTPS webhook, optional raw-body HMAC, classified failures, no retries.
- P04: Independent endpoint buffers and workers; lease-safe Submit / Retire and drain tests.
- P05: Build candidate, publish complete snapshot, retire previous generation; shutdown and rollback tests.
- P06: Outcome counters, depth, duration, reload and actual last-success metrics; secret-safe logs.
- P07: Source-fact emission; original hook coverage is incomplete (P10-P12).
- P08: Independent Telegram regression harness covering every push rejection / failure class.
- P09: Package and deployment contract; targeted tests, race, Linux full tests and vet; record evidence.

For each behavioral change, record a real red path before its green result.
Use synctest for timer and lifecycle tests. Keep existing Telegram tests unchanged.
Windows can test the standalone push package; TDLib-dependent packages require Linux.
Validation failures retain the published snapshot. Partial candidates are retired
before returning an error. No rollback may discard already accepted events.

Review gates: no retries, no breaker, no raw payload, no TLS bypass, no real
configuration in git; snapshot-fixed recipients and non-blocking submission;
unchanged Telegram queue, limiter, ID mapping and statistics semantics.

## Reopened Correction Plan

- P10: Specify edit eligibility/routing independently of copy mappings; add
  known-red webhook-only new/edit/delete and caption tests, then repair.
- P11: Add known-red album member/empty-caption/duplicate tests; emit v1 member
  events without changing Telegram album scheduling.
- P12: Re-run pipeline invariance, snapshot/reload/race and Linux CI; correct
  documentation and record evidence before a replacement test image.

Correction state (2026-10-12): P10/P11 and AC10/AC11 pass against real Linux
TDLib after explicit user authorization. Five isolated behavioral mutations
fail their regression tests; restored source passes all internal tests, focused
race, whole-repository vet/build and engine build. Windows standalone push
tests/vet also pass; Windows handler cgo failure is not acceptance evidence.
P12's isolated Linux gate is complete, but GitHub CI and any replacement test
publication remain pending separate authorization. Corrective code/specs were
subsequently committed locally as `4243ddf8cffc70ef50420bd196f56f83ced2cbea`
after commit-plan confirmation; no push or publication occurred. See progress.md and
research/linux-correction-verification.md for exact evidence.

### Correction Check Gates

- [x] P10: `push_source_routing_test.go` binds all source routes independent of
  copy state, no edit re-filter, cleared captions, nil exclusion, captured
  snapshot, mapped Telegram edits and disabled no-copy fast path.
- [x] P11: `push_album_invariance_test.go` binds forward/copy behavior across
  disabled/full/retired/success/HTTP-failure/timeout outcomes, virtual 3s quiet
  period, push-only filtering and unchanged Telegram list/mapping/statistics.
- [x] P11: `push_coverage_test.go` binds source-only lifecycle, empty/unsupported
  and duplicate album members, and shared ingress time before edits/deletes.
- [x] P12: Run real TDLib mutation reds, restore/hash-check source, then handler,
  internal, race, whole-repository vet/build and engine build. Preserve raw logs.
- [x] P12: Obtain commit-plan confirmation and commit corrective code/specs
  locally as `4243ddf8cffc70ef50420bd196f56f83ced2cbea`.
- [ ] P12: Separately authorize push, green GitHub CI and replacement
  test-image publication. No main merge or production deployment is implied.
