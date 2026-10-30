# Diagnose push pipeline stall (deliveries stopped reaching the target)

## Goal

2026-10-29: budva engine deliveries to the configured push target stopped.
The engine stays alive and the log stays clean, but
new messages from the active source chats no longer reach the configured
endpoint; a manual container restart immediately restores delivery and the
stop recurs after uptime. Diagnose where in the budva producer the pipeline
stopped, and deliver the minimal fix.

The target endpoint is treated as a black box from this task: "accepted",
"rejected (class)", or "unreachable/timeout".

## Requirements

1. Diagnosis-first: record producer-side evidence under `research/` before
   any fix commit.
2. Trace the budva pipeline to the exact stop point, anchored to
   producer-side observations (container state, engine log, goroutine
   stacks, the endpoint's boundary behavior). The engine exposes no pprof
   endpoint — a Go SIGQUIT goroutine dump taken while stalled
   (`docker exec <engine> kill -QUIT 1`) is the decisive probe.
3. Deliver the minimal fix, or a precise reproduction + root-cause writeup
   when the fix needs a decision.

## Acceptance Criteria

- [x] Stall explained: specific stop point + cause + proof (the SIGQUIT
      dump taken while stalled; see `research/root-cause.md`).
- [ ] After the fix, a fresh send to a source chat produces a new
      delivery accepted by the target, verified by producer-side signal
      and the user's own confirmation on the target's side. (The fix is
      committed; CI, deployment, and endpoint-side confirmation remain
      pending.)
- [x] No target-side change made from within this task.

## Notes

- Scope boundary: producer side only. The target's internals are out of
  scope and must not be re-derived or assumed.
- Follow-up (not a closure gate): is there a missing signal (log/metric/
  alert) in budva that would have surfaced this stall early? — answered in
  `research/root-cause.md` (log-silent success path + unconfigured metrics
  backend; no dead-man's-switch on last successful delivery).
