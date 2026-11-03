# Precedent survey — openChat lifecycle & chat-type (10-30, Problem 1)

Stage 0b, research-led precedent survey. Companion to `facts-and-model.md` and
`design-and-position.md`. **No code.** This file answers, per the "behave like an
official client" benchmark, how **mature, production MTProto/Telegram user-client
projects** solve the four mechanisms the Problem-1 fix touches.

**Scope of "precedent" (kept precise).** Two architectural families exist, and
they must not be conflated:

- **Pure-MTProto hand-built clients** (Telethon, Pyrogram, mtcute, teleproto,
  GramJS): the library reimplements the MTProto update machinery in its own
  language and **rolls its own `getChannelDifference` re-poll**. Because they do
  not link the C++ TDlib, they **cannot** send the TDlib application-level
  `openChat` RPC the way Budva will; instead they maintain their **own**
  "open set" and their own re-poll loop. This is exactly the hand-built poller
  the record *rejected* for Budva (P1-R5).
- **TDlib-wrapped clients** (Budva via go-tdlib; WTelegramClient; TDLib mobile):
  the C++ engine is linked in-process and **owns** `open_count` + the
  `getChannelDifference` re-poll. The app only sends `openChat` / `closeChat`.
  **Budva is in this family.** The C++ engine's behavior is already pinned in
  `facts-and-model.md` (commit `22d49d5`, `MessagesManager.cpp`).

So for Budva the pure-MTProto hand-built libraries are **shape precedent** (they
show a production client *keeping a SET of channels persistently acquired*),
while the **OpenChat-RPC-on-N-hold-statically** mechanism itself is the C++
engine's own behavior. I call out which is which in every section.

## Method

Deep source reads, each pinned to a version and (where code is quoted) file:line.
A small number of targeted web lookups confirmed a behavior and located the exact
source file to pin. **Shallow / source-only; no builds.**

| Precedent | Pinned to | Family | Read |
|---|---|---|---|
| **Telethon** (Python) | `v1.42.0` (`telethon/version.py`) | hand-built MTProto | deep (cloned, source-read) |
| **Pyrogram** (Python) | `2.0.106` (`pyrogram/__init__.py`) | hand-built MTProto | deep (cloned, source-read) |
| **mtcute `@mtcute/core`** (TS/JS) | `0.32.4` (open-chat doc) + `master` (re-poll; see pin caveat) | hand-built MTProto | deep (source-read) |
| **teleproto** (Go→WASM) | `1.232.1` | hand-built MTProto | deep (source-read) |
| **GramJS** (TS) | archived/read-only as of 2026-07-14; dev moved to teleproto | hand-built MTProto | **not cloned** (recurring GitHub auth prompt on that repo; see caveat) |
| **TDlib C++ engine** | commit `22d49d5` (go-tdlib v0.7.6 pin) | TDlib-wrapped | already pinned in `facts-and-model.md` |
| **normative spec** | `core.telegram.org/api/updates` | — | already quoted in `facts-and-model.md` |

**Pin caveat (mtcute).** `openChat`/`closeChat` (the 5–10 warning) were read at
**`@mtcute/core` 0.32.4** (`jsr.io/.../0.32.4/highlevel/methods/chats/open-chat.ts`).
The re-poll internals (`channelsOpened`, `notifyChannelOpened`,
`_fetchChannelDifference`) were read from **`master`**
(`packages/core/src/highlevel/updates/manager.ts`) — a moving ref, so treat exact
line numbers as indicative; the *behavior* (ref-counted open set + server-
`timeout`-paced re-arm gated on membership in the open set) is stable across the
`0.32.x` line and is consistent with the 0.32.4 `openChat` doc. **GramJS** could
not be cloned (its repo returns a GitHub credential prompt; the org is now
`gram-js` and the project is archived), so it is cited only as "archived;
successor = teleproto," not source-read.

