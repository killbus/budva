# Mitigate webhook delivery latency

## Goal

Bound the **finite, minutes-scale** latency between a Telegram message and its
inbound webhook POST, on the **producer side only**. Observed (2026-10-30, box
running `v0.3.1` / `:latest`): a message visible in the official client is not
seen as an inbound request by the **same-host** receiver until later. The delay
is finite and self-recovering — it is *not* the 10-29 self-deadlock (infinite;
fixed at `415d90c`). **Two distinct problems** (model corrected 2026-10-30
after the owner's total-silence clarification):

1. **The 10-minute zero-request silence window** (headline symptom): it covers
   **text as well as media**, so it is an **absence of `SubmitV2` calls**,
   not slow delivery. With `timeoutSeconds: 60` the serial per-endpoint worker
   can contribute **at most ~60 s** and is **not** the explainer. **Model
   correction 2 (2026-10-30, owner open-chat observation):** the silence is
   *coupled to the official client not having the chat open* — the client and
   Budva are separate sessions, and for channels/supergroups (any size — the
   open-state gating has no protocol threshold; "large" is a maintainer
   paraphrase, see facts-and-model terminology note) Telegram
   does not guarantee real-time delivery (it is gated by chat-open/active
   state); opening the chat triggers a catch-up **burst**. The leading causal
   class is **open-chat-triggered update acquisition (B1a–B1d, cross-session
   passive-update for accessible non-member channels/supergroups)**, with
   **(B2)** transient Run-loop stall, **(B3)** endpoint backlog, and **(D)**
   ruleset hot-reload gap as secondary candidates. Decisive next step = the
   **boundary-localization experiment + one live SIGQUIT dump**, plus the
   owner-supplied **source chat type** (the fork that selects which documented
   mechanism applies). **Not yet proven which mechanism / at which boundary**
   (`research/facts-and-model.md` — Model correction 2;
   `research/team-b-tdlib-open-chat-ground-fact-check.md`).
2. **Steady-state per-message delay**: the serial per-endpoint worker +
   backlog (bounded ≤60 s at current config) — a source-confirmed mechanism
   reproduced and bounded by the Phase 0 **controlled slow-receiver test**
   (R0), not by production timing collection.

Which interval dominates (endpoint queue-wait `T2−T3` vs HTTP/receiver-ingress
`T3−T4`) is **not yet established** and is a R0 prerequisite (`research/
facts-and-model.md`).

## Constraints

- **Producer-side only.** The webhook receiver is a black box (accepted /
  rejected(class) / unreachable). Receiver-side **fast-ack** (validate/enqueue/
  2xx, process async) and **idempotent ingest** (de-dup by event id) are
  *contracts this design depends on*, confirmed by the owner — not implemented
  here.
- **Native / minimal maintenance surface.** No bespoke queue microservice.
  Default design is standard Go inside the existing
  `internal/service/push` package; a durable external queue (an off-the-shelf
  wheel **evaluated as a decision**, e.g. NATS JetStream or Redis Stream) is in
  scope **only** under a must-not-lose loss policy.
- **Behavior stays in-repo and under existing CI** (vet + test + the
  TDLib-provisioned handler tests).
- **Evidence boundary.** The serial-worker mechanism is a **confirmed
  mechanism and a ≤60 s bounded contributor** for steady-state delay — **not**
  a proven root cause of the headline 10-minute silence (the candidate set
  there is (B1)/(B2)/(D), R0p). Until the entry gate is closed, the silence
  fix direction follows the probe's frame attribution and the steady-state
  concurrency/timeout changes stay **blocked** behind R0. **Production-side
  timing collection is not required and is not used** as a gate (non-
  reproducible, invasive); the deterministic test is the evidence for Problem
  2, and the one-shot **active probe (R0p)** is the single sanctioned
  production-side forensic for Problem 1 (no instrumentation, no continuous
  metrics).

## Requirements

- **R0p — Attribute the silence window once (active probe; PRIMARY for the
  headline symptom).** The owner runs the one-shot probe on the box
  (documented in `research/facts-and-model.md`): plain-text probe with a
  unique token; declare the window at the **≥2-minute probe-absence** boundary
  (real windows ≥10 min; normal worst case 60 s — clean boundary, ~no false
  positives); while the stall is live, capture engine logs since window start
  + `kill -QUIT 1` (goroutine dump on stderr; `restart: always` revives the
  process — expected, same as 10-29, and delivery resuming re-confirms
  self-recovery); record the probe token's catch-up arrival (measured silence
  duration). The Run-loop dump frame + in-window log grep
  (`Ruleset reloaded` / `Failed to reload` / `buffer_full`-absent) select
  among **(B1)** update-stream gap / **(B2)** Run-loop stall / **(D)** reload
  gap. One-shot forensic — **no instrumentation shipped, no continuous
  metrics**. The fix direction for Problem 1 follows this attribution.
