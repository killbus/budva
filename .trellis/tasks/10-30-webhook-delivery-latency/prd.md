# Mitigate webhook delivery latency

**Status (2026-11-01):** the **Problem-1 root cause and fix are CONVERGED**
(closed by three-layer evidence — normative spec + pinned TDlib engine source
+ go-tdlib architecture — **not** by an owner-run experiment, which is
withdrawn). The record was reset off main (`bb0519b`) and lives on branch
`docs/10-30-type-agnostic-requirement` pending owner sign-off. No code yet;
implementation is a later gate.

## Goal

Fix the webhook delivery latency, **producer-side only**. Two distinct
problems, **not at the same stage**:

1. **The 10-minute zero-request silence window (headline) — CONVERGED.** It
   covers **text as well as media** and is **coupled to the official client not
   having the chat open** (the client and Budva are separate sessions). The
   root cause: **Budva does not own the acquisition boundary of its ruleset
   sources** — it is a passive subscriber that never `OpenChat`s, so for
   open-state-gated classes (**channels/supergroups, any size — no protocol
   threshold**; "large" is a maintainer paraphrase, see facts-and-model
   terminology note) its own session is not an active acquirer and goes silent
   until *some* session opens the chat (cross-session passive updates → the
   observed open→burst / close→silence). The **fix**: `OpenChat` each
   channel/supergroup source on Budva's own session and **keep it open**; the
   in-process TDlib engine (go-tdlib is a CGO shell over the C++ TDlib) then
   owns gap-recovery + the open-gated periodic `getChannelDifference` re-poll.
   **Native, no new component, no hand-built poller.**
2. **Steady-state per-message delay (Problem 2) — DOWNGRADED to "potential,
   unobserved."** The serial per-endpoint worker + cap-256 buffer +
   single-attempt sink (bounded ≤60 s at `timeoutSeconds:60`) is a real code
   property, but there is **no owner-observed per-message queueing delay** — so
   per the "does the problem even hold" criterion it is a *potential*
   contributor, verified **if** such delay is ever seen (a controlled
   slow-receiver test, not production timing collection). It is a **separate
   leg** and **does not gate Problem 1.**

Authoritative detail: `research/facts-and-model.md` (facts + converged root
cause + three-layer evidence + conformance assessment + corrections),
`research/design-and-position.md` (converged Problem-1 design + remaining
source-confirms + the downgraded Problem-2 leg), and the Team B reports
(cited below in the Team B scope note — ground-fact / mechanism corroboration,
not design approval).

## Constraints

- **Producer-side only.** The webhook receiver is a black box (accepted /
  rejected(class) / unreachable). Receiver-side **fast-ack** and **idempotent
  ingest** are *contracts Problem 2 depends on* — out of scope to implement
  here.
- **Native / minimal maintenance surface.** No bespoke queue microservice.
  Problem 1 uses a first-class TDlib API (`OpenChat`) on the already-linked
  engine — no new component, no hand-built poller.
