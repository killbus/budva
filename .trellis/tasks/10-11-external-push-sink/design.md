# Design: external push sink (fire-and-forget dispatcher)

Status: converged across three dbs-chatroom review rounds (Kleppmann / Kent Beck / Beyer / Hohpe) on 2026-10-11; user approved with "同意". Judge dispositions are recorded per section. See `prd.md` for requirements and authorization.

## 1. Decision boundary

The accepted requirement is external delivery of forwarder events (new/edited/deleted) as data, not metrics export and not a Telegram feature. The push channel is additive: the Telegram forwarding pipeline, its serial queue, the per-chat limiter, and edit/delete sync are untouched except for non-blocking event emission at three integration points.

This task establishes a v1 webhook sink with at-most-once semantics. It does not establish durability, replay, ordered delivery, or a translator framework for product-specific payloads. Those are separately triggered upgrades (§11).

The analogy the user chose — mature projects feeding data to consumers — resolves to a push webhook, not a Prometheus-style pull surface: the consumer is a downstream receiver, not a scraper.

## 2. Converged decisions (traceability)

| # | Decision | Rounds | Rejected alternatives |
| --- | --- | --- | --- |
| D1 | One canonical envelope; all sinks speak it | R1 all four | Dedicated per-product sinks (Kleppmann: layering question, not parallel option) |
| Delivery semantics | At-most-once with explicit terminal outcomes: buffer-full rejection, retired-generation rejection, one successful or failed delivery attempt, and process-boundary loss | R1 user, audit correction | Retry/backoff queues; durable buffers; exhaustive two-drop-point wording |
| D2 | Self-contained events (edited carries full text) | R1 Kleppmann | Delta events; notification-only "something happened" |
| D3 | No ordering promise in contract; occurred_at LWW downstream | R1/R2 Kleppmann, judge | "Best-effort ordering" wording (more dangerous than explicit unordered); per-rule sequence numbers (drop-induced gap ambiguity) |
| D4 | v1: single worker per endpoint; order comes free | R2 Beck, judge | N workers + chat_id hashing now (Hohpe: kept as documented upgrade path); N workers unordered |
| D5 | Route resolution at enqueue time from the atomic snapshot | R3 all four, Hohpe framing | Send-time live config read (decision/act split across reload) |
| D6 | Immutable dispatcher generation with concurrency-safe `Submit`/`Retire`; retire blocks new leases, waits for active submitters, then closes and drains channels | Audit correction | Closing snapshot-owned channels immediately after publication; channel registry keyed by URL+token+headers; silent drop-on-reload |
| D7 | Webhook sink with one bounded-time HTTP attempt for every accepted event; no retry and no circuit-breaker fast rejection in v1 | Audit correction | Cooldown breaker that converts accepted events into zero HTTP attempts; retry/backoff queues |
| D8 | Security: URL sensitive in yml, TLS forced, redirects ≤3, reject `http://`, HMAC switch default off; raw TDLib payload excluded from v1 | R2 Beyer + audit correction | HMAC by default; TLS off-switch; plaintext-http endpoints; unbounded opaque TDLib serialization |
| D9 | Outcome-specific counters + reload counter; last-success absent or zero until a real successful delivery | Audit correction | Initializing last-success to process start; treating all loss and failure paths as two undifferentiated drops |
| D10 | MQ trigger clause written narrowly, not pre-built | R1 all, Hohpe R2 | Deleting the clause (invites re-litigation); building MQ adapter speculatively |

## 3. Architecture

~~~go
// internal/service/push (or internal/app/push — package placement at implement review)
type Envelope struct { ... } // §4

type Sink interface {
    Push(ctx context.Context, env Envelope) error // one method; worker applies timeout, logs error
}

type Dispatcher struct { ... }
// per ruleset snapshot: one bounded channel + one worker goroutine per endpoint
~~~

~~~text
processMessage / editMessages / deleteMessages
        │  non-blocking enqueue (select with default)
        ▼
generation.Submit(envelope)            ── submit-time: snapshot-fixed recipients;
        │                                  acquire short-lived generation lease
        ├─ retiring → reject + log + count
        ▼
generation.channels[endpoint]          ── buffered chan (capacity 256, bounded)
        ├─ full → reject + log + count
        ▼
single worker goroutine per endpoint   ── serial drain: per-endpoint order is incidental
        │  one bounded-time HTTP attempt
        ▼
