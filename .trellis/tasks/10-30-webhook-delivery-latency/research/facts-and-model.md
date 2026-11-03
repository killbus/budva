# Facts & converged model — webhook delivery latency (10-30)

Forward, self-contained record for task `10-30-webhook-delivery-latency`.
Baseline: code `main` @ `415d90c` (post 10-29 push-stall fix, PR #7, v0.3.1).
**Status (2026-11-01):** the root cause of the headline symptom is
**converged** and the fix direction is **converged**; all three
implementation source-confirms (idle-open re-poll, >10 queueing,
member-channel idle delivery) are **resolved** by reading the pinned source.
What remains is lifecycle/stagger design + the owner's go-ahead. The record
was reset off main (`bb0519b`) on 2026-11-01 and lives on branch
`docs/10-30-type-agnostic-requirement` pending owner sign-off. All paths
repo-relative. **Producer-side only**: the webhook receiver is a black box
(accepted / rejected(class) / unreachable); nothing here changes it.

## The two problems, and which is real

| | Symptom | Status |
|---|---|---|
| **Problem 1 (HEADLINE)** | 10-minute window of **zero** inbound requests (text + media), coupled to the official client not having the chat **open**; a burst on open | **Root cause converged** (this record) |
| **Problem 2 (secondary)** | steady-state per-message queue-wait behind the serial per-endpoint worker (≤60 s at `timeoutSeconds:60`) | **Downgraded to "potential, unobserved"** — no owner-observed per-message delay; see its section |

Everything below leads with Problem 1. The pipeline / G-G-1 / exclusion
sections are kept because Problem 2 reuses them.

## Symptom (as observed, 2026-10-30)

A message visible in the official Telegram client is **not** observed as an
inbound POST by the same-host receiver until **several minutes later** — or,
in the headline case, not at all for a 10-minute window, then a catch-up burst.
Same machine — not network transit. The delay is **finite** (requests
eventually arrive); it is **not** the 10-29 self-deadlock (which was *infinite*:
zero inbound, idle push worker, restart required).

**The decisive coupling (owner, 2026-10-30, repeated):** while watching the
gateway inbound log, staying on the official client's **chat list** (chat NOT
opened) shows chat activity but produces **no** webhook request; the moment the
chat is **opened**, a **burst** of inbound requests appears. This invalidates
the assumption that had been silently carrying the model:

> *Once the official client shows a message, Budva's independent TDLib session
> must already have received the same `updateNewMessage`.*

The official client and Budva are **separate sessions** (distinct local DB,
update state, transport). The client's chat-list rendering establishes the
state of *that* client, not the update-publication stage reached by Budva.

## Root cause (converged 2026-11-01): Budva does not own the acquisition boundary of its ruleset sources

**What is broken (stated at the first-principles level).** The product contract
is declared by the ruleset: "source chat X's messages → endpoint Y". But *how*
X's updates reach **Budva's own session** is never asserted by Budva. Budva is a
**passive** subscriber that rides the account's update stream and never claims
any chat-open state. Telegram's per-chat delivery policy for channels/supergroups
is a function of *which sessions are actively acquiring that channel*, so a
session that never opens a channel can go **silent** for that channel — with the
recovery timing controlled by a condition **outside** Budva (someone else opening
it). **No latency bound exists for those messages; there is an acquisition gap,
not a slow delivery.** "Finite delay" is the wrong noun for the headline: it is
*delivery absence until an external trigger*.

**The mechanism, by chat class (open-state gating):**
- **Channels / supergroups (any size — no protocol threshold):** updates are
  **open-state-gated**. The pinned engine recovers a detected gap **only if** the
  dialog is open or the account is a member (below); the periodic
  `getChannelDifference` re-poll runs **only while** the dialog's open state is
  held. A session that never opens the channel is not an active acquirer for it.
