# Team B Webhook Delivery Latency Follow-up Audit

## 1. Audit identity and purpose

- **Audit party:** Team B, acting as an independent reviewer
- **Audit date:** 2026-10-30
- **Code baseline:** current repository `main`
- **Observed production symptom:** a message is visible in the official Telegram
  client, while the webhook receiver on the same machine does not observe the
  inbound POST until several minutes later
- **Audit mode:** source and architecture review; no production code change,
  test execution, Git commit, or branch operation was performed

This record preserves Team B's current findings about the newly reported
finite webhook delay. It is a follow-up to the completed
`10-29-budva-push-stall` incident, but it does not alter that incident's
accepted root cause or runtime acceptance.

## 2. Executive verdict

The current code contains a concrete mechanism capable of producing the
reported delay:

> Each configured push endpoint has one delivery worker. That worker performs
> HTTP requests strictly serially. A slow request occupies the only worker
> until it completes or reaches the configured timeout, while later events
> wait in the endpoint's in-memory channel and have not yet initiated an HTTP
> request.

This mechanism explains why the webhook service can see **no inbound request**
for a delayed event even when producer and receiver run on the same machine.
The dominant interval can be local queue waiting before `http.Client.Do`, not
network transit time.

For a deployment using `timeoutSeconds: 60`, three preceding deliveries that
each occupy the worker for approximately 60 seconds can delay the next event
by approximately three minutes:

```text
event accepted into endpoint buffer
  -> wait behind request A, up to about 60 seconds
  -> wait behind request B, up to about 60 seconds
  -> wait behind request C, up to about 60 seconds
  -> worker finally starts this event's HTTP POST
```

**Team B disposition: the source-level explanation is confirmed as possible
and highly plausible, but it is not yet proven to be the cause of a particular
production occurrence.**

Proving the production occurrence requires timestamps around update receipt,
submission, worker pickup, and receiver arrival.

## 3. Findings

### TB-LAT-P1-01: one slow delivery blocks the entire endpoint

**Confirmed source fact.**

`internal/service/push/dispatcher.go` creates exactly one goroutine for each
configured endpoint. Its `runWorker` loop reads one envelope, calls
`worker.sink.Push`, waits for that call to return, and only then reads the next
envelope.

Consequences:

- all chats and event types routed to the same endpoint share one serial
  delivery domain;
- a slow or non-responsive receiver request causes head-of-line blocking;
- later events can be accepted by Budva but remain invisible to the receiver
  because their HTTP request has not started;
- the configured request timeout is also the maximum time one failed attempt
  can monopolize the worker.

Relevant implementation:

- `internal/service/push/dispatcher.go:77`
- `internal/service/push/dispatcher.go:86`
- `internal/service/push/dispatcher.go:100`
- `internal/service/push/dispatcher.go:214`
- `internal/service/push/dispatcher.go:216`
- `internal/service/push/dispatcher.go:217`
- `internal/service/push/dispatcher.go:219`

### TB-LAT-P1-02: submission success does not mean the POST has started

**Confirmed source fact.**

`SubmitV2` ultimately performs a non-blocking send to the endpoint channel.
When that send succeeds, the result means only that the event was placed into
the local endpoint buffer. It does not mean the worker has picked it up or
that `http.Client.Do` has begun.

The default endpoint buffer capacity is 256. When the buffer fills, submission
is rejected immediately as `buffer_full`; Budva does not wait for space.

Relevant implementation:

- `internal/service/push/dispatcher.go:16`
- `internal/service/push/dispatcher.go:78`
- `internal/service/push/dispatcher.go:86`
- `internal/service/push/dispatcher.go:127`
- `internal/service/push/dispatcher.go:157`
- `internal/service/push/dispatcher.go:161`

### TB-LAT-P1-03: the HTTP delivery path deliberately makes one attempt

**Confirmed source fact.**

The webhook sink does not contain an application-level retry loop. It renders
the payload, constructs one request, and enters `http.Client.Do`. HTTP status
and transport failures are classified after that attempt.

This means the observed multi-minute gap is not explained by hidden Budva
retry backoff in this delivery code. It is consistent with waiting behind
earlier requests in the serial endpoint worker.

Relevant implementation:

- `internal/service/push/delivery.go:131`
- `internal/service/push/delivery.go:141`
- `internal/service/push/delivery.go:153`
- `internal/service/push/delivery.go:167`
- `internal/service/push/delivery.go:176`

### TB-LAT-P1-04: a slow receiver response matters even on the same machine

**Confirmed behavioral consequence of the source.**

The worker remains occupied until `WebhookSink.Push` returns. Receiving the
TCP request is not enough: the sink waits for the HTTP response and drains its
body before the attempt is considered complete.

Therefore a receiver that accepts the connection and then synchronously
performs long-running business work before responding can block Budva's only
endpoint worker. Localhost or same-host deployment removes most network
distance, but it does not remove application response time, proxy waiting,
connection-pool behavior, TLS behavior, or process scheduling.

Relevant implementation:

- `internal/service/push/delivery.go:167`
- `internal/service/push/delivery.go:171`
- `internal/service/push/delivery.go:172`