WebhookSink.Push ── HTTP POST JSON ──► external endpoint
        ├─ success → record actual last-success time
        └─ failure → classify + log + count, no retry
forced process exit → accepted in-memory events may be lost
~~~

Dependency direction is fixed: the handler/kernel imports the envelope type only, never the sink implementation package (Hohpe). The engine wires `Dispatcher` as a new dependency of `handler.Service` (constructor parameter), emitting at three points:

1. `processMessage` — after the FiltersOK branch resolves, per rule (emits `new`; emitted at enqueue-time decision, not after Telegram send success — the push event describes the source-side fact).
2. `editMessages` — after copies resolve and text is available (`edited`).
3. `deleteMessages` — for each permanently deleted source message (`deleted`, `text` empty).

Emission is a non-blocking submit; it must not add blocking, ordering, or error paths to the existing pipeline (R3, AC9).

## 4. Envelope (canonical data model)

~~~json
{
  "schema_version": 1,
  "event": "new | edited | deleted",
  "chat_id": -1001234567890,
  "message_id": 12345,
  "occurred_at": "2026-10-11T12:00:00Z",
  "text": "full formatted text (empty for deleted)"
}
~~~

- `chat_id` + `message_id` are the correlation key for edited/deleted; they are never omitted (Hohpe's iron rule).
- `edited` carries the full post-edit text: events are self-contained facts ("as of occurred_at, message X reads Y"), not deltas (Kleppmann). A consumer that missed the `new` can still apply the `edited`.
- `deleted` has `text` empty. Absence is part of the schema (Hohpe: the canonical model must declare what is not there).
- `occurred_at` is the single LWW key for downstream conflict resolution. It cannot distinguish "quiet source" from "lost events" — recorded in the downstream notes as a structural caveat (Kleppmann R3); `session_id` remains a v2 signal, not built here.
- `schema_version` exists from day one; envelope evolution bumps it.
- Product-specific display intent (ntfy priority, Slack blocks) never enters the envelope — that is translator territory, deferred.
- Raw TDLib objects are excluded from v1. A future raw field requires a versioned schema, explicit opt-in, field allowlist and redaction policy, payload-size bound, compatibility policy, and defined serialization-failure outcome before it can be added.

## 5. Routing and snapshot lifecycle

- `pushTargets` is a new top-level ruleset section; rules reference endpoints via `pushTo: [name, ...]`. Names resolve at load; `ruleset.Load` validation rejects unknown references, `http://` URLs, and missing URLs, so a validated snapshot is fully wired.
- Each validated ruleset snapshot references an immutable dispatcher generation. The generation owns one bounded channel and one worker per endpoint; the Envelope does not contain routing data.
- `Submit` acquires a short-lived submission lease from the generation. Acquisition fails immediately when retirement has begun. While holding the lease, `Submit` performs only non-blocking channel sends and then releases the lease. A full buffer is an enqueue rejection; a generation that is already retiring is a retired-generation rejection. Both are counted and neither blocks the Telegram pipeline.
- `Retire` is idempotent and follows one close-side ownership sequence: atomically mark the generation retiring so no new lease can be acquired; wait for all already-acquired submission leases to release; close each owned channel exactly once; let workers drain accepted events with `for range`; wait for workers to exit. Channel closure therefore cannot overlap any sender that acquired the generation.
- Reload builds and validates a fresh snapshot and generation first, atomically publishes the replacement, and only then retires the old generation. A handler may retain an old snapshot across publication, but its later `Submit` either completes under a lease acquired before retirement or is rejected because retirement has started. It can never send to a closed channel.
- Shutdown uses the same `Retire` protocol. Endpoint workers drain concurrently. For each endpoint, graceful retirement is bounded by at most one in-flight attempt plus the bounded channel capacity, multiplied by the configured per-attempt timeout; with the default capacity 256 and timeout 5s, the conservative per-endpoint upper bound is 257 attempts × 5s. This is a correctness bound, not a target shutdown latency. Abrupt process termination may lose accepted in-memory events and cannot reliably increment a post-mortem counter.
- Terminal outcomes are explicit rather than compressed into an exhaustive “two drop points” claim: enqueue rejection, retired-generation rejection, one successful delivery attempt, one failed delivery attempt, or process-boundary loss. Reload itself does not silently lose accepted events.

## 6. Sink contract and webhook behavior

~~~go
type Sink interface { Push(ctx context.Context, env Envelope) error }
~~~

- Webhook implementation: `POST` JSON, `Content-Type: application/json`, short timeout (default 5s, config), redirects capped at 3 hops, TLS verification via system roots with no bypass switch, `http://` rejected at load.
- Every event accepted into an endpoint buffer receives exactly one bounded-time HTTP attempt. v1 deliberately has no circuit breaker or cooldown fast-rejection state, because that would violate AC3 by converting accepted events into zero HTTP attempts. Endpoint throughput remains bounded by the single worker and configured request timeout.
- Response and failure classification: only a final 2xx response is successful. HTTP 4xx, HTTP 5xx, redirect-policy failure, timeout, and TLS/transport failure have separate classes, counters, and one log line per failed attempt. A failed attempt is not described as an enqueue drop and is never retried.
- HMAC signing: optional per-endpoint switch (`hmacSecret`), default off; when on, `X-Budva-Signature: sha256=<hex>` over the raw body. No verification, no replay concerns (at-most-once has no replay window — Beyer R2).
- ntfy/Bark/Slack adapters are Message Translators, not sinks; deferred without stubs.

## 7. Configuration

~~~yaml
pushTargets:            # new top-level section, loaded and validated with the ruleset
  ntfy-main:
    url: https://ntfy.example.com/mytopic
    timeoutSeconds: 5
    headers: { Authorization: "Bearer ..." }   # optional static headers
    hmacSecret: ""                              # optional, default off
forwardRules:
  rule-1:
    from: -1001234
    to: [-1005678]
    pushTo: [ntfy-main]      # new; name references, validated at load
~~~

- URLs are bearer secrets: ruleset.yml carries a sensitive-data marker in its header comment; the repo keeps only the empty template (`ruleset.yml` at repo root stays empty); deployment guidance requires 0600 and real configs out of git (R8).
- Validation failure keeps the current table (existing `Load` error path); configuration and references are validated before candidate workers start. If candidate startup fails after partially starting workers, the unpublished candidate generation is retired completely before returning the error, while the current published generation remains active. Emit a `reload_failed` log with reason (R6). Success logs one diff line: endpoints added/removed (Beyer R3).

## 8. Observability

All instruments register through the global OTel meter, same contract-first pattern as `internal/infra/telegram/metrics.go` (registered now, exported once `METRICS_PORT` is set):

| Instrument | Type | Labels |
| --- | --- | --- |
| `budva.push.enqueued_total` | counter | endpoint |
| `budva.push.rejected_buffer_full_total` | counter | endpoint |
| `budva.push.rejected_retired_total` | counter | endpoint |
| `budva.push.delivery_failed_total` | counter | endpoint, class(http_4xx/http_5xx/redirect/timeout/tls_transport) |
| `budva.push.buffer_depth` | gauge | endpoint |
| `budva.push.send_duration_ms` | histogram | endpoint |
| `budva.push.last_success_unix` | gauge | endpoint — absent or zero until the first successful delivery, then set to that success time |
| `budva.push.reload_total` | counter | outcome(ok/failed) |

Alert guidance (documentation, not code): distinguish “never succeeded” (`last_success` absent or zero after the startup grace period) from “previously succeeded but stale.” TLS failures and receiver rejections such as an HTTP 4xx response to an invalid signature count only — no page by default.

## 9. Security

- Threat model: the push URL is a bearer credential (an ntfy topic is a password); envelopes carry private chat content in plaintext.
- TLS mandatory, no switch; attackers cannot downgrade, operators cannot silently disable.
- Redirects ≤3 hops; Go's client already strips Authorization cross-domain — keep the cap as the only redirect logic.
- HMAC optional switch (§6). Default off is a decision, not an omission: at-most-once has no replay window and single-tenant receivers gain nothing (Beyer R2).
- Endpoint URLs, static authorization headers, and `hmacSecret` are sensitive values stored in the external ruleset configuration by design. Real ruleset files remain out of git, carry a sensitive-data marker, and require 0600 deployment guidance. Existing engine credentials such as `TELEGRAM_API_HASH` continue to use their current envconfig path; this task does not migrate secret storage.

## 10. Downstream contract (verbatim, ships in package doc)

> Delivery is at-most-once and events may be lost. Events are self-contained: an `edited` carries the full current text. Order is not guaranteed — apply last-write-wins by `occurred_at`; tolerate events without predecessors. To render deletions, cache received messages yourself; `deleted` envelopes carry only identifiers. A submission can be rejected because its endpoint buffer is full or its dispatcher generation is retiring. An accepted event receives one HTTP attempt, which may fail and is never retried. A process boundary can truncate the in-memory stream, and no persistence is built.

"Best-effort ordering" language is banned: it trains consumers to assume order that drops will break (Kleppmann R2).

## 11. Documented upgrade paths (triggers, not backlogs)

- **MQ sink**: when the downstream already operates a bus with persistence and fan-out, or replay / a second full-stream consumer appears. Same `Sink` interface, second adapter; the clause exists so this is not re-litigated (Hohpe R3: written as a trigger, not an option).
- **chat_id-partitioned workers**: when a single endpoint cannot keep up (no such signal today). Preserves per-chat order; cross-chat stays unordered (Hohpe R2).
- **session_id field**: when a downstream builds session state and needs stream-truncation visibility. Backward-compatible field addition; occurred_at's silence-ambiguity caveat is the recorded rationale (Kleppmann R3).
- **Product translators** (ntfy/Bark/Slack templates): when a real user reports a product-specific rendering gap, not speculatively (Beck R2).

## 12. Verification strategy

Acceptance correction (2026-10-12): original hook locations do not exclude
webhook-only edits or album events. Separate source-event eligibility from copy
bookkeeping; specify edit filtering and emit album member text/caption without
changing Telegram batching. Keep schema_version=1 for this repair. Versioned
rich content/media/relationships are owned by the linked protocol task.

### P10-P11 source eligibility (2026-10-12)

- New single messages retain the existing source, supported-text/content,
  saveability and `FiltersOK` guards. Protected-message routing is not changed
  in this correction.
- Edited events use every forward rule whose `From` matches the captured
  source chat, irrespective of copy mappings, `To`, `CopyOnce` or saveability.
  Supported text/caption is required; an empty caption is valid. Do not re-run
  new-message filters: an edit leaving a filter or clearing its caption must
  update a consumer that already holds the message. As with deletion, consumers
  can receive edits without a matching new event. No eligibility history is added.
- Album events use the captured rule/dispatcher after the existing quiet-period
  collection. Each unique `(chat_id, message_id)` in that rule's album bucket
  emits its supported text/caption only when `FiltersOK`; empty captions are
  evaluated normally. Filter checks apply only to push, never to Telegram album
  forwarding. The Telegram member list, timing, queue, limiter, mapping and
  statistics remain unchanged. Duplicate members are suppressed only in the
  push loop; there is no durable/global duplicate cache or retry.
- Album new facts share the bucket's first-ingress timestamp captured before
  queueing collection. This is a conservative bucket timestamp, not an exact
  per-member receipt time: no per-member state is added. Collection completion
  time would incorrectly let an old caption overwrite later edits/deletes under
  the existing downstream `occurred_at` LWW contract.
- With push disabled, preserve original message-service/filter call patterns.
  Submit carries the captured dispatcher even if the live snapshot has changed;
  retired-generation rejection remains the established at-most-once outcome.

- Unit: envelope marshaling (schema fields, deleted-text-empty, no raw TDLib object), enqueue and retired-generation rejection accounting, `Submit`/`Retire` lease invariants, idempotent retirement, config validation (`http://` rejection, unknown `pushTo` reference), and zero/absent last-success before first success.
- Behavior (red-first per repo conventions): AC2 full-buffer non-blocking rejection; AC3 exactly one HTTP attempt plus failure classification and no retry; AC4 concurrent old-snapshot submit/reload/retire interleavings under `go test -race`, drain of accepted events, retired-generation rejection, invalid-reload retention, and goroutine-exit assertions; AC5 endpoint isolation (stall A, assert B's latency); AC1 three-event flow against `httptest.Server` through production emit points.
- Pipeline invariance: retain the existing delivery/edit/delete/statistics suites unchanged. Add a parameterized regression that injects push buffer rejection, timeout, and delivery failure and compares Telegram send/edit/delete/statistics outcomes plus serial-queue and per-chat-limiter scheduling with push disabled.
- Export check: `METRICS_PORT` set → `/metrics` exposes `budva.push.*`; last-success is zero or absent before success and changes only after a successful response. Linux CI is the runtime validation channel; Windows static checks are not runtime evidence.
