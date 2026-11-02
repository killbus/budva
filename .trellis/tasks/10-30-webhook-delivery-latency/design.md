# 10-30 Problem-1 design — OpenChat lifecycle for ruleset sources

> Delivered via the Design Delivery Loop
> (`.trellis/spec/guides/design-review-loop.md`): Stage 0 Frame → Stage 0b
> precedent survey → Stage 1 draft → Gate A (dimensions) → Stage 2 revise →
> Gate B (adversarial) → Stage 3 converge. Verdicts persist to
> `research/<design>-dimensions.md`; precedent findings to
> `research/precedent-openchat-lifecycle.md`.

**Status**: Stage 2 revised; Gate B pending.

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
   the same instant. *[Revised Gate A / R6: the stagger is a local per-operation
   pace inside the opener, NOT `internal/service/limiter` — that limiter is a
   per-chatID forward throttle, a different axis; an N-distinct-chat open batch
   never shares a chatID. See §4.]*
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
(`open_dialog`:19524, **`open_dialog` early-returns on `!have_input_peer`:19527**,
`close_dialog`:19640, gap-gate:7206, re-arm:37423, 10-cap FIFO:36854/36860);
**go-tdlib** = v0.7.6 (`function.go` `OpenChat`:7790 / `CloseChat`:7816 /
`OpenChatRequest{ChatId int64}`:7784 / `CloseChatRequest{ChatId int64}`:7810;
`type.go` `Chat{Id, Type ChatType}`:14877, `ChatTypeSupergroup{SupergroupId,
IsChannel}`:14113-14118, `ChatType*`:14059-14142); **loader** =
`internal/infra/ruleset` (`transform()` negates every source key unconditionally,
`repo.go:215-221`; convention doc `doc.go:19-20`); **precedents** = Telethon
v1.42.0, Pyrogram 2.0.106, mtcute `@mtcute/core` 0.32.4 (+re-poll internals on
master), teleproto 1.232.1 (all source-pinned in the 0b file).

> **Stage 2 (Gate A) note:** revisions R1–R9 below amend the Stage 1 draft. Where
> a load-bearing claim changed, a one-line "**Revised (Gate A):** …" note marks the
> diff from Stage 1. Stage 0 Frame and Stage 0b are unchanged.

### §1 — Mechanism & layer placement

**Decision.** Add a small **`SourceOpener`** component in
**`internal/infra/telegram`** (alongside `repo.go` / `client_adapter.go`), plus
**two 1:1 adapter methods** `OpenChat` / `CloseChat` (dimension A). The opener
holds the in-process open-set state and the failure/pending state, and exposes a
single idempotent entry point:

```
SourceOpener.ApplySet(ids []domain.ChatID)   // called on every edge; diff-driven
```

`ApplySet` is the ONLY driver. **Exactly two edges** call it (not `OpenChat`
directly):

- **startup adapter-swap (full open):** the repo fires a **registered callback**
  in its `runAuthLoop` adapter-swap critical section — the same place it does
  `swapSessionTable()` + `clientAdapter = tdlibClient` (`repo.go:366-367`). The
  callback calls `ApplySet(currentChannelIDs)`. A brand-new C++ client instance
  has `open_count == 0` for every chat by construction, so this is **always a
  full open**; it also covers **process restart** (same `runAuthLoop` re-runs) and
  **re-auth** (which collapses to a fresh client — see §3).
- **hot-reload diff (added→open / removed→close):** from the existing
  `cmd/engine/main.go:168` `pushManager.Replace(configs, callback)` convergence
  callback — the *same* callback that already runs `handlerService.SetRuleSetWithPush(rs, g)`,
  extended to also call `sourceOpener.ApplySet(sourceChannelIDs(rs))`. Both the
  first load (main.go:175) and the watcher (main.go:183-196) funnel through
  `loadSnapshot` → `Replace`, so **one** call site covers the reload edge.

