# Design & position — webhook delivery latency (10-30)

Design space and the chosen direction for task `10-30-webhook-delivery-latency`.
Companion to `research/facts-and-model.md` (the facts and the converged root
cause this builds on). **No code yet** — this phase lands the design;
implementation is a later gate. Status (2026-11-01): **the Problem-1 root
cause and fix direction are converged**; the record was reset off main
(`bb0519b`) and lives on branch `docs/10-30-type-agnostic-requirement` pending
owner sign-off.

> **Evidence boundary (corrected 2026-11-01).** Two distinct problems, two
> routes — and they are **not** at the same stage:
>
> 1. **The 10-minute total-silence window (headline) — CONVERGED.** The root
>    cause is **not** the serial per-endpoint worker (with `timeoutSeconds: 60`
>    that is a ≤60 s contributor and emits a request at least every 60 s under
>    load — it cannot produce a 10-minute *zero-request* window). The silence
>    is **coupled to the official client not having the chat open** (client and
>    Budva are separate sessions). The root cause is now **converged**: Budva
>    does **not own the acquisition boundary** of its ruleset sources — it is a
>    passive subscriber that **never `OpenChat`s**, so for open-state-gated
>    classes (channels/supergroups) its session is not an active acquirer and
>    goes silent until *some* session opens the chat. This is closed by
>    **three-layer evidence** (the normative spec, the pinned TDlib engine
>    source at the exact commit go-tdlib pins, and the go-tdlib CGO
>    architecture) — **not** by an owner-run boundary experiment (withdrawn,
>    see below). The **fix is converged**: `OpenChat` each channel/supergroup
>    source on Budva's own session and keep it open; the in-process engine then
>    owns gap recovery + periodic re-poll.
> 2. **Steady-state per-message delay (Problem 2) — DOWNGRADED to "potential,
>    unobserved."** The serial per-endpoint worker + cap-256 buffer +
>    single-attempt sink is a real code property but there is **no
>    owner-observed per-message queueing delay**, so per the "does the problem
>    even hold" criterion it is a *potential* contributor, verified **if** such
>    delay is ever seen (a controlled slow-receiver test, not production timing
>    collection). It is a **separate leg** from Problem 1 and **does not gate
>    the Problem-1 fix.**

> Nothing in this document authorizes the Problem-1 **code change** until the
> entry gate (last section) is closed — the source-confirms are **already
> resolved**, so it now requires only the lifecycle/stagger design plus the
> owner's go-ahead, **not** any owner-run experiment and **not** the Problem-2
> contract decisions.

## Guiding constraints (owner-stated)

1. **Producer-side only.** The webhook receiver is a black box. No receiver
   change from this task; **fast-ack** and **idempotent ingest** are
   *receiver-side contracts* this design depends on (Problem 2), not
   implements.
2. **Native / no reinventing the wheel; minimize maintenance surface.** Prefer
   standard Go + already-present abstractions over a new bespoke service. A
   behavior that leaves this repo also leaves this repo's CI coverage — a real
   maintenance cost.
3. **Conformance discipline.** Every new behavior must be one an official
   client performs, at a client-plausible volume (see facts-and-model
   "Conformance assessment").

## The converged Problem-1 design: own the acquisition boundary

