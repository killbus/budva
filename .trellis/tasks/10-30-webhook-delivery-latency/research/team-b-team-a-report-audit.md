# Team B Independent Audit of Team A's 10-30 Webhook Delivery Latency Report

## 1. Audit identity and scope

- **Audit party:** Team B, acting as an independent reviewer
- **Audit date:** 2026-10-30
- **Audited task:** `10-30-webhook-delivery-latency`
- **Audited Team A artifacts:**
  - `prd.md`
  - `task.json`
  - `research/facts-and-model.md`
  - `research/design-and-position.md`
  - `implement.jsonl`
  - `check.jsonl`
- **Supporting independent record:** `research/team-b-webhook-latency-follow-up.md`
- **Code baseline stated by Team A:** `main` at `415d90c`
- **Audit mode:** documentation, source-model, architecture, and implementation-readiness review
- **Excluded actions:** no production-code change, test execution, deployment change, Git commit, or branch operation

This report records Team B's independent assessment of the research and design
landed by Team A for the finite webhook-delivery latency reported on
2026-10-30.

The review distinguishes:

1. facts directly established by the current source;
2. mechanisms capable of explaining the production symptom;
3. production causality that still requires runtime evidence;
4. design choices that require explicit product or operational contracts.

## 2. Executive verdict

**Team A's research direction is sound, but the current report is not ready to
serve as an implementation authorization.**

Team B agrees with the central source analysis:

- each configured endpoint currently has one strictly serial delivery worker;
- successful submission means local enqueue, not that an HTTP request has
  started;
- a slow preceding request can cause head-of-line blocking for later events;
- same-host deployment does not exclude local queue waiting before the wire;
- ordinary NEW text normally reaches `SubmitV2` before shared task-queue work;
- the finite, self-recovering delay is materially different from the 10-29
  permanent self-deadlock;
- the present delivery model is best-effort and in-memory rather than durable
  reliable delivery.

However, Team A crosses the current evidence boundary in several places. It
treats the serial-worker mechanism as the established production root cause,
although the production queue depth, worker pickup delay, preceding request
durations, and receiver response behavior have not been captured.

The report also selects or strongly implies implementation parameters before
the necessary contracts and measurements exist:

- default per-endpoint concurrency of four;
- timeout near the receiver's P95/P99;
- full T1-T4 or equivalent queue-wait instrumentation excluded as an
  implementation gate;
- ordering behavior left conditional;
- loss policy left open while timeout and concurrency changes are already
  framed as the default implementation.

**Overall disposition: CHANGES REQUESTED.**

- **Research direction:** PASS.
- **Source mechanism analysis:** PASS WITH EVIDENCE-BOUNDARY CORRECTIONS.
- **Queueing model:** CHANGES REQUESTED.
- **Design parameterization:** CHANGES REQUESTED.
- **Implementation readiness:** NOT YET APPROVED.

## 3. Findings

### TB-TA-P1-01: the report promotes a plausible mechanism to a proven production root cause

**Severity:** P1  
**Status:** Open

The PRD states that the observed delay "is the pre-existing serial
per-endpoint delivery worker plus a small backlog." `task.json` similarly
describes that mechanism as the root mechanism.

The source establishes that the mechanism exists and is sufficient to produce
the symptom:

- one worker is started per endpoint;
- that worker handles deliveries serially;
- later events can wait in the endpoint channel before their POST begins;
- one request can occupy the worker until the configured timeout;
- the default endpoint channel capacity is 256;
- a full buffer rejects rather than blocks submission.

What has not been established for the production occurrence:

- endpoint queue depth during the delay;
- actual queue-wait time for the affected event;
- which preceding request occupied the worker;
- actual preceding-request service times;
- whether the receiver delayed its response;
- whether the affected event followed the ordinary NEW-text path;
- the precise time at which Budva received the Telegram update.

Accordingly, the strongest defensible conclusion is:

> The current source contains a confirmed serial head-of-line-blocking
> mechanism that is the leading hypothesis for ordinary NEW-text
> minute-scale delay. Production causality remains unproven until runtime
> queue-wait and request timing evidence is captured.

The current wording should not state or imply that the production incident has
already been root-caused.

**Required correction:**

- revise `prd.md`, `task.json`, and `research/facts-and-model.md`;
- consistently distinguish source-confirmed capability from production-proven
  causality;
- preserve the serial endpoint worker as the leading hypothesis, not a settled
  incident conclusion.

### TB-TA-P1-02: queue-wait observability is treated as optional verification after behavior changes

