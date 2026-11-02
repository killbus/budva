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

*(pending — runs on the Stage 2 revision. Targets pre-seeded from the 0b
"What no precedent shows" list + Gate A: (1) `OpenChat`-RPC-holds-N
statically from a non-interactive daemon — box-closed, the design's
foundation; (2) `openChat` for an unknown dialog — `Td.cpp` source-confirm
(not cached); (3) classifier completeness after R1 (1e12 boundary,
unreachable rows); (4) N>10 FIFO no-starvation; (5) `CloseChat` on
ruleset-removal as a config event.)*