**Budva-internal facts used below (source-verified in this worktree):**
- The ruleset stores sources as `map[ChatID]*Source`, `ChatID = int64`
  (`internal/domain/ruleset.go:15`, `internal/domain/chat.go:4`) — **id only, no
  type** on the domain object.
- YAML ids are written **positive**; the loader **negates them all** on load
  (`internal/infra/ruleset/doc.go:19-20`: *"Идентификаторы чатов в YAML
  записываются положительными, transform инвертирует их (Telegram использует
  отрицательные ID для групп и каналов)"*; `negateChatIDs` at
  `internal/infra/ruleset/repo.go:291`). Test fixtures use **marked** mags
  (`1002641439846` = `1e12 + 2641439846`, a channel) and basic-group mags
  (`1001000`), asserting `rs.Sources[-1002641439846]` (repo_test.go:17-21,
  139-147). So the **stored** id is the marked/negated form.
- A `GetChat` oracle already exists in Budva's Telegram infra
  (`internal/infra/telegram/client_adapter.go:40`, `GetChat(*client.GetChatRequest)
  (*client.Chat, error)`), and a chat-warming path resolves id → typed `client.Chat`
  (`internal/infra/telegram/warmup.go`).

---

## 1. Open-set derivation + chat-type detection

**Question.** The ruleset stores only a chat id. To decide whether to `OpenChat`
we need the type (channel/supergroup → open; private/basic-group → don't). Do
mature projects call `getChat` first to branch on type, and how do they detect
channel vs supergroup vs basic-group vs private? Is a per-chat `getChat` on load
established, or do they get the type another way (updateChat, cached dialog, id-sign)?

### What each project does

**Both Telethon and Pyrogram classify a *bare id* by id-sign / magnitude with
NO API call** — the "Bot API" marked-id convention. This is the single most
direct precedent for our decision.

- **Pyrogram** `utils.py:201-250` — a pure function, no network:
  ```python
  MAX_CHANNEL_ID = -1000000000000          # :202
  MIN_CHAT_ID    = -2147483647
  def get_peer_type(peer_id):              # :236
      if peer_id < 0:
          if MIN_CHAT_ID <= peer_id:  return "chat"        # basic group
          if MIN_CHANNEL_ID <= peer_id < MAX_CHANNEL_ID:  return "channel"  # channel/supergroup
      elif 0 < peer_id <= MAX_USER_ID: return "user"       # private
      raise ValueError(...)
  ```
  i.e. **sign + magnitude → {user, basic-group, channel}**, no `getChat`.

- **Telethon** `utils.py:1005-1063` — same convention:
  ```python
  def resolve_id(marked_id):
      if marked_id >= 0:  return marked_id, types.PeerUser      # private
      marked_id = -marked_id
      if marked_id > 1000000000000:                             # channel
          return marked_id - 1000000000000, types.PeerChannel
      return marked_id, types.PeerChat                          # basic group
  ```
  (channel encoded as `-(100_000_000_000 + channel_id)`, :1050).

- **Telethon distinguishes channel (broadcast) from supergroup (megagroup) by a
  flag on the resolved *entity*, not the id** — `ChatGetter`
  (`tl/custom/chatgetter.py`): `is_private` = peer is `PeerUser` (:118);
  `is_channel` = peer is `PeerChannel` (:145); `is_group` = `PeerChat` **or**
  `PeerChannel and not chat.broadcast` (:132-138). The broadcast/megagroup
  distinction requires the `Channel.broadcast` field, obtainable only from a
  resolved `Channel` (via dialog/entity cache or `get_entity`→`getChat`),
  *not* from the id.

**mtcute / teleproto: the "open a channel" path branches on peer type, not id.**
mtcute `openChat` (0.32.4) does `resolvePeer(chat)` then
`if (isInputPeerChannel(peer)) { ...notifyChannelOpened(peer.channelId, pts) }`
— i.e. it needs a **resolved input peer** (it even fetches the dialog for `pts`),
so it is type-aware by construction, and it simply **no-ops for non-channels**
(private/basic-group never enter the open set). teleproto `arm` (1.232.1) is
identical in spirit: it resolves each watched chat via `getInputEntity` and only
arms `if (input instanceof Api.InputPeerChannel)`.

### What it does **not** show

- **Neither Telethon nor Pyrogram ever sends `openChat`/`closeChat`.** Confirmed
  by whole-tree search: zero matches in either's code *or* generated TL schema
  (`telethon_generator/data/api.tl` has `messages.readHistory`/`channels.readHistory`
  but no `openChat`). They are hand-built and drive channel acquisition by their
  **own** polling (see §4), so "open a chat" as a server RPC is simply not in
  their model. They therefore provide **no precedent for the OpenChat-RPC
  itself** — that is a TDlib-family mechanism (the C++ engine).
- **No precedent derives the type from a bare *raw-positive* channel id** (an id
  with no `-100…`/negation mark). The id-sign shortcut assumes the id is
  already in the **marked** form. If a source is stored as a raw positive channel
  id, the magnitude test misclassifies it (e.g. `2641439846` looks like a basic
  group). This is a real edge — see "What no precedent shows."

### Implication for our design

**Adopt id-magnitude as the cheap primary classifier, with the `GetChat` oracle as
the authoritative fallback.** Two independent signals both already exist in
Budva and both match a mature pattern:
1. **Id magnitude is the documented, stored convention.** `ruleset/doc.go:19-20`
   fixes that stored ids are negated marked ids; the fixtures prove the
   `1e12`-channel boundary. So a Telethon/Pyrogram-style
   `|id| > 100_000_000_000 → channel/supergroup; small |id| → basic group;
   id ≥ 0 → private` classifier (a ~5-line pure function, no RPC) covers the
   **documented ruleset** and is exactly what two mature libraries do.
2. **`GetChat` is the type-authoritative path** (mtcute/teleproto branch on the
   resolved peer; Telethon uses `get_entity`/dialog for the broadcast flag), and
   Budva already has it wired (`client_adapter.go:40` + `warmup.go`). Use it to
   (a) resolve the broadcast-vs-megagroup sub-distinction if that ever matters,
   (b) handle any raw/underspecified id, and (c) supply the `pts`/access-hash the
   open path needs.

The design's P1-R1 already says "type is diagnostic only, not a design input";
this precedent sharpens *how* the diagnostic is read: **id-magnitude for the
open/don't-open split (free, matches both Python libraries), `GetChat` to
confirm/complete.** Do **not** invent a per-source `getChat`-always loop just to
get the type — that is the expensive path the Python libraries avoid with the id
sign, and it is only needed where the id alone is ambiguous.

---

## 2. Lifecycle: when is openChat fired, and is it kept open?

**Question.** At startup / after re-auth / on config hot-reload (add/remove a
subscribed chat): when do clients issue `openChat`, do they ever `closeChat`, and
is "open once, keep open" how a real client leaves a channel it has viewed (vs
open/close as the user navigates)?

### What each project does

**mtcute is the cleanest precedent and matches our design almost exactly.**
`openChat` (0.32.4) "Inform the library that the user has opened a chat … the
library will periodically ping the server to keep the updates flowing."
`closeChat` is documented as **"Un-does the effect of {@link openChat}."**
Internally (`manager.ts`): `notifyChannelOpened` adds to a **ref-counted**
`channelsOpened: Map<channelId, count>` — first open sets count 1 and forces an
immediate `_fetchChannelDifference`; a second open just increments (returns
false, no re-fetch) (`:380-397`). `notifyChannelClosed` decrements and **deletes
the entry only when the count reaches 0** (`:409-423`). This is a literal
`open_count` model — the same one the C++ engine uses (`open_dialog`/`close_dialog`
at `:19524`/`:19640` in the pinned source). **Kept open until explicitly closed;
not a heartbeat; re-open is idempotent.**

**teleproto (Go) `watch` (1.232.1) shows the re-auth lifecycle explicitly.**
`watch(chats, handler)` takes a **set** of chats, keeps each channel in a `Set`
of watched ids, and returns a **stop** function (→ close). Crucially, its
`_resumeAuthorization`→`_resume` (1.232.1) **re-arms all watches after a
re-auth** (`watchedChannelIds()` is diffed; missing channels are re-armed and
`catchUp()` is called). This is direct precedent for **"re-open the full set at
re-auth"** — exactly our P1-R2 state-reset edge. Its `arm` also **dedups**
(`if (entry.channels.has(channelId)) continue`), so re-arming is idempotent —
matching mtcute's ref-count-no-double-fetch.

**The hand-built libraries' *steady-state* is "keep open, zero extra calls."**
Once Telethon's MessageBox has a channel entry, it re-polls only on the
server-returned `timeout` (or the 15-min `NO_UPDATES_TIMEOUT` floor); it never
re-issues an "open" because it has no open concept — the entry just *is*. mtcute
likewise: after `notifyChannelOpened`, the **only** recurring activity is the
`channelDiffTimeouts` re-arm gated on `channelsOpened.has(channelId)`
(`manager.ts:1056-1062`). So **"open once, then zero open/call churn, engine
self-re-polls, never close"** is the real terminal state in a production client.

### Conformance nuance (the honest part)

- **"Keep open" is a real *client* terminal state (an unclosed tab/window) —
  corroborated.** It is **not** what a *phone* user does (a phone client opens
  ~1 and closes on switch); it is **desktop multi-window** behavior. This matches
  the record's already-corrected three-layer conformance assessment — the
  precedent *confirms* the correction rather than contradicting it.
- **"Open/close as the user navigates" is also a real client behavior** (mtcute's
  `closeChat`, Telethon's remove-from-set). Our daemon does *not* navigate, so it
  uses the *keep* end of the same API. That is the conformant choice for a
  non-interactive process: the same RPCs a client uses, held at the "tab left
  open" state.

### What it does **not** show

- **No precedent issues `openChat`/`closeChat` for *server*-side open-state the
  way TDlib does.** mtcute/teleproto "open" is a *local* re-poll trigger (they do
  not send `messages.openChat`). So the precedent corroborates the **shape**
  (open-once, keep, close-on-remove, ref-count, re-open at re-auth) but the
  **specific OpenChat-RPC mechanism** rests on the pinned C++ engine, not on a
  hand-built library. (Both are "client-performed"; only the TDlib path goes
  through the server open-state.)
- **No hand-built precedent *auto-closes on a timer* or *rotates* the open set.**
  Nobody closes a kept channel to save load; load is bounded by the re-poll
  cadence and the ~10 cap, not by opening fewer. This supports "keep ALL, do not
  shrink" (P1-R1/source-confirm 3) — no precedent rotates to one-open.

### Implication for our design

**Adopt the full lifecycle; it is precedent-backed edge-for-edge.**
- *Startup / restart:* open the whole channel/supergroup source set (teleproto
  `watch` takes a set; mtcute ref-counts).
- *Keep open, zero churn:* open once, no heartbeat, no re-open — mtcute's
  idempotent ref-count and the hand-built steady state both confirm "open once,
  zero calls."
- *Re-auth:* **re-open the full set** — direct precedent (teleproto
  `_resumeAuthorization`→`_resume` re-arms all watches).
- *Hot-reload:* **add → `OpenChat`; remove → `CloseChat`** — mtcute's
  `closeChat` = "un-does `openChat`", ref-count to 0 removes the entry; exactly
  the P1-A1 add/remove contract. **No precedent rotates or auto-closes**, so
  "keep ALL sources, close only on removal" is the conformant and simple choice.

---

## 3. Stagger / throttling of the open batch

**Question.** When a client (re)opens many chats at once (startup / re-auth), is
there any rate-limiting or staggering of `openChat`, or are they fired
immediately? Precedent for throttling a batch?

### What each project does

- **mtcute — the only explicit batch-size guard.** The `openChat` doc (0.32.4)
  carries a **verbatim warning**:
  > "Opening a chat with `openChat` will make the library make additional
  > requests every so often. Which means that you should **avoid opening more
  > than 5-10 chats at once**, as it will probably trigger server-side limits and
  > you might start getting **transport errors or even get banned**."

  This is a *user-facing caution* against opening a large batch, not an internal
  stagger — mtcute leaves the "don't fire 30 at once" judgment to the caller,
  but it **documents the risk and the ~10 bound.**
- **The re-poll itself is inherently serialized / server-paced, not a burst.**
  - Telethon: the update loop pulls **one** `getChannelDifference` per iteration
    (`client/updates.py:368-372`: `get_diff = ...; diff = await self(get_diff)`,
    then loop) — channel polls are **naturally serial**, one outstanding at a
    time; no simultaneous open burst to stagger.
  - mtcute: each channel's re-poll is its own `setTimeout`
    (`manager.ts:1056-1062`), paced at the **server-returned `timeout`**; the
    initial fetches from a batch of `openChat`s are issued sequentially as the
    caller calls them, each re-armed independently.
  - The pinned C++ engine enforces the cap *internally*: `MAX_CONCURRENT_GET_CHANNEL_DIFFERENCES = 10`
    (`:36860`) + the unbounded FIFO `pending_get_channel_differences_`
    (`:36854`) — so even N simultaneous opens cannot exceed 10 in-flight polls.

### What it does **not** show

- **No precedent has a *built-in* open-batch stagger** (a limiter that spaces N
  `openChat` calls). mtcute warns the caller; Telethon/Pyrogram have no open
  batch at all (they poll); the TDlib engine caps *polls*, not the open *calls*.
  So "stagger the initial `OpenChat` batch via `internal/service/limiter`" (P1-R3)
  is **our own conformance guardrail** — it is *corroborated* by mtcute's warning
  (the risk is real) but is **not copied from a library's mechanism.**

### Implication for our design

**Deviate-from-mtcute (adopt the guardrail, not the silence) — because the
conformance benchmark is the *official client*, and the record already treats
stagger as a standing guardrail.** mtcute *documents* the risk of a big open
batch (limits / transport errors / possible ban) but does *not* implement
mitigation — it trusts the caller. A daemon opening its whole ruleset set at
startup/re-auth has no human to honor that warning, so **we must be the caller
that staggers.** Reuse `internal/service/limiter` to space the initial/re-auth
`OpenChat` batch (P1-R3). This is a **novel-because-source-verified** guardrail:
the *risk* is source-verified (mtcute's verbatim 5–10 warning + the pinned
engine's 10-poll cap), the *mitigation* (stagger) is our deliberate addition, and
it matches "a real client won't fire N opens in the same instant." Do **not**
fight the engine's 10 cap — the cap is on *polls*, and stagger is on *open calls*;
the two are orthogonal and both fine.

---

## 4. A "keep a SET of chats persistently open" daemon pattern

**Question.** Is there a mature precedent for a *non-interactive* client that
keeps a **set** of channels/supergroups open (not one-at-a-time user-driven)? If
yes, how? If **no** precedent for the set-at-once shape, say so — it is a
load-bearing conformance finding.

### What each project does — the set-at-once shape IS corroborated

**This is the headline. The shape is real and production-used.**

- **mtcute — an explicit, ref-counted open *set* with a documented multi-chat
  limit.** `channelsOpened: Map<number, number>` (`manager.ts:153`) holds
  **many** channels simultaneously; `channelDiffTimeouts` (`:152`) holds a
  re-poll timer per open channel; the open→re-poll→re-arm loop is gated on
  `channelsOpened.has(channelId)` (`:1056`); the 5–10 warning (`openChat` doc) is
  *specifically about opening a SET at once.* This is a non-UI-agnostic
  library-level "keep this set of channels acquiring" API. **Closest match to
  Budva's intent.**
- **Telethon — the strongest "non-interactive, set, no-user" precedent.** Its
  `MessageBox.map` holds **every** channel the account has (populated from the
  dialog list, `client/dialogs.py:87-88`), each with its own `pts` + `deadline`,
  and the update loop re-polls *each* expired entry on a deadline
  (`client/updates.py:368-490`). There is **no "open" concept and no user** — a
  daemon holding every channel it has. This is precisely a non-interactive client
  keeping a *set* of channels persistently acquired, **indefinitely.**
- **teleproto — `watch` takes a *set* and keeps it open across re-auth.**
  `watch(chats, ...)` accepts an array; `_resume` re-arms the surviving set after
  re-auth; a `polling` governor exposes "shared polling load and per-channel
  request timing" (a shared, capped poll budget across the set).

### The key mechanism distinction (load-bearing for conformance)

The hand-built clients keep the set open by **their own re-poll** (client-side);
Budva keeps the set open by the **C++ engine's open-state re-poll** (server open
via `OpenChat`). **Same shape, different owner of the re-poll.** For a
TDlib-wrapped client the OpenChat-on-N-hold-statically mechanism is what the
pinned engine does natively (`open_count` at `:19524`, the open-gated re-arm at
`:37423`, the 10-cap FIFO at `:36854/:36860`). The hand-built set-open
precedent **corroborates that "keep a set persistently" is normal, production
client behavior**; it does *not* itself use the `OpenChat` RPC.

### What it does **not** show

- **No hand-built precedent sends the `OpenChat` server RPC to hold a set.**
  (mtcute/teleproto/Telethon/Pyrogram are all client-side-re-poll; they can't or
  don't send `messages.openChat`.) So the *set* shape is corroborated, but the
  *OpenChat-RPC-holds-N-statically* mechanism rests on the **C++ engine** —
  already source-verified in the record. **This is the one dimension precedent
  does not close on its own; the pinned engine closes it.**
- **No precedent opens a set *and* closes it as a load-shedding strategy** —
  supporting "keep ALL, don't shrink."
- **No precedent documents a hard "max N open" beyond mtcute's 5–10 *caution*;**
  the *binding* cap is the engine's 10 (source) and the spec's 10 (normative).

### Implication for our design

**Adopt — the set-at-once shape has real, production precedent (three
independent libraries), and the OpenChat mechanism is the engine's native
behavior.** "A non-interactive client keeping a set of channels/supergroups open"
is *not* novel or out-of-distribution: mtcute (ref-counted open set + 5–10
warning), Telethon (whole-set persistent re-poll, no user), and teleproto
(`watch` a set, re-armed at re-auth) all do it. The only genuinely
engine-specific part is *how* the re-poll is owned (C++ `open_count` vs
hand-built timer) — and that is the pinned, already-verified mechanism. So the
conformance risk on the *shape* is low and precedent-backed; the risk that
remains is purely the **footprint at large N** (already assessed in
`facts-and-model.md`), not the legitimacy of the pattern.

---

## What no precedent shows (Gate B falsification targets)

Edges/mechanisms **no** mature project covers — these are where the design is
least backed by precedent and most needs the pinned source / a controlled test.

1. **`OpenChat` on N *server*-state-held channels, statically, from a non-interactive
   daemon.** Every hand-built client keeps the set open **client-side** (its own
   re-poll); **none** sends the `OpenChat` RPC to hold N. So the *RPC-holds-N*
   mechanism is engine-native and **precedent-closed only by the pinned C++
   source**, not by a library. *Falsify:* confirm at runtime that N `OpenChat`
   + idle (no re-open) sustains near-real-time channel delivery independently of
   the owner's client (the P1-A1/A2 open-set regression + a live acceptance, per
   the 10-29 runtime-acceptance method).
2. **A per-source `getChat`-always loop is NOT a precedent** — but the reverse
   edge is: **type from a raw-positive (unmarked) channel id.** The id-magnitude
   shortcut assumes the marked convention (`doc.go:19-20`). A source stored as a
   raw positive id would misclassify as a basic group and **never be opened** →
   silent (re-introduces the bug). *Falsify:* unit-test the classifier on raw vs
   marked vs the `-100…` boundary; route ambiguous ids through the `GetChat`
   oracle (`client_adapter.go:40`).
3. **No precedent implements an *open-batch stagger*.** mtcute warns, Telethon
   serializes naturally, the engine caps polls — but nobody staggers open calls.
   So the limiter-stagger (P1-R3) is our own guardrail; the *risk* it mitigates is
   source-verified (mtcute's ban warning), the *mechanism* is novel. *Falsify:*
   assert the initial/re-auth batch is limiter-paced (P1-A2) and that staggering
   does not delay first-diff beyond the server `timeout`.
4. **No precedent auto-closes on a timer or rotates the open set.** This *supports*
   "keep ALL," but means "keep ALL at large N" has no *negative* precedent either
   — the footprint at large N is bounded by the engine's 10-poll FIFO (source),
   not by a demonstrated client. *Falsify:* at N > 10, confirm via the engine's
   FIFO that deferred sources still acquire (no starvation) and that the 11th+
   is deferred-not-dropped (matches source-confirm 2).
5. **No hand-built precedent models `closeChat`-on-removal as a *config* event.**
   mtcute's `closeChat` is user-driven; our trigger is a ruleset hot-reload
   removal. The ref-count-to-0 removal (mtcute `:409-423`) is the closest, but the
   *trigger* (file-watch removal → close) is ours. *Falsify:* P1-A1 remove-edge —
   a `CloseChat` fires on ruleset removal and the engine stops polling it.

---

## Bottom line per mechanism

| # | Mechanism | Verdict | One-line reason |
|---|---|---|---|
| 1 | Open-set derivation + type detection | **Adopt (id-magnitude) + `GetChat` fallback** | Telethon `resolve_id` & Pyrogram `get_peer_type` classify a bare id by sign/magnitude with no RPC; the `-100…`/negated form is Budva's documented stored convention (`doc.go:19-20`); `GetChat` (already in Budva, `client_adapter.go:40`) resolves broadcast/megagroup + any raw-id ambiguity, as mtcute/teleproto branch on the resolved peer. |
| 2 | Lifecycle: when to open, keep open? | **Adopt** | mtcute's ref-counted `channelsOpened` (open-once, idempotent re-open, close-removes-at-0) + teleproto `watch` re-arming the full set after re-auth = open-once-keep, re-open at re-auth, `CloseChat` on ruleset removal; no precedent rotates/auto-closes, so "keep ALL" is the simple conformant choice. |
| 3 | Stagger / throttle the open batch | **Deviate-from-mtcute (add the guardrail)** | mtcute *documents* the 5–10 open-batch ban/limit risk but leaves mitigation to the caller; a daemon has no human caller, so stagger the initial/re-auth `OpenChat` via `internal/service/limiter` (P1-R3) — risk source-verified, mitigation novel. |
| 4 | Keep a *set* persistently open | **Adopt (shape precedent-backed; mechanism engine-native)** | mtcute (ref-counted open set), Telethon (whole-set persistent re-poll, no user), teleproto (`watch` a set) all keep a SET open non-interactively; the set *shape* is corroborated, while the `OpenChat`-holds-N *mechanism* is the pinned C++ engine's own `open_count` behavior. |

**Net.** The conformance benchmark "behave like an official client" is met on all
four: the *shape* (open-once-keep, re-open at re-auth, close-on-remove, keep a
SET, don't shrink) is directly precedent-backed across three production
hand-built libraries **and** the pinned C++ engine; the one place precedent alone
does not close the claim — `OpenChat`-RPC-holds-N-statically — is closed by the
pinned engine source (already in `facts-and-model.md`). The three load-bearing
novel points are (a) the OpenChat-RPC mechanism (engine-native, needs the P1-A1/A2
runtime regression), (b) type-from-raw-id (use `GetChat`, not magnitude alone),
and (c) the open-batch stagger (our guardrail, risk verified by mtcute's warning).
