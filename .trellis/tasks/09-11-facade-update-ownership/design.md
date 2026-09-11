# Design: explicit business-update ownership

Status: reviewed design v2, implementation authorized by the user on 2026-09-11. Independent reviews of v1/v1.1 approved the bounded design; v2 incorporates their nonblocking corrections and verification clarifications. See research/review.md for dispositions and limits.

## 1. Decision boundary

The accepted requirement is facade liveness without a business subscriber. The proposed implementation removes this specific unused publication edge while preserving the shared adapter's internal event handling and the existing subscribed behavior.

This task does not establish a general non-blocking TDLib runtime. Engine overload and SDK send/close races remain separately identified risks. Neither a successful local test nor a narrower task name can close those risks.

The existing binary responsibilities support this boundary: facade serves request APIs, engine runs forwarding handlers, stand performs fixture-management requests, and LiveStack consumes updates for mappings/edit/delete checks. README.md explicitly describes this split within the message-forwarding product and separate TDLib instances for facade and engine. Product-wide forwarding does not require an unused forwarding outlet in each process. See research/findings.md and the independently researched responsibility table.

## 2. Three distinct layers

1. The SDK's native receive pump and response dispatcher deliver request responses and incoming updates.
2. Repo's existing SDK listener processes internal send results, chat-readiness notifications, and convergence observations.
3. Repo's optional business outlet supplies filtered events to engine or LiveStack.

Only layer 3 becomes optional. Do not disable layer 1, remove layer 2, or treat a call to Updates() as a dynamic subscription operation.

## 3. Construction contract

Proposed internal Go API:

~~~go
type UpdateMode uint8

const (
    NoBusinessUpdates UpdateMode = iota
    BusinessUpdates
)

func New(cfg config.TelegramConfig, mode UpdateMode) *Repo
~~~

- Make mode a required argument, not a variadic option with a silent default. Migration compilation forces every current caller to choose.
- NoBusinessUpdates leaves the private business channel nil: no queue, no business worker, no discard loop.
- BusinessUpdates creates the same capacity-100 channel as today, before Start. Do not make capacity configurable in this task.
- Reject an invalid enum value immediately as an internal programming error. No environment variable or transport option selects this mode.
- The mode is immutable for Repo lifetime. Do not infer it from channel length, the number of readers, or whether Updates() was called.
- Keep the existing Updates() signature for subscribed callers. Calling it on a NoBusinessUpdates Repo is an assembly programming error and fails immediately with an explanatory panic; do not return a nil channel that makes handler.Run silently wait forever. This is not an error exposed by the facade request interface, which has no Updates method.
- Do not add a subscriber registry, consumer callback interface, registration locks, dynamic unsubscribe, or new channel-close ownership.

| Construction site | Mode | Existing owner of business events |
| --- | --- | --- |
| cmd/facade/main.go | NoBusinessUpdates | None; request service only |
| cmd/stand/main.go | NoBusinessUpdates | None; fixture-management requests |
| cmd/engine/main.go | BusinessUpdates | handler.Service.Run |
| internal/test/support/live_stack.go | BusinessUpdates | LiveStack.processUpdates |
| Telegram package tests | Explicit per case | Fixture-specific; not blindly all enabled |
| Package usage example | NoBusinessUpdates, with a separate subscribed example | Document both obligations |

This is an internal-package source migration, not a public RPC/schema change. The table is part of the review gate: compile success or generic constructor tests cannot prove the actual binary selected the correct mode. The disabled-access panic detects misuse when Updates() is called; handler waits for ClientDone first, so this is not a pre-authentication startup check. An incorrectly enabled facade that never calls Updates() is detected by assembly review and the actual-binary acceptance probe, not by that panic.

## 4. Update processing

Keep the existing production listenUpdates loop and its listener lifetime. On each update:

1. Dispatch SendMessageAndWait success/failure results through the existing pending-send path.
2. Process UpdateNewChat using the current readiness table and convergence tracker, with their existing semantics.
3. If the business outlet is absent, continue receiving without attempting any business send.
4. Otherwise preserve the current relevance filter and ordered blocking send into the existing business channel.

The absence guard must be after required internal dispatch and before the business send. Sending to a nil channel blocks forever; leaving allocation out without guarding publication is an invalid repair.

Do not put a default/drop branch on the subscribed path. Do not add a goroutine per update, a second staging queue, an unbounded list, or a background drain. Do not change engine shutdown/drop semantics as a side effect of this task.