- **Cross-session passive updates (why the owner's observation works):** for a
  **non-member** public channel/supergroup (or a private one via `chatInvitePeek`),
  **any** session that keeps polling that channel's difference causes the API to
  passively push that channel's updates to **all** logged-in sessions. So the
  official client opening the chat starts its own poll → the account's Budva
  session (and any other session) starts receiving → **burst**; closing the chat
  stops the poll → silence resumes.
- **Member channels/supergroups:** the API *usually* sends passive updates but
  *may stop or reduce* them; a member session that is not actively polling is
  still subject to deferral.
- **Private chats / basic groups:** NOT open-state-gated — they ride the **global
  update stream** (common message-ID sequence + `getDifference` seq-gap recovery),
  not per-channel difference. A 10-minute silence on a private/basic-group
  source is a **separate bug** (transport/session, local backpressure, or reload
  gap), not this mechanism.

**Three-layer evidence (this is why the experiment was withdrawn):**

1. **Normative spec** — `core.telegram.org/api/updates`, "Subscribing to updates
   of channels/supergroups" (fetched 2026-11-01): *"The API will usually send
   passive updates ... for channels/supergroups the user/bot is a member of.
   However, in some cases the API may stop sending updates (or send fewer
   updates) ... thus clients (user accounts only) should also additionally invoke
   `updates.getChannelDifference` periodically for channels and supergroups the
   user is currently viewing (i.e. explicitly opened ...)".* Non-final diff →
   re-invoke immediately; final → re-invoke after the returned `timeout` seconds
   (else 1 s). *"This mechanism may also be used to enable passive reception of
   updates from channels or supergroups we're not a member of ... the API will
   start passively sending updates ... to all logged-in sessions, as long as any
   of the sessions continues to periodically invoke `updates.getChannelDifference`
   every `timeout` seconds."* *"Clients should stop ... polling once the user
   closes the channel/supergroup: the API may continue emitting passive updates
   only if the user is a member."* *"Clients should also limit to 10 the maximum
   number of channels/supergroups short-polled."*

2. **Pinned engine source** — TDlib commit `22d49d5` (the exact commit go-tdlib
   v0.7.6 pins), `td/telegram/MessagesManager.cpp`:
   - `open_dialog` (:19524): `CHECK(!is_bot)`, `open_count++`, `was_opened = true`
     (and only in the empty-chat edge case calls `read_history_inbox` — a normal
     open does **not** mark the channel read).
   - `add_pending_channel_update` (:7206): on a detected pts gap,
     `if (d->was_opened || is_member || sponsored) { get_channel_difference(...) }
     else { /* drop */ }` — **gap recovery is gated on open/member state**; a
     never-opened, non-member channel is dropped, not recovered.
   - `on_update_channel_too_long` (:6274): triggers `get_channel_difference`.
   - re-poll is **open-gated**: `on_get_channel_difference` reads the returned
     `timeout_` and `if (timeout > 0 && d->open_count > 0) add_timeout_in(...)`
     — **the periodic poll self-continues only while the dialog stays open.**
   - `close_dialog` (:19640): `open_count--`; at 0 the poll loop stops and
     viewed/interaction state is cleared.
   - `MAX_CONCURRENT_GET_CHANNEL_DIFFERENCES = 10` (:36860) — the engine itself
     caps simultaneous channel-difference polls at 10; `retry_get_channel_difference_later`
     (:37233) backs off 1 s → ×2 → cap 60–80 s.

3. **go-tdlib v0.7.6 architecture** — module cache: it is a **CGO shell over the
   C++ TDlib** (`client/tdlib.go`: `#include <td/telegram/td_json_client.h>`,
   `td_execute`/`td_receive`, links `-ltdjson`). So **the full TDlib engine —
   including the channel-difference poller above — already runs inside the Budva
   process.** The Go side only needs to send the one application-level RPC
   `OpenChat` (wrapper present at `client/function.go:7790`); it does **not**
   reimplement MTProto. **Budva's session currently never calls `OpenChat`**
   (source-verified: no `OpenChat`/`CloseChat`/`ViewMessages`/`getChannelDifference`
   anywhere in the repo; the `ViewedMessages` symbols are Budva's own
   forwarding-stats counter, not the TDlib API).

Read together, the observation is exactly reproduced: **list-only → no session
is polling the channel → Budva's session is not an active acquirer → silence;
open → the client's session polls → (non-member: cross-session; member: that
session's own polling) Budva's session receives the deferred updates → burst.**

## The fix (converged): `OpenChat` each channel/supergroup source, keep it open

For **each channel/supergroup source chat in the ruleset**, Budva calls
`OpenChat` **on its own session** and **keeps it open** (never sends `CloseChat`
unless the chat is removed from the ruleset). That one call makes Budva's session
an **active acquirer**: the in-process engine then owns gap recovery
(`was_opened=true` at :7206) and the periodic re-poll (`open_count>0` re-arm),
so the channel's updates reach Budva **independently of whether the official
client has it open**.

- **Open once, keep — no periodic re-open.** The poll self-continues while
  `open_count>0`; `OpenChat` establishes state, it is not a token to refresh.
  Re-open happens only at **state-reset points**: after **re-auth** (TDlib resets
  dialogs) and after **process restart** (`open_count` is in-process).
  `OpenChat`/`CloseChat` fire only on state-change edges (startup, re-auth,
  ruleset add/remove), never as a heartbeat.
- **Keep ALL sources open — one `OpenChat` per source, not one held open at a
  time.** Acquisition is per-channel state: closing/rotating source A drops A's
  poll and re-introduces the silence for A; rotation means N−1 sources are
  *permanently* un-acquired. The fix requires N opens for N channel/supergroup
  sources. (See the engine's own 10-poll cap below for the >10 case.)
- **Private/basic-group sources are NOT `OpenChat`'d** — they are not
  open-state-gated and ride the global stream; "support" = the session's global
  diff is healthy. This also matches a client (a client does not open every
  chat). **So "support every source type" = one intent (own the acquisition
  boundary) realized by each class's own client-conformant mechanism** —
  channels/supergroups via `OpenChat`, private/basic-group via the global stream.
  The type selects *which mechanism*; each mechanism is a known fact, so the
  current production type is **diagnostic, not a design input**.
- **Native, no new component, no hand-built poller.** This is a first-class
  go-tdlib/TDlib API; the engine already does the polling.

**Scale fact (engine-enforced, not something Budva must write):** the engine caps
**simultaneous** channel-difference polls at 10 (`MAX_CONCURRENT_GET_CHANNEL_DIFFERENCES`),
re-polls at the **server-returned** `timeout` (server-paced, not amplifiable),
and does not emit open/close churn. With >10 channel/supergroup sources the
engine queues them (11th waits), so **not all sources are strictly real-time at
very large N** — an inherent scale trade-off to note, not a reason to "keep only
one open".

## Conformance assessment (owner's first-principles benchmark)

The design benchmark is **"what would an official client do?"** — re-evaluated
across three layers (correcting an earlier over-claim that "open-once-keep is the
most client-like N-open behavior"):

- **Mechanism layer — 0 risk.** Every RPC Budva sends (`openChat` /
  `getChannelDifference` / `closeChat`) is one an official client sends
  continuously; there is no non-client RPC.
- **Single-chat shape — conformant.** "open once, then zero calls, engine
  self-re-polls, never close" **is** a client's real terminal state (a tab/dialog
  left open). No client heartbeats or re-opens.
- **Combined / scale shape — NOT typical of a phone user; it is desktop
  multi-window client behavior.** "N sources all open, held statically" is what
  a **desktop client with N channel windows left open** does, **not** what a
  phone user does (a phone client holds ~1 open and closes on switch). Correcting
  the record: keep-open is *a behavior a client performs* (an unclosed tab), not
  *the typical* client behavior. The footprint (N channels open, held) is the
  desktop-multi-window shape. Whether/how that reads as "conspicuous" is a
  **non-client-baseline observation** — it evaluates against a hypothetical
  detector, not against "does a client do this," so it is **outside the
  conformance benchmark and not a design variable**. The dedicated-vs-shared
  account axis does not enter the conformance computation (revised, owner
  correction 2026-11-02); the only load-bearing account fact is user-vs-bot.

**Inherent (document, don't mitigate):** view/active counts — an open dialog
contributes by definition; the footprint **is** the fix.

**Dissolved:** compliance (dedicated-account / datacenter-IP / flood — a
user-account daemon is the product's pre-existing architecture; the marginal RPC
profile is "a user who keeps their N channels open"); **read receipts** — an
official client does not mark a channel read on `openChat` (that is a separate
`readHistory`-class call the UI issues); conformance = **Budva never calls any
read API** (the default — it does not forward reads).

**Standing discipline + four guardrails (the client-plausible part):**
(1) open **only ruleset source chats**, never the account's other chats;
(2) **stagger** the initial/re-auth `OpenChat` batch (reuse
`internal/service/limiter`) — do not fire N `openChat` in the same instant;
(3) **open-once-keep, no periodic toggle** (toggle churn is the non-human
signature, and re-introduces the deferred state = the bug);
(4) **do not fight the engine's 10 cap** (no logic to bypass its queueing).

## Problem 2 (secondary, downgraded to "potential, unobserved"): steady-state endpoint HOL

The serial per-endpoint worker + cap-256 buffer + single-attempt sink is a real
**code property**, but there is **no owner-observed per-message queueing delay** —
so per the "does the problem even hold" criterion it is not yet a *problem*, only
a **potential contributor** to verify **if** per-message delay is ever observed.
It is **not** a conformance question (a client does not queue webhook HTTP POSTs)
and is a **separate leg** from Problem 1. The 4 product decisions (loss /
duplicate / ordering / receiver `T`+fast-ack) **belong to this leg**, and this leg
has no observed symptom — so **none of them gates the Problem-1 fix**.

Its analysis (kept for when/if it is needed):

### The delivery pipeline, with the latency-relevant segments

```
TDlib update
  -> listenUpdates -> r.updates (cap-100 chan)          [ms]
  -> handler.Run loop (single consumer):
       OnNewMessage -> rule loop -> processMessage
         -> SubmitV2(...)      [IN the Run loop; NOT in the task queue]
  -> per-endpoint queue (chan, cap 256)                 [the backlog lives here]
  -> runWorker (ONE goroutine per endpoint, serial)
         -> WebhookSink.Push = single http.Client.Do (no app-level retry)
  -> receiver (black box)
```

Source anchors (source-confirmed facts):
- `internal/infra/telegram/repo.go:431` — `r.updates <- typ` (cap-100, ms).
- `internal/app/handler/service.go:352` — `SubmitV2` called **in the Run loop**,
  before any forwarding/statistics task is `Add`-ed to the shared task queue
  (`service.go:365`). Ordinary NEW text does **not** traverse the shared task
  queue on its push path.
- `internal/service/push/dispatcher.go:100` — one `runWorker` **per endpoint**.
- `internal/service/push/dispatcher.go:214-237` — `runWorker` is a **serial**
  `for rend := range worker.queue`; blocks until `sink.Push` returns.
- `internal/service/push/dispatcher.go:16,86` — per-endpoint buffer, default
  capacity **256**.
- `internal/service/push/dispatcher.go:157-165` — `Submit` is **non-blocking**;
  a full buffer → `SubmitRejectedBufferFull` + WARN `buffer_full`.
- `internal/service/push/webhook.go:11` — `defaultWebhookTimeout = 5s`;
  overridable per endpoint by `timeoutSeconds` (`internal/domain/ruleset.go:35`).
- `internal/service/push/delivery.go:167-172` — `Push` calls `client.Do` then
  **drains the response body**; the worker is occupied until the response returns.
- `internal/service/push/delivery.go:133-139` — `singleAttemptTransport`: exactly
  one attempt; no application retry loop.

### How a message's delay decomposes (if Problem 2 ever manifests)

| span | what it is | source |
|---|---|---|
| `T2−T1` | update receipt → `SubmitV2` | `service.go:352` |
| `T2−T3` | `SubmitV2` accepted → worker starts `http.Client.Do` (**endpoint queue wait**) | `dispatcher.go:214-219` |
| `T3−T4` | `http.Client.Do` start → receiver arrival (HTTP/TLS/ingress) | `delivery.go:167` |

### The queueing model — G/G/1

One endpoint, one serial worker. `λ` = arrival rate, `B` = actual per-request
service time (the full `client.Do` round-trip incl. body drain), `E[B]` its
expectation, `timeout` an **upper bound** on `B` (so `1/timeout` is a
worst-case **lower** service-rate bound, not the actual rate).

- **Stability** `ρ = λ·E[B] < 1`: conditionally stable; a burst drains when
  `ρ<1`, grows when `ρ>1`. Bound uses `E[B]`, not the timeout.
- **Worst-case wait** `queue_wait(event) ≤ requests_ahead × timeout` — a
  **worst-case upper bound only**; the wait grows **linearly** (arithmetic) in
  the service times ahead, **not** exponentially.

A minute-scale delay would be *consistent with* a small backlog (3×60 s or
tens×5 s) **or** many short requests — a plausible example, **not** a production
conclusion. Production backlog size is not asserted (needs post-deploy
queue-depth data).

### Confirmed exclusions (source does not support, for Problem 2)

- **The 1 s task-queue ticker is not the cause of ordinary NEW-text push delay.**
  Push runs on the per-endpoint **channel** worker (no tick); the 1 s ticker
  (`internal/infra/queue/repo.go:51-62`) drives only the shared *task* queue
  (forwards/stats/album), which ordinary NEW text does not enter on its push path.
- **The 10-29 self-deadlock recurring.** That was *infinite* (zero inbound, idle
  worker, restart cures); this is *finite and self-recovering*.
- **A hidden Budva retry/backoff explaining the gap.** The sink is
  single-attempt (`delivery.go:133-139`).
- **Endpoint HOL alone explaining the 10-minute silence.** HOL is "slow but
  continuous" (a request at least every `timeoutSeconds`=60 s); a 10-minute
  *zero-request* window is an **absence of `SubmitV2`**, i.e. Problem 1, not HOL.

**Evidence gate for Problem 2 (if pursued) = a controlled slow-receiver test**
(in-repo, fixed `T`, reproduces queue-wait growth linearly, race-clean,
CI-runnable) — **not** production timing collection. The receiver's per-request
`T` (the effective `B`) is a black-box number only the owner can supply (one
`curl -w '%{time_total}'` or the access log); the payload already carries
`OccurredAt` (`service.go:351`) so the receiver can measure arrival-minus-submit
for free.

## Corrections to my earlier record (recorded honestly)

1. **`ChannelPoller.cpp` does not exist.** That filename was invented in an
   earlier draft (a GitHub code search for it returns 0 results). The
   channel-difference polling lives in `td/telegram/MessagesManager` (pinned
   commit `22d49d5`).
2. **"go-tdlib has no `GetChannelDifference` wrapper → Budva must hand-build a
   raw `Send` poller" was the wrong direction.** Because go-tdlib is a CGO shell
   over the C++ TDlib engine, the MTProto layer (including channel-difference
   polling) runs **inside the engine already**; Budva only sends the
   application-level `OpenChat`. No hand-built poller.
3. **source-confirm (a) is resolved:** the pinned TDlib **does** auto-poll the
   difference of an opened channel (the `open_count>0` re-arm at
   `on_get_channel_difference`, unconditional on the success path). So the fix
   is **literally "one `OpenChat` call + keep"**, not "OpenChat + our own poll
   loop" and no re-poll nudge (all three source-confirms resolved, below).

## Source-confirms — all three resolved (agent's job; closed 2026-11-01 by
reading the pinned source, no experiment)

1. **Idle-open delivery vs gap-only — RESOLVED: the engine keeps an open
   channel polling unconditionally (no gap required).** The re-poll sits on the
   **success** path of `on_get_channel_difference`: if the diff is `final`,
   `if (timeout > 0 && d->open_count > 0) channel_get_difference_timeout_
   .add_timeout_in(dialog_id, timeout)` (`:37423`); when that timer fires,
   `on_channel_get_difference_timeout` (`:36707`) calls
   `get_channel_difference(dialog_id, d->pts, ...)` — **there is no
   gap / too-long check anywhere on this path**. So an **idle-but-open** channel
   keeps polling at the **server-returned** `timeout` and stays near-real-time.
   **The fix form is therefore literally "one `OpenChat` + keep open"** — no
   re-poll nudge is needed. (This is the load-bearing confirm; it fixes the fix
   shape.)
2. **Engine's >10 behavior — RESOLVED: unbounded FIFO pending queue, no loss,
   no starvation.** Every channel-diff request funnels through
   `do_get_channel_difference`, which pushes to `pending_get_channel_differences_`
   (`:36854`); `process_pending_get_channel_differences` (`:36860-36863`) drains
   the queue **only while** `get_channel_difference_count_ < 10`; each
   completion re-drains, and a per-chat dedup keeps one outstanding request per
   chat. Steady state with >10 open sources: **10 in flight, the rest rotate**
   at their own server `timeout`; a channel with a large backlog re-queues its
   next page immediately (per-page FIFO fairness, not per-channel), so it can
   hold a slot for several consecutive turns. The 11th+ source is **deferred,
   never dropped**; the extra per-channel latency ≈ queue position × one diff
   RTT (small at a handful-to-a-dozen sources).
3. **Member-channel idle delivery — RESOLVED (decision): keep the open set =
   ALL channel/supergroup sources; do NOT shrink to non-member only.** The spec
   says passive updates for member channels are sent "usually" but "in some
   cases the API may stop (or send fewer)" — that is **not a guarantee**, so
   relying on idle member delivery would re-introduce the silence. The engine's
   own gap-recovery gate (`:7206`) treats `was_opened` and `is_member` as two
   *separate* sufficiency conditions, corroborating that member-ness alone does
   not keep the stream active. The volume saved by excluding member channels is
   small (polling is already server-paced and 10-capped) versus the risk of
   re-opening the exact bug. **Set = all sources; a member-channel reduction is
   an optimization to revisit only if the footprint becomes a real concern,
   with production evidence — not preset now.**

**Remaining before implementation (not source-confirms):** (4) **lifecycle +
stagger design** (re-auth re-open, hot-reload add/remove, limiter-staggered
initial batch) against the seams at `cmd/engine/main.go:168` / `main.go:183-196`
— an implementation-design step. (5) **dedicated vs shared account — not a
design variable (revised, owner correction 2026-11-02).** Conformance is a
client-fact and account-agnostic: it is judged by what a client does, never by
the account's attributes. The footprint's conspicuousness to a hypothetical
detector is a non-client-baseline observation, outside the "behave like a
client" benchmark — **no owner input required, and no remaining gate.**

## Owner inputs are NOT blocking the Problem-1 fix (reframe)

- **Source chat type** = **diagnostic** only (which documented mechanism
  explains the observed coupling; a silence on a private/basic-group = a separate
  bug). It does **not** select the fix — the fix covers every class by its own
  mechanism. Not required from the owner.
- **The 4 product decisions** (loss / duplicate / ordering / receiver `T`+fast-ack)
  belong to **Problem 2**, which is downgraded (no observed symptom). They do
  **not** gate the Problem-1 fix.
- **Net: no owner input is required to proceed to the Problem-1 fix.** The
  source-confirms are all **resolved** (above); what remains is the
  lifecycle/stagger design (an implementation step) and, for Problem 2, owner
  decisions made only if/when that leg is pursued.

## Relation to the 10-29 record

Supersedes nothing. This is a *finite-latency / acquisition* follow-up to the
completed `10-29-budva-push-stall` (infinite self-deadlock, fixed at `415d90c`).
Both can be true at once: the deadlock is fixed and runtime-accepted, **and** the
latest build can still show the Problem-1 acquisition gap (and, separately, a
potential Problem-2 HOL). Team B's independent audit of the *mechanism*:
`research/team-b-webhook-latency-follow-up.md`. Team B's independent audit of
*this task's* research/design: `research/team-b-team-a-report-audit.md`
(**CHANGES REQUESTED**; corrections applied). Team B's **ground-fact check of the
open-chat observation**: `research/team-b-tdlib-open-chat-ground-fact-check.md`
(**CHANGES REQUESTED**, 2026-10-30) — the origin of the open-chat framing; cited
as **ground-fact evidence**, kept **verbatim** (its "large supergroups" phrasing
is a maintainer paraphrase, not a protocol class — see the terminology note),
**not** design approval.

**Terminology precision (verified against the pinned go-tdlib v0.7.6 module):**
"**large supergroup**" is a Team B / maintainer **paraphrase, not a protocol
class** — the TDlib API has no size threshold and no size qualifier in the
normative contract. The normative distinction is by **protocol class**: basic
group (`GroupChat`) vs supergroup (`SuperGroup`) vs channel. The open-state-gating
clause — the pinned module's `OpenChat` contract verbatim
(`client/function.go:7789`) — reads: *"Many useful activities depend on the chat
being opened or closed (e.g., in **supergroups and channels** all updates are
received only for opened chats)"* — **no size qualifier**, i.e. ALL supergroups
(any member count) and ALL channels; basic groups are outside the gating. The
"large" modifier belongs only to the *not-guaranteed-real-time* behavioral
expectation, never to the gating scope. **Practical effect: none** — the correct
labels are "supergroup (any size) / channel" (gated) and "basic group"
(non-gated).