**Severity:** P1  
**Status:** Open

Team A excludes full Phase-0 T1-T4 instrumentation from the gate and proposes
queue depth and oldest age largely as verification of the later concurrency
change.

This sequencing risks changing or suppressing the original symptom before the
suspected interval is measured.

The existing payload `OccurredAt` timestamp is useful, but receiver arrival
minus `OccurredAt` yields only a combined interval:

```text
T4 - T2 = endpoint queue wait + HTTP/TLS/proxy/receiver-ingress time
```

It does not distinguish:

```text
T2 -> T3 = accepted into endpoint buffer -> worker starts delivery
T3 -> T4 = worker starts delivery -> receiver observes ingress
```

Team B does not require a heavyweight tracing platform. A proportional Phase 0
is sufficient:

- record enqueue time;
- record worker-pickup time or queue-wait duration;
- expose current queue depth;
- expose oldest queued-event age;
- retain a stable event or message correlation key;
- correlate the producer record with receiver arrival time.

**Required correction:**

- make queue-wait measurement an implementation prerequisite;
- capture at least one representative delayed occurrence before changing
  endpoint concurrency;
- do not classify this observability only as a non-gating, post-change
  verification aid.

### TB-TA-P1-03: timeout guidance near P95/P99 can convert normal tail latency into silent loss

**Severity:** P1  
**Status:** Open

The Team A documents repeatedly recommend setting `timeoutSeconds` near the
receiver's P95/P99.

That rule is too mechanical for the current at-most-once delivery behavior:

- a timeout near P95 can reject approximately the slowest five percent of
  otherwise valid requests under a representative distribution;
- a timeout near P99 can still classify legitimate tail requests as failures;
- current delivery has no durable outbox, application-level retry, replay, or
  dead-letter recovery;
- shortening the timeout therefore trades a smaller head-of-line-blocking
  bound for a potentially higher event-loss rate.

Timeout selection must consider:

- the receiver latency distribution;
- the acceptable delivery-failure budget;
- retry and replay semantics;
- receiver-side fast-ack behavior;
- operational safety margin;
- the selected best-effort or must-not-lose policy.

**Required correction:**

Replace "set near P95/P99" with a risk-based rule such as:

> Select the timeout from measured receiver response-time distribution,
> delivery semantics, acceptable failure rate, and an explicit safety margin.
> In best-effort mode, document the latency-versus-loss tradeoff. In
> must-not-lose mode, do not rely on timeout tuning without durable retry and
> idempotency.

No concrete timeout should be accepted until the receiver's response behavior
and the loss policy are recorded.

### TB-TA-P1-04: default concurrency of four is unsupported and changes ordering semantics

**Severity:** P1  
**Status:** Open

Changing an endpoint from one worker to four consumers is not only a throughput
parameter change. It changes the observable event-ordering contract.

Potential consequences include:

- events for one endpoint can arrive out of order;
- NEW, EDIT, and DELETE events for one chat can overlap;
- a later event can complete before an earlier event;
- response-time variance can reorder completion even if dequeue order is
  preserved;
- receiver behavior may be incorrect without idempotency, version checks, or
  stale-event rejection.

The Team A report acknowledges ordering but leaves chat partitioning as an
implementation-time conditional. That is not sufficient. A shared channel
consumed by four goroutines cannot preserve same-chat ordering.

If same-chat order is required, implementation needs stable routing such as:

```text
partition = hash(chat_id) % N
```

with one serial consumer per partition. If global endpoint order is part of the
contract, concurrency cannot be increased without changing that contract.

The value four is also not derived from:

- receiver concurrency capacity;
- observed arrival rate;
- observed service-time distribution;
- connection limits;
- desired latency bound;
- available CPU or memory;
- ordering requirements.

**Required correction:**

1. settle the ordering contract before implementation;
2. distinguish globally unordered worker-pool mode from chat-partitioned mode;
3. make concurrency configurable;
4. derive the initial value from measurement and receiver capacity rather than
   fixing an undocumented default of four;
5. add ordering-focused tests for NEW, EDIT, and DELETE event sequences.

### TB-TA-P1-05: loss policy remains open while dependent implementation decisions are already selected

**Severity:** P1  
**Status:** Open

Team A correctly identifies the product fork:

- best-effort, in-memory, droppable delivery;
- must-not-lose delivery using durable queuing, retry, DLQ, replay, and
  idempotency.

However, the loss policy is still an owner decision while the PRD already
positions timeout reduction and bounded concurrency as the default solution.

Those decisions cannot be evaluated independently:

- in best-effort mode, timeout and full-buffer failures can be accepted only if
  event loss is explicitly tolerated and observable;
- in must-not-lose mode, increasing concurrency does not address process
  restart loss, buffer-full loss, or single-attempt failure;
- timeout reduction without durable retry can increase loss;
- retries without receiver idempotency can create duplicates.

**Required correction:**

Before implementation begins, record:

1. whether events may be lost;
2. whether duplicates are acceptable;
3. whether ordering is global, per endpoint, per chat, or not guaranteed;
4. whether the receiver supports idempotent ingestion;
5. whether the receiver can acknowledge quickly after durable acceptance.

The selected architecture and acceptance criteria must then match those
answers.

### TB-TA-P2-01: the queueing stability model uses service-time bounds incorrectly

**Severity:** P2  
**Status:** Open

`research/facts-and-model.md` uses `S` as a per-request timeout or service-time
upper bound, then states:

```text
lambda < 1/S
```

and describes `1/S` as the maximum service rate.

This is not a sound general stability statement:

- queue stability is normally expressed using traffic intensity such as
  `rho = lambda * E[S] < 1` for a single-server system under the applicable
  assumptions;
- if `S` is the request timeout upper bound, `1/S` is a conservative service
  rate under worst-case occupancy, not the actual maximum service rate;
- when most requests complete well below the timeout, actual throughput can be
  much higher than `1/S`;
- the document names the model G/G/1 and later refers to "classic M/G/1,"
  which mixes assumptions.

The robust source-derived bound is simpler:

```text
queue_wait(event) = sum(actual service times of requests ahead)
queue_wait(event) <= requests_ahead * configured_timeout
```

The upper bound demonstrates how a long timeout can amplify delay, but it
cannot infer the actual queue length without actual service-time data.

**Required correction:**

- remove the unconditional `lambda < 1/S` stability claim when `S` denotes a
  timeout bound;
- use `E[S]` when discussing expected service time and stability;
- keep timeout-based calculations explicitly labeled as worst-case upper
  bounds;
- use one queue-model notation consistently.

### TB-TA-P2-02: "minute-scale delay implies a small backlog" is not established

**Severity:** P2  
**Status:** Open

Team A infers that a few minutes of delay means approximately three to five
requests ahead when timeout is about 60 seconds, or tens of requests when the
default is five seconds.

Those examples are valid scenarios, but the current evidence does not justify
the stronger conclusion that the real backlog is small.

A three-minute wait could result from:

- three requests consuming about 60 seconds each;
- many requests consuming shorter and variable service times;
- a combination of queue wait and HTTP ingress delay;
- receiver connection constraints;
- an event path with additional upstream waiting.

`timeoutSeconds` supplies an upper bound on one request's occupancy. It does
not supply the observed request duration.

**Required correction:**

- present the three-to-five and tens-of-requests calculations as examples;
- remove "the backlog is small" as a production conclusion;
- state that actual backlog size requires queue depth or worker-pickup data.

### TB-TA-P2-03: the receiver-throughput claim is overly absolute

**Severity:** P2  
**Status:** Open

The design states that if the receiver performs synchronous business work
before returning 2xx, no queue design can increase throughput and a queue only
relocates the backlog.

That is only true when the receiver or its downstream dependency is itself
strictly serialized or saturated at the same capacity.

Different cases must be distinguished:

- if the receiver can process requests concurrently, bounded producer
  concurrency may increase receiver ingress and end-to-end throughput;
- if the receiver is serialized, producer concurrency mainly moves the
  backlog and may increase contention;
- an external durable queue may not increase final business-processing
  throughput, but it can absorb bursts, isolate producer availability,
  preserve events, control retries, and support replay;
- fast-ack changes HTTP occupancy even if downstream business throughput stays
  unchanged.

**Required correction:**

Replace the absolute claim with a conditional one that separates:

- webhook-ingress throughput;
- final business-processing throughput;
- backlog location;
- reliability and decoupling benefits.

### TB-TA-P2-04: acceptance criterion A1 presents a conditional estimate as a worst-case guarantee

**Severity:** P2  
**Status:** Open

PRD acceptance criterion A1 uses approximately:

```text
(N / concurrency) * T
```

as the expected worst-case delay.

That approximation requires conditions that are not stated:

- `T` is a hard service-time upper bound, not an average or percentile;
- the queue is initially empty;
- N events form a bounded burst;
- no retries occur;
- connection-pool and scheduling delay are negligible or included in `T`;
- the receiver sustains the configured concurrency;
- ordering constraints do not force serialization;
- each worker receives a balanced share of work.

