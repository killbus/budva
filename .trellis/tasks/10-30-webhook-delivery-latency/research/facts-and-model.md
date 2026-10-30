# Facts & queueing model — webhook delivery latency (10-30)

Forward, self-contained record for task `10-30-webhook-delivery-latency`.
Baseline: `main` @ `415d90c` (post 10-29 push-stall fix, PR #7, v0.3.1).
All paths repo-relative. **Producer-side only**: the webhook receiver is a
black box (accepted / rejected(class) / unreachable); nothing here changes it.

## Symptom (as observed, 2026-10-30)

A message visible in the official Telegram client is **not** observed as an
inbound POST by the same-host receiver until **several minutes later**. Same
machine — not network transit. The delay is **finite** (requests eventually
arrive); it is **not** the 10-29 self-deadlock (which was *infinite*: zero
inbound, idle push worker, restart required).

## The delivery pipeline, with the latency-relevant segments

```
TDlib update
  -> listenUpdates -> r.updates (cap-100 chan)          [ms]
  -> handler.Run loop (single consumer):
       OnNewMessage -> rule loop -> processMessage
         -> SubmitV2(...)      [IN the Run loop; NOT in the task queue]
  -> per-endpoint queue (chan, cap 256)                 [the backlog lives here]
  -> runWorker (ONE goroutine per endpoint, serial)
         -> WebhookSink.Push = single http.Client.Do (no app-level retry)
  -> receiver (black box)
```

Source anchors (these are the **source-confirmed facts**):

- `internal/infra/telegram/repo.go:431` — `r.updates <- typ` (cap-100, ms).
- `internal/app/handler/service.go:352` — `SubmitV2` called **in the Run
  loop**, *before* any forwarding/statistics task is `Add`-ed to the shared
  task queue (`service.go:365`). So ordinary NEW text does **not** traverse
  the shared task queue on its push path.
- `internal/service/push/dispatcher.go:100` — `go generation.runWorker(worker)`
  once **per endpoint**.
- `internal/service/push/dispatcher.go:214-237` — `runWorker` is a **serial**
  `for rend := range worker.queue`; it calls `worker.sink.Push` and blocks
  until it returns before reading the next envelope.
- `internal/service/push/dispatcher.go:16,86` — per-endpoint buffer, default
  capacity **256**.
- `internal/service/push/dispatcher.go:157-165` — `Submit` is a **non-blocking**
  send; a full buffer → `SubmitRejectedBufferFull` + WARN `buffer_full`.
- `internal/service/push/webhook.go:11` — `defaultWebhookTimeout = 5s`;
  overridable per endpoint by `timeoutSeconds`
  (`internal/domain/ruleset.go:35`).
- `internal/service/push/delivery.go:167-172` — `Push` calls `client.Do` and
  then **drains the response body**; the worker is occupied until the
  **response** returns, not when the request is sent.
- `internal/service/push/delivery.go:133-139` — `singleAttemptTransport`: the
  sink makes **exactly one** attempt; no application retry loop.

## What a message's end-to-end delay decomposes into

For one ordinary NEW-text event, three intervals sum to the observed gap:

| span | what it is | source |
|---|---|---|
| `T2−T1` | update receipt → `SubmitV2` (TDlib receipt, dispatch, filter) | `service.go:352` |
| `T2−T3` | `SubmitV2` accepted (enqueue) → worker starts `http.Client.Do` (**endpoint buffer queue wait**) | `dispatcher.go:214-219` |
| `T3−T4` | `http.Client.Do` start → receiver records arrival (HTTP/TLS/ingress) | `delivery.go:167` |

(`T1` = Budva receives the Telegram update; `T2` = accepted into the endpoint
queue; `T3` = worker starts the delivery attempt; `T4` = receiver records
arrival; `T5` = response completes at Budva.)

For per-message delay in steady operation, the suspected dominant interval
is **`T2−T3`** (local queue wait before the request is sent), which is why
same-host deployment does **not** by itself disprove the mechanism. (This
interval is *bounded* — HOL is capped at `timeoutSeconds` — so it cannot
explain the owner's 10-minute **total-silence** window; that is a separate,
upstream mechanism, corrected in the hypothesis section below.) **However, `T3−T4` (HTTP / receiver ingress) is not
yet excluded** — the receiver's own response behavior has not been observed —
so which interval actually dominates in production is **open**. The design
resolves this **without production timing collection**: a **controlled
slow-receiver test** (Phase 0) holds `T3−T4` constant (the stub sleeps a fixed
`T`) and shows the Budva-side delay landing in `T2−T3` growing linearly — the
dominant-interval question is thereby answered in a deterministic, reproducible
way before the concurrency boundary changes. The payload carries `OccurredAt`
(= `T2`, stamped at `SubmitV2`), so the receiver can later measure
arrival-minus-`OccurredAt` (= `T2→T4`) with zero Budva code change (a
post-deploy aid, Phase 0b); the producer-side enqueue/pickup stamps add the
`T2−T3` / `T3−T4` split for the same post-deploy verification.

## The queueing model — G/G/1, with a correct stability statement

One endpoint, one serial worker. Model it as a **G/G/1** queue (arbitrary
arrival interarrival and arbitrary service-time distributions, one server).
Let:
- `λ` = event arrival rate to the endpoint,
- `B` = the **actual** per-request service time (the full `client.Do`
  round-trip including response-body drain, `delivery.go:171-172`),
- `E[B]` = its **expected** value,
- `timeout` = the configured per-request timeout, which bounds `B` from above
  (`B ≤ timeout`), so `1/timeout` is a **worst-case (lower) service-rate
  bound**, not the actual maximum service rate.

- **Stability** (the general single-server condition, traffic intensity
  `ρ = λ·E[B] < 1`): the queue is *conditionally* stable. A burst drains and
  the system returns to ~steady when `ρ < 1` (the observed *finite,
  self-recovering* delay); when `λ·E[B] > 1` the backlog grows in time. The
  bound uses `E[B]` (actual mean service time), **not** the timeout.
- **Worst-case wait bound** (the robust, source-derived inequality):
  ```
  queue_wait(event) = sum of the actual service times of the requests ahead
                    <= requests_ahead * timeout        [worst case]
  ```
  This is the statement the timeout *can* support: a large `timeout`
  amplifies the **upper bound** on a request's worst-case occupancy. It is a
  **worst-case upper bound only** — it does **not** yield the actual queue
  length without actual service-time data.

The wait grows **linearly** (arithmetic) in the service times ahead, **not
exponentially** — a delayed request does not slow the ones behind it beyond its
own bounded service time; there is no compounding.

### What a "minute-scale delay" does and does not imply

A few minutes of delay is **consistent with** a small backlog (e.g. 3 requests
× ~60s each, or tens of requests × a few seconds) **or** with many short
requests, or with a queue-wait + HTTP-ingress combination. `timeout` gives an
**upper bound** on one request's occupancy; it does **not** give the observed
request duration or the actual backlog size. The **production** backlog size is
therefore not asserted here (it would require post-deploy queue-depth data —
Phase 0b) and is given only as *plausible examples*, not a conclusion. The
**dominant interval** (queue wait vs HTTP/ingress), by contrast, is established
by the **Phase 0 controlled slow-receiver test**, which does not need
production data.

## Confirmed exclusions (source does not support)

- **"The 1s task-queue ticker is the cause of ordinary NEW-text push delay."**
  Push delivery runs on the per-endpoint **channel** worker
  (`dispatcher.go:214`), which has **no tick**; the 1s ticker
  (`internal/infra/queue/repo.go:51-62`) drives only the shared *task* queue
  (forwards/stats/album), and ordinary NEW text does not enter it on its push
  path.
- **The 10-29 self-deadlock recurring.** That incident was *infinite* (zero
  inbound, idle worker, restart cures). This is *finite and self-recovering*.
  The fix (`415d90c`) removed the reentrancy; it did not touch the two
  pre-existing serialization points this symptom implicates (the shared
  1-task/second task worker and the serial per-endpoint delivery worker).
- **A hidden Budva retry/backoff explaining the gap.** The sink is
  single-attempt (`delivery.go:133-139`); a multi-minute gap is not explained
  by retry backoff in this code.

**Not excluded (open):** same-host network/TLS transit and the receiver's own
response time (`T3−T4`) are **not** ruled out by source alone. The receiver
response time is a **black-box number the owner supplies** (one `curl` / the
receiver's access log) and is part of the Phase 1 contract; the Budva-side
split of the delay into `T2−T3` vs `T3−T4` is pinned down by the **Phase 0
controlled test** (which holds `T3−T4` constant), with the post-deploy Phase
0b metrics confirming it on the box. The only transit claim that *is*
source-supported is that same-host deployment does not *disprove* the pre-wire
queue-wait mechanism.

## Model correction (supersedes the endpoint-HOL "leading hypothesis"): the 10-minute silence is an ABSENCE of submissions, not slow delivery

**Observed signature the hypothesis must fit (owner, 2026-10-30):**
1. There is a **window of 10 minutes or more with ZERO inbound requests**,
   after which delivery resumes (a catch-up burst).
2. **The silence is total**: it covers **text and media and everything** — not
   just albums. (Owner clarification: "静默 = 没有任何信息,当然包括文本/其他
   内容".)
3. Production config **`timeoutSeconds: 60`**. Per
   `cmd/engine/main.go:146-163` this bounds **both** the worker ctx
   (`dispatcher.go:217`) and the `http.Client`, so one delivery attempt
   occupies the endpoint worker **at most 60 s**.

**What the signature rules out:**

- **Endpoint head-of-line blocking (HOL) alone.** HOL is "slow but
  continuous": the worker emits one request per completed attempt, so the
  receiver would see a request at least every `timeoutSeconds` = **60 s**. A
  10-minute *zero-request* window cannot be a slow drain — it is an **absence
  of `SubmitV2` calls for ~10 minutes**. Endpoint HOL is at most a ≤60 s
  contributor, **not** the explainer.
- **(C) endpoint worker wedged >10 min on one request.** Excluded by the 60 s
  `timeoutSeconds` upper bound.
- **(A) album starved behind the shared task queue, *alone*.** The shared task
  queue (`internal/infra/queue/repo.go:51-62`, 1 task/s, serial) also carries
  every forward task (each = up to a 3 s limiter wait,
  `internal/service/limiter/service.go:12,30-44` + TDlib RPC + retry); album
  members are pushed from a closure on that worker (`service.go:573` → push at
  `:616/:624`), so a forward-task backlog can delay *album* pushes by minutes.
  **But text and single media are pushed immediately, in the Run loop**
  (`service.go:352` / `:259`), never via that queue. Since the silence also
  covers text, (A) alone cannot explain it — though it may still contribute to
  album latency on top of the silence.

**Path split (why "media is also a message" still matters here):**
- text (`service.go:352`) and textless **single** media (`:259`) → push
  **immediately in the Run loop**;
- **album** members (`MediaAlbumId != 0`, `:281-287`) → deferred to the shared
  task queue (3 s quiet window + serial worker).
The silence covering *text* is the fact that shifts the suspicion upstream.

**Remaining candidates (the 10-minute zero-submission window):**

- **(B1) update-stream gap.** The Budva↔Telegram update stream delivered
  nothing for ~10 min (MTProto/session/connection stall, self-recovering) →
  zero updates → zero `SubmitV2` → zero requests, then a flush.
- **(B2) Run-loop stall.** The Run loop (the sole `Updates()` consumer,
  `service.go:157-176`) blocked on a synchronous call for ~10 min, then
  recovered. (If it blocked *permanently* it would be 10-29-shaped and need a
  restart; this one recovers, so it is a *transient* stall or (B1).)
- **(D) ruleset hot-reload gap.** `ruleset.yml` is a bind-mount hot-reloaded
  via fsnotify (`docker-compose.yml:47`); a reload during the window that
  dropped or swapped the push submitter could leave a no-push gap, closing
  when the reload completes. (To check: ruleset-reload log lines inside the
  window.)

All three affect **everything** (text + media), matching the total-silence
signature. **(B) and (D) are now the primary suspects; (A) and endpoint HOL
are secondary contributors at most.**

**The decisive, cheap probe (one-shot active probe, not a timing campaign):**

Owner protocol (box-side, repeatable, ~1 message + 1 log + 1 QUIT per run):
1. **Send a probe**: a **plain text** message with a unique token (e.g.
   `probe <epoch> <rand>`) into a ruleset source chat, confirmed as sent in
   the client. Text is required: it is the shortest path (immediate Run-loop
   `SubmitV2`, `service.go:352`), bypassing the album quiet window and the
   shared task queue — so its absence is a clean "zero submissions upstream"
   signal.
2. **Decision ladder** (watch the gateway inbound log for the token):
   - arrives ≤10 s → normal (baseline is 1–5 s);
   - arrives 10–60 s → watch only, still within the **normal worst case**
     (the worker may be occupied ≤60 s on one `timeoutSeconds: 60` request —
     ordinary HOL); do **not** conclude;
   - **absent ≥2 min** → declare **we are inside the silence window** (real
     windows are ≥10 min; normal max gap is 60 s — 2 min is a clean
     boundary with ~no false positives).
3. **At the moment of declaration (while the stall is live — it
   self-recovers, hesitation loses the evidence):**
   - `docker logs <engine> --since <window start> > stall-logs.txt`;
   - `docker exec <engine> kill -QUIT 1` → the Go goroutine dump goes to
     stderr → capture via `docker logs`; `restart: always` brings the process
     back (expected, same as 10-29; delivery resuming on restart
     re-confirms self-recovery);
   - record the exact arrival time of the probe token in the catch-up burst →
     the measured silence duration.
4. **Read the dump — three decisive frames:**
   - `handler.(*Service).Run`: parked in `select` on `<-updates` with an
     empty channel → **(B1)** update-stream gap (then check the go-tdlib
     receiver / session goroutines: alive and delivering, or wedged); parked
     inside a concrete call frame (RPC / lock / IO) → **(B2)** Run-loop stall
     — the frame names the blocker.
   - the auth goroutine (`auth.(*Service).run`): alive in its select →
     session alive; in a Closed state → the session-death variant of (B1).
   - log grep in the window: `Ruleset reloaded` / `Failed to reload` →
     **(D)**; `Push submission rejected ... buffer_full` (expected **absent**
     — a full buffer is the *slow-continuous* signature and is logged at WARN,
     `dispatcher.go:164`).

**Production root cause: NOT yet determined.** The candidate set is now
(B1)/(B2)/(D) for the silence, with (A)+endpoint-HOL as secondary. The fix
direction accordingly shifts from "raise endpoint delivery rate" to
"**find and fix the update-stream / Run-loop / reload gap**" (plus, if the
dump confirms (B1), session-health observability/reconnect), with
endpoint-concurrency + timeout tuning demoted to a secondary ≤60 s HOL
improvement.

## What is NOT established (and how each is closed — no production timing campaign)

1. **Which of (B1)/(B2)/(D) produces the 10-minute silence** — the
   **active probe protocol above**: declare the window at the ≥2-minute
   probe-absence boundary, then capture the SIGQUIT dump *while the stall is
   live* (protocol step 3) plus the in-window log greps (step 4). One run
   costs one message + one log capture + one QUIT — a forensic one-shot,
   *not* continuous production timing collection (which is not realistic).
   For the secondary items ((A) album starve, endpoint HOL), a **controlled
   test** (slow receiver / backlogged shared queue) is the deterministic
   reproduction.
2. **The receiver's per-request response time `T`** (= the effective `B`) and
   whether it does synchronous work before 2xx — the black-box side, only the
   owner can confirm; supplied as a single number (one
   `curl -w '%{time_total}' -X POST` or the receiver's access log), not a
   measurement campaign.
3. **The delivery loss policy** (product decision) and the **ordering,
   duplicate, fast-ack, and idempotency contracts** — Phase 1 owner decisions
   that gate implementation.

**Configured facts (owner-supplied, 2026-10-30):** `timeoutSeconds: 60` on the
endpoint. Per `cmd/engine/main.go:146-163` this bounds both the worker ctx and
the `http.Client`, so a single attempt occupies the endpoint worker **≤ 60 s**
— which **excludes mechanism (C)** (a >10-minute single-request wedge) and
caps the endpoint-HOL contribution at 60 s.

The payload already carries `OccurredAt` (`service.go:351`), stamped at
`SubmitV2` time, in both the v1 six-key envelope and the v2 notification — so
the receiver-side arrival timestamp minus `OccurredAt` measures `T2→T4`
**with zero Budva code change** (the cheapest *post-deploy* verification aid);
the producer-side enqueue/pickup stamps (Phase 0b) add the `T2−T3` / `T3−T4`
split for the same post-deploy purpose.

## Relation to the 10-29 record

Supersedes nothing. This is a *finite-latency* follow-up to the completed
`10-29-budva-push-stall` (infinite self-deadlock, fixed at `415d90c`). Both can
be true at once: the deadlock is fixed and runtime-accepted, **and** the latest
build can still show finite minute-scale webhook latency from the pre-existing
serial per-endpoint worker + backlog. Team B's independent audit of the
*mechanism*: `research/team-b-webhook-latency-follow-up.md` (findings
TB-LAT-P1-01..04 / P2-01..02 corroborate the source anchors above). Team B's
independent audit of *this task's* research/design:
`research/team-b-team-a-report-audit.md` — **CHANGES REQUESTED**; the
corrections it required are applied in this revision (evidence boundary →
leading hypothesis, **controlled slow-receiver test as the evidence gate**
(production timing collection is not a realistic prerequisite and is not used),
rejected P95/P99 timeout rule, no preset concurrency, G/G/1 stability
notation, backlog as example, conditional receiver-throughput claim, A1 as
measurable acceptance, citation scope).
