# 10-30 Problem-1 design — OpenChat lifecycle for ruleset sources

> Delivered via the Design Delivery Loop
> (`.trellis/spec/guides/design-review-loop.md`): Stage 0 Frame → Stage 0b
> precedent survey → Stage 1 draft → Gate A (dimensions) → Stage 2 revise →
> Gate B (adversarial) → Stage 3 converge. Verdicts persist to
> `research/<design>-dimensions.md`; precedent findings to
> `research/precedent-openchat-lifecycle.md`.

**Status**: Stage 1 draft done; Gate A pending.

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

Scope: the four lifecycle/stagger seams from the Frame. Every non-obvious
mechanism cites a precedent (pinned) from
`research/precedent-openchat-lifecycle.md`, or a source-verified reason it is
novel. Pins reused below: **TDlib** = commit `22d49d5` `MessagesManager.cpp`
(`open_dialog`:19524, `close_dialog`:19640, gap-gate:7206, re-arm:37423,
10-cap FIFO:36854/36860); **go-tdlib** = v0.7.6 (`function.go` `OpenChat`:7790 /
`CloseChat`:7816 / `OpenChatRequest{ChatId int64}`:7784 / `CloseChatRequest{ChatId
int64}`:7810; `types.go` `Chat{Id, Type ChatType}`:14877, `ChatType*`:14059-14142);
**precedents** = Telethon v1.42.0, Pyrogram 2.0.106, mtcute `@mtcute/core`
0.32.4 (+re-poll internals on master), teleproto 1.232.1 (all source-pinned in
the 0b file).

### §1 — Mechanism & layer placement

**Decision.** Add a small **`SourceOpener`** component in
**`internal/infra/telegram`** (alongside `repo.go` / `client_adapter.go`), plus
**two 1:1 adapter methods** `OpenChat` / `CloseChat` (dimension A). The opener
holds the in-process open-set state and the failure/pending state, and exposes a
single idempotent entry point:

```
SourceOpener.ApplySet(ids []domain.ChatID)   // called on every edge; diff-driven
```

`ApplySet` is the ONLY driver. Edges call it (not `OpenChat` directly):

- **initial load & hot-reload**: from the existing
  `cmd/engine/main.go:168` `pushManager.Replace(configs, callback)` convergence
  callback — the *same* callback that already runs `handlerService.SetRuleSetWithPush(rs, g)`,
  extended to also call `sourceOpener.ApplySet(sourceChannelIDs(rs))`. Both the
  first load (main.go:175) and the watcher (main.go:183-196) already funnel
  through `loadSnapshot` → `Replace`, so **one** call site covers both edges.
- **re-auth**: a small **auth-state observer in the same package** subscribes to
  the repo's auth-state stream; on each transition into `AuthStateReady` it calls
  `ApplySet(currentIDs)`. (See §3 re-auth for the dimension-C hazard and why the
  observer lives in-package rather than reusing `AuthStates()`.)

**Dimension-A rationale (why this placement):**
1. **RPC calls stay in the infra/telegram layer** next to the client adapter —
   `OpenChat`/`CloseChat` are thin `clientAdapter` methods exactly like the
   existing `GetChat` (`client_adapter.go:40` / `repo.go:262`), added per the
   adapter's own rule *"метод добавляется сюда только когда у него появился живой
   потребитель"* (the `SourceOpener` is that consumer).
2. **The domain type crossing the boundary is a set of chat IDs** (`[]domain.ChatID`).
   `SourceOpener.ApplySet` takes bare IDs; it never leaks `client.Chat` /
   `*ChatType` into the app/cmd layer. The only richer type (`client.Chat` from
   `GetChat`) is consumed *inside* the telegram layer and reduced to a boolean
   (`isChannel`).
3. **No new CGO surface.** `OpenChat`/`CloseChat` are existing go-tdlib `*Client`
   methods (v0.7.6); go-tdlib is already the CGO shell over the C++ TDlib. Adding
   two wrappers touches no cgo.
4. **No hand-built poller (P1-R5).** The component only *issues* `OpenChat` /
   `CloseChat`; the re-poll that keeps an opened channel acquiring is owned by the
   linked C++ engine (`TDlib` re-arm:37423, gated on `open_count`). The opener has
   **no timer of its own** — this is the deliberate deviation from mtcute/teleproto,
   which each hand-roll a re-poll because they do *not* link TDlib. A hand-built
   client needs its own poll; a TDlib-wrapped client does not. **This is the
   central layering difference** between Budva's precedent and the precedent
   itself (see §7).