- **R0 — Reproduce + bound the mechanism by a controlled test (a GATE before
  behavior change).** In-repo unit/contract tests in
  `internal/service/push` against a **slow-receiver stub** (fixed response
  time `T`): a serial-baseline test reproduces the queue-wait (`T3−T2`)
  growing linearly for N enqueued events (the observed finite delay), and a
  post-fix test shows the same burst held within budget under the
  concurrency/timeout change. **No production timing collection is required**
  — the deterministic test is the evidence (and a stronger one than capturing
  a live delayed occurrence). Post-deploy producer-side observability (queue
  depth, oldest age, enqueue→pickup) ships as a **verification aid** (R0b),
  not a prerequisite.
- **R0b — Post-deploy verification aid (not a gate).** Expose per endpoint:
  queue depth, oldest queued-event age, and enqueue→pickup duration
  (alongside the existing `enqueued / full / failed / duration / lastSuccess`
  metrics); retain a stable event/message correlation key. The payload's
  `OccurredAt` (= `T2`) already lets the receiver compute arrival-minus-
  `OccurredAt` for free. This is what verifies the box's real `T`/arrival
  rate keep queue-wait in budget after deploy, and the missing
  dead-man's-switch on last delivery.
- **R1 — Bound worst-case per-request occupancy (test-justified,
  configurable).** The per-endpoint delivery timeout is chosen from the
  **receiver's response time** — a single black-box number the owner supplies
  (one `curl -w '%{time_total}' -X POST` or the receiver's access log, **not**
  a production measurement campaign) — plus delivery semantics, an acceptable
  failure budget, an explicit safety margin, and the loss policy; the
  per-endpoint `timeoutSeconds` rule field (`internal/domain/ruleset.go`) is
  the mechanism. A mechanical "set to P95/P99" rule is **rejected**: under the
  current at-most-once semantics a shorter timeout is a *loss knob* (it
  classifies the slow tail as failed and drops it). **No concrete value is set
  until the receiver's response behavior and the loss policy (R4) are
  recorded.**
- **R2 — Raise per-endpoint service rate (test-justified, configurable).** A
  bounded, **configurable** concurrency per endpoint replaces the single serial
  worker in `internal/service/push/dispatcher.go`. The initial value is a
  **small bounded number justified by the R0 test budget** (the smallest N
  such that the N-event burst against the slow stub holds queue-wait / oldest
  age in budget) and the receiver-supported-concurrency contract — **not a
  preset default (4 is not assumed) and not from production arrival-rate
  measurement** (revisited later via R0b metrics if saturation appears). The
  worker **topology** is chosen from the settled ordering scope (R3): unordered
  worker pool vs stable chat partitioning (`hash(chat_id) % N`, one serial
  consumer per partition).
- **R3 — Ordering + delivery contracts are settled preconditions.** Before
  implementation, record: loss policy (best-effort vs must-not-lose), duplicate
  policy, ordering scope (global / per-endpoint / per-chat / none), receiver
  fast-ack capability, receiver idempotent-ingest capability, and
  receiver-supported concurrency. The worker topology and acceptance thresholds
  must match these.
- **R4 — Loss policy is explicit and settled in this task (owner decision).**
  Best-effort (in-memory, droppable) vs must-not-lose (durable queue + retry +
  DLQ + idempotent event id). The implementation must match whichever is
  chosen; the loss policy is an **implementation blocker** (see R0/R3 gate).
- **R5 — No regression of the 10-29 fix.** The task-queue lock release
  (`415d90c`) is untouched; the concurrency change applies only to the
  per-endpoint delivery worker, not the shared task queue (unless a sub-task
  scopes it and records it).

## Acceptance criteria

- [ ] **A1 (measurable, not a formula):** Under a **controlled slow-receiver
      stub** with defined response behavior, on the endpoint worker pool:
      queue-wait (`T3−T2`) P95/P99 and max are within the recorded budget;
      oldest queued-event age is within budget; in-flight request count never
      exceeds the configured concurrency; **zero unexpected `buffer_full`
      outcomes** under the test workload; and receiver-arrival minus enqueue
      time is bounded. Separate thresholds are defined for **unordered** vs
      **chat-partitioned** modes. (The `(N/concurrency)×T` capacity formula is
      a *planning estimate* only, not an acceptance guarantee.)
- [ ] **A2:** Ordering tests for **NEW, EDIT, DELETE** event sequences pass in
      the selected topology (same-chat order preserved under
      chat-partitioned mode; documented relaxed order under unordered mode).
- [ ] **A3:** Generation retirement + goroutine-lifecycle tests pass; **no
      goroutine leak** in the new worker pool; buffer-full, timeout, and
      cancellation behavior are correct.
- [ ] **A4:** The change passes `go vet ./...` and `go test ./internal/...`
      green in CI (including the TDLib-provisioned handler package); **no new
      data race** in the queue/dispatcher packages under `-race`.
- [ ] **A5:** The delivery contracts (R3: loss, duplicate, ordering, fast-ack,
      idempotency) and the loss policy (R4) are **recorded** in this task's
      record, and the implementation matches them (best-effort: bounded buffer
      + drop-with-WARN preserved; must-not-lose: durable queue seam present).