- **Conformance discipline.** Every new behavior must be one an official client
  performs, at a client-plausible volume (facts-and-model "Conformance
  assessment").
- **Behavior stays in-repo and under existing CI** (vet + test + the
  TDLib-provisioned handler tests).
- **Evidence boundary.** Problem 1 is **closed** by three-layer evidence; no
  owner-run experiment is required. Problem 2's mechanism is a **confirmed
  code property and a ≤60 s bounded contributor** — a *potential*, unobserved
  issue; its evidence, if pursued, is the in-repo **controlled slow-receiver
  test**, **not** production-side timing collection.

## Requirements

### Problem 1 (primary, converged) — own the acquisition boundary

- **P1-R1 — Open set = the ruleset's channel/supergroup sources.** At
  generation build, Budva calls `OpenChat` **on its own session** for **every
  channel/supergroup source chat** in the ruleset. Private-chat and basic-group
  sources are **not** `OpenChat`'d (not open-state-gated; global stream). The
  open set is derived from the ruleset's source chat types, so the production
  chat type is **diagnostic, not a design input** — "support every type" = one
  intent (own the acquisition boundary) realized by each class's own
  client-conformant mechanism.
- **P1-R2 — Keep open; re-open only at state-reset edges.** No `CloseChat`
  except when the chat is **removed** from the ruleset. Re-open fires only at
  **re-auth** (dialogs reset) and **process restart** (`open_count` is
  in-process). **No periodic re-open, no toggle** (churn is the non-human
  signature and re-introduces the deferred state = the bug).
- **P1-R3 — Conformance guardrails.** (a) open **only** ruleset source chats,
  never the account's other chats; (b) **stagger** the initial/re-auth `OpenChat`
  batch via the existing `internal/service/limiter` (no N `openChat` in the same
  instant); (c) **do not fight the engine's 10 simultaneous-diff-poll cap**
  (no logic to bypass its queueing).
- **P1-R4 — Never call a read API.** Conformance = Budva does **not** issue
  `readHistory` / `readChannelInbox` or any mark-read RPC (a normal open does
  not mark a channel read; this is the default — Budva does not forward reads).
  Read receipts are thereby **conformed**, not a risk.
- **P1-R5 — Do not hand-build a poller.** Rely on the in-process TDlib engine's
  channel-difference polling (go-tdlib links the C++ TDlib, which already
  polls opened channels). The Go side issues **only** `OpenChat`. This **rejects**
  an earlier draft that proposed a raw `Send`-built poller on the mistaken
  premise that go-tdlib is a pure-Go MTProto client.
- **P1-R6 — No regression of the 10-29 fix.** The task-queue lock release
  (`415d90c`) is untouched.

### Problem 2 (secondary, DOWNGRADED — pursued only if/when a per-message delay
is actually observed) — bound the endpoint worker

These do **not** gate Problem 1 and are made **if/when** that leg is pursued:

- **R0 — Reproduce + bound the mechanism by a controlled test (a GATE for this
  leg).** In-repo unit/contract tests in `internal/service/push` against a
  **slow-receiver stub** (fixed response time `T`): a serial-baseline test
  reproduces the queue-wait (`T3−T2`) growing linearly for N enqueued events,
  and a post-fix test shows the same burst held within budget under the
  concurrency/timeout change. **No production timing collection required** — the
  deterministic test is the evidence (stronger than capturing a live delayed
  occurrence).
- **R0b — Post-deploy verification aid (not a gate).** Expose per endpoint:
  queue depth, oldest queued-event age, enqueue→pickup duration (alongside the
  existing `enqueued / full / failed / duration / lastSuccess` metrics); retain
  a stable correlation key. The payload's `OccurredAt` (= `T2`) already lets the
  receiver compute arrival-minus-`OccurredAt` for free.
- **R1 — Bound worst-case per-request occupancy (test-justified,
  configurable).** The per-endpoint timeout is chosen from the **receiver's
  response time** (one `curl -w '%{time_total}'` or the access log — a single
  black-box number, **not** a measurement campaign) + delivery semantics +
  failure budget + safety margin + loss policy; the `timeoutSeconds` rule field
  (`internal/domain/ruleset.go`) is the mechanism. "Set to P95/P99" is
  **rejected** (a loss knob under at-most-once). **No concrete value until the
  receiver behavior and loss policy (R4) are recorded.**
- **R2 — Raise per-endpoint service rate (test-justified, configurable).** A
  bounded, **configurable** concurrency per endpoint replaces the single serial
  worker in `internal/service/push/dispatcher.go`. Initial value from the **R0
  test budget** (smallest N holding the burst in budget) and the
  receiver-concurrency contract — **not a preset 4**, not production-
  arrival-derived. Topology from the ordering scope (R3): unordered pool vs
  stable chat partitioning (`hash(chat_id) % N`).
