# 10-30 Problem-1 design — OpenChat lifecycle for ruleset sources

> Delivered via the Design Delivery Loop
> (`.trellis/spec/guides/design-review-loop.md`): Stage 0 Frame → Stage 0b
> precedent survey → Stage 1 draft → Gate A (dimensions) → Stage 2 revise →
> Gate B (adversarial) → Stage 3 converge. Verdicts persist to
> `research/<design>-dimensions.md`; precedent findings to
> `research/precedent-openchat-lifecycle.md`.

**Status**: Stage 0 (Frame) done; Stage 0b (precedent survey) in flight.

---

## Stage 0 — Frame

**Design goal (the benchmark):** Budva's Telegram session **owns the
acquisition boundary** of its ruleset sources, through the
**client-conformant** mechanism of each chat class — i.e. "what would an
official client do?". Every dimension below is scored against this goal, not
against a hypothetical non-client baseline.

**Converged mechanism (from the record — not re-litigated here):**
for **each channel/supergroup source chat**, `OpenChat` on Budva's own session
+ **keep open** (open once; re-open only at state-reset edges). The pinned
C++ engine (TDlib commit `22d49d5`) already owns gap-recovery and the
open-gated `getChannelDifference` re-poll (idle-open = unconditional re-poll,
:37423; >10 = unbounded FIFO, :36854/:36860; member/idle = keep all, :7206).
**Private/basic-group sources are NOT opened** (not open-state-gated; global
stream). Chat **type is diagnostic** — it selects which mechanism applies.

**Scope of THIS design** (the four lifecycle/stagger seams):

1. **Open-set derivation** — `RuleSet.Sources map[ChatID]*Source` stores no
   chat type; the open decision needs it → one `GetChat` per source to read
   `chat.Type` (channel/supergroup vs private/basic-group).
2. **Lifecycle edges** — where the open set is (re)applied:
   - **initial load** and **hot-reload add/remove**: both funnel through
     `cmd/engine/main.go:168` (`pushManager.Replace(configs, callback)`);
     the watcher is `main.go:183-196`.
   - **re-auth**: dialogs reset, `open_count` lost → full re-open; the client
     lifecycle lives in `internal/infra/telegram/repo.go` / `internal/app/auth`.
   - **process restart**: `open_count` is in-process → same as initial load.
3. **Stagger** — the initial / re-auth batch of N `openChat`s must not fire in
   the same instant (reuse `internal/service/limiter`).
4. **In-process state + failure edges** — the opened-set (idempotent
   re-open); error edges: chat inaccessible / chat closed / bot-guard /
   mid-batch interruption; hot-reload **removal** → `CloseChat`.
5. **Test/acceptance shape** — the open-set regression test asserts
   `OpenChat` is **CALLED** correctly for the ruleset's channel/supergroup
   sources; it CANNOT assert real-time arrival (needs live TDlib + server).
   Final acceptance = post-deploy observation on the box.

**Explicit exclusions** (re-test at convergence that they were not absorbed):

- hand-built Go `getChannelDifference` poller — the engine owns it (rejected);
- Problem 2 (endpoint HOL) — separate leg, downgraded, does not gate this;
- asserting real-time delivery in any test — box-only acceptance;
- any read API (`readHistory`-class) — conformance = Budva never calls one;
- shrinking the open set to non-member channels only — resolved: keep ALL
  (member/idle silence is a real risk; the volume saved is not worth it).

**Evidence hierarchy for this design** (Stage 0b + Gates):
pinned source / normative contract **>** mature-project precedent **>**
issue reports / maintainer paraphrase. Precedent corroborates but does not
close a load-bearing claim; closing requires the source/contract layer.

**Exit (Stage 3) = all five**: every load-bearing claim traceable to
evidence; no ENGAGED dimension unresolved; exclusions honored; every
non-obvious mechanism cites a precedent (or a source-verified reason it is
novel); acceptance signal named (CI-assertable vs box-only).

**The owner go-ahead to implement is a decision made AFTER Exit — it is not a
loop input.**

---

## Stage 0b — Precedent survey

*(in flight — output: `research/precedent-openchat-lifecycle.md`)*

## Stage 1 — Draft

*(pending Stage 0b; each non-obvious mechanism must cite precedent or a
source-verified reason it is novel)*

## Gate A / Gate B verdicts

*(pending — output: `research/design-dimensions.md`)*
