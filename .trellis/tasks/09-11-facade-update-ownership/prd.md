# Facade update ownership and liveness

## Goal

Facade must continue serving its existing request API when no business-update consumer exists. A caller must not need to drain an unpublished internal queue to keep request handling alive.

This is a bounded repair of the facade zero-consumer failure, not a claim that every engine overload or SDK lifecycle failure has been repaired.

## Authorization and status

- The user accepted the goal and requested Trellis design plus independent review on 2026-09-11.
- The original planning turn completed design v2 and independent reviews without runtime changes.
- The user authorized implementation of the reviewed bounded scope with “开始推进” on 2026-09-11. Commit, remote, merge, and rollout gates remain separate.

## Requirements

- R1: Zero business subscribers is a supported facade operating state, including long idle periods with incoming Telegram updates.
- R2: Existing facade query/send operations, authorization handling, and bounded chat-readiness behavior retain their current contracts. Business-subscription absence must not disable necessary internal event handling.
- R3: Existing engine and LiveStack business consumers retain their event types, ordering, and delivery behavior. This repair must not silently introduce dropping, retries, or duplicate forwarding.
- R4: Request-only uses of the shared Telegram adapter, including stand, must not inherit a hidden business-consumption obligation.
- R5: Assembly intent is explicit and reviewable; external facade clients need no new subscription endpoint, configuration switch, or companion process.
- R6: Verification distinguishes request-service liveness, preservation of internal events, and unchanged business-consumer behavior. Process health endpoints alone are insufficient evidence.

## Constraints and exclusions

- Keep current TDLib and go-tdlib versions; no fork, custom pump, or dependency upgrade is authorized by this task.
- Do not change engine queue scheduling, capacity, overload/recovery policy, or acceptance/loss guarantees in this bounded repair.
- SDK send/close races and existing cancellation hazards remain explicit residual risks; do not call them fixed or substitute undocumented usage restrictions for a repair.
- No new external update streaming API, dummy drain, durable event log, or general subscription framework.
- Do not reopen the shelved sparse-history query investigation.
- Native verification requires the existing Linux/TDLib CI channel; Windows static checks are not runtime validation.
- Preserve unrelated dirty work and the bootstrap-guidelines task. Implementation iterates on a task branch, with green CI before any separately authorized squash/merge.

## Acceptance Criteria

- AC1: A request-only Repo consumes a controlled update burst larger than the previous blocking buffers without any business consumer, and still processes a tail internal event. Evidence uses the production update loop.
- AC2: Under that burst, send success/failure delivery and chat-readiness wakeups preserve their existing semantics. Disabling the listener or bypassing internal dispatch cannot satisfy the tests.
- AC3: With a real business consumer, relevant events retain order and completeness across a bounded pause and resume. A drop-on-full mutation must fail this check.
- AC4: Every production/support constructor has an explicit, correct role; invalid use is detectable rather than silently turning forwarding into a nil-channel wait.
- AC5: Existing internal tests and whole-repository vet pass in Linux CI; new behavior-binding tests have recorded red paths. API migration compile errors are not behavioral red proofs.
- AC6: Review and verification reports distinguish adapter-seam proof from untested native SDK/Telegram integration. No blanket pipeline-health claim is made.
- AC7: Package/spec documentation records the zero-subscriber contract and residual risks. Deployment verification and rollback instructions do not require a new consumer to keep facade alive.