> **Revised (Gate A):** the Stage 1 draft had a **third** edge — a separate
> "re-auth observer" reacting to `AuthStateReady` — plus an `authGen`
> generation counter. Both are **deleted** (R2/R3): `runAuthLoop` is one-shot and
> `LogOut`→`CleanUp()` deletes the DB, so there is **no in-process re-auth path**;
> every session reset collapses to the adapter-swap. The open trigger is the
> **adapter-swap**, NOT `AuthStateReady` (which lags the swap and can be lost —
> `repo.go:355-361`).

**`ApplySet` execution model (dimension C, R5).** `pushManager.Replace`
(`internal/service/push/manager.go:24-45`) holds `m.mu` and runs the publish
callback **synchronously** (line 37 `publish(candidate)`). A blocking
N×(staggered `OpenChat` RPC) inside that callback would **hold the push manager's
lock** and stall the generation swap + the push send path. Therefore `ApplySet`:
1. **computes the diff synchronously** (fast, pure: `added` = `ids − opened`,
   `removed` = `opened − ids`, reconciling `pending`), and
2. **runs the actual RPC calls (`GetChat`→`OpenChat`) in a background goroutine**
   with the stagger (§4). `ApplySet` returns quickly; it tracks in-flight work
   (the `opened` set + a **per-ID in-flight guard**) so a concurrent edge cannot
   double-fire the same ID.

**Wiring the startup-swap edge (dimension C, R3).** The opener **registers a
callback on the repo**, fired in the `runAuthLoop` swap critical section (the repo
is the *same package*, `internal/infra/telegram`). **Do not add a raw fan-out
channel:** `Updates()`/`AuthStates()` are **compete-not-broadcast**
(`repo.go:140-148` — "multiple readers compete for events rather than receiving a
broadcast"), so a second raw channel would re-create the starvation hazard (the
auth service already stops reading `AuthStates()` after the first Ready,
`auth/service.go:66-68`). A one-callback registration in the swap section avoids
both the starvation and the "Ready can be lost" problem (R2). This is a **small,
isolated** `infra/telegram` change; the exact shape (a callback field vs. a
method) is an implementation detail.

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
function over an ID — it can live in `internal/domain` (pure, no deps, trivially
unit-testable) or as an unexported helper in `internal/infra/telegram`.
**Recommendation: `internal/domain`** (exported
`func IsChannelOrSupergroup(ChatID) (open bool)`) because (a) it depends only on
the ID convention documented in `internal/infra/ruleset/doc.go:19-20`, and
(b) keeping it in domain makes it testable without cgo (dimension G). **Revised
(Gate A):** the Stage 1 draft's rationale cited a "GetChat-fallback result folds
back to a bool"; that is superseded — **id-magnitude alone decides open/not**
(§2); `GetChat` is no longer a type fallback, it **resolves the input peer before
the `OpenChat` call** (also §2). The `GetChat` call (the only RPC in the
open path) stays in the telegram layer.
### §2 — Open-set derivation (classification)

**Decision (Revised, Gate A).** **ID-magnitude alone decides open/not — one RPC
per opened source to resolve the input peer before `OpenChat`.** The Stage 1
draft framed `GetChat` as a *type* fallback for ambiguous ids; that is wrong in
light of the loader. The classifier is the single boundary **1e12** (`|id| >
1_000_000_000_000`) — Telethon `resolve_id` (v1.42.0 `utils.py`,
`if marked_id > 1000000000000: → PeerChannel`) and the Budva fixture
`1002641439846 = 1e12 + 2641439846` (`internal/infra/ruleset/repo_test.go`).

**The loader makes stored source keys *always negative*.** `transform()`
(`internal/infra/ruleset/repo.go:215-221`) does `newSources[-id] = src`
**unconditionally** for every source key, and YAML keys are written positive
(`doc.go:19-20`). Therefore a stored source key is `-yamlKey`:

- **the `id ≥ 0 → private` row is unreachable for source keys** (a source key is
  never `≥ 0` after negation);
- **the "raw/ambiguous *positive* → GetChat fallback" row is unreachable for
  source keys** (same reason).

Both rows are kept in the pure function as **defensive / dead-on-the-source-path**
branches — they document the classifier's full contract and are unit-tested as
pure-function properties, but **no live source key reaches them**.