An immutable private nil/non-nil outlet can encode the mode after construction; an extra mutable boolean is unnecessary. No authorization, session-replacement, pending-send, or readiness lifecycle mechanism is redesigned. Removing a permanent publication stall can make deferred listener closure reachable again; do not claim unchanged race frequency or newly safe cancellation. See research/concurrency-boundaries.md.

## 5. Internal obligations and compatibility

- Keep authorization event delivery and the auth-service consumer as currently assembled.
- Preserve the readiness edge in addition to the ticker fallback. Do not silently remove an existing readiness accelerator to avoid maintaining the listener.
- Keep the existing private send-result route independent of the business filter/outlet as shared-Repo compatibility; LiveStack is a verified SendMessageAndWait caller. Stand performs fixture-management requests, and facade sends retain their initial-call return semantics. Do not invent a stand send-completion obligation or a new facade final-delivery guarantee.
- Do not alter raw facade send return semantics, add retries, or interpret missing confirmation as proof of failed delivery.
- Existing business event types, per-listener order, filter rules, and channel capacity remain unchanged for engine and LiveStack. Per-listener publication order is not a promise of asynchronous handler completion order, exactly-once forwarding, or final delivery.
- A subscribed consumer can still stall the shared pump in that process. Preserving this known behavior is not approval of it as a long-term architecture; resolving it requires a separately explicit overload/acceptance contract.

## 6. Observability

- Record the chosen business-update mode once in structured startup logging, without account/chat identifiers.
- Backlog is a queue observation, not a request-service health signal. Record it only for an existing business outlet; absence of a series or a zero must not be presented as end-to-end health.
- No drop counter is introduced: an event for which no business delivery was requested is not a dropped subscribed event. Internal events still execute.
- This repair must not rely on an exporter/alert to maintain liveness. The earlier claim that metrics are necessarily no-op is unverified; actual exporter configuration and scraping are deployment facts.
- Deployment checks use a real request probe, not only /live, /ready, or /healthcheck.

## 7. Verification design

### 7.1 Adapter-seam tests in the existing internal test scope

Drive the actual listenUpdates loop through clientAdapter.GetListener using a controlled SDK Listener fixture. The pinned source permits &client.Listener{Updates: ch} with a non-nil channel: the zero-value mutex suffices, and Close requires no private initialization. The literal is inactive and is not registered with the SDK receiver; its purpose is to exercise Repo, not native fan-out. Let the production loop be the sole fixture closer and stop/join synthetic producers before cancellation. Do not introduce a production event framework just to make testing easier.

- T1 — construction: both modes, channel allocation/capacity, unchanged auth/client-done initialization, invalid mode, and Updates misuse detection.
- T2 — idle request-only burst: no reader of the business outlet; feed 4096 relevant updates, then an internal tail marker. Assert all input advances and the tail has its actual internal effect. Keep injection completion, effect completion, and loop exit distinct; GetListener acquisition and ClientDone are not processing barriers. Use synchronization and virtual deadlines, not wall-clock sleeps. The count exceeds the known 100/1000/1000 buffers but does not emulate or prove the native global pump.
- T3 — internal semantics after the relevant burst: with no business outlet, establish real pending-send entries before injecting matching success/failure events through the listener, and check the permanent result/error plus entry removal. Establish a same-chat readiness waiter and an explicit materialization-state oracle; inject UpdateNewChat through the listener and require operation success before the first ticker opportunity. Helper calls, eventual polling success, or call-ordinal-only fixtures do not pass.
- T4 — subscribed compatibility: pause the consumer, fill the capacity-100 channel, and synchronize an attempted 101st relevant publication while it remains full. Use a controlled input handoff plus quiescence/barriers; producer enqueue completion or len(outlet)==cap(outlet) alone is insufficient. Only then resume, observe an end-of-processing marker, and assert exact event identities/contents/order/count and absence of extras. Feeding exactly 100 before resuming cannot expose drop-on-full. Include relevance filtering, send-success business delivery plus internal delivery, edit events, and permanent versus non-permanent deletes. Bound the pause; do not require indefinite-stall liveness from this task.
- T5 — lifecycle of the test fixture: cancel the normal receive wait, join the loop, and ensure the fixture has no concurrent send when the SDK listener is closed. This is test cleanup, not a proof that SDK cancellation races are repaired.