**Root cause (converged).** Budva's session never `OpenChat`s any source chat,
so it holds no open state for its ruleset sources. For channels/supergroups
(open-state-gated, any size — no protocol threshold) that means its session is
not an **active acquirer**: gap recovery and the periodic `getChannelDifference`
re-poll in the pinned engine are gated on open/member state, and a never-opened,
non-member channel is dropped rather than recovered. Updates reach Budva's
session only when *another* session (e.g. the owner's client) opens the chat and
starts polling (cross-session passive updates) — which is exactly the
open→burst / close→silence the owner observed.

**Fix (converged).** For **each channel/supergroup source chat in the ruleset**,
call `OpenChat` **on Budva's own session** and **keep it open** (no `CloseChat`
except on ruleset removal). One call makes Budva's session an active acquirer;
the in-process engine (already running via go-tdlib's CGO linkage) then owns
gap recovery and the open-gated periodic re-poll, so updates arrive
**independently of the official client**.

- **Open once, keep — no periodic re-open.** The engine's re-poll self-continues
  while `open_count > 0`; `OpenChat` establishes state, it is not a token to
  refresh. Re-open fires only at **state-reset points**: after **re-auth**
  (dialogs reset) and after **process restart** (`open_count` is in-process).
  No heartbeat, no toggle (toggle churn is the non-human signature and
  re-introduces the deferred state = the bug).
- **Keep ALL sources open — one `OpenChat` per source, not one held at a time.**
  Acquisition is per-channel state; rotation/closing drops that channel's
  acquirer and re-introduces its silence. (The engine itself caps simultaneous
  diff polls at 10; see the scale note.)
- **Private/basic-group sources are NOT `OpenChat`'d.** They are not
  open-state-gated (global update stream, common message-ID sequence +
  `getDifference`), and a client does not open every chat. "Support every type"
  = one **intent** (own the acquisition boundary) realized by each class's own
  **client-conformant mechanism**: channels/supergroups via `OpenChat`,
  private/basic-group via the global stream. The chat **type is diagnostic only**
  — it selects which mechanism, and each mechanism is a known fact, so the
  current production type does **not** block the design.
- **Native, no new component, no hand-built poller.** A first-class TDlib API;
  the engine already polls.

**Scale fact (engine-enforced, Budva need not write it):** the engine caps
**simultaneous** channel-difference polls at 10, re-polls at the **server-
returned** `timeout` (server-paced), and emits no open/close churn. With >10
channel/supergroup sources the engine queues them (the 11th waits), so **not all
sources are strictly real-time at very large N** — an inherent trade-off to note,
not a reason to keep only one open.

### Conformance guardrails (the client-plausible part)

(1) open **only ruleset source chats**, never the account's other chats; (2)
**stagger** the initial/re-auth `OpenChat` batch (reuse
`internal/service/limiter`) — no N `openChat` in the same instant; (3)
**open-once-keep, no periodic toggle**; (4) **do not fight the engine's 10
cap**. View/active-count contribution is **inherent** (the footprint is the
fix); read receipts are **conformed** (Budva never calls a read API); compliance
is **dissolved** (standard user RPCs; a user-account daemon is the product's
pre-existing architecture). Full three-layer assessment in facts-and-model.

## Problem 1 — source-confirms RESOLVED (2026-11-01); then lifecycle design, then implement

The three source-confirms are **closed by reading the pinned source** (no
experiment). They fix the fix shape:

1. **Idle-open delivery vs gap-only — RESOLVED: unconditional re-poll.** The
   re-arm is on the **success** path of `on_get_channel_difference`:
   `if (timeout > 0 && d->open_count > 0) add_timeout_in(dialog_id, timeout)`
   (`:37423`), and the timer's handler `on_channel_get_difference_timeout`
   (`:36707`) fires `get_channel_difference(dialog_id, d->pts, ...)` with **no
   gap check**. An idle-but-open channel keeps polling at the server-returned
   `timeout`. **Fix form = literally "one `OpenChat` + keep open"; no re-poll
   nudge.**
2. **>10 behavior — RESOLVED: unbounded FIFO pending queue, no loss, no
   starvation.** `do_get_channel_difference` pushes to
   `pending_get_channel_differences_` (`:36854`); drained only while
   `count < 10` (`:36860-36863`); each completion re-drains; one outstanding
   request per chat. With >10 open sources: 10 in flight, the rest rotate at
   their own server `timeout`; a large-backlog channel re-queues its next page
   immediately (per-page FIFO fairness). The 11th+ is **deferred, never
   dropped**.
3. **Member-channel idle delivery — RESOLVED (decision): keep the open set =
   ALL channel/supergroup sources; do not shrink to non-member only.** The spec
   sends passive member-channel updates "usually" but "in some cases ... may
   stop (or send fewer)" — **not a guarantee**; relying on idle member delivery
   would re-introduce the silence. The engine's gap-recovery gate (`:7206`)
   treats `was_opened` and `is_member` as separate sufficiency conditions. The
   saved volume is small vs the risk. **Set = all sources**; a member reduction
   is a later optimization requiring production evidence, not preset.

**Then, before coding:** lifecycle + stagger design (re-auth re-open, hot-reload
add/remove, limiter-staggered initial batch) against the seams at
`cmd/engine/main.go:168` and `main.go:183-196`. The dedicated-vs-shared-account
fact (owner) affects only a footprint-risk term, not the design.

Then implement (native, small): the `OpenChat` set for channel/supergroup
sources at generation build + re-auth + hot-reload edges, with the stagger
guardrail; a regression test asserting the open set matches the ruleset's
channel/supergroup sources.

## Problem 2 (secondary, downgraded to "potential, unobserved")

The serial per-endpoint worker + backlog is *conditionally stable* G/G/1 (a
burst drains when arrival < service rate; wait grows **linearly**, bounded by
`timeout`, not exponentially). **No owner-observed per-message delay exists yet**,
so this leg is **not pursued now** and **does not gate Problem 1**. Its 4 product
decisions (loss / duplicate / ordering / receiver `T`+fast-ack) are **this leg's**
blockers, made **if/when** per-message delay is actually observed.

### The decision axes (for when Problem 2 is pursued)

| axis | options |
|---|---|
| Where the queue lives | **A** external queue microservice · **B′** local outbox + external reader · **C** all in-budva (outbox+retry) — *C rejected: "thickens" budva* |
| First-hop semantics | unacked (droppable) vs acked append (at-least-once, local buffer 1–2 ms) |
| Who owns retry / ordering / observability | queue side (Budva only appends) |

### Receiver-throughput effect (conditional, not absolute)

Whether Budva-side concurrency + a receiver fast-ack raise end-to-end throughput
depends on the receiver's own processing model:

- If the **receiver can process concurrently**, bounded producer concurrency can
  raise webhook-ingress and end-to-end throughput, and fast-ack directly raises
  the Budva worker's service rate.
- If the **receiver (or its downstream) is strictly serialized / saturated**,
  producer concurrency mainly *moves the backlog* and can add contention; final
  business throughput is bounded by the receiver, not Budva.
- An **external durable queue** generally does **not** raise final business
  throughput, but absorbs bursts, isolates producer availability from receiver
  outages, preserves events, controls retries, and supports replay —
  reliability/decoupling benefits, separate from latency.

### Phase 0 — reproduce + bound the mechanism (a GATE, by controlled test)

In-repo tests (`internal/service/push`, no production traffic):
- **serial baseline reproduction:** a `WebhookSink`/test sink backed by a
  **slow-receiver stub** that sleeps `T` before 2xx; enqueue N events on the
  current single-worker endpoint and assert the queue-wait (`T3−T2`) grows
  **linearly** to ≈ `(N−1)·T`. Confirms the mechanism is real and **locates the
  dominant interval** (`T2−T3` grows; `T3−T4` held constant by the stub).
- **bounded, reproducible bounds:** assert the resulting queue-wait P-max /
  oldest-age against an explicit budget (a **measurable acceptance**).
- **the fix proven in the same suite:** with the concurrency/timeout change, the
  same burst shows queue-wait and oldest age **within budget**, in-flight ≤
  configured concurrency, and zero unexpected `buffer_full`.

### Phase 0b — post-deploy verification aid (ships with the change, not a gate)

Producer-side observability (in-repo, small): stamp **enqueue time** and
**worker-pickup time** (queue-wait `T3−T2`); expose **queue depth** and
**oldest queued-event age** per endpoint (alongside the existing
`enqueued / full / failed / duration / lastSuccess`); retain a stable
**correlation key** (chat + message + event type) in structured logs. Receiver
correlation is **free**: the payload carries `OccurredAt` (`service.go:351`),
so the receiver can compute arrival-minus-`OccurredAt` with zero Budva change.
Also supplies the dead-man's-switch on last delivery the 10-29 record flagged.

### Phase 1 — settle the delivery contracts (owner decisions; this leg only)

loss policy (best-effort vs must-not-lose) · duplicate policy · ordering scope
(global / per-endpoint / per-chat / none — decides worker topology) · receiver
fast-ack · receiver idempotent ingest · receiver-supported concurrency ceiling.

### Phase 2 — choose parameters and topology (test-justified + configurable)

- **timeout** from the receiver's response time (one `curl -w
  '%{time_total}'` or the access log — a single black-box number, **not** a
  measurement campaign) + delivery semantics + failure budget + safety margin +
  loss policy. **No concrete value before the receiver behavior and loss policy
  are recorded.** "Set to P95/P99" is **rejected** (a loss knob under
  at-most-once).
- **concurrency** configurable; initial value from the **Phase 0 test budget**
  (smallest N that holds the burst in budget) and the receiver-concurrency
  contract — **not a preset 4**, not production-arrival-derived.
- **topology:** unordered worker pool (ordering scope none / receiver
  idempotent+order-independent) vs stable chat partitioning
  (`hash(chat_id) % N`) if same-chat order is contractual.
- **buffer-full behavior** + meaningful **latency/saturation alerts** from the
  Phase 0b metrics.

### Phase 3 — implement and validate (this leg)

bounded per-endpoint concurrency (native Go); the **Phase 0** slow-receiver
suite as the regression suite + ordering tests for NEW/EDIT/DELETE +
generation-retirement / goroutine-lifecycle tests + buffer-full + timeout/
cancellation + **race testing of the dispatcher change**; the unchanged 10-29
task-queue regression tests; TDLib-provisioned CI for handler tests; the
**Phase 0b** observability alongside.

### Phase 4 — reliable delivery, only if must-not-lose (this leg)

Only if the Phase 1 policy is must-not-lose: durable append before acknowledging
acceptance, a stable idempotency key, retry classification + exponential
backoff with jitter, `Retry-After`, a DLQ, inspection/replay, receiver-side
duplicate suppression. The durable queue, if needed, is an **off-the-shelf wheel
evaluated as a decision** (e.g. NATS JetStream or Redis Stream) — *not
pre-approved by name* — wired through the existing `push.Sink` seam
(`internal/service/push/dispatcher.go:32-36`) so the Budva-side diff stays
small: a `queueSink` does one acked `append` (ms); `WebhookSink` retired or
demoted to fallback. First hop is an **acked append**, not unreliable UDP.

## Excluded (and why)

- **Bespoke in-repo queue microservice** (new `cmd/`, new image, new CI path,
  new failure domain, hand-rolled retry/DLQ) — most wheel-inventing; violates
  constraint 2.
- **C (in-budva durable outbox + retry)** — "thickens" Budva with exactly the
  mechanics the owner does not want to own; must-not-lose is better met by an
  off-the-shelf durable queue through the `push.Sink` seam.
- **B′ (local outbox + external reader) on its own merits** — buys
  engine-offline resilience but adds a second process *sharing* Budva's store
  plus a **self-invented** claim/mark/de-dup handoff (the least-reusable part).
  On the same host with the queue in the same compose, offline resilience is
  moot. B′ only wins if the first hop *must* be local.
- **Presetting `concurrency = 4` or `timeout ≈ P95/P99`** — no test budget or
  recorded contract supports those; the timeout rule is a loss knob under
  at-most-once. Both become test-justified, configurable values (Phase 2).
- **A hand-built `getChannelDifference` poller in Go for Problem 1.** Rejected
  **because go-tdlib is a CGO shell over the C++ TDlib engine, which already
  performs channel-difference polling** — Budva only sends `OpenChat`. (This
  corrects an earlier draft that proposed a raw `Send` poller on the mistaken
  premise that go-tdlib is a pure-Go MTProto client.)
- **Production-side timing collection as an implementation gate** (Problem 2).
  Non-reproducible, invasive; the delayed occurrence is the thing being fixed.
  The evidence is the **controlled slow-receiver test** (Phase 0); production
  observability is a post-deploy aid (Phase 0b). No full T1–T4 tracing platform.

## Implementation entry gate (the Problem-1 code change stays blocked until closed)

1. **Root cause and fix are recorded and converged** (this document +
   facts-and-model): Budva does not own the acquisition boundary of its ruleset
   sources (never `OpenChat`s); fix = `OpenChat` each channel/supergroup source
   on its own session and keep it open; private/basic-group ride the global
   stream. Closed by three-layer evidence (spec + pinned engine source +
   go-tdlib architecture), **no owner-run experiment required**.
2. **Source-confirms 1–3 above resolved against the pinned source (2026-11-01,
   no experiment):** idle-open delivery = **unconditional re-poll** (the re-arm
   is on the success path, gated only on `open_count>0`, no gap check) → the
   fix is literally "one `OpenChat` + keep", **no re-poll nudge**; >10 =
   **unbounded FIFO pending queue, deferred never dropped**; member-channel
   idle delivery = **keep the open set = all sources, do not shrink** (the
   spec's "usually" is not a guarantee). These fix the fix shape.
3. **Lifecycle + stagger guardrails designed** (re-auth re-open, hot-reload
   add/remove, limiter-staggered initial batch) against the seams at
   `main.go:168` / `main.go:183-196`.
4. **Owner go-ahead** to implement (a decision, not an input): no owner-run
   experiment, no Problem-2 contract, and no source-chat-type answer are
   required to proceed.

Closing this needs the lifecycle/stagger design + a small in-repo open-set
test + the owner's go-ahead (the source-confirms are already resolved) —
**not** a production collection, **not** an owner experiment, and **not** a
platform.

## Owner decisions

- **Problem 1: none outstanding.** The type-agnostic requirement is **resolved**
  (converged to: one intent, each class by its own client-conformant mechanism).
  The only owner item is the **go-ahead to implement** (and, optionally, the
  dedicated-vs-shared-account fact, which affects only a risk term).
- **Problem 2 (only if/when that leg is pursued):** (1) loss policy; (2)
  duplicate policy; (3) ordering scope; (4) receiver `T` + fast-ack /
  idempotency / concurrency-ceiling contracts. These are **this leg's** blockers,
  **not** the Problem-1 fix's.

## Team B citation scope (explicit)

`research/team-b-webhook-latency-follow-up.md` independently confirms the
**Problem-2 source mechanism** (serial per-endpoint worker, pre-wire queue wait,
single attempt, occupied-until-response) and the **evidence boundary**. It does
**not** constitute approval of this task's parameter values, implementation
ordering, product contracts, or final architecture. Cited as Problem-2 mechanism
corroboration only.

`research/team-b-team-a-report-audit.md` is Team B's audit of this task's
research/design (**CHANGES REQUESTED**); corrections applied. One point is
resolved **differently, by deliberate design**: Team B's P1-02 frames the
evidence as capturing a representative *production* delayed occurrence; this
task uses a **controlled slow-receiver test** as the (Problem-2) gate
(deterministic, reproducible, race-clean, CI-runnable) and demotes production
observability to a post-deploy aid. The evidence *boundary* Team B asks to
preserve is honored.

`research/team-b-tdlib-open-chat-ground-fact-check.md` is Team B's **ground-fact
check of the owner's open-chat observation** (**CHANGES REQUESTED**,
2026-10-30) — kept **verbatim** (its "large supergroups" phrasing is a
maintainer paraphrase, not a protocol class). It originated the open-chat
framing and the B1 decomposition; the **three-layer evidence in
facts-and-model supersedes** its open B1a–B1d candidate set with the converged
mechanism. Cited as **ground-fact evidence, not design approval**.

## Boundary restated

Producer-side only. The receiver is a black box (accepted / rejected(class) /
unreachable). fast-ack and idempotent-ingest are *required of* the receiver
(Problem 2), confirmed by the owner, and out of scope to implement here.