- [ ] **A6:** The 10-29 task-queue regression tests still pass; shared task
      queue behavior unchanged unless explicitly scoped in a sub-task.
- [ ] **A7:** Receiver-side contract (fast-ack + idempotent ingest) is either
      confirmed by the owner in this record or flagged as the remaining
      external dependency.

## Out of scope

- Receiver (black box) implementation, ruleset content, target-side changes.
- **Production-side timing collection** (instrumenting the box before the
  change, continuous per-request metrics) — non-reproducible and invasive; the
  evidence for Problem 2 is the in-repo **controlled slow-receiver test**
  (R0), and post-deploy observability (R0b) is a verification aid, not a gate.
  (The R0p **active probe** is the single exception for Problem 1: a one-shot
  forensic capture of the silence window — it ships no instrumentation and
  collects no continuous metrics.)
- The shared 1-task/second task queue's redesign (a separate hardening item
  from the 10-29 residual list; may be carved into a sub-task if R2's scope
  needs it).
- A full T1–T4 tracing *platform* — R0 is a small in-repo test suite that
  closes the evidence boundary, not a heavyweight system.

## Implementation entry gate (implementation stays blocked until closed)

1. Evidence boundary corrected: the silence is attributed to the candidate
   set (B1)/(B2)/(D); the endpoint serial worker is a confirmed mechanism and
   bounded ≤60 s contributor, **not** a proven root cause of the headline
   symptom.
2. **Silence attributed (R0p / Model correction 2):** the **boundary-
   localization experiment** (contrast A list-only / B open-after-activity /
   C client-closed, chat types separated, boundary timestamps + **one SIGQUIT
   dump while the symptom is active**) localizes *at which boundary* the
   trigger releases the updates. The **source chat type** is diagnostic (which
   documented mechanism explains the coupling; a silence on a private/basic-
   group = a separate B1a/B2/D bug). The Problem 1 fix is the **type-agnostic
   active-subscriber** design (owner requirement: support **every** source
   chat type — `OpenChat` on all ruleset source chats in Budva's own session;
   a no-op for private/basic-group, the real fix for channels/supergroups),
   confirmed by that attribution.
3. Mechanism **reproduced and bounded by the controlled slow-receiver test**
   (R0) — **no production timing collection required**.
4. Delivery contracts settled (R3) + loss policy recorded (R4); the receiver's
   response time `T` is a single owner-supplied number (one `curl`/access-log),
   not a measurement campaign.
5. Concurrency + timeout are configurable and **test-justified** (R1, R2).

## Notes

- Research + design land first (this phase): `research/facts-and-model.md`
  (facts + G/G/1 model + evidence boundary), `research/design-and-position.md`
  (design space + evidence-ordered sequence), `research/team-b-
  webhook-latency-follow-up.md` (independent **mechanism** audit — corroboration
  only, not design approval), `research/team-b-team-a-report-audit.md`
  (independent audit of this task's research/design — **CHANGES REQUESTED**;
  corrections applied in this revision). Implementation is a later gate behind
  `task.py start` and the entry gate above.
- Baseline: `main` @ `415d90c` (post PR #7, v0.3.1).
- **Model correction (2026-10-30, owner total-silence clarification):** the
  symptom is a **10-minute zero-request window covering text and media** with
  `timeoutSeconds: 60`. This rules out endpoint HOL alone (≤60 s cap, emits at
  least every 60 s) and a single-request wedge (>60 s impossible), and rules
  out album-queue starvation *alone* (text pushes immediately in the Run
  loop). The leading frame is now an **upstream absence of submissions**:
  (B1) update-stream gap / (B2) Run-loop transient stall / (D) ruleset
  hot-reload gap — attributed by the R0p active probe; (A) album deferral and
  endpoint HOL are secondary ≤60 s contributors.
- **Model correction 2 (2026-10-30, owner open-chat observation + Team B
  ground-fact check):** the silence is *coupled to the official client not
  having the chat open* — the client and Budva are **separate sessions**, and
  for **channels/supergroups** (any size — the open-state gating has no
  protocol threshold; "large" is a maintainer paraphrase, see facts-and-model
  terminology note) Telegram does not guarantee real-time
  delivery (gated by chat-open/active state); opening the chat triggers a
  catch-up **burst** (difference recovery). The leading causal class is
  **open-chat-triggered update acquisition (B1a–B1d)** — including the
  *documented* cross-session passive-update effect for accessible non-member
  channels/supergroups — not the downstream worker. The fix direction shifts
  to **making Budva an active subscriber** (`OpenChat` on the ruleset source
  chats; native, no new component) — *pending* the boundary-localization
  experiment and the owner-supplied source chat type. (B2) Run-loop stall,
  (B3) endpoint backlog, (D) reload remain secondary. See
  `research/facts-and-model.md` (Model correction 2) and
  `research/team-b-tdlib-open-chat-ground-fact-check.md`.