- **R3 — Ordering + delivery contracts (this leg's preconditions).** loss
  policy, duplicate policy, ordering scope (global / per-endpoint / per-chat /
  none), receiver fast-ack, receiver idempotent ingest, receiver-supported
  concurrency.
- **R4 — Loss policy (this leg, owner decision).** best-effort (in-memory,
  droppable) vs must-not-lose (durable queue + retry + DLQ + idempotent event
  id; the durable queue, if needed, is an **off-the-shelf wheel evaluated as a
  decision** — e.g. NATS JetStream or Redis Stream — *not pre-approved by
  name* — through the existing `push.Sink` seam `dispatcher.go:32-36`).

## Acceptance criteria

### Problem 1 (primary, converged)

- [ ] **P1-A1 (the open-set regression test):** the set of `OpenChat` calls
      equals the ruleset's channel/supergroup source set (and excludes
      private/basic-group sources); on ruleset hot-reload **add** an `OpenChat`
      fires and on **remove** a `CloseChat` fires; on **re-auth** the full set
      is re-opened.
- [ ] **P1-A2 (keep-open + no read API):** no `CloseChat` in steady state; the
      initial/re-auth batch is **staggered** (limiter-invoked); **no read API**
      (`readHistory`/`readChannelInbox`/mark-read) is issued.
- [ ] **P1-A3:** `go vet ./...` and `go test ./internal/...` green in CI
      (including the TDLib-provisioned handler package); **no new data race**.
- [ ] **P1-A4:** the 10-29 task-queue regression tests still pass; shared task
      queue behavior unchanged.

### Problem 2 (secondary, downgraded — only if that leg is pursued)

- [ ] **A1 (measurable, not a formula):** under a **controlled slow-receiver
      stub**, on the endpoint worker pool: queue-wait (`T3−T2`) P95/P99 and max
      within budget; oldest queued-event age within budget; in-flight ≤
      configured concurrency; **zero unexpected `buffer_full`**;
      receiver-arrival-minus-enqueue bounded. Separate thresholds for
      **unordered** vs **chat-partitioned** modes. (The `(N/concurrency)×T`
      formula is a *planning estimate* only.)
- [ ] **A2:** ordering tests for **NEW, EDIT, DELETE** pass in the selected
      topology.
- [ ] **A3:** generation-retirement + goroutine-lifecycle tests pass; **no
      goroutine leak** in the new worker pool; buffer-full / timeout /
      cancellation behavior correct.
- [ ] **A4:** `go vet ./...` and `go test ./internal/...` green; **no new data
      race** in the queue/dispatcher packages under `-race`.
- [ ] **A5:** the delivery contracts (R3) and loss policy (R4) are **recorded**
      and the implementation matches them.
- [ ] **A6:** the 10-29 task-queue regression tests still pass.
- [ ] **A7:** receiver-side contract (fast-ack + idempotent ingest) confirmed
      by the owner or flagged as the remaining external dependency.

## Out of scope

- Receiver (black box) implementation, ruleset content, target-side changes.
- **A hand-built `getChannelDifference` poller in Go for Problem 1** — rejected
  because the C++ engine already polls (go-tdlib is a CGO shell).
- **Production-side timing collection** (instrumenting the box before the
  change, continuous per-request metrics) — non-reproducible and invasive; the
  Problem-2 evidence is the in-repo controlled test (R0), and post-deploy
  observability (R0b) is a verification aid, not a gate.
- **A boundary-localization experiment / active probe for Problem 1** —
  **withdrawn**; the root cause is closed by three-layer evidence, not by an
  owner-run probe.
- The shared 1-task/second task queue's redesign (a separate hardening item from
  the 10-29 residual list; may be carved into a sub-task if a Problem-2 scope
  needs it).
- A full T1–T4 tracing *platform*.
- Problem 2's durable external queue (only under a must-not-lose policy, through
  the `push.Sink` seam).

## Implementation entry gate (the Problem-1 code change stays blocked until
closed)

1. **Root cause + fix recorded and converged** (this PRD + facts-and-model +
   design-and-position): Budva does not own the acquisition boundary (never
   `OpenChat`s); fix = `OpenChat` each channel/supergroup source on its own
   session and keep it open; private/basic-group ride the global stream. Closed
   by three-layer evidence — **no owner-run experiment required**.