### TB-LAT-P2-01: ordinary NEW text normally bypasses the shared task queue

**Confirmed source fact.**

For an ordinary NEW text event that passes filtering, `SubmitV2` is called
before Telegram-forwarding tasks and statistics work are added to the shared
task queue.

Accordingly, if ordinary NEW text is delayed for minutes, the shared task
queue is not the first explanation. The primary candidates are:

1. Budva/TDLib received the update late;
2. the endpoint delivery buffer already had a backlog;
3. the endpoint worker had started the request but the HTTP path was slow.

Relevant implementation:

- `internal/app/handler/service.go:342`
- `internal/app/handler/service.go:350`
- `internal/app/handler/service.go:352`
- `internal/app/handler/service.go:365`
- `internal/app/handler/service.go:398`

### TB-LAT-P2-02: albums, edits, and deletes can have additional queue delay

**Confirmed source fact for albums; architectural exposure for queued event
paths.**

The shared task queue has one ticker-driven worker. It wakes once per second
and executes at most one task per tick. An album task occupies this worker
while waiting until three seconds have elapsed since the most recently
received album member.

An album event can therefore accumulate:

```text
shared task queue wait
  + album quiet-window wait
  + endpoint delivery buffer wait
  + HTTP request/response duration
```

Relevant implementation:

- `internal/infra/queue/repo.go:27`
- `internal/infra/queue/repo.go:51`
- `internal/infra/queue/repo.go:58`
- `internal/infra/queue/repo.go:100`
- `internal/app/handler/service.go:573`
- `internal/app/handler/service.go:575`
- `internal/app/handler/service.go:583`
- `internal/app/handler/service.go:597`

## 4. Relationship to the 10-29 push-stall incident

The newly reported finite delay must not be described as proof that the 10-29
self-deadlock has returned.

The accepted 10-29 incident had these defining properties:

- the queue worker re-entered the same non-reentrant `RWMutex`;
- the update pipeline became permanently wedged;
- later updates never reached the push submitter;
- the push worker was idle;
- a process restart was required to restore delivery.

The current observation differs:

- the webhook request eventually arrives;
- the delay is finite and measured in minutes;
- the behavior is consistent with queue backlog and serial HTTP delivery.

The 10-29 fix correctly releases the task-queue mutex before executing a task.
It removes the confirmed self-deadlock precondition, but it does not change
either of these pre-existing serialization points:

1. the shared task queue still has one ticker-driven worker;
2. each push endpoint still has one strictly serial delivery worker.

Therefore both statements can be true:

- the 10-29 deadlock was correctly fixed and runtime-accepted;
- the latest build can still exhibit finite minute-scale webhook latency.

## 5. Assessment of the current delivery practice

### 5.1 What the current design gets right

The implementation is coherent for a low-volume, best-effort delivery mode:

- endpoint buffers and workers isolate one configured endpoint from another;
- handler submission is non-blocking;
- bounded buffers prevent unbounded memory growth;
- delivery attempts have a configurable timeout;
- delivery errors are classified;
- there is no uncontrolled retry storm;
- generation retirement drains workers in an orderly way.

### 5.2 Where it falls short of mature reliable webhook delivery

The current implementation is not a mature reliable-event-delivery design if
events must arrive promptly and must not be lost:

1. **The ordering scope is too broad.**
   Every event routed to one endpoint is serialized, even if events belong to
   unrelated chats. One slow request delays all of them.

2. **A long timeout amplifies head-of-line blocking.**
   A 60-second timeout may be defensible elsewhere, but it is expensive when
   attached to the only worker for an endpoint. The timeout should be based on
   measured receiver latency and the receiver should acknowledge ingestion
   quickly.

3. **The in-memory buffer provides best-effort, at-most-once behavior.**
   Process termination loses queued events. A full buffer rejects new events.
   A failed request is not retried. There is no durable outbox, dead-letter
   queue, replay operation, or end-to-end delivery state.

4. **The system cannot currently distinguish queue delay from HTTP delay in
   production.**
   Existing duration measurement starts when the worker begins the sink call.
   It does not expose how long an accepted event waited in the endpoint
   buffer.

5. **The shared task queue remains a latency multiplier for queued paths.**
   It polls once per second, starts one task per tick, and permits a task to
   occupy the only worker while sleeping or waiting on external work.

**Overall architecture verdict:** acceptable only when explicitly documented
as low-volume best-effort delivery. It is not sufficient as a production-grade
reliable webhook pipeline.

## 6. Required runtime evidence

For one delayed ordinary NEW-text event, capture these four timestamps:

```text
T1 = Budva receives the Telegram update
T2 = handler calls SubmitV2
T3 = endpoint worker starts WebhookSink.Push / http.Client.Do
T4 = webhook receiver records request arrival
```

Interpretation:

- large `T1 -> T2`: investigate TDLib receipt, update dispatch, filtering, or
  handler execution;
