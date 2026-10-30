# Design & position — webhook delivery latency (10-30)

Design space and the chosen direction for task
`10-30-webhook-delivery-latency`. Companion to
`research/facts-and-model.md` (the facts this builds on). No code yet — this
phase lands the design; implementation is a later gate.

> **Evidence boundary.** Two distinct problems, two evidence routes:
>
> 1. **The 10-minute total-silence window** (the owner's headline symptom):
>    the serial per-endpoint worker is **not** the leading explanation — with
>    `timeoutSeconds: 60` it can contribute at most a ≤60 s gap, and it emits
>    a request at least every 60 s under load. The signature (zero requests
>    for ≥10 min, covering **text as well as media**, self-recovering) is an
>    **absence of `SubmitV2` calls**, pointing at **(B1) an update-stream gap,
>    (B2) a transient Run-loop stall, or (D) a ruleset hot-reload gap** — none
>    yet proven. Evidence = the owner's **active probe protocol** (Phase 0p):
>    plain-text probe, window declared at ≥2 min absence, SIGQUIT dump +
>    in-window log grep — a one-shot forensic (the 10-29 method), not a
>    timing campaign.
> 2. **Steady-state per-message delay** (endpoint HOL + album deferral): the
>    serial per-endpoint worker + cap-256 buffer + single-attempt sink
>    (occupied until the HTTP response returns) is a source-confirmed
>    mechanism, a **≤60 s worst-case contributor** at current config,
>    reproduced and bounded deterministically by the Phase 0 **controlled
>    slow-receiver test** — not by production timing collection.
>
> Nothing in this document authorizes implementation until the entry gate in
> the last section is closed.

## Guiding constraints (owner-stated)

1. **Producer-side only.** The webhook receiver is a black box. No receiver
   change from this task; **fast-ack** (validate/enqueue/2xx, process async)
   and **idempotent ingest** (de-dup by event id) are *receiver-side contracts*
   this design depends on, not implements.
2. **Native / no reinventing the wheel; minimize maintenance surface.** Prefer
   standard Go + already-present abstractions over a new bespoke service. A
   behavior that leaves this repo also leaves this repo's CI coverage — a real
   maintenance cost, not an aesthetic one.

## The decision axes

| axis | options |
|---|---|
| Where the queue lives | **A** external queue microservice · **B′** local outbox + external reader · **C** all in-budva (outbox+retry) — *rejected: "thickens" budva* |
| First-hop semantics | unacked (fire-and-forget, droppable) vs acked append (at-least-once, local buffer 1–2 ms) |
| Who owns retry / ordering / observability | queue side (Budva only appends) |

## Positioning: two problems, two evidence routes — proportional, not a platform

**Problem 1 — the 10-minute total-silence window (headline symptom).**
This is not a queue-draining story: a serial endpoint worker emits a request
at least every `timeoutSeconds` (=60 s) while it has work, so the worst
endpoint-side gap is **bounded by ~60 s**. A zero-request window of ≥10 min
that also covers *text* (which is submitted immediately in the Run loop,
`service.go:352`, and never passes the shared task queue) is an **absence of
submissions upstream of the endpoint worker**. The candidate set is therefore
**(B1)** the Budva↔Telegram update stream delivered nothing for ~10 min
(self-recovering MTProto/session/connection stall), **(B2)** the Run loop
(the sole `Updates()` consumer) transiently stalled on a synchronous call, or
**(D)** a ruleset hot-reload gap (fsnotify reload of the bind-mounted
`ruleset.yml` retiring the push generation mid-window; log signatures
`Ruleset reloaded` / `Failed to reload` / `Push submission rejected ...
retired`). The fix direction is **"find and fix the gap"** — plus, if the
dump confirms (B1), session-health observability/reconnect — **not** a
delivery-rate change. Evidence = one live capture of the window (active probe
protocol, Phase 0p), the cheapest and most decisive test available.

**Problem 2 — steady-state per-message delay (the ≤60 s contributor).**
The serial per-endpoint worker + backlog is *conditionally stable* G/G/1: a
burst drains when the arrival rate is below the service rate (finite,
self-recovering delay); wait grows **linearly** in the service times ahead
(bounded by `timeout`), not exponentially. The production backlog size and the
dominant interval (`T2−T3` queue wait vs `T3−T4` HTTP/ingress) are not
established from production data — and **do not need to be**: the dominant
interval is **reproduced deterministically by a controlled slow-receiver test
(Phase 0)**. The warranted response is *proportional*: (1) reproduce and bound
the queue-wait mechanism in a unit test against a slow-receiver stub, then
(2) bound per-request occupancy and raise service rate by a *test-justified,
configurable* amount, in-repo — not a new always-on queueing platform.

A durable outbox is justified **only** by an explicit must-not-lose policy,
not by latency alone — and it does not address Problem 1 at all.

## Receiver-throughput effect (conditional, not absolute)

Whether Budva-side concurrency and a receiver fast-ack actually raise
end-to-end throughput depends on the receiver's own processing model:

- If the **receiver can process requests concurrently**, bounded producer
  concurrency can raise webhook-ingress and end-to-end throughput, and
  fast-ack (short HTTP occupancy) directly raises the Budva worker's service
  rate even if downstream business throughput is unchanged.
- If the **receiver (or its downstream dependency) is strictly serialized or
  saturated** at some capacity, producer concurrency mainly *moves the
  backlog* and can add contention; final business-processing throughput is
  then bounded by the receiver, not by Budva.
- An **external durable queue** generally does **not** raise final
  business-processing throughput, but it does absorb bursts, isolate producer
  availability from receiver outages, preserve events, control retries, and
  support replay — reliability/decoupling benefits, separate from
  latency containment.

So "the receiver is slow, therefore Budva concurrency is pointless" is only
true in the serialized/saturated case. The Phase 0 test (which interval the
Budva-side delay lands in) and the Phase 1 receiver contract (fast-ack?
concurrent? what ceiling?) decide which of these cases applies.

## Sequencing (evidence-ordered) — no heavyweight platform

The levers are real but must be applied in an order that (1) **reproduces and
bounds the mechanism before the concurrency boundary changes**, and (2) does
**not** preset parameters that require contracts to be chosen.

> **The evidence is a controlled test, not production collection.** The
> dominant interval and the queueing behavior are established **by unit/
> contract tests against a controlled slow-receiver stub** — deterministic,
> reproducible, race-clean, and run in CI. Production-side collection of
> timing (capturing a live delayed event, per-request timing logs from the box)
> is **not** a gate: it is non-reproducible, invasive, and the delayed
> occurrence is the very thing being fixed. Producer-side observability
> (queue depth, oldest age, enqueue→pickup duration) ships as a **post-deploy
> verification aid** (and a cheap production dead-man's-switch), not as a
> pre-implementation prerequisite. This is the deterministic equivalent of
> "capturing one representative delayed occurrence": the stub *is* the
> controlled occurrence.

### Phase 0p — capture the silence window once (active probe; the PRIMARY
evidence for the headline symptom)

The **owner runs this on the box** (the agent has no box access); it is
documented step-by-step in `research/facts-and-model.md` and costs one
message + one log capture + one SIGQUIT per run:

1. **Probe**: a **plain text** message with a unique token into a ruleset
   source chat (text required — it is the shortest path, immediate Run-loop
   `SubmitV2`; albums would confound via the quiet window / shared queue).
2. **Declare** the window at the **≥2-minute probe-absence** boundary
   (real windows are ≥10 min; the normal worst case is 60 s — so the boundary
   has ~no false positives and beats waiting for a natural window).
3. **While the stall is live**: capture engine logs since window start, then
   `kill -QUIT 1` (the Go goroutine dump lands on stderr via `docker logs`;
   `restart: always` revives the process — expected, same as 10-29, and the
   delivery resuming on restart re-confirms self-recovery).
4. **Read the dump + logs**: Run-loop frame parked in `<-updates` with an
   empty channel → (B1) (then inspect the go-tdlib receiver / session
   goroutines); parked in a concrete call frame → (B2) (the frame names the
   blocker); in-window log grep `Ruleset reloaded` / `Failed to reload` →
   (D); `buffer_full` WARN expected **absent** (a full buffer is the
   slow-continuous signature, not a silence). The probe token's catch-up
   arrival time measures the silence duration.

This is a **one-shot forensic, not a timing campaign**: a single run yields a
decisive frame attribution. It is *production-side* only in the sense that it
happens on the box; it captures no continuous metrics and ships no
instrumentation. **Outcome determines the fix direction** (session health /
Run-loop blocker / reload handling — vs endpoint tuning).

### Phase 0 — reproduce + bound the mechanism (a GATE, by controlled test)

In-repo tests (`internal/service/push`, no production traffic):
- **serial baseline reproduction:** a `WebhookSink` (or a test sink) backed by a
  **slow-receiver stub** that sleeps `T` before responding 2xx; enqueue N
  events on the current single-worker endpoint and assert the queue-wait
  (`T3−T2`) grows **linearly** to ≈ `(N−1)·T` — reproducing the observed
  finite minute-scale delay from the confirmed mechanism. This both confirms
  the mechanism is real and **locates the dominant interval** (`T2−T3` grows;
  `T3−T4` is held constant by the stub, so any delay beyond `(N−1)·T` is
  attributable to ingress, not the queue).
- **bounded, reproducible bounds:** the test asserts the resulting
  queue-wait P-max / oldest-age against an explicit budget (a **measurable
  acceptance**, not the `(N/concurrency)×T` planning formula).
- **the fix is proven in the same suite:** with the concurrency/timeout change
  applied, the same N-event burst against the same slow stub shows queue-wait
  and oldest age **within budget**, in-flight ≤ configured concurrency, and
  zero unexpected `buffer_full`.

The stub makes the test deterministic: fixed `T`, fixed N, no network, no
production data. It is the evidence that the design changes the right interval
and by a bounded amount — **before** that change is merged.

### Phase 0b — post-deploy verification aid (ships with the change, not a gate)

Producer-side observability for the deployed box (in-repo, small):
- stamp **enqueue time**; record **worker-pickup time** (queue-wait `T3−T2`);
- expose **current queue depth** and **oldest queued-event age** per endpoint
  (alongside the existing `enqueued / full / failed / duration / lastSuccess`
  metrics);
- retain a stable **event/message correlation key** (chat + message + event
  type) in structured logs — no message content, no secrets.

Correlation with the receiver is **free** in the common case: the payload
already carries `OccurredAt` (= `T2`, stamped at `SubmitV2`), so the receiver
can compute arrival-minus-`OccurredAt` (= `T2→T4`) with zero Budva change. This
phase is what lets the owner *verify after deploy* that the box's real `T` and
arrival rate keep the queue-wait in budget — and provides the missing
dead-man's-switch on last delivery the 10-29 record flagged. It is **not**
required to land the change.

### Phase 1 — settle the delivery contracts (owner decisions; preconditions)

Record **before** implementation:
- **loss policy:** best-effort (droppable) vs must-not-lose;
- **duplicate policy:** are duplicate deliveries acceptable (drives the
  receiver idempotency requirement and any retry);
- **ordering scope:** global / per-endpoint / per-chat / none — this decides
  the worker topology (worker pool vs chat partitioning);
- **receiver fast-ack:** can the receiver validate/enqueue/2xx and process
  async?
- **receiver idempotent ingest:** can the receiver de-dup by event id?
- **receiver-supported concurrency:** the ceiling for the producer
  concurrency.

### Phase 2 — choose parameters and topology (test-justified + configurable,
not preset, and not production-measured)