**Decision table.** For each source key `id` (stored, always `< 0`), `m = |id|`:

| Stored `ChatID` (`int64`) | Meaning (mark) | Open? | Mechanism | Authority |
|---|---|---|---|---|
| `id < 0` and `m > 1_000_000_000_000` (1e12) | channel **or** supergroup (`PeerChannel`) | **Yes** | resolve peer (`GetChat`) → `OpenChat` | id-magnitude: Telethon `resolve_id` `>1e12`; fixture `1002641439846` |
| `id < 0` and `m ≤ 1_000_000_000_000` (1e12) | basic group (`PeerChat`) | **No** | global stream | id-magnitude: Telethon `resolve_id` else-branch |
| `id ≥ 0` (defensive) | private (`PeerUser`) | **No** | global stream | id-sign — **unreachable for source keys** (keys are always `<0`) |

**Revised (Gate A) — the two misquotes fixed:**
- the go-tdlib field is **`IsChannel`**, not `Broadcast`
  (`client/type.go:14113-14118` `ChatTypeSupergroup{ SupergroupId int64; IsChannel
  bool }`);
- the loader path is **`internal/infra/ruleset`**, not `internal/domain/ruleset`.

**The real footgun is a *loader/validator* concern, not a classifier one.** If a
user writes a channel's **unmarked** id (e.g. `2641439846`) in the YAML, the
loader negates it to `-2641439846` — a **basic-group** magnitude — so the
classifier could *never* see it as a channel. The documented convention
(`internal/infra/ruleset/doc.go:19-20`) is "**write the marked magnitude, loader
negates**." Rejecting/flagging an unmarked raw channel id belongs to **loader
validation** and is **adjacent / out of scope for this open-set design** (R1).
The open-set design does **not** keep a `GetChat` fallback as the defense for it.

**`GetChat`'s real role: resolve the input peer before `OpenChat`.** The pinned
`open_dialog` **silently early-returns when `!have_input_peer`**
(`MessagesManager.cpp:19527`). For a fresh client — empty dialog map right after
the adapter-swap — an `OpenChat` on a not-yet-known chat can be a **silent no-op**
(open never registered → silence). So the per-ID open sequence is ordered
**`GetChat` (resolve/prime the peer) → `OpenChat`**.
**Honest flag (Gate B source-confirm):** the exact `openChat` *RPC-handler*
semantics for a still-unknown dialog live in `Td.cpp`, which is **not in our
pinned-source cache** (we pin `MessagesManager.cpp`). The `GetChat`→`OpenChat`
ordering is therefore a **defensive design that is safe either way** — if the
RPC handler does tolerate an unknown dialog, the extra `GetChat` is harmless; if
it does not (or if `open_dialog` early-returns as `:19527` indicates), it is
required. **Confirm the unknown-dialog behavior against `Td.cpp` before final
sign-off.**

### §3 — Lifecycle edges

One driver, `SourceOpener.ApplySet(ids)`. The opener maintains an in-memory
`opened map[ChatID]struct{}` and diffs `ids` against it. **The edge set is
exactly two** (Revised, Gate A — the Stage 1 "re-auth" third edge is collapsed;
`authGen` is deleted).

**Why two, not three (R2).** `runAuthLoop` (`internal/infra/telegram/repo.go:
342-371`) is **one-shot** (no loop): `client.NewClient` →
`swapSessionTable()` + `clientAdapter = tdlibClient` → `close(clientDone)`. It is
invoked once from `Start` (`repo.go:115`). `LogOut` → `CleanUp()`
(`repo.go:183-190`) **deletes the TDlib DB directory** → a clean restart is
required. So **there is no in-process re-auth path**: logout/re-auth, session
expiry, and session reload **all collapse to the next process start**, which goes
through the **same** `runAuthLoop` adapter-swap. The Stage 1 "re-auth observer"
and `authGen` counter were solving a mid-process re-auth that the current code
does not have.

