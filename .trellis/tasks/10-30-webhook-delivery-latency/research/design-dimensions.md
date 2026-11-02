# 10-30 Problem-1 design — Gate A / Gate B verdicts

Design Delivery Loop gates for the Stage 1 draft
(`design.md`, "Stage 1 — Draft"). Precedent file:
`research/precedent-openchat-lifecycle.md`. Pinned engine: TDlib commit
`22d49d5` `MessagesManager.cpp` (cached reads cited as `mm.cpp:<line>`).

---

## Gate A — dimensional review (2026-11-02)

**Room**: Munger / Popper / Kahneman / Franklin (dbs-chatroom, 4 parallel
experts, self-contained briefs with the draft's load-bearing decisions, the
0b findings, and the orchestrator's pre-seeded questions Q1–Q3 + owner
decisions D1–D2). **Orchestrator independent checks** (not agent
paraphrase): `transform()` key negation (repo.go:219), `runAuthLoop`
one-shot + CleanUp (repo.go:342-371, :183-190), the adapter-swap comment
(repo.go:355-361), `Updates()`/`AuthStates()` compete-not-broadcast
(repo.go:140-148), `auth/service.go:66-68` first-Ready-then-return,
`pushManager.Replace` synchronous-under-lock (push/manager.go),
`open_dialog` silent early-return on `!have_input_peer` (mm.cpp:19527),
unload guard `open_count == 0` (mm.cpp:33400), `ChatTypeSupergroup.IsChannel`
(go-tdlib `client/type.go`).

**Outcome: CONDITIONAL PASS.** 7 dimensions hold (A/E/F/H/I/J/K); 4 hold
with load-bearing caveats (B/C/D/G); no N/A, no outright fail. The caveats
are the nine revisions applied in Stage 2 (R1–R9 below). Gate B runs after
the Stage 2 revision.

### Dimension verdicts (consensus, with the source of each finding)

