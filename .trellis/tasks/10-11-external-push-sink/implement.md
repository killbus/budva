# Implementation Plan

Execute P01-P09 from implement.jsonl in dependency order. Do not commit,
push, open a PR, or merge without separate approval.

- P01: Canonical envelope and Sink contract; JSON schema tests.
- P02: pushTargets / pushTo model and validation; invalid configurations fail before startup.
- P03: Bounded HTTPS webhook, optional raw-body HMAC, classified failures, no retries.
- P04: Independent endpoint buffers and workers; lease-safe Submit / Retire and drain tests.
- P05: Build candidate, publish complete snapshot, retire previous generation; shutdown and rollback tests.
- P06: Outcome counters, depth, duration, reload and actual last-success metrics; secret-safe logs.
- P07: Source-fact emission in processMessage / editMessages / deleteMessages only.
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