The fixture must make lack of progress observable without leaking goroutines into other tests. Do not close its input to stop the loop: the existing deferred Close would close it again. In enabled-mode saturation cases, resume business draining before waiting for normal cancellation. Failure-path cleanup must not rescue a stalled publication before recording the failed assertion. The pinned-source fixture is feasible; implementation must still compile and validate it in Linux. Do not quietly replace the production-loop requirement with direct helper tests.

Required failure-path protocol: install teardown before assertions/producers; give synthetic sends their own stop path; stop and join producers; record a failed progress/missing-event verdict before rescue; if the actual private business outlet is non-nil and blocked, drain it during teardown so loop cancellation can complete; then join the loop and any cleanup worker. Do not use the disabled public Updates() accessor or Repo.Close as a cleanup shortcut, and do not close the business outlet. The prescribed old-behavior control restores both allocation and publication, so its outlet is drainable. An optional nil-outlet/bare-send control cannot be rescued by cancellation or another channel and needs bounded isolated execution if included. Detailed source-specific test preconditions are in research/review-validation.md.

### 7.2 Required negative controls

Keep the migrated API so these are behavioral failures, not compiler failures:

- Restore unconditional allocation/publication: T1/T2 must fail for allocation or blocked progress.
- Bypass production listening or move the absence guard before internal dispatch: T2/T3 must fail on the missing tail/result/readiness event.
- Add drop-on-full to the enabled path: T4 must fail on missing event identities/count.
- Omit a required constructor migration: compilation fails, but record this only as migration coverage, not a liveness red proof.

Record the red reason and corresponding green run in the implementation review. Strict-mock reasoning can explain a red path; it must not be reported as an executed mutation.

### 7.3 Native evidence boundary

Existing Linux CI runs whole-repository vet and internal tests against native TDLib libraries. That compiles the adapter and exercises the controlled listener tests; it does not by itself exercise the real SDK receive pump or a live Telegram account.

Before production closure, use a separately authorized isolated test-account/session run: start the actual facade binary without a business consumer, allow a known relevant update burst exceeding the original failure threshold while making no request calls, then perform repeated existing gRPC requests and a cold-chat readiness check. Predeclare the incoming load, actual client-backed request, request count, latency/error acceptance bounds, and cold/waiting precondition. Require expected successful content for valid requests against known test data, such as GetChatHistory; fast validation errors and health 200s are not request-response progress. Capture latency/error output, the request/response expectation, and binary revision; do not send test traffic to production chats. If such an environment is unavailable, report native end-to-end acceptance as pending rather than substituting health endpoints.

## 8. Alternatives considered

| Alternative | Decision and falsifier |
| --- | --- |
| Add a real consumer | Valid if facade gains an actual business responsibility; no such obligation is present in the accepted request-service contract. Reopen if a concrete missing responsibility is found. |
| Add a drain | Avoids this block mechanically but adds a permanently running component whose only job is to discard an unused output. Not selected. |
| Dynamically allocate when Updates() is first called | Changes accessor semantics, introduces registration/startup races, and can lose startup events. Not selected. |
| Variadic opt-in/default-disabled constructor | Smaller source migration but silently disables an omitted engine/LiveStack subscription. Required typed mode makes the migration explicit. |
| Separate request/streaming Repo types | Stronger compile-time capability separation, but duplicates or broadens current interface/assembly changes for this one bounded need. Revisit only if disabled-access misuse is a recurring problem. |
| Disable all Repo listening | Violates internal-event compatibility; prohibited by T3. |
| SDK fork/custom pump/close patch | Not necessary to remove this confirmed unused outlet. Independent runtime risks remain; new evidence of an inseparable regression would block this scope and return to planning. |

## 9. Rollout and rollback boundary

- Implementation starts only after the user accepts this scope/design and the Trellis review gate. Work on a task branch; do not switch or stage unrelated dirty files implicitly.
- No configuration or persisted-data migration; facade API and storage shape are unchanged.
- Deploy only a separately approved, CI-green image. Verify chosen mode in startup logs and real request progress after controlled update load.
- Stop rollout on loss of internal event behavior, missing engine/LiveStack events, or renewed timeout signatures. Roll back to the prior image only as an incident-control action: it restores the known idle-freeze bug and requires an explicit follow-up, not a claim of health.
- SDK close races, engine overload, and full shutdown/session ownership remain listed residual risks. This task's closure wording must remain facade-zero-consumer specific.