| # | Dimension | Verdict | Findings |
|---|-----------|---------|----------|
| A | Layering (native vs glue) | **Holds** (all 4) | `SourceOpener` in infra/telegram, 1:1 adapter methods, no own timer (re-poll is engine-owned), pure classifier in domain — the draft's strongest dimension. |
| B | Protocol / state-machine fidelity | **Conditional** | Pinned pins verified true (open_dialog:19524, re-arm:37423, FIFO:36854/36860, gap-gate:7206). §2 table self-contradictory (R1). "open_count zeroed after swap" was cited as a reset path; the true reason is **constructive** — a fresh C++ client instance per `runAuthLoop` (repo.go:355 comment: new client's dialog map is empty). |
| C | Lifecycle & sequencing | **Conditional** | Single driver + diff idempotency correct. **Trigger point was wrong**: re-open hung on `AuthStateReady`, which the repo's own comment (repo.go:355-361) says "lags the whole handshake and can be lost" — the reason the dialog-table swap is at the swap line, not Ready (R2). Network mid-session reconnect is **not** an edge: the unload guard (mm.cpp:33400) protects `open_count > 0` dialogs, so a live client self-heals; the edge set is exactly {startup adapter-swap, hot-reload diff} (R2). |
| D | Concurrency & boundaries | **Conditional** | Single opener + mutex holds (avoids the 10-29 reentrancy class). `authStates` is single-consumer (auth service stops after first Ready, auth/service.go:66-68) — the opener must not reuse it; repo callback only (R3). §2 boundary off-by-one (R1). |
| E | Resources / backpressure / scale | **Holds** | No open-set shrink; >10 deferred by engine FIFO (no starvation) as a declared trade-off; no unbounded growth. Large-N (20–50) catch-up tail latency is an owner-informed-acceptance item, not a design defect (R8). |
| F | Failure & rollback | **Holds** | Never fail the ruleset load; pending set; bot-guard fatal-to-feature (loud, correct). **Caveat folded into R4**: "retry on the next edge" does not converge when no next edge exists (single source, no reload, no re-auth) — in-edge bounded retry is REQUIRED. |
| G | Observability & acceptance | **Conditional** | CI/box split honest; "no read API in client log" is a good box signal. **Fake CI identified**: the re-auth re-open test bumped a counter and bypassed the real signal path (R7); stagger had no acceptance signal (R7); cgo-CI is a link signal, not behavior. |
| H | Conformance / compliance | **Holds** | Shape precedent-backed (3 production libs); RPC 0-risk; no read API; view/active footprint inherent. Design is **identical under either account fact** (dedicated vs shared) — the account fact moves only the risk weight, not the mechanism → finalizable now; account fact is a sign-off input (R8). |
| I | Scope & exclusions | **Holds** (all 4) | §9 re-verified: no hand-built poller, no Problem-2 absorption, no in-test real-time assertion, no read API, no shrinkage. |
| J | Prior art / reinvention | **Holds** | Adopt/deviate/novel each cited; OpenChat-holds-N honestly marked NOVEL/engine-closed and a Gate B target, not falsely backed by precedent. |
| K | Cost / risk / inversion | **Holds** | Cheap, reversible (plain set; ref-count only if a second open-origin ever appears). Asymmetry: missed-open = the bug recurs; extra-open = harmless footprint. |

### Silent-failure inventory (Kahneman, all cross-checked)

1. **Classifier misclassifies a real supergroup as basic-group → never
   `OpenChat` → permanent zero-request silence** — the closest recurrence of
   the original bug, and *deterministic* (every load). Root causes per R1:
   wrong boundary values + the unreachable `id ≥ 0` row.
2. **Pending retry can be starved of its edge**: an `OpenChat` failure at
   startup → pending → "next edge" that never comes (static ruleset) →
   silence until the next restart. Fixed by R4 (in-edge bounded retry).
3. **Reconnect edge missed**: resolved — not an edge (mm.cpp:33400 unload
   guard); the draft's two-edge set is correct once the re-auth edge is
   collapsed (R2).

### Orchestrator's own findings (added to the room's input)

- **F1 — the "re-auth edge" is a ghost in the current code.** `runAuthLoop`
  is one-shot (repo.go:342-371); `LogOut` → `CleanUp` deletes the DB
  (repo.go:183-190) → re-auth requires a process restart → the only re-open
  edge is the startup adapter-swap. Consequences: **delete `authGen` and the
  separate re-auth edge** (R2); **D2 collapses** — the only new signal is the
  adapter-swap, handled by a repo callback (R3).
- **F2 — `pushManager.Replace` holds `m.mu` and runs the publish callback
  synchronously** (push/manager.go). A blocking staggered open batch in the
  `main.go:168` callback would stall the generation swap + the push send
  path → **stagger must be async** (R5).
- **F3 — `open_dialog` silently early-returns on `!have_input_peer`**
  (mm.cpp:19527). For a fresh client (empty dialog map right after the swap)
  an `OpenChat` on a not-yet-known chat can be a **silent no-op** → the open
  batch is ordered **GetChat (resolve/prime the peer) → OpenChat** (R1). The
  exact `openChat` RPC handler for an unknown dialog lives in `Td.cpp` (not
  in the pinned cache) → Gate B source-confirm; the ordering is safe either
  way.
- **Correction to a pre-seeded question**: the Telethon channel boundary is
  **1e12** (`> 1000000000000`, 0b file ~line 108; fixture `1002641439846 =
  1e12 + 2641439846`), not 999_999_999 — the Q3 brief's boundary figure was
  misstated and Munger's "1e11" was a miscount; R1 uses 1e12.

### Owner decisions D1/D2 (room leanings; the decision is the owner's)

- **D1 — stagger pacing surface.** Lean (b) local pacing, 2:1 (Munger: even
  a Ticker is heavy — a ctx-aware short sleep in the batch loop; Popper:
  don't grow the per-chat limiter's responsibility; Kahneman for (a) but on
  the same "it's rate-limiting, it belongs with rate-limiting" logic — which
  the shape fact defeats: the limiter is keyed per-chatID and an N-distinct
  batch never hits it). **Resolved as R6** (local pacing in the opener;
  P1-R3 wording fixed). Effectively decided by the shape fact; the owner may
  still veto.
- **D2 — re-auth observation wiring.** Collapsed by F1: the only new signal
  is the adapter-swap; a repo callback (same package), no fan-out (the
  existing channels are compete-not-broadcast, repo.go:140-148). **Resolved
  as R3.** No longer an owner decision beyond the normal implementation
  review.

### Stage 2 revisions dispatched (R1–R9)

R1 §2 table rewrite (1e12 single boundary; mark unreachable rows; GetChat =
input-peer resolution, not type fallback; `IsChannel`; `infra/ruleset` path;
loader/validator owns the raw-id footgun). R2 edge collapse + `authGen`
deletion + adapter-swap trigger. R3 repo-callback wiring, no fan-out. R4
in-edge bounded retry REQUIRED. R5 stagger async (Replace is
sync-under-lock). R6 local pacing + P1-R3 wording. R7 §6 fake-CI fix +
stagger acceptance signal + 1e12 classifier test. R8 §8 conformance update
(account fact = risk weight only; large-N tail latency = owner-informed
acceptance). R9 §8/§9 citation + exclusion re-check.

---

## Gate B — adversarial / inversion

### Gate B — orchestrator independent source-confirm (T1, before the room returned)

Munger's T1 (his top "most dangerous" target) claimed the re-poll chain
could (a) never start for a quiet channel, or (b) be gated on an unshown
user-activity/view condition so a non-interactive daemon never triggers it.
**The pinned source refutes both.** The chain is now pinned to three lines:

| Link | Line | Gate |
|------|------|------|
| **Start** | `open_dialog`, `case DialogType::Channel`, `get_channel_difference(dialog_id, d->pts, 0, MessageId(), true, "open_dialog")` | **mm.cpp:19593** — unconditional |
| non-final | `if (!is_final) get_channel_difference(...)` (immediate re-fetch) | **mm.cpp:37413** — no gap check |
| final re-arm | `if (timeout > 0 && d->open_count > 0) channel_get_difference_timeout_.add_timeout_in(dialog_id, timeout)` | **mm.cpp:37423** — no user-activity/view/foreground gate (37395–37435 scanned) |

**Start = our own `OpenChat`** (`open_dialog:19593`), so the chain is
initiated by the fix itself and needs no user activity. This **corrects a
gap in source-confirm (a)**: that confirm covered the *re-arm* (:37423, no
gap check) but had not located the *start* (:19593); the foundation is now
pinned as start + continuation, both unconditional. The conformance risk on
the mechanism is therefore lower than the room was told.

**Munger's three valid points are absorbed as Stage-2.5 revisions:**
- **B1 (acceptance blind spot):** a point-in-time box test proves "it can
  start", not "it stays alive across a re-arm period" — add a "quiet
  channel, re-test after one re-arm period" box check.
- **B2 (T8 wiring lifetime):** the swap callback must be registered **before
  `Start`**; add a WARN if the callback is nil at swap time; the box
  acceptance must include a real cold restart.
- **B3 (T7 invariant relaxed):** the async stagger (R5) does relax "source
  open before the first push of a newly-added rule"; the consequence is
  "late by seconds + backfill", not "lost" (opening triggers the diff
  backfill) — state this, and order `ApplySet` before
  `SetRuleSetWithPush` in the Replace callback.

*(room verdicts below; the T1 source-confirm above ran before the room
returned and directly addresses Munger's top target.)*

### Gate B — room verdicts (Munger inversion + Popper falsification, 2026-11-02)

**Verdict: PASS with Stage-2.5 revisions (statement/test-level only; no
architectural change).** No target is fatal.

**T1 (the foundation) — Munger's mechanism REFUTED by pinned source; the
residual is the true box-only half.** Munger's T1 claimed the re-poll chain
could never start for a quiet channel or be gated on an unshown
user-activity condition. The pinned source refutes both (the three-line
chain above: start :19593 unconditional, non-final :37413, final re-arm
:37423 with no activity gate). Popper's T1 ("the pinned source only proves
the if-path exists") was an **epistemic limit** — his machine did not have
the cached `mm.cpp`, so he could not see :19593; that "evidence-layer gap"
is closed by the orchestrator's confirm above. What remains box-only is the
**server-side half** (does the server actually deliver to a session that
holds N chats open) — that is the genuine "RPC-holds-N" behavioral claim,
and it is box-only by nature. The foundation is therefore **stronger** than
"if-path exists": the client-side firing is source-pinned; only the
server's delivery behavior awaits the box.

**Munger B1 — acceptance blind spot (ABSORBED).** A point-in-time box test
proves "it can start", not "it stays alive across a re-arm period". Add a
box check: a **quiet channel, re-test after one full re-arm period** (not
minutes after deploy). This is the only acceptance that closes the
server-side half of T1.

**Munger B2 / Popper T8 — wiring lifetime (ABSORBED, now verifiable).** The
swap callback must be **registered before `Start`** (before the one-shot
`runAuthLoop` reaches its swap critical section), else the whole fix never
fires and the failure is silent. Add a **WARN if the callback is nil at
swap time**. The box acceptance must include a **real cold restart**.

**Munger B3 / Popper T7 — the async invariant (ABSORBED, the one real race
R5 introduced).** R5 (async stagger) relaxes "source open before the first
push of a newly-added rule". The correct statement is **self-healing, not
lost**: a push that lands in the open in-flight window is at worst **delayed**,
because the open's own diff fetch (`open_dialog:19593`) **backfills** any
missed messages as updates once the open completes. The invariant is
"**eventually open + backfilled**", not "open before first push". Order
`ApplySet` **before** `SetRuleSetWithPush` in the Replace callback so the
backfill starts as soon as possible. (The "backfill flows through the update
pipeline" claim is engine behavior; the B1 quiet-channel box check also
covers it.)

**Popper T6 — retry ctx (ABSORBED).** The in-edge retry (R4) goroutine must
use the **opener's lifecycle ctx**, not a per-reload ctx — a hot-reload
must not cancel an in-flight retry for a *different* (earlier) batch.

**T2 — stays a source-confirm target.** The `openChat`-for-unknown-dialog
handler is in generated code (`Td.cpp`/`RpcClient`) not in the source tree
(orchestrator fetch: pinned `Td.cpp` is init-only, 1429 lines, no RPC
dispatch; `RpcClient.cpp`/`Rpc.cpp`/`td_api.cpp` all 404). The
GetChat→OpenChat ordering is **safe either way**; the box confirms the
actual no-op behavior.

**T3 — classifier unambiguous (Popper counter-example passed).** `m == 1e12`
exactly → not open (`≤` includes the boundary); `m == 1e12 + 1` → open.
No fix. The real footgun (unmarked channel id in YAML) is correctly
loader/validator scope, not the classifier's.

**T4 / T5 — box-only, as expected** (N>10 FIFO no-starvation; CloseChat
stops the poll).

**Count (Popper + orchestrator):** 5 load-bearing claims **closed now** by
pinned source / go-tdlib contract / Budva Go source (foundation firing,
classifier table, GetChat→OpenChat ordering, two-edge set, in-edge retry
consistency); the rest **box-only** (T1 server delivery, T2 no-op, T4
starvation, T5 poll-stop, T7 window, large-N tail). Popper independently
re-verified the pinned commit `22d49d5` (go-tdlib `Makefile:1`) and the
`openChat` contract in `td_api.tl` ("…in supergroups and channels all
updates are received **only for opened chats**") — the foundation's
normative layer holds.

**Stage 2.5 revisions dispatched:** B1 (quiet-channel box check), B2 (callback
before `Start` + nil-WARN + cold-restart box), B3+T7 (self-healing invariant
statement + `ApplySet` before `SetRuleSetWithPush`), T6 (retry uses opener
lifecycle ctx). Then Stage 3 convergence (the five exit conditions).