- **timeout:** chosen from the **receiver's response time** — the single
  black-box number the **owner provides** (one `curl -w '%{time_total}' -X
  POST` at the endpoint, or the receiver's own access log; **not** a
  production measurement campaign) — plus delivery semantics, an acceptable
  failure budget, an explicit safety margin, and the loss policy. In
  **best-effort** mode, document the latency-versus-loss tradeoff — under
  at-most-once semantics a shorter timeout *is* a loss knob (it classifies the
  slow tail as failed and drops it). In **must-not-lose** mode, timeout tuning
  alone is insufficient — durable retry + idempotency are required. **No
  concrete value is set until the receiver's response behavior and the loss
  policy are recorded.** A mechanical "set to P95/P99" rule is **rejected**:
  under at-most-once delivery it would silently drop the slow tail.
- **concurrency:** configurable; the initial value is a **small, bounded
  number justified by the Phase 0 test budget** (the smallest N such that the
  N-event burst against the slow stub holds queue-wait / oldest-age in
  budget) and by the receiver-supported-concurrency contract — **not a fixed
  undocumented default and not derived from production arrival-rate
  measurement** (a low-volume forwarder's arrival rate is low and
  non-reproducible to collect; it is revisited later via the Phase 0b
  post-deploy metrics if saturation ever shows up).
