# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Standards below are distilled from real CI failures and review findings of this repository (warmup-wait-ready task, Sept 2026 — 3 CI round-trips spent on four test failures, all rooted in tests that were never proven able to fail). Each rule carries its incident anchor so future readers can judge its weight.

---

## Forbidden Patterns

### F1 — Assertions that cannot fail (self-fulfilling tests)

An assertion that is true by construction does not test anything. Real incidents:

- `assert.Zero(t, m)` on a `*mock.Mock` — a mock object is never a zero value, so the assertion is also **always false** (it failed in CI while proving nothing). To assert "this was never called", use `m.AssertNotCalled(t, "LoadChats", mock.Anything)`.
- Success gated on a call counter instead of the real condition: in `TestWarmChatsOnce_AfterIntervalBootstrapResumes` the mock returned success when `chatCalls == 4`, so a **skipped LoadChats bootstrap still let the retry "succeed"** — the test stayed green while the behavior under test never happened. Gate success on the actual event (e.g. bootstrap-completed channel), not on call ordinals.

**Rule**: before writing an assertion, state what observable fact makes it false. If you cannot, it is decoration.

### F2 — Tests never proven able to fail (self-confirmation loop)

A test that has never been red cannot witness a regression. Prove each behavior-binding test red at least once, by one of (cheapest first):

1. **Strict mocks as the red mechanism**: mockery strict mode fails on unexpected calls — "LoadChats must not run" is proven by the mock itself, not by a redundant assertion. Leave one un-satisfied expectation or an unexpected call path in place deliberately.
2. **Real CI failure**: if a test went red for the right reason during development, that counts as its proof (record it in the test comment).
3. **Manual mutation** (when 1–2 don't apply): temporarily delete the behavior the test binds to (e.g. remove the `signal` branch in `listenUpdates` → `TestWithReady_ColdMissEdgeReleases` must go red at deadline; make `isChatNotFound` always true → passthrough tests must go red) and confirm red before restoring.

**Rule**: a green test without a known red path is unverified. In this repo each CI round-trip is expensive (go-tdlib does not build on Windows), so prove red **before** pushing, via strict-mock reasoning or local mutation, not after.

### F3 — Redundant mock wiring duplicated per test

Two tests wiring the same nine-method mock differently is ten o'clock problems waiting: one gets the fix, the other drifts. Real finding (structure-audit): `TestAllWrappers_MissDrivesLoadChatsAndRetries` and `TestAllWrappers_WrappedErrorContext` each carried an inline copy of the same 9-entry wrapper table and 9-block mock wiring (~230 duplicated lines).

**Rule**: shared table-driven case sets and mock-arrangement helpers (`arrangeAllMiss`, `arrangeRetrySucceeds`) live at package level; test bodies stay under ~15 lines of Arrange.

---

## Required Patterns

### R1 — Success conditions mirror the domain event

Mock success must be gated on the event the test is about (bootstrap completed, edge delivered), not on call count or wall-clock. See F1's incident.

### R2 — Mock call-count expectations assert ordering facts only

`Times(n)` on a mocked call is a contract about the code under test — countable, deterministic. Use it for facts ("exactly one bootstrap under rate-limit"), never as the success path of the fixture itself.

### R3 — Comment the red path

Tests that bind to subtle behavior carry a one-line comment stating what regression makes them fail ("mock strictness: any unexpected LoadChats fails the test"). This is the durable trace of F2's proof.

---

## Testing Requirements

- Table-driven tests with subtests for per-case parallelism (`t.Parallel()` inside `t.Run`).
- `testing/synctest` for anything involving timers, deadlines, or rate limits — virtual time only; no real sleeps in tests.
- Shared-error discipline: `errors.Is` on `client.ResponseError` compares by pointer — all mock calls in one test must return one instance created outside the closure.
- Windows reality: go-tdlib does not compile locally; gofmt + targeted vet + strict-mock reasoning are the pre-push gate, CI is the compile/test oracle. Self-review imports and mock wiring before every push — each mistake costs a CI round-trip.

---

## Code Review Checklist

- [ ] Every new test has a stated red path (strict mock / recorded CI failure / mutation).
- [ ] No assertion that cannot fail (check `assert.Zero` on non-zeroable objects, counter-gated success).
- [ ] Mock wiring for repeated fixtures is hoisted to helpers, not copied.
- [ ] `Times()` expectations state domain facts, not fixture plumbing.