| Edge | Trigger point | Action | Ordering hazard (dimension C) |
|---|---|---|---|
| **(1) startup adapter-swap** (covers restart + re-auth) | the repo's registered callback, fired in `runAuthLoop`'s swap critical section next to `swapSessionTable()` + adapter assignment (`repo.go:366-367`) | **full** `ApplySet(currentChannelIDs)` — open all | a fresh client has `open_count == 0` for every chat *by construction* → a full open is always correct; no generation counter needed |
| **(2) hot-reload diff** | `main.go:168` `Replace` callback (via watcher main.go:183-196) | diff: `added → OpenChat` (staggered), `removed → CloseChat` | see below |

**Trigger point = the adapter-swap, NOT `AuthStateReady` (R2).** The repo's own
comment (`repo.go:355-361`) says `AuthorizationStateReady` **lags the swap line
by the whole handshake and "can be lost"** — which is exactly why the dialog-table
swap is done *at the swap line*, not at Ready. Triggering the re-open at `Ready`
would inherit that loss → **silent silence after a session reset**. Firing in the
swap critical section (the same line the dialog table is swapped) is the correct,
loss-free point.

**The remaining (single) ordering hazard — reload vs. startup-swap overlap.**
A hot-reload `ApplySet` (edge 2) and the startup-swap `ApplySet` (edge 1) run on
the **same** `SourceOpener` (one mutex). `ApplySet` is a pure diff against
`opened`, so it is **idempotent under re-apply**: a reload firing while the
startup open-batch is still running just re-diffs the same set → **no double-open**
(an ID already in `opened` *or* in-flight is not re-`OpenChat`'d; the per-ID
in-flight guard, §1, closes the in-flight window). This is the mtcute/teleproto
idempotency: mtcute's `notifyChannelOpened` increments a ref-count and *does not
re-fetch* if already open (`manager.ts:383-388`); teleproto `arm` skips
`if (entry.channels.has(id))` (1.232.1). We get the same "no double-open" from the
diff + in-flight guard. **Revised (Gate A):** the Stage 1 hazard was
"reload racing re-auth"; with re-auth collapsed to the startup-swap, the only
overlap is reload-vs-startup-swap, and it is resolved by the same diff + the
in-flight guard (plus async execution, §1, so a slow open batch cannot block the
reload's diff).

### §4 — Stagger

**Decision (Revised, Gate A — D1).** Stagger the N `OpenChat` calls of an open
*batch* with a **per-operation** pace **implemented locally in the opener** — a
ctx-aware short sleep (or `time.Ticker`) **between** opens, in the background
goroutine from §1. The *stagger requirement* (don't fire N `openChat` in the same
instant) is **unchanged**; only the mechanism is local.

**Why local, NOT the `internal/service/limiter` (R6).** `internal/service/limiter`
is a **per-chatID forward throttle**: `WaitForForward(ctx, chatID)` spaces forwards
*to the same chatID* by a fixed 3 s (`internal/service/limiter/service.go:12,
30-44`). An open **batch is N *distinct* chats**, so no two opens share a `chatID`
and a per-chat limiter **never fires** — reusing it does nothing. **Do not** add a
per-operation primitive to that package. **Revised (Gate A):** this supersedes
Stage 1's "I lean (a) add a per-op token-bucket to `limiter`" and the record's
P1-R3 "reuse `internal/service/limiter`" — **the limiter is for forwarding, a
different axis; the stagger is local to the opener.** (P1-R3 wording corrected in
the Frame's point 3, flagged, and here.)

**Shape.** For each ID to open in the batch, in the background goroutine:
`acquire pacing slot (ctx-aware sleep ≤ pace) → GetChat (resolve/prime, §2) →
OpenChat`. The **pace is a tunable** (a few hundred ms; **not a preset**, and
**injectable** for tests — R7). The re-poll itself is server-paced by the engine
and already ≤10-concurrent (`TDlib`:36854/36860), so **only the open *calls* are
staggered**; we do not touch the engine's poll cadence (guardrail 4: don't fight
the 10 cap). **DEVIATE from mtcute** (which documents the 5-10 open-batch
ban/limit risk — "…may start getting transport errors or even get banned",
`openChat` doc 0.32.4 — but leaves mitigation to the caller; a daemon has no
human caller to honor that warning, so Budva staggers itself).

### §5 — In-process state & failure edges

**State: plain set + per-ID in-flight guard, NOT ref-counting (decision).**
mtcute keeps `channelsOpened: Map<id, count>` and increments/decrements
(`manager.ts:153, 380-423`) because a *UI* can open the same chat in multiple
windows and only remove it on the last close. Budva has a **single opener — the
ruleset** — a chat is either in the set or not. Ref-counting would model N
"windows" that don't exist; it buys nothing for one integer. **Use
`map[ChatID]struct{}` (`opened`) + a per-ID in-flight guard** (R2 deleted
`authGen`; idempotency under re-apply is the **diff** + the in-flight guard, not a
counter). This is the same *effect* as mtcute's ref-count for a count that never
exceeds 1, at less machinery. (If a future feature ever opens a chat from a second
origin, ref-count can be added then; not now.)

**Failure edges (dimension F).** `ApplySet` returns nothing about per-chat
outcomes to the caller; the `Replace`/`loadSnapshot` path must **not** abort the
generation on an open failure (a ruleset load must not be blocked by one
inaccessible chat). **Revised (Gate A, R4): in-edge bounded retry is REQUIRED**
(not "allowed but not required"). A failed `GetChat`/`OpenChat` goes to `pending`
and is **retried within the same apply edge, with a bounded backoff**, by the
background goroutine from §1. **Rationale (Popper/Kahneman):** with a single
source, no reload, and no re-auth, there is **no "next edge"** — "retry on the
next edge" would **never converge**, so the in-edge retry is what makes a failed
open recover without an external event. Per ID:

| Failure | Detection | Semantics |
|---|---|---|
| **`GetChat` fails** (resolve/prime, §2 — runs for **every** opened source) | `*client.Error` from `GetChat` | do NOT `OpenChat` yet; keep ID **pending**; **retry the `GetChat`→`OpenChat` sequence in-edge with bounded backoff**; on exhaustion log WARN. (Revised: Stage 1 had this only for the "ambiguous" row; now `GetChat` primes the peer for *all* opened sources, so it can fail for any of them.) |
| **`OpenChat` fails — inaccessible** (no-access / `CHANNEL_PRIVATE`-class) | go-tdlib error on `OpenChat` | mark **pending**; log WARN; **in-edge bounded backoff retry**; on exhaustion stay pending (next edge, if any, also retries). Never fail load. |
| **`OpenChat` fails — kicked / member-removed** | membership error | mark **pending** + log; same in-edge retry. (If a source the account can no longer see is in the ruleset, that is a *ruleset* problem, surfaced by the log, not a load failure.) |
| **`OpenChat` fails — bot-guard / auth** (account is a bot; or not authorized) | `CHECK(!is_bot)`-class / auth error | **fatal-to-feature**: log ERROR once, **stop retrying** (do not spam) for this session. `OpenChat`'s `CHECK(!is_bot)` (`TDlib`:19524) means a **bot** session cannot open — if Budva's session were a bot this fix is inapplicable; precondition = Budva uses a **user** account (per the record). |
| **Mid-batch interruption** (ctx cancelled / process stop during the N-batch) | ctx.Done in the pacing loop | stop opening; the **pending** set (never-opened IDs) is retained in-memory and re-applied on the next edge (the startup-swap re-opens all; a restart re-opens all). Because the startup-swap does a *full* re-open, a mid-batch stop self-heals on the next lifecycle event. |

**`CloseChat` failure (removal edge):** non-fatal — log, drop from `opened`;
if the ID reappears the next `ApplySet` re-opens it. A lingering open after a
failed close is *harmless* (an extra open chat just keeps its channel acquiring;
it does not cause the silence bug).

**Pending-set note:** "pending" = "we intended to open but the call failed."
It is **retried in-edge (bounded backoff, R4)** and, if still failing, carried to
the next edge (startup-swap re-opens all). Consistent with "open-once-keep, no
heartbeat" (guardrail 3): the retry is *bounded* within one edge, not a periodic
re-open heartbeat.

### §6 — Test & acceptance (CI-assertable vs box-only)

**CI-assertable (pure-Go, no cgo) — dimension G:**
- **Classifier table test** (`internal/domain`): use the **1e12** boundary —
  channel/supergroup `-100_000_000_000 - 1` (i.e. `|id| > 1e12`) → open; basic
  group `-999_999_999` (`|id| ≤ 1e12`) → no; and the **positive-id row
  (`id ≥ 0` → private → no)** is asserted **only as a defensive pure-function
  property** — *not* a live source path, because the loader negates every source
  key to `< 0` (`internal/infra/ruleset/repo.go:219`), so a stored source key is
  never `≥ 0`. (Revised, Gate A: Stage 1's table used the wrong 1e11 boundary and
  treated the positive/raw rows as live; both corrected.)
- **Open-set diff test** with a **mocked adapter** (the `clientAdapter` interface
  is already mockable — `GetChat`/`OpenChat`/`CloseChat` are on it): build a
  `SourceOpener` over a fake adapter; call `ApplySet([A,B,C])` then
  `ApplySet([B,C,D])`; **assert** `OpenChat` called with `{A,B,C}` (1st) and `{D}`
  (2nd), `CloseChat` called with `{A}` (2nd), and **idempotent re-apply** →
  `ApplySet([B,C,D])` again calls neither. Assert `OpenChat`/`CloseChat` are
  called with the **exact marked IDs** (the values that would go to go-tdlib).
  Also assert the **`GetChat`→`OpenChat` ordering** per opened ID (R1): `GetChat`
  observed *before* `OpenChat` for each opened source.
- **Stagger pacing test** (Revised, Gate A — was missing in Stage 1): for an N-ID
  open batch, with the **pace injectable** (test sets a small/nonzero pace and
  records call timestamps), assert the `OpenChat` calls are **paced, not all in
  the same instant** — e.g. the number of `OpenChat`s in the first 10 ms is < N,
  or inter-call gaps ≥ the (small) injected pace. This is the CI signal that the
  stagger actually fires.
- **Adapter-swap callback test** (Revised, Gate A — replaces the Stage 1
  "re-auth re-open" test, which was a *fake* CI test: bumping the deleted `authGen`
  / simulating `AuthStateReady` tests the *reaction*, not the **trigger wiring**):
  mock the repo's registered startup-swap callback, fire it, and assert it invokes
  a **full** `ApplySet` (all current channel IDs opened). The **actual**
  re-auth → open wiring (the callback firing inside a real `runAuthLoop` swap) is
  **box-only** — it needs a live session reset (below).
- **Failure-edge tests** (Revised, R4): mocked adapter returns an error on
  `OpenChat` (and separately on `GetChat`) for one ID → assert the ID is marked
  pending, `ApplySet` does **not** propagate the error (load path unaffected), and
  the **in-edge bounded retry** re-issues the `GetChat`→`OpenChat` sequence within
  the same edge (assert a second call attempt before the goroutine yields, and
  that it stops after the bounded count).
- **cgo-CI** (the TDLib-provisioned handler package): builds the two new
  `clientAdapter` methods (`OpenChat`/`CloseChat` compile against go-tdlib v0.7.6)
  — i.e. the adapter additions type-check and link. This is the "the wrapper
  exists and is 1:1" signal, not a behavior test.

**Box-only (live TDlib + server) — final acceptance, post-deploy:**
- **Real-time arrival**: with the owner's official client **closed** on a
  channel/supergroup source, Budva's session still receives that source's new
  messages (the actual bug symptom: silence when no session has it open).
  This CANNOT be asserted in any test (needs a live account + server open-state).
- **Startup-swap / re-auth open wiring**: after a **live** session reset
  (re-auth, or process restart), the open set is fully re-opened — the adapter-swap
  callback firing inside a real `runAuthLoop` swap (see CI test above for the
  mock; this is the live trigger).
- **N>10 FIFO behavior**: with >10 channel/supergroup sources, deferred sources
  still acquire (no starvation) — observed at runtime (the engine's unbounded FIFO,
  `TDlib`:36854/36860; source-confirm 2).
- **Large-N (20–50 sources) tail latency (owner-informed acceptance, R8):**
  deferred sources still acquire via the engine FIFO, but there is **real
  catch-up latency after a cold start** (the 11th+ source waits on the 10-poll
  cap). Named as an **owner-informed-acceptance** item — the owner decides the
  acceptable catch-up budget for their N, not a CI threshold.
- **No read API issued**: the box's TDlib client log shows `openChat` /
  `closeChat` / `getChannelDifference` but **no** `readHistory` /
  `readChannelHistory` / mark-read (conformance = never call a read API).

**Acceptance signals named (Frame exit):** *CI* = classifier table (1e12;
positive row defensive-only), diff/`OpenChat`/`CloseChat` id-set + `GetChat`-before-
`OpenChat` ordering + idempotency, stagger pacing (injectable pace), adapter-swap
callback → full `ApplySet`, in-edge bounded retry (pure-Go, mocked adapter), and
cgo-CI adapter link. *Box-only* = real-time arrival with owner's client closed;
live re-auth/restart open wiring; N>10 no-starvation; large-N tail-latency budget
(owner-informed); no read API in the client log.

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
- **Identical under either account fact (Franklin).** The design is **the same
  whether Budva runs a dedicated forwarding account or the owner's daily phone
  account**. The account fact moves only the **risk weight** (how conspicuous the
  footprint is at large N — a dedicated account has no "normal human profile" to
  deviate from, so N open channels read as a normal active user; a daily phone
  account with N channel windows held open is the desktop-multi-window shape). It
  does **not** change the mechanism. **The design can be finalized now**; the
  account fact is a **sign-off risk input, not a design gate** (it does not block
  Gate A/B or the go-ahead).
- **Large-N (20–50 sources) tail latency — owner-informed acceptance.** At N>10
  the engine defers (10-poll cap, FIFO, `TDlib`:36854/36860); deferred sources
  still acquire but with **real catch-up latency after a cold start**. This is
  named as an **owner-informed-acceptance** item (§6): the owner sets the
  acceptable catch-up budget for *their* N; it is not a CI threshold and not a
  design blocker (there is no alternative that avoids the engine cap — guardrail 4).

### §8 — Precedent citations

| Mechanism | Verdict | Pinned project + version | One line |
|---|---|---|---|
| Open/not by **id-magnitude, single 1e12 boundary** | **ADOPT** | Telethon v1.42.0 `resolve_id` (`if marked_id > 1000000000000: PeerChannel`); Pyrogram 2.0.106 `get_peer_type` (`utils.py:236`); fixture `1002641439846 = 1e12+…` (`internal/infra/ruleset/repo_test.go`) | `|id|>1e12` → channel/supergroup (open); `|id|≤1e12` (stored, always `<0`) → basic group (no); the `id≥0` private row is **defensive-only / unreachable** for source keys (loader negates all, `repo.go:219`). |
| `GetChat` to **resolve the input peer** before `OpenChat` | **ADOPT** | mtcute 0.32.4 `openChat` (`resolvePeer` before `notifyChannelOpened`); teleproto 1.232.1 `arm` (`getInputEntity`) | Order **`GetChat`→`OpenChat`** per ID so the peer is primed; `open_dialog` early-returns on `!have_input_peer` (`MessagesManager.cpp:19527`). **Revised (R1):** not a "type fallback" — id-magnitude already decides; `GetChat` primes the peer. |
| Open-once-keep, idempotent re-open | **ADOPT** | mtcute 0.32.4 `notifyChannelOpened` (`manager.ts:380-397`); teleproto 1.232.1 `arm` dedup | Re-apply is a no-op for already-open/in-flight IDs (no double-open); a plain set + per-ID in-flight guard gives this for a single opener. |
| Full re-open at **session reset** (startup-swap; re-auth collapsed into it) | **ADOPT (shape)** | teleproto 1.232.1 `_resumeAuthorization`→`_resume` (re-arms the set after a session reset) | A fresh client has `open_count==0` for all chats → full open at the **adapter-swap** (not `AuthStateReady`); **Revised (R2):** re-auth edge deleted, no `authGen`; the live trigger is box-only. |
| Close-on-removal | **ADOPT** | mtcute 0.32.4 `closeChat` = "un-does `openChat`" (`notifyChannelClosed` `manager.ts:409-423`) | Ruleset removal → `CloseChat`; entry dropped from the set (set-delete, not ref-count). |
| Keep a **set** open non-interactively | **ADOPT (shape)** | mtcute (`channelsOpened` map); Telethon v1.42.0 `MessageBox` (whole-set re-poll, no user); teleproto 1.232.1 `watch` (a set) | The set-at-once shape is real, production client behavior — not novel. |
| Stagger the open batch (**local**, not the per-chat limiter) | **DEVIATE from mtcute** | mtcute 0.32.4 `openChat` doc (5-10 ban/limit warning) | mtcute documents the risk but leaves mitigation to the caller; a daemon has no human caller → Budva staggers itself, **locally in the opener** (per-op pace). **Revised (R6):** `internal/service/limiter` is a per-chatID forward throttle — shape mismatch, **not** reused; P1-R3 wording corrected. |
| **In-edge bounded retry** on failed open | **NOVEL (source-verified reason)** | Popper/Kahneman (no "next edge" exists for a single source with no reload/re-auth) | A failed `GetChat`/`OpenChat` is retried **within the same edge** with bounded backoff, else it never converges (R4). No precedent; justified by the edge-collapse in R2. |
| `OpenChat`-RPC-holds-N mechanism | **NOVEL (engine-native)** | TDlib `22d49d5` (`open_count`:19524, `!have_input_peer` early-return:19527, re-arm:37423, 10-cap FIFO:36854/36860) | The re-poll is owned by the linked C++ engine, not hand-built (P1-R5); precedent cannot close this — **Gate B target** (incl. the `openChat`-for-unknown-dialog semantics in `Td.cpp`, not in our pinned cache), closed at runtime by the box-only real-time-arrival acceptance. |

### §9 — Exclusions re-check (from the Frame; confirm none absorbed)

Re-verified after R1–R8 (the re-auth collapse in R2 does **not** absorb any of
the five exclusions).

| Frame exclusion | Status in this revised draft |
|---|---|
| **Hand-built Go `getChannelDifference` poller** | **Not absorbed.** `SourceOpener` has **no timer/re-poll of its own**; it only issues `GetChat`/`OpenChat`/`CloseChat` (the stagger sleep is pacing, not a channel-difference poller). The re-poll is the engine's (P1-R5). §1/§7 make this the *central* layering point. |
| **Problem 2 (endpoint HOL)** | **Not absorbed.** R5's async execution keeps `ApplySet` out of the `pushManager.Replace` critical section — it **relieves** the send path, and adds no dispatcher/concurrency/timeout work; the draft still only adds the open-set lifecycle. |
| **Asserting real-time delivery in any test** | **Held.** Real-time arrival, live re-auth/restart open wiring, and N>10/large-N no-starvation are **box-only** (§6). CI asserts only that `OpenChat`/`CloseChat` are *called* correctly (diff, ordering, idempotency, stagger pace, adapter-swap → full `ApplySet`) — the adapter-swap CI test asserts the *trigger fires a full ApplySet* (mocked), **not** that updates arrive. |
| **Any read API (`readHistory`-class)** | **Not added.** The opener issues no read RPC; `GetChat` (resolve/prime, §2) is a *read of chat metadata*, not a `readHistory`/mark-read — it does not advance `pts`/mark read. Box-verified in §6 (client log shows no mark-read). |
| **Shrinking the open set to non-member channels** | **Not absorbed.** The open set = **all** channel/supergroup sources (no membership filter, no shrink-to-non-member); §2/§5 keep all. |

**Exclusions honored.** No hand-built poller, no Problem-2 scope, no in-test
real-time assertion, no read API, no open-set shrinkage — after the re-auth
collapse (R2), the local stagger (R6), and the async execution (R5).


## Gate A / Gate B verdicts

*(pending — output: `research/design-dimensions.md`)*