- **topology:** unordered worker pool (if the ordering scope is none, or the
  receiver is idempotent and order-independent) vs **stable chat
  partitioning** (`hash(chat_id) % N`, one serial consumer per partition) if
  same-chat order is part of the contract.
- **buffer-full behavior** and meaningful **latency/saturation alerts**
  (from the Phase 0b metrics) defined.

### Phase 3 — implement and validate

- bounded per-endpoint concurrency (native Go: the endpoint channel consumed
  by N goroutines, or N chat partitions);
- the **Phase 0 controlled slow-receiver tests** (baseline reproduction +
  bounded post-fix) merged as the regression suite, plus: **ordering tests
  for NEW/EDIT/DELETE sequences**; generation retirement + goroutine-lifecycle
  tests; buffer-full behavior; timeout + cancellation behavior; **race testing
  of the dispatcher change**; the unchanged 10-29 task-queue regression tests;
  TDLib-provisioned CI for handler tests;
- the **Phase 0b observability** (queue depth, oldest age, enqueue→pickup)
  shipped alongside, for post-deploy verification.

### Phase 4 — reliable delivery, only if must-not-lose

Only if the Phase 1 policy is must-not-lose: durable append before
acknowledging producer acceptance, a stable idempotency key, retry
classification + exponential backoff with jitter, `Retry-After` support, a
DLQ, inspection/replay, and receiver-side duplicate suppression. The durable
queue, if needed, is an **off-the-shelf wheel evaluated as a decision** (e.g.
NATS JetStream or Redis Stream) — *not pre-approved by name* — wired through
the existing `push.Sink` seam
(`internal/service/push/dispatcher.go:32-36`) so the Budva-side diff stays
small: a `queueSink` does one acked `append` (ms, returns immediately);
`WebhookSink` is retired or demoted to fallback. First-hop is an **acked
append**, not unreliable UDP (on one host UDP buys no performance and worsens
the loss semantics).