**Required correction:**

Define acceptance using measurable behavior instead:

- queue-wait P95/P99 and maximum under a controlled burst;
- oldest queued-event age;
- in-flight request count;
- receiver-arrival time minus enqueue time;
- zero unexpected `buffer_full` outcomes under the test workload;
- an explicit receiver stub or test server with defined response behavior;
- separate acceptance thresholds for unordered and chat-partitioned modes.

The capacity formula may remain as a planning estimate, but not as an
unconditional worst-case guarantee.

### TB-TA-P2-05: Team B's report is cited beyond the scope of its independent approval

**Severity:** P2  
**Status:** Open

Team A copied Team B's mechanism audit into the new task and cites it in
`task.json`, `implement.jsonl`, and `check.jsonl` as corroboration.

Reusing the report is appropriate if its identity and scope are preserved.
However, the current wording can be read as Team B approval of the entire Team
A design.

Team B independently confirmed:

- the serial endpoint-worker mechanism;
- the possibility of local pre-wire queue delay;
- the distinction from the 10-29 deadlock;
- the need for queue-wait observability;
- bounded concurrency, partitioning, and durable delivery as possible staged
  evolutions.

Team B did not approve:

- a completed production root-cause claim;
- concurrency four as the correct default;
- exclusion of queue-wait observation as a precondition;
- timeout equal or near P95/P99;
- a particular ordering contract;
- best-effort or JetStream as the selected owner decision;
- Team A's acceptance formula.

**Required correction:**

Add an explicit statement:

> Team B's cited report independently confirms the source mechanism and
> evidence boundary. It does not constitute approval of Team A's parameter
> values, implementation ordering, product contracts, or final architecture.

## 4. Confirmed strengths

The requested corrections do not invalidate the Team A research direction.
Team B confirms the following parts should be retained:

1. **Correct incident separation.**  
   The documents correctly distinguish the finite, self-recovering 10-30
   symptom from the permanent 10-29 self-deadlock.

2. **Correct ordinary NEW-text path.**  
   `SubmitV2` occurs before forwarding and statistics tasks are added, so the
   shared one-task-per-second queue is not the first suspect for ordinary NEW
   text.

3. **Correct endpoint-worker facts.**  
   The endpoint has one serial worker, a bounded channel, non-blocking
   submission, and `buffer_full` rejection.

4. **Correct HTTP occupancy observation.**  
   The worker remains occupied through `http.Client.Do` and response-body
   draining.

5. **Correct retry observation.**  
   The webhook sink intentionally makes a single application attempt; hidden
   retry backoff does not explain the delay.

6. **Appropriate resistance to an unnecessary bespoke service.**  
   A new custom queue microservice should not be introduced merely because a
   bounded in-process worker is currently serial.

7. **Correct identification of the loss-policy fork.**  
   Durable delivery should be driven by a must-not-lose requirement, not by
   latency alone.

8. **Useful observability targets.**  
   Queue depth and oldest queued-event age are directly relevant and should be
   implemented, with queue-wait duration added as a gating measurement.

9. **Recognition of ordering risk.**  
   Team A correctly notes that increased concurrency can require partitioning.
   The remaining correction is to settle that contract before implementation.

## 5. Required sequence before implementation

Team B recommends the following order:

### Phase 0: establish production timing

For at least one delayed ordinary NEW-text event, capture:

```text
T1 = Budva receives the Telegram update
T2 = event is accepted into the endpoint queue
T3 = endpoint worker starts the delivery attempt
T4 = receiver records request arrival
T5 = receiver response completes at Budva
```

The minimum gating measurements are:

- queue depth at enqueue;
- queue-wait duration `T3 - T2`;
- oldest queued-event age;
- HTTP attempt duration `T5 - T3`;
- correlated event/message identity;
- receiver arrival timestamp.

### Phase 1: settle delivery contracts

Record explicit owner decisions for:

- best-effort versus must-not-lose;
- acceptable event-loss rate;
- duplicate-delivery policy;
- ordering scope;
- receiver idempotency;
- receiver fast-ack capability;
- receiver-supported concurrency.

### Phase 2: choose parameters and topology

Based on Phase 0 and Phase 1:

- select timeout with safety margin and loss budget;
- select configurable concurrency;
- choose unordered worker pool or stable chat partitioning;
- define buffer-full behavior;
- define meaningful latency and saturation alerts.

### Phase 3: implement and validate

Required validation should include:

- serial baseline reproduction with a controlled slow receiver;
- delayed-event queue-wait measurement;
- configured-concurrency throughput test;
- ordering tests for NEW, EDIT, and DELETE;
- generation retirement and goroutine-lifecycle tests;
- buffer-full behavior;
- timeout and cancellation behavior;
- race testing of dispatcher changes;
- unchanged 10-29 task-queue regression tests;
- TDLib-provisioned CI for handler tests.

### Phase 4: reliable delivery only if required

If the owner selects must-not-lose:

- durable append before acknowledging producer acceptance;
- stable idempotency key;
- retry classification and exponential backoff with jitter;
- `Retry-After` support;
- DLQ;
- inspection and replay;
- receiver-side duplicate suppression.

The external durable queue choice should be evaluated separately. Team B does
not approve NATS JetStream merely because it is named in the current design.

## 6. Required document changes

Before `task.py start` or equivalent implementation authorization, Team A
should update:

### `prd.md`

- replace the asserted root cause with the leading-hypothesis wording;
- make queue-wait observation a prerequisite;
- remove default concurrency four unless justified by recorded data;
- define ordering before selecting the worker topology;
- replace the P95/P99 timeout rule;
- rewrite A1 as measurable latency acceptance, not an unconditional formula;
- record the selected loss policy before implementation.

### `task.json`

- remove notes that characterize the production mechanism as proven;
- remove the unsupported "small backlog" conclusion;
- remove or qualify the default concurrency and timeout values;
- distinguish Team B source-mechanism agreement from design approval;
- list the unresolved owner decisions as implementation blockers.

### `research/facts-and-model.md`

- correct the stability notation;
- distinguish timeout bound from expected service time;
- treat inferred backlog sizes only as examples;
- remove unsupported production exclusions such as categorically excluding
  HTTP ingress without `T3-T4` evidence;
- preserve the direct source facts and the evidence boundary.

### `research/design-and-position.md`

- make observability Phase 0;
- make ordering and loss policy preconditions;
- make concurrency and timeout data-derived configuration;
- qualify receiver-throughput and queue-relocation claims;
- separate latency containment from reliable-delivery architecture.

### `implement.jsonl` and `check.jsonl`

- add the corrected evidence-boundary requirements;
- add ordering-contract validation;
- add queue-wait instrumentation and controlled slow-receiver tests;
- avoid using Team B's mechanism report as approval of Team A's parameter
  choices.

## 7. Implementation entry gate

Team B recommends that implementation remain blocked until these four minimum
conditions are closed:

1. **Evidence boundary corrected:** the production root cause is described as
   a leading hypothesis until runtime timings confirm it.
2. **Queue-wait observation added:** enqueue-to-pickup delay is measurable
   before concurrency changes land.
3. **Delivery contracts settled:** loss, duplicate, ordering, fast-ack, and
   idempotency policies are recorded.
4. **Parameters made evidence-based:** concurrency and timeout are configurable
   and derived from measured behavior and risk budget.

Closing these items does not require a heavyweight platform. It requires a
small amount of observability and explicit product semantics before changing a
concurrency boundary.

## 8. Final Team B determination

- **Does Team A correctly identify a source mechanism capable of explaining
  the symptom?** Yes.
- **Does Team A correctly distinguish the 10-30 finite delay from the 10-29
  deadlock?** Yes.
- **Has the production cause been proved?** No.
- **Is queue-wait observation optional before changing concurrency?** No.
- **Is concurrency four currently justified?** No.
- **Is timeout near P95/P99 currently justified?** No.
- **Are ordering and loss semantics sufficiently defined?** No.
- **Is the research worth retaining?** Yes.
- **May implementation proceed from the current report without correction?**
  No.

**Final verdict: CHANGES REQUESTED.**

The correct progression is:

```text
confirm the suspected interval
  -> settle loss and ordering contracts
  -> derive timeout and concurrency
  -> implement
  -> validate under controlled load and on the deployed box
```

It should not be:

```text
treat the mechanism as the proven root cause
  -> set concurrency to four
  -> shorten timeout near a percentile
  -> observe only after behavior changes
```

## 9. Independence statement

This report is explicitly authored by **Team B** as an independent audit of
Team A's `10-30-webhook-delivery-latency` research and design.

Team B did not author or modify Team A's PRD, task metadata, facts/model
document, design document, implementation manifest, check manifest, production
code, tests, deployment configuration, or webhook receiver.

Team B's earlier `team-b-webhook-latency-follow-up.md` remains the independent
source-mechanism audit. This report evaluates how Team A interpreted and used
that mechanism when defining the new task.