2. **Source-confirms resolved against the pinned source (2026-11-01, no
   experiment):** (a) **idle-open delivery = unconditional re-poll** — the
   re-arm `if (timeout > 0 && d->open_count > 0)` is on the *success* path of
   `on_get_channel_difference` (`MessagesManager.cpp:37423`) with **no gap
   check**, so an idle-but-open channel stays near-real-time → **fix form is
   literally "one `OpenChat` + keep open", no re-poll nudge**; (b) **>10 =
   unbounded FIFO pending queue, deferred never dropped** (`:36854/:36860`);
   (c) **member-channel idle delivery = keep the open set = all sources, do not
   shrink** (the spec's passive member delivery is "usually" / "may stop", not
   a guarantee; engine gate `:7206` treats `was_opened` and `is_member`
   separately). These fix the fix shape.
3. **Lifecycle + stagger guardrails designed** (re-auth re-open, hot-reload
   add/remove, limiter-staggered initial batch) against the seams at
   `cmd/engine/main.go:168` and `main.go:183-196`.
4. **Owner go-ahead** to implement (a decision, **not** an input): no owner-run
   experiment, no Problem-2 contract, and no source-chat-type answer are
   required to proceed.

Closing this needs the lifecycle/stagger design + a small in-repo open-set
regression test + the owner's go-ahead (the source-confirms are already
resolved) — **not** a production collection, **not** an owner experiment, and
**not** a platform. Problem 2's R0/R3/R4 gate is reached
**separately, only if/when** that leg is pursued.

## Notes

- Research + design land first (this phase): `research/facts-and-model.md`
  (facts + converged root cause + three-layer evidence + conformance assessment
  + corrections), `research/design-and-position.md` (converged Problem-1 design
  + remaining source-confirms + the downgraded Problem-2 leg), the Team B
  reports (scope below). Implementation is a later gate behind `task.py start`
  and the entry gate above.
- Baseline: code `main` @ `415d90c` (post PR #7, v0.3.1); record reset to
  `bb0519b` (2026-11-01).
- **Convergence (2026-11-01):** supersedes the earlier "Model correction 1
  (two-problem, (B1)/(B2)/(D) candidates, R0p active probe)" and "Model
  correction 2 (open-chat trigger, B1a–B1d, boundary-localization experiment)"
  framings. The open-chat coupling was **real**; what is now **converged** is
  its **mechanism** (Budva never `OpenChat`s → not an active acquirer for
  open-state-gated classes), closed by the three-layer evidence. The boundary
  experiment and the "owner-supplied source chat type as the decisive fork" are
  **withdrawn** — the type is **diagnostic only** and **no owner input blocks
  the Problem-1 fix.**
- **Corrections recorded in facts-and-model (honesty):** (1) "ChannelPoller.cpp"
  was invented (does not exist) — the poller is `MessagesManager`; (2) "go-tdlib
  has no `GetChannelDifference` wrapper → hand-build a raw `Send` poller" was the
  wrong direction (go-tdlib is a CGO shell over the C++ TDlib, which already
  polls); (3) the "open-once-keep is the most client-like N-open behavior"
  over-claim is corrected to a three-layer conformance assessment (keep-open is
  *a* client behavior / an unclosed tab, not the *typical* phone-user behavior).
- **Team B citation scope.** `team-b-webhook-latency-follow-up.md` — Problem-2
  **mechanism** corroboration only. `team-b-team-a-report-audit.md` — **CHANGES
  REQUESTED**, corrections applied. `team-b-tdlib-open-chat-ground-fact-check.md`
  — **CHANGES REQUESTED** (2026-10-30), kept **verbatim**; originated the
  open-chat framing and B1 decomposition; the three-layer evidence in
  facts-and-model **supersedes** its open candidate set with the converged
  mechanism. All cited as ground-fact / mechanism corroboration, **not** design
  approval.