## Excluded (and why)

- **Bespoke in-repo queue microservice** (new `cmd/`, new image, new CI path,
  new failure domain, hand-rolled retry/DLQ) — the most wheel-inventing
  option; violates constraint 2.
- **C (in-budva durable outbox + retry)** — "thickens" Budva with exactly the
  mechanics the owner does not want to own; the must-not-lose need is better
  met by an off-the-shelf durable queue through the `push.Sink` seam.
- **B′ (local outbox + external reader) on its own merits** — buys
  engine-offline resilience but adds a second process *sharing* Budva's store
  plus a **self-invented** claim/mark/de-dup handoff protocol (the highest
  uniqueness / least-reusable part of any option). On the same host with the
  queue in the same compose the offline resilience is moot. B′ only wins if
  the first hop *must* be local.
- **Presetting `concurrency = 4` or `timeout ≈ P95/P99`** — no test budget or
  recorded contract supports those values and the timeout rule is a loss knob
  under at-most-once; both become test-justified, configurable values
  (Phase 2).
- **Production-side timing collection as an implementation gate.** Capturing a
  live delayed event or instrumenting the box before the change is
  non-reproducible, invasive, and the occurrence being fixed is itself
  intermittent. The evidence is the **controlled slow-receiver test** (Phase
  0); production observability is a post-deploy aid (Phase 0b). A full T1–T4
  tracing *platform* is likewise not required. (The Phase 0p **active probe
  is the single exception** and deliberately so: it is a one-shot forensic of
  the *headline symptom* — the 10-minute silence — that no in-repo test can
  reproduce, and it ships no instrumentation and collects no continuous
  metrics.)