**Where the "is it a channel" decision lives.** The classifier (§2) is a pure
function over an ID (primary) — it can live in `internal/domain` (pure, no deps,
trivially unit-testable) or as an unexported helper in `internal/infra/telegram`.
**Recommendation: `internal/domain`** (exported `func IsChannelOrSupergroup(ChatID) (bool, bool)`)
because (a) it depends only on the ID convention documented in
`internal/domain/ruleset/doc.go:19-20`, (b) the `GetChat`-fallback *result* is a
`bool` that folds back into the same decision, so the decision is ID-shaped
in and ID-shaped out, and (c) keeping it in domain makes it testable without cgo
(dimension G). The `GetChat` fallback call (the only RPC in the decision) stays in
the telegram layer.

### §2 — Open-set derivation (classification)

**Decision.** **ID-magnitude primary (ADOPT) + `GetChat` fallback.**
Rationale: Telethon `resolve_id` (v1.42.0 `utils.py:1053-1063`) and Pyrogram
`get_peer_type` (2.0.106 `utils.py:236-246`) both classify a bare ID by
**sign/magnitude with no RPC**; Budva's stored `ChatID` is already in that
marked/negated form (`ruleset/doc.go:19-20`; loader `negateChatIDs`
`ruleset/repo.go:291`). `GetChat` is the fallback for IDs the magnitude test
cannot classify (raw-positive / ambiguous) — a **raw positive channel id
misclassifies as a basic group → never opened → silent** (0b finding 2); that is
the exact bug being fixed, so the fallback must exist.