- large `T2 -> T3`: endpoint delivery backlog and head-of-line blocking;
- large `T3 -> T4`: investigate HTTP/TLS/proxy/connection/receiver ingress;
- small `T3 -> T4` but long worker duration: the receiver accepted the request
  quickly but returned the HTTP response slowly.

The following measurements should be exposed per endpoint:

- current queue depth;
- age of the oldest queued event;
- in-flight request count;
- enqueue timestamp and delivery-start timestamp;
- HTTP request duration;
- accepted, delivered, failed, and `buffer_full` totals;
- last-success timestamp;
- stable event ID, chat ID, message ID, and event type in structured logs
  without logging message content or secrets.

The lowest-cost immediate check is:

1. obtain the receiver's arrival timestamp for a delayed message;
2. correlate it with Budva's event/message identity;
3. inspect whether the delay clusters near multiples of the configured
   60-second timeout;
4. determine whether the receiver performs synchronous business work before
   returning its 2xx response.

## 7. Recommended evolution

### Phase 1: latency containment and observability

- allow a small bounded concurrency per endpoint, for example four workers;
- reduce the delivery timeout using measured receiver P95/P99 latency rather
  than a generic large value;
- require the receiver to validate, persist or enqueue, and return 2xx
  quickly, with business processing performed asynchronously;
- add endpoint queue-depth and oldest-event-age metrics;
- log enqueue and worker-start timestamps using stable event identity;
- move album quiet-window waiting out of the sole shared task worker;
- replace one-task-per-second polling with event-driven queue wake-up.

This phase reduces likely latency but does not prevent message loss on process
failure.

### Phase 2: reliable delivery

If event loss is unacceptable, use a durable outbox:

```text
Telegram update
  -> construct stable event_id
  -> persist event in durable outbox
  -> worker claims event
  -> POST to receiver
  -> mark delivered only after 2xx
  -> retry transient failures with exponential backoff and jitter
  -> move exhausted/permanent failures to a DLQ
  -> support inspection and replay
```

Suggested failure policy:

- `2xx`: delivered;
- connection failures, timeouts, `408`, `429`, and `5xx`: retryable;
- other `4xx`: normally permanent, subject to an explicit endpoint contract;
- serialization or invalid configuration: permanent failure plus alert;
- honor `Retry-After` when supplied.

Retries require receiver-side idempotency. A stable key can be derived from
the event type, chat ID, message ID, and event version.

### Phase 3: preserve local ordering without global serialization

If ordering is required only within one Telegram chat, partition work by
chat:

```text
partition = hash(chat_id) % N
```

Events for the same chat remain serial while unrelated chats can progress in
parallel. This limits the blast radius of a slow request without introducing
unbounded concurrency.

## 8. Evidence classification

### Directly proved by current source

- one endpoint owns one delivery worker;
- deliveries for an endpoint execute serially;
- the endpoint channel defaults to capacity 256;
- submission is non-blocking;
- a full buffer rejects the event;
- worker timeout is configurable;
- the webhook sink performs no application-level retry loop;
- ordinary NEW text submits before subsequent shared-queue work;
- the shared task queue starts at most one task per one-second tick;
- album quiet-window waiting occurs inside the shared task worker.

### High-confidence explanation, not production proof

- endpoint worker backlog is the leading explanation for minute-scale delay
  affecting ordinary NEW text;
- delay clustering around multiples of 60 seconds would strongly support this
  mechanism;
- a receiver that returns responses slowly can create the backlog even on the
  same host.

### Not yet established

- the endpoint queue depth during the reported delay;
- which preceding request occupied the worker;
- the real duration of preceding attempts;
- the precise time Budva received the affected Telegram update;
- the actual `T2 -> T3` wait for the affected event;
- whether the receiver delayed its HTTP response;
- whether every reported delayed item was ordinary text rather than an album,
  edit, or delete path.

Team B therefore does not state that the production incident has been fully
root-caused. We state that the current implementation contains a sufficient,
source-confirmed mechanism and that a small timestamp instrumentation change
can prove or refute it decisively.

## 9. Final Team B determination

- **Can the current source explain the observation?** Yes.
- **Is same-host deployment evidence against this explanation?** No. The
  suspected delay occurs primarily before the affected request is sent.
- **Is this the 10-29 deadlock recurring?** There is currently no evidence for
  that claim; the finite-delay shape points elsewhere.
- **Is the current design reasonable?** Yes, only as an explicitly
  best-effort, low-volume design.
- **Is it a mature reliable webhook architecture?** No.
- **Is production causality fully proven?** No; the four timestamps and queue
  measurements are still required.
- **Recommended next step:** add queue-wait observability before changing
  concurrency so that the current production behavior is captured and the
  improvement can be measured.

## 10. Independence and scope statement

This report is explicitly authored by **Team B** as an independent follow-up
audit. Team B did not modify Budva production code, tests, deployment
configuration, or the webhook receiver. The report distinguishes source facts,
architectural deductions, and production claims that still require runtime
evidence.

This report supplements rather than supersedes:

- `research/root-cause.md`, the accepted 10-29 deadlock record;
- `research/team-b-audit-report.md`, the independent root-cause,
  implementation, and remediation audit trail.