## Implementation entry gate (implementation remains blocked until closed)

1. **Evidence boundary corrected:** the 10-minute total-silence window is
   attributed to the candidate set **(B1)/(B2)/(D)** (absence of submissions
   upstream of the endpoint worker; endpoint HOL is a bounded ≤60 s
   contributor, not the explainer), and the endpoint serial worker is held as
   a confirmed mechanism, **not** a proven root cause of the headline
   symptom.
2. **The silence is attributed once (Phase 0p):** the active probe captures
   one live window (probe → declare at ≥2 min → SIGQUIT dump + in-window log
   grep), and the Run-loop frame + log grep select among (B1)/(B2)/(D) — the
   fix direction follows that attribution.
3. **Mechanism reproduced + bounded by a controlled test (Problem 2):** the
   Phase 0 slow-receiver test suite reproduces the queue-wait and shows the
   concurrency/timeout change holds it in budget — **no production timing
   collection required** (the deterministic equivalent of capturing one
   delayed occurrence).
4. **Delivery contracts settled:** loss, duplicate, ordering, fast-ack, and
   idempotency policies are recorded; the receiver's response time `T` is a
   single black-box number the owner supplies (one `curl`/access-log), not a
   measurement campaign.
5. **Parameters test-justified + configurable:** concurrency and timeout are
   configurable and chosen from the Phase 0 test budget and the recorded
   contracts/risk budget.

Closing these needs the one live probe capture, a small in-repo test suite,
and explicit product semantics — not production collection and not a
platform.

## Open owner decisions (the implementation blockers)

1. **Loss policy:** best-effort (droppable) vs must-not-lose?
2. **Duplicate policy:** are duplicate deliveries acceptable?
3. **Ordering scope:** global / per-endpoint / per-chat / none?
4. **Receiver `T` + contracts:** the receiver's per-request response time
   (one `curl -w '%{time_total}' -X POST` or its access log — a single number,
   **not** a production measurement) and whether the receiver can fast-ack +
   idempotently ingest + support what concurrency ceiling?

**No production-side timing collection is required from the owner.** The
evidence for the mechanism and its bound is the in-repo controlled
slow-receiver test (Phase 0); the owner's input is the black-box contract
decisions above plus the single `T` number.

## Team B citation scope (explicit)

`research/team-b-webhook-latency-follow-up.md` independently confirms the
**source mechanism** (serial per-endpoint worker, pre-wire queue wait, single
attempt, occupied-until-response) and the **evidence boundary** (production
causality unproven). It **does not** constitute approval of this task's
parameter values, implementation ordering, product contracts, or final
architecture. It is cited as mechanism corroboration only.

`research/team-b-team-a-report-audit.md` is Team B's audit of this task's
research/design (**CHANGES REQUESTED**); its corrections are applied in the
current revision. One point is resolved **differently, by deliberate design**:
Team B's P1-02 frames the evidence as capturing a representative *production*
delayed occurrence. This task instead uses a **controlled slow-receiver
test** as the gate (deterministic, reproducible, race-clean, CI-runnable) and
demotes production-side observability to a post-deploy aid — production
timing collection is not a realistic or required prerequisite. The evidence
*boundary* Team B asks to preserve (do not claim the production incident is
root-caused; measure the suspected interval before changing the concurrency
boundary) is fully honored: the controlled test both reproduces the mechanism
and bounds the change before it lands.

## Boundary restated

Producer-side only. The receiver is a black box (accepted / rejected(class) /
unreachable). fast-ack and idempotent-ingest are *required of* the receiver,
confirmed by the owner, and out of scope to implement here.