**Decision table.** For each source ID (the ruleset's `Sources` keys), `m = |id|`:

| Stored `ChatID` (`int64`) | Meaning (mark) | Open? | Mechanism | Authority |
|---|---|---|---|---|
| `id ≥ 0` | private (`PeerUser`) | **No** | global stream | id-sign (both libs) |
| `id < 0` and `m ≤ 999_999_999_9` | basic group (`PeerChat`) | **No** | global stream | id-sign (both libs) |
| `id < 0` and `m > 100_000_000_000` | channel **or** supergroup (`PeerChannel`) | **Yes** | `OpenChat` | id-sign (both libs); *channel-vs-sg split is diagnostic only* |
| raw / ambiguous (magnitude test inconclusive) | unknown | **fallback** → `GetChat` | see below | `client.Chat.Type` (go-tdlib `type.go:14877`) |

**The `GetChat` fallback branch** (only for the "raw/ambiguous" row): read
`chat.Type` and map to the open decision by go-tdlib `ChatType` —
`*client.ChatTypeSupergroup` → **open** (this covers **both** channels and
supergroups: in go-tdlib a channel *is* a `ChatTypeSupergroup` with
`Broadcast: true`; `type.go:14113`); `*client.ChatTypeBasicGroup` → no;
`*client.ChatTypePrivate` → no; `*client.ChatTypeSecret` → no. **Open iff
`chat.Type` is `*client.ChatTypeSupergroup`.** This matches mtcute `openChat`
(which branches `isInputPeerChannel` then no-ops for non-channels, 0b §1) and
teleproto `arm` (arms only `InputPeerChannel`, 1.232.1) — both branch on the
resolved peer type, not the id.

**Note (honest):** the **channel-vs-supergroup** sub-split is *never needed* to
decide open/not — both open. It is only the diagnostic the record carries; the
open decision needs just the 3-way {channel/supergroup} vs {basic-group} vs
{private} split, which id-magnitude gives for free.

### §3 — Lifecycle edges

One driver, `SourceOpener.ApplySet(ids)`, invoked from each edge. The opener
maintains an in-memory `opened map[ChatID]struct{}` and diffs `ids` against it.

| Edge | Where it fires (function/layer) | Action | Ordering hazard (dimension C) |
|---|---|---|---|
| **Initial load** | `main.go:168` `Replace` callback (via `loadSnapshot`, main.go:175) | `ApplySet(channelIDs(rs))` → open all; no closes (empty base) | none (first call) |
| **Hot-reload add** | `main.go:168` `Replace` callback (via watcher main.go:183-196) | diff: `added → OpenChat` (staggered) | **must not race re-auth** — see below |
| **Hot-reload remove** | same callback | diff: `removed → CloseChat` | same |
| **Re-auth** | auth-state observer in `infra/telegram`; on each `AuthStateReady` | `ApplySet(currentIDs)` full re-open (dialogs + `open_count` lost) | **see below (the real hazard)** |
| **Process restart** | = initial load (`open_count` is in-process; `RunAuthLoop` re-runs) | `ApplySet(channelIDs(rs))` | none |

**The re-auth ↔ reload ordering hazard (the one that matters).** `open_count` is
in-process and is **lost whenever the TDlib session is swapped**. `runAuthLoop`
(`repo.go:342-371`) re-creates the client and swaps the adapter on (re)auth; the
`loadSnapshot`/`Replace` path and the re-auth path can therefore interleave.
Resolution, in order of preference:

1. **Single source of truth for `currentIDs`: the opener.** Both the `Replace`
   callback and the re-auth observer call `ApplySet` on the **same** `SourceOpener`
   (guarded by one mutex), passing the **latest** ruleset's channel IDs. `ApplySet`
   is a pure diff against `opened`, so it is **idempotent under re-apply**: a
   reload firing during re-auth just re-diffs the same set → **no double-open**
   (an ID already in `opened` is not re-`OpenChat`'d), and a re-auth firing after a
   reload re-opens the new set. This is the mtcute/teleproto idempotency: mtcute's
   `notifyChannelOpened` increments a ref-count and *does not re-fetch* if already
   open (`manager.ts:383-388`); teleproto `arm` skips `if (entry.channels.has(id))`
   (1.232.1). We get the same "no double-open" from the diff.
2. **The observer must run in-package, not reuse `AuthStates()`.**
   `AuthStates()` (`repo.go:129`) is a **single channel already consumed** by the
   auth service (`auth/service.go:58`), and that service **stops reading after the
   first `Ready`** (`auth/service.go:66-68`) — a second consumer would starve. So
   the repo must expose a **secondary auth-state signal** the opener can observe
   (e.g. the repo internally notifies the opener of each `AuthStateReady`, or a
   fan-out of the auth-state event). This is a **small, isolated change** in
   `infra/telegram` (dimension C); it is *not* a redesign of auth. **This is the
   one new wiring point** the draft introduces and the one I flag as open: the
   exact plumbing (fan-out channel vs an observer callback registered on the repo)
   is an implementation choice for the owner to confirm (see §8, the re-auth row,
   and the report-back digest).
3. **After a session swap, `open_count` is 0 for all chats**, so `ApplySet` must
   treat re-auth as a **full re-open** even though `opened` still lists the IDs:
   the opener keys its "needs (re)open" decision on the **auth generation**, not
   just on the diff. Concretely: the opener holds an `authGen int`; on each
   `AuthStateReady` it bumps `authGen`, and `ApplySet` re-opens every ID when the
   recorded per-ID generation < current `authGen`. This guarantees re-auth
   re-opens **all** sources even when the ruleset set is unchanged (a no-op diff
   would otherwise do nothing). This mirrors teleproto `_resumeAuthorization`→
   `_resume` **re-arming the surviving set after re-auth** (1.232.1).

### §4 — Stagger

**Decision (DEVIATE from mtcute).** mtcute *documents* the 5-10 open-batch
ban/limit risk ("…may start getting transport errors or even get banned",
`openChat` doc 0.32.4) but **leaves mitigation to the caller**; a daemon has no
human caller to honor that warning. So Budva staggers the initial/re-auth open
batch itself.

**Shape (per-call token, not per-chat).** Stagger the N `OpenChat` calls of an
open *batch* by spacing the **RPC invocations**, e.g. `limiter`-paced
`ApplySet` → for each ID to open: *acquire pacing slot → `OpenChat` → release*.
A **pacing interval** between opens (e.g. a few hundred ms, **a tunable, not
presets**) keeps the batch from firing N `openChat` in the same instant. The
re-poll itself is server-paced by the engine and already ≤10-concurrent
(`TDlib`:36854/36860), so **only the open *calls* are staggered**; we do not touch
the engine's poll cadence (guardrail 4: don't fight the 10 cap).

**⚠ Honest finding (route to Gate A): the existing `limiter.Service` does not fit.**
`internal/service/limiter` is a **per-chat forward throttle** — `WaitForForward(ctx,
chatID)` spaces forwards *to the same chat* by a fixed 3 s (`service.go:12,30-44`),
keyed by `chatID`. An open **batch is N *distinct* chats**, so a per-chat limiter
would never fire (no two opens share a `chatID`). The record's "reuse
`internal/service/limiter`" (P1-R3) is therefore a **shape mismatch**. Options:
(a) add a **per-operation** pacing primitive to `limiter` (a token bucket over
*time*, not over `chatID`) — reuse the package, add a method; (b) a small
`time.Ticker`-paced loop *inside* `SourceOpener` (no limiter reuse). **I lean
(a)** to honor "reuse the limiter package," but (b) is arguably simpler and more
local. **This is a decision the owner should make** (which pacing surface to use);
the *stagger requirement* itself is uncontested.

### §5 — In-process state & failure edges

**State: plain set, NOT ref-counting (decision).** mtcute keeps
`channelsOpened: Map<id, count>` and increments/decrements (`manager.ts:153,
380-423`) because a *UI* can open the same chat in multiple windows and only
remove it on the last close. Budva has a **single opener — the ruleset** — a chat
is either in the set or not. Ref-counting would model N "windows" that don't
exist; it buys nothing for one integer. **Use `map[ChatID]struct{}` + `authGen`
(§3).** Idempotency under re-apply is achieved by the **diff** (already-opened IDs
are skipped) + the **auth-generation** bump on re-auth, not by a counter. This is
the same *effect* as mtcute's ref-count for a count that never exceeds 1, at less
machinery. (If a future feature ever opens a chat from a second origin, ref-count
can be added then; not now.)

**Failure edges (dimension F) — never fail the ruleset load.** `ApplySet` returns
nothing to the caller about per-chat outcomes; the `Replace`/`loadSnapshot` path
must **not** abort the generation on an open failure (a ruleset load must not be
blocked by one inaccessible chat). Per ID:

| Failure | Detection | Semantics |
|---|---|---|
| **`GetChat` fails** (fallback path) | `*client.Error` from `GetChat` | do NOT open on this edge; mark ID **pending** (reclassified next edge). Log WARN. (A magnitude-classified ID needs no `GetChat`, so this only hits the ambiguous row.) |
| **`OpenChat` fails — inaccessible** (`CHAT_NOT_MODIFIED` / no-access / `CHANNEL_PRIVATE`-class) | go-tdlib error on `OpenChat` | mark **pending**; log WARN; **retry on the next `ApplySet` edge** (next reload, or next re-auth, or a bounded retry). Never fail load. |
| **`OpenChat` fails — kicked / member-removed** | membership error | mark **pending** + log; same retry. (If a source the account can no longer see is in the ruleset, that is a *ruleset* problem, surfaced by the log, not a load failure.) |
| **`OpenChat` fails — bot-guard / auth** (account is a bot; or not authorized) | `CHECK(!is_bot)`-class / auth error | **fatal-to-feature**: log ERROR once, disable the opener for this auth generation (do not spam). `OpenChat`'s `CHECK(!is_bot)` (`TDlib`:19524) means a **bot** session cannot open — if Budva's session were a bot this fix is inapplicable; flag as a precondition (Budva uses a **user** account per the record). |
| **Mid-batch interruption** (ctx cancelled / process stop during the N-batch) | ctx.Done in the pacing loop | stop opening; the **pending** set (never-opened IDs) is retained in-memory and re-applied on the next edge (re-auth re-opens all; a restart re-opens all). Because re-auth/restart do a *full* re-open, a mid-batch stop self-heals on the next lifecycle event. |

**`CloseChat` failure (removal edge):** non-fatal — log, drop from `opened`;
if the ID reappears the next `ApplySet` re-opens it. A lingering open after a
failed close is *harmless* (an extra open chat just keeps its channel acquiring;
it does not cause the silence bug).

**Pending-set note:** "pending" = "we intended to open but the call failed."
It is **reconciled on the next edge**, not on a timer (consistent with
"open-once-keep, no heartbeat" — guardrail 3). A bounded optional in-edge retry
for transient errors is allowed but not required.

### §6 — Test & acceptance (CI-assertable vs box-only)

**CI-assertable (pure-Go, no cgo) — dimension G:**
- **Classifier table test** (`internal/domain`): feed the decision table rows
  (private `+id`, basic-group `-small`, channel/supergroup `-100…`, and a
  raw-positive ambiguous id) → assert {open, no-open, fallback} outcomes. Pure
  function, no TDlib.
- **Open-set diff test** with a **mocked adapter** (the `clientAdapter` interface
  is already mockable — `GetChat` etc. are on it): build a `SourceOpener` over a
  fake adapter; call `ApplySet([A,B,C])` then `ApplySet([B,C,D])`; **assert**
  `OpenChat` called with `{A,B,C}` (1st) and `{D}` (2nd), `CloseChat` called with
  `{A}` (2nd), and **idempotent re-apply** → `ApplySet([B,C,D])` again calls
  neither. Assert `OpenChat`/`CloseChat` are called with the **exact marked
  IDs** (the values that would go to go-tdlib).
- **Re-auth re-open test**: bump `authGen` (simulate `AuthStateReady`) then
  `ApplySet(sameIDs)` → assert `OpenChat` called for **all** IDs again (full
  re-open), and **no double-open** when a reload and a re-auth interleave (assert
  `OpenChat` not called twice for an ID still in `opened` within one generation).
- **Failure-edge tests**: mocked adapter returns an error on `OpenChat` for one
  ID → assert that ID is marked pending, `ApplySet` does not propagate the error
  (load path unaffected), and the next `ApplySet` retries it.
- **cgo-CI** (the TDLib-provisioned handler package): builds the two new
  `clientAdapter` methods (`OpenChat`/`CloseChat` compile against go-tdlib
  v0.7.6) — i.e. the adapter additions type-check and link. This is the "the
  wrapper exists and is 1:1" signal, not a behavior test.

**Box-only (live TDlib + server) — final acceptance, post-deploy:**
- **Real-time arrival**: with the owner's official client **closed** on a
  channel/supergroup source, Budva's session still receives that source's new
  messages (the actual bug symptom: silence when no session has it open).
  This CANNOT be asserted in any test (needs a live account + server open-state).
- **N>10 FIFO behavior**: with >10 channel/supergroup sources, deferred sources
  still acquire (no starvation) — observed at runtime (the engine's unbounded FIFO,
  `TDlib`:36854/36860; source-confirm 2).
- **No read API issued**: the box's TDlib client log shows `openChat` /
  `closeChat` / `getChannelDifference` but **no** `readHistory` /
  `readChannelHistory` / mark-read (conformance = never call a read API).

**Acceptance signals named (Frame exit):** *CI* = classifier table,
diff/`OpenChat`/`CloseChat` id-set + idempotency + re-auth re-open + failure-edge
(pure-Go, mocked adapter), and cgo-CI adapter link. *Box-only* = real-time
arrival with owner's client closed; N>10 no-starvation; no read API in the client
log.

### §7 — Conformance (shape vs mechanism)

- **Shape — precedent-backed (3 production libraries).** "A non-interactive client
  keeps a **set** of channels/supergroups open, open-once-keep, re-opens the full
  set at re-auth, closes on removal, does not rotate/shrink" is done by
  mtcute (ref-counted `channelsOpened` open set, 0.32.4), Telethon (whole-set
  persistent re-poll, no user, v1.42.0), and teleproto (`watch` a set, re-armed at
  re-auth, 1.232.1). **The shape is not novel.**
- **Mechanism — closed only by the pinned engine (state honestly).** Every
  hand-built precedent keeps the set open by **its own re-poll** (client-side);
  **none** sends the `OpenChat` *server* RPC to hold N. Budva's mechanism —
  `OpenChat` on N, held statically, with the **C++ engine** owning the
  open-gated `getChannelDifference` re-poll — is closed **only** by the pinned
  TDlib source (`open_count` re-arm:37423, 10-cap FIFO:36854/36860, gap-gate:7206).
  **Precedent corroborates the shape but does NOT close the mechanism** — that is a
  **Gate B target**, not a defect. The box-only "real-time arrival with owner's
  client closed" acceptance (§6) is what closes it at runtime.
- **RPC conformance (0 risk).** Every RPC Budva issues (`openChat` / `closeChat`
  / the engine's `getChannelDifference`) is one an official client issues
  continuously; no non-client RPC.
- **No read API.** Conformance = Budva never calls `readHistory` /
  `readChannelHistory` / mark-read. A normal open does **not** mark a channel read
  (`TDlib open_dialog`:19524 only calls `read_history_inbox` in the empty-chat
  edge). Box-verified in §6.
- **View / active-count footprint is inherent.** An open dialog contributes to
  view/active counts by definition; the footprint *is* the fix (the session must
  own the open state). Not a risk to mitigate — documented, per the record.

### §8 — Precedent citations

| Mechanism | Verdict | Pinned project + version | One line |
|---|---|---|---|
| Type by id-sign/magnitude (primary) | **ADOPT** | Telethon v1.42.0 `resolve_id` (`utils.py:1053`); Pyrogram 2.0.106 `get_peer_type` (`utils.py:236`) | Classify a bare marked ID as {private, basic-group, channel} with no RPC; matches Budva's stored negated form (`ruleset/doc.go:19-20`). |
| Type from resolved peer (fallback) | **ADOPT** | mtcute 0.32.4 `openChat` (`isInputPeerChannel` branch); teleproto 1.232.1 `arm` (`InputPeerChannel` only) | Resolve peer / `GetChat`, open iff channel/supergroup, no-op otherwise; used only for magnitude-ambiguous IDs. |
| Open-once-keep, idempotent re-open | **ADOPT** | mtcute 0.32.4 `notifyChannelOpened` (`manager.ts:380-397`); teleproto 1.232.1 `arm` dedup | Re-apply is a no-op for already-open IDs (no double-open); a plain set + auth-gen gives the same effect for a single opener. |
| Full re-open at re-auth | **ADOPT** | teleproto 1.232.1 `_resumeAuthorization`→`_resume` (re-arms surviving set after re-auth) | Re-auth loses `open_count` → re-open all sources even if the set is unchanged (auth-generation bump). |
| Close-on-removal | **ADOPT** | mtcute 0.32.4 `closeChat` = "un-does `openChat`" (`notifyChannelClosed` `manager.ts:409-423`) | Ruleset removal → `CloseChat`; ref-count-to-0 removes the entry (we: set-delete). |
| Keep a **set** open non-interactively | **ADOPT (shape)** | mtcute (`channelsOpened` map); Telethon v1.42.0 `MessageBox` (whole-set re-poll, no user); teleproto 1.232.1 `watch` (a set) | The set-at-once shape is real, production client behavior — not novel. |
| Stagger the open batch | **DEVIATE from mtcute** | mtcute 0.32.4 `openChat` doc (5-10 ban/limit warning) | mtcute documents the risk but leaves mitigation to the caller; a daemon has no human caller → Budva staggers itself. **Note: existing `limiter` is per-chat, a shape mismatch — pacing surface is an open owner decision (§4).** |
| `OpenChat`-RPC-holds-N mechanism | **NOVEL (engine-native)** | TDlib `22d49d5` (`open_count`:19524, re-arm:37423, 10-cap FIFO:36854/36860) | The re-poll is owned by the linked C++ engine, not hand-built (P1-R5); precedent cannot close this — **Gate B target**, closed at runtime by the box-only real-time-arrival acceptance. |

### §9 — Exclusions re-check (from the Frame; confirm none absorbed)

| Frame exclusion | Status in this draft |
|---|---|
| **Hand-built Go `getChannelDifference` poller** | **Not absorbed.** `SourceOpener` has **no timer/re-poll**; it only issues `OpenChat`/`CloseChat`. The re-poll is the engine's (P1-R5). §1/§7 make this the *central* layering point. |
| **Problem 2 (endpoint HOL)** | **Not absorbed.** No dispatcher/concurrency/timeout work; the draft only adds the open-set lifecycle. |
| **Asserting real-time delivery in any test** | **Held.** Real-time arrival + N>10 no-starvation are **box-only** (§6); CI asserts only that `OpenChat`/`CloseChat` are *called* correctly. |
| **Any read API (`readHistory`-class)** | **Not added.** The opener issues no read RPC; box-verified in §6 (client log shows no mark-read). |
| **Shrinking the open set to non-member channels** | **Not absorbed.** The open set = **all** channel/supergroup sources (no membership filter, no shrink-to-non-member); §2/§5 keep all. |

**Exclusions honored.** No hand-built poller, no Problem-2 scope, no in-test
real-time assertion, no read API, no open-set shrinkage.


## Gate A / Gate B verdicts

*(pending — output: `research/design-dimensions.md`)*
