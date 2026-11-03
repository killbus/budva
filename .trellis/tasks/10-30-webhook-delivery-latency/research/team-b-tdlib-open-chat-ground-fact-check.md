# Team B Ground-Fact Check: TDLib Open-Chat and Delayed Webhook Delivery

## 1. Audit identity and scope

- **Audit party:** Team B, acting as an independent reviewer
- **Audit date:** 2026-10-30
- **Audited task:** `10-30-webhook-delivery-latency`
- **Triggering observation:** across repeated user-run tests, the official
  Telegram client showed new activity in the chat list while Budva produced no
  webhook request; opening the target chat was then repeatedly followed by a
  burst of inbound webhook requests
- **Budva dependency:** `github.com/zelenin/go-tdlib v0.7.6`
- **Pinned TDLib source:** commit
  `22d49d5b87a4d5fc60a194dab02dd1d71529687f`
- **Audit mode:** repository, dependency, and pinned-supply-chain source review
- **Excluded actions:** no production-code change, runtime experiment,
  dependency installation, deployment change, Git commit, or branch operation

This report supplements, but does not overwrite:

- `research/team-b-webhook-latency-follow-up.md`
- `research/team-b-team-a-report-audit.md`
- Team A's research and design artifacts

Its purpose is narrower: determine what the current source can and cannot prove
about the apparent relationship between opening a chat in the official client
and Budva releasing delayed webhook events.

## 2. Executive verdict

The new observation is material and invalidates any model that assumes:

> Once the official Telegram client shows a message, Budva's independent
> TDLib session must already have received the same `updateNewMessage`.

That assumption is not supported. The official client and Budva have distinct
client sessions and local databases. Activity visible in the official client's
chat list establishes the state of that client, not the update-publication
stage reached by Budva.

The pinned TDLib API and implementation also require an important correction:
`openChat` is not merely a cosmetic UI-lifecycle notification. Its documented
contract states that useful activities depend on open/closed state and that,
for supergroups and channels, all updates are received only for opened chats.
In the pinned C++ implementation, opening a channel explicitly invokes
`get_channel_difference`. Therefore chat-open state can affect update
acquisition within the TDLib instance that receives the `openChat` call.

However, Budva does not call `OpenChat`, `CloseChat`, or `ViewMessages`.
Opening a target chat in the official client calls those operations in the
official client's own session, not in Budva's TDLib instance. Budva source does
not contain a direct client-to-client path. External Telegram documentation
does, however, establish a server-mediated cross-session mechanism for
channels/supergroups that the account has not joined but can still access:
public ones, and private ones temporarily available through `chatInvitePeek`.
For that specific scope, short polling with `updates.getChannelDifference`
from any session can enable passive updates to all logged-in sessions. The
reviewed documentation does not establish the same mechanism for joined
channels/supergroups, private chats, or basic groups.

The repeated nature of the user's tests materially changes the evidentiary
weight. Team B no longer considers accidental temporal coincidence a credible
leading explanation. The repeated intervention-response pattern supports a
real coupling between opening/viewing the chat in the official client and the
subsequent release of updates observed through Budva.

Accordingly:

> Repeated field tests support a real open-chat-triggered effect and make
> accidental timing coincidence an implausible explanation. What remains
> unresolved is not whether the effect exists, but which Telegram/TDLib
> mechanism connects the official-client action to Budva's later update
> publication.

**Team B disposition: CHANGES REQUESTED.**

- Team A's current model remains useful but is incomplete.
- The source-confirmed serial webhook worker remains one possible downstream
  latency mechanism, not a complete explanation of this new observation.
- The upstream TDLib receipt interval must be split from Budva processing and
  endpoint queueing before implementation parameters are changed.
- A controlled experiment with timestamps at each pipeline boundary is now an
  implementation prerequisite.

## 3. Findings

### TB-OPEN-P1-01: official-client visibility does not prove Budva update receipt

**Severity:** P1  
**Status:** Open

The official Telegram client and Budva are separate consumers. Each maintains
its own session, transport state, local database, update state, and TDLib/client
lifecycle.

Therefore these observations are not equivalent:

```text
official client chat list shows new activity
Budva TDLib publishes updateNewMessage
Budva handler processes the message
Budva endpoint worker starts an HTTP request
webhook receiver observes request ingress
```

The first event establishes only that the official client knows enough to
update its own chat-list presentation. It does not establish when Budva reached
any later boundary.

TDLib's own API contract strengthens this evidence boundary:
`updateChatLastMessage` may report an unknown last message, and while the last
message is unknown, messages can be added to a chat without a corresponding
`updateNewMessage`. A chat-list indication is therefore not a universal proxy
for complete message-update publication, even within a single TDLib model.

**Required correction:** Team A must not use official-client display time as
Budva's Telegram-update receipt time. The two must be recorded separately.

### TB-OPEN-P1-02: `openChat` is behaviorally significant inside a TDLib instance

**Severity:** P1  
**Status:** Confirmed source fact

The pinned `td_api.tl` declaration says:

> Many useful activities depend on the chat being opened or closed (e.g., in
> supergroups and channels all updates are received only for opened chats).

The fixed C++ source confirms concrete behavior for channel dialogs:

- the dialog is marked opened and retained from unloading;
- channel participant or story information may be refreshed;
- `get_channel_difference(dialog_id, d->pts, ..., "open_dialog")` is called;
- linked-channel and dialog information may be loaded.

This means the older proposition that `openChat` is only a UI hint, unrelated
to update retrieval, is incorrect.

The scope must remain precise. The API wording must not be expanded into the
unsupported claim that every ordinary `updateNewMessage` for every chat type
is emitted only while that chat is open.

Relevant pinned source:

- `td/generate/scheme/td_api.tl:9671`
- `td/telegram/MessagesManager.cpp:19535`
- `td/telegram/MessagesManager.cpp:19593`

### TB-OPEN-P1-03: Budva does not propagate official-client chat-open state

**Severity:** P1  
**Status:** Confirmed source fact

The current Budva Go source contains no call to:

- `OpenChat`
- `CloseChat`
- `ViewMessages`

The generated go-tdlib dependency exposes all three operations, but Budva does
not invoke them.

Consequently, when the user opens a chat in the official client:

1. the official client can open or view that dialog in its own session;
2. the official client's TDLib instance may fetch a channel difference;
3. no reviewed Budva path mirrors that call into Budva's TDLib instance.

The source alone does not prove the proposed direct causal chain:

```text
official client opens chat
  -> Budva TDLib opens the same chat
  -> Budva fetches missing messages
  -> webhook burst
```

The user's repeated intervention tests provide behavioral evidence for a
cross-session or shared-account-state effect through Telegram. External
protocol documentation now grounds one exact mechanism for accessible
non-member public channels/supergroups and private channels/supergroups
temporarily available through `chatInvitePeek`: active channel-difference
polling by one session enables passive updates to all logged-in sessions. The
reviewed documentation does not establish that mechanism for joined
channels/supergroups, private chats, or basic groups. For those cases, the
exact cross-session mechanism remains unresolved. Plausible classes include
server-side read/history state changes, channel-difference eligibility,
push-triggered synchronization, or another account-scoped update-recovery edge
not represented by a direct client-to-client call.

### TB-OPEN-P1-04: TDLib difference recovery can legitimately publish messages in a burst

**Severity:** P1  
**Status:** Confirmed source mechanism; production causality unproven

The pinned TDLib implementation maintains ordered update state and performs
difference recovery. In `process_get_difference_updates`, messages returned in
`new_messages` are passed to `MessagesManager::on_get_message` with
`from_update=true`.

`on_get_message`:

- creates or loads the dialog when needed;
- adds the message to the dialog;
- calls `send_update_new_message` when the message requires publication.

Thus messages recovered through `updates.getDifference` can become TDLib
`updateNewMessage` events after recovery. A group of recovered messages can be
published close together even though the messages were originally sent over a
longer interval.

For channels, opening the channel inside the same TDLib instance explicitly
starts a channel-difference request. This is a real same-instance mechanism.
It does not by itself establish that opening the official client's instance
starts a difference request in Budva's instance.

Relevant pinned source:

- `td/telegram/UpdatesManager.cpp:1800`
- `td/telegram/UpdatesManager.cpp:1865`
- `td/telegram/UpdatesManager.cpp:1937`
- `td/telegram/MessagesManager.cpp:7256`
- `td/telegram/MessagesManager.cpp:13935`
- `td/telegram/MessagesManager.cpp:13955`
- `td/telegram/MessagesManager.cpp:19593`

### TB-OPEN-P1-05: Budva's local update pipeline can accumulate and release work

**Severity:** P1  
**Status:** Confirmed source mechanism; production causality unproven

The local update path contains several finite buffers and blocking handoffs:

```text
TDLib receive
  -> go-tdlib client.responses            capacity 1000
  -> go-tdlib listener.Updates            capacity 1000
  -> Budva listenUpdates
  -> Budva r.updates                       capacity 100
  -> handler.Run                           one synchronous consumer
  -> SubmitV2
  -> per-endpoint delivery worker
  -> webhook receiver
```

go-tdlib's global receiver performs a blocking send into the target client's
`responses` channel. The client receiver then performs a blocking send to each
active listener in sequence. Budva's `listenUpdates` performs a blocking send
to `r.updates`, and `handler.Run` uses one synchronous consumer for business
updates.

Therefore sustained downstream non-consumption can propagate back through the
Go-side update channels. When consumption resumes, buffered updates can appear
as a burst.

Relevant source:

- go-tdlib `client/tdlib.go:59`
- go-tdlib `client/tdlib.go:72`
- go-tdlib `client/client.go:49`
- go-tdlib `client/client.go:72`
- go-tdlib `client/client.go:87`
- go-tdlib `client/client.go:130`
- Budva `internal/infra/telegram/repo.go:395`
- Budva `internal/infra/telegram/repo.go:431`
- Budva `internal/app/handler/service.go:157`
- Budva `internal/app/handler/service.go:173`

This mechanism can explain local batching, but receiver-only logs cannot prove
that it occurred. A goroutine dump taken during the stall, or timestamps before
and after each blocking boundary, are required.

### TB-OPEN-P1-06: downstream webhook queueing alone does not explain the open-chat correlation

**Severity:** P1  
**Status:** Model correction required

Team B previously confirmed that the per-endpoint worker is serial and can
delay a request for minutes before `http.Client.Do` begins. That remains a
valid mechanism.

It is insufficient as a complete explanation of the new observation:

- if the endpoint worker was simply draining a backlog, opening the official
  chat should have no necessary effect;
- repeated release after the same user intervention is not adequately
  explained by an independent endpoint backlog, local consumer resumption, or
  accidental timing;
- only upstream and downstream timestamps can distinguish these cases.

The serial-worker hypothesis must therefore remain in the candidate set, but
must not crowd out TDLib receipt, difference recovery, or Go-side backpressure.

## 4. Candidate causal model

### B1a: Budva transport or session recovery

Budva's TDLib session temporarily stops receiving live updates. Reconnect or
state recovery later publishes multiple messages.

**Source support:** TDLib has update-state and difference-recovery machinery.  
**Production proof:** absent.

### B1b: PTS or channel-difference recovery

A PTS gap or channel state gap delays complete application. Difference results
are later processed and messages are published together.

**Source support:** confirmed.  
**Production proof:** absent.

### B1c: dialog materialization or unknown-last-message state

The chat/dialog state is incomplete in Budva's local TDLib database. Chat-list,
last-message, and message-publication updates do not necessarily align.

**Source support:** confirmed as a semantic possibility.  
**Production proof:** absent.

### B1d: open-chat-triggered cross-session or account-state coupling

Opening or viewing the chat in the official client changes Telegram server
state, which indirectly causes Budva's session to receive missing data.

**Behavioral support:** strong, based on repeated user-run intervention tests.  
**Source-level mechanism:** established by official Telegram documentation for
accessible non-member public channels/supergroups and private
channels/supergroups temporarily available through `chatInvitePeek`; not
established for joined channels/supergroups, private chats, basic groups, or
other chat classes.  
**Production status:** leading causal class. Applicability to the observed
production case depends on identifying the target chat type.

### B2: local Go-side backpressure

The update has reached TDLib or go-tdlib but is waiting in, or behind, one of
the bounded blocking channels. Consumption resumes and the buffered updates
are released.

**Source support:** confirmed mechanism.  
**Production proof:** absent.

### B3: endpoint delivery backlog

Budva has handled and submitted the event, but it waits behind previous
requests in the serial endpoint worker.

**Source support:** confirmed mechanism.  
**Production proof:** absent for this occurrence.

### D: ruleset publication or routing gap

The update is received, but the active ruleset, snapshot, or routing state does
not publish it at the expected time.

**Source support:** remains a candidate from Team A's model.  
**Production proof:** absent.

## 5. Correction to Team A's model

Team A currently emphasizes:

- B1: update-stream gap;
- B2: handler run-loop stall;
- D: ruleset hot-reload gap;
- downstream serial webhook delivery.

`B1` is too coarse. It should be decomposed into:

1. transport/session interruption;
2. global PTS difference recovery;
3. channel-difference recovery;
4. pending update ordering and release;
5. dialog materialization or unknown-last-message state;
6. update classes whose receipt depends on same-instance chat-open state;
7. official-client state versus Budva-session state;
8. a documented cross-session passive-update effect for accessible non-member
   public channels/supergroups and private channels/supergroups temporarily
   available through `chatInvitePeek`, with joined channels/supergroups,
   private chats, basic groups, and other chat classes still unresolved.

Team A should also revise any implication that official-client visibility proves
Telegram has already delivered the same message update to Budva. It proves only
that the official client has enough local state to render its own list.

## 6. Required controlled experiment

The user has already repeated the open-chat intervention enough times that
Team B does not require another repetition merely to decide whether the effect
is coincidence. The next experiment must localize the mechanism, not re-prove
the existence of the effect.

Use ordinary text messages containing unique tokens and instrument these
contrasting states:

```text
A: official client remains on the chat list; target chat is not opened
B: official client opens the target chat after list activity appears
C: official client is fully closed
```

Separate private/basic-group chats from supergroups/channels. The pinned TDLib
source gives channel open state a concrete difference-recovery behavior, so
mixing chat types would make the result ambiguous.

Record these timestamps with a stable chat ID, message ID, and test token:

```text
S0 = message send/server timestamp
C1 = official-client chat list first shows activity
C2 = official-client target chat is opened
B0 = go-tdlib listener receives any relevant update
B1 = Budva receives UpdateNewMessage
B2 = Budva attempts the send to r.updates
B3 = handler starts processing the event
B4 = SubmitV2 accepts or rejects the event
B5 = endpoint worker picks up the envelope
B6 = http.Client.Do begins
W1 = webhook receiver observes request ingress
W2 = webhook receiver sends its response
```

Interpretation:

- If `B0/B1` precede `C2` but `B3-B6` occur later, the delay is local to
  Budva/go-tdlib or endpoint delivery.
- If `B0/B1` consistently appear only after `C2`, the coupling lies upstream
  of Budva's business handler and the investigation must focus on
  Telegram/TDLib update acquisition or recovery.
- If A, B, and C all show similar recoveries, transport/difference recovery or
  local backpressure is more likely than cross-session coupling.
- If `B4` is prompt but `B5/B6` are late, the serial endpoint worker is the
  direct latency interval.
- If `B6` is prompt but `W1` is late, investigate the actual HTTP/proxy/socket
  boundary.
- If `W1` is prompt but the receiver's application log is late, the receiver's
  logging or processing stage is responsible.

Receiver ingress logs alone cannot identify these intervals.

## 7. Minimum diagnostic evidence

Before changing delivery concurrency or timeout policy, capture:

- TDLib authorization and connection-state transitions;
- TDLib `getDifference` and channel-difference start/end evidence;
- listener receipt time and update type;
- `r.updates` depth and blocking duration;
- handler start/end duration;
- endpoint queue depth, oldest age, and worker pickup time;
- HTTP request-start time;
- receiver socket/route ingress time;
- one goroutine dump while the symptom is active.

Instrumentation must avoid logging message bodies, credentials, session
material, webhook secrets, or other sensitive content.

## 8. External ground-fact research

Team B extended the audit to Telegram's official MTProto documentation,
TDLib's official API documentation and issue tracker, and Telegram Desktop's
official source. This external evidence materially narrows the unresolved
mechanism.

### E1: delayed channel delivery is an intentional Telegram behavior

TDLib's maintainer has repeatedly stated that messages in large
channels/supergroups are not necessarily delivered in real time:

- TDLib issue `#2695` records the same externally visible symptom: incoming
  messages did not appear until the chat was opened in an official client.
  The maintainer answered that messages in large chats are not expected in
  real time and are expected faster while the chat is opened.
- TDLib issue `#2853` states that channel messages are not intended to be
  delivered in real time, can differ from official applications, and may be
  delivered when the service considers it appropriate.
- TDLib issue `#366` explains channel gaps: after enough missed updates,
  Telegram may omit individual channel updates and transmit current channel
  state instead. Channel updates are explicitly not guaranteed to arrive
  immediately.

Sources:

- https://github.com/tdlib/td/issues/2695
- https://github.com/tdlib/td/issues/2853
- https://github.com/tdlib/td/issues/366

**Effect on the Budva model:** for channels and large supergroups, a delay of
minutes before Budva receives individual message updates is compatible with
the documented Telegram update model. It is not, by itself, evidence of a
Budva webhook-worker defect.

**Boundary:** these sources do not establish the same delivery policy for
ordinary private chats or basic groups. The affected chat type must therefore
be recorded before this explanation is applied.

### E2: an opened channel activates difference polling

Telegram's official update documentation says each channel/supergroup has its
own `pts` sequence. A gap is recovered with
`updates.getChannelDifference`. For channels currently being viewed, clients
should periodically call this method and continue according to the returned
timeout.

The official `openChat` contract independently states that many useful
activities depend on open/closed state and that, in supergroups and channels,
all updates are received only for opened chats.

Sources:

- https://core.telegram.org/api/updates
- https://core.telegram.org/method/updates.getChannelDifference
- https://core.telegram.org/tdlib/docs/classtd_1_1td__api_1_1open_chat.html

This confirms that opening a channel is an update-acquisition intervention,
not merely a presentation change.

### E3: Telegram documents a cross-session passive-update effect

The most important new external fact appears in Telegram's official
`Working with Updates` documentation. Its relevant paragraph concerns a
channel/supergroup that the account has not joined but can still access:
either a public one, or a private one temporarily accessible through
`chatInvitePeek`. For that specific scope, periodically calling
`updates.getChannelDifference` from any session starts passive updates to all
logged-in sessions while that polling continues.

Source:

- https://core.telegram.org/api/updates

This is a documented server-mediated cross-session coupling and supplies a
concrete mechanism that the repository-only audit could not establish:

```text
official client opens/views channel
  -> official client starts channel-difference short polling
  -> Telegram enables passive channel updates for logged-in sessions
  -> Budva's separate session receives previously deferred channel updates
  -> TDLib publishes recovered messages
  -> Budva emits webhook requests in a burst
```

**Applicability boundary:** the documentation states this effect explicitly
for accessible non-member public channels/supergroups and private
channels/supergroups being peeked. It must not be generalized without evidence
to joined channels/supergroups, private chats, or basic groups. If the user's
ruleset observation concerns an accessible channel/supergroup that the account
has not joined, this is now the leading source-grounded explanation. If the
account has joined the target channel/supergroup, or the target is a private
chat or basic group, the documented mechanism is an analogy rather than proof.

### E4: Telegram Desktop performs the expected open-channel request sequence

Telegram Desktop's official source implements the documented behavior:

- `Updates::addActiveChat` schedules a channel difference after the channel
  becomes active;
- the delay is one second after showing channel history;
- `Updates::getChannelDifference` sends
  `MTPupdates_GetChannelDifference`;
- while the channel remains active, final responses schedule another short
  poll using the server timeout, or one second when no timeout is present;
- opening a history also loads history, and read state is later sent through
  `channels.readHistory` or `messages.readHistory`.

Sources:

- https://github.com/telegramdesktop/tdesktop/blob/dev/Telegram/SourceFiles/api/api_updates.cpp
- https://github.com/telegramdesktop/tdesktop/blob/dev/Telegram/SourceFiles/history/history_widget.cpp
- https://github.com/telegramdesktop/tdesktop/blob/dev/Telegram/SourceFiles/data/data_histories.cpp

This makes the user's intervention sequence technically coherent. Remaining
on the dialog list and opening the target history are materially different
states: the latter activates channel-specific history acquisition and
difference polling.

**Version boundary:** these links describe the current Telegram Desktop
development source reviewed on 2026-10-30, not necessarily the exact build
installed by the user. The behavior nevertheless agrees with the official
protocol documentation and with the pinned TDLib implementation.

### E5: revised causal ranking

For an accessible non-member public channel/supergroup, or a private
channel/supergroup temporarily accessible through `chatInvitePeek`, Team B now
ranks the explanations as follows:

1. **Official-client active-channel polling enabling passive updates across
   sessions:** directly supported by official protocol documentation and
   consistent with Telegram Desktop source.
2. **Channel gap/difference recovery followed by batched TDLib publication:**
   directly supported by official documentation and pinned TDLib source.
3. **Budva/go-tdlib backpressure or endpoint queueing:** real local mechanisms
   that can add latency or amplify the burst, but do not independently explain
   the repeatable open-chat trigger.
4. **Accidental timing:** rejected as a serious explanation given the repeated
   intervention-response tests and the now-documented cross-session behavior.

For a joined channel/supergroup, private chat, or basic group, item 1 is not
established by the reviewed sources. The stage-by-stage timestamps remain
necessary.

## 9. What is proved and what remains unproved

### Proved by source

- `openChat` has behavioral significance in TDLib.
- Opening a channel in a TDLib instance invokes channel-difference recovery.
- Difference recovery can feed messages into normal new-message publication.
- For accessible non-member public channels/supergroups and private
  channels/supergroups temporarily accessed via `chatInvitePeek`, difference
  polling by one session can enable passive updates to all logged-in sessions.
- Telegram Desktop treats an opened channel as active and starts repeated
  channel-difference polling.
- Channel messages are not guaranteed to be delivered in real time.
- Budva does not call `OpenChat`, `CloseChat`, or `ViewMessages`.
- go-tdlib and Budva contain bounded blocking handoffs capable of backpressure.
- Budva's endpoint worker can independently cause pre-request queue delay.

### Not proved

- Whether the observed production chat is an accessible
  channel/supergroup that the account has not joined, which is the scope for
  which Telegram documents cross-session passive-update activation.
- The exact mechanism for joined channels/supergroups, private chats, basic
  groups, and any other chat type outside that documented scope.
- Telegram propagates one session's `openChat` to another session.
- Budva must open a chat to receive all ordinary new messages.
- The production delay occurred inside TDLib rather than in Budva or delivery.
- The production delay was caused by the endpoint worker.
- The official client's list display and Budva's message receipt represented
  the same update at the same time.

## 10. Final Team B position

The user's repeated tests are sufficient for Team B to reject accidental timing
coincidence as a serious explanation. The report therefore accepts the
open-chat intervention as behaviorally causal for the observed release pattern.
For accessible non-member public channels/supergroups and private
channels/supergroups temporarily available through `chatInvitePeek`, the
lower-level mechanism is now grounded in Telegram's documented cross-session
passive-update behavior. For joined channels/supergroups, private chats, basic
groups, and other chat types, the exact cross-session mechanism remains
unresolved.

The most important new ground fact is now four-sided:

1. chat-open state genuinely affects some TDLib behavior, especially channel
   difference acquisition in the same TDLib instance;
2. Telegram officially documents a server-mediated cross-session effect for
   accessible non-member public channels/supergroups and private
   channels/supergroups temporarily available through `chatInvitePeek`:
   difference polling by any session starts passive updates to all logged-in
   sessions;
3. Telegram Desktop source confirms that opening a channel starts this
   difference-polling lifecycle;
4. applicability to the user's exact observation depends on the target chat
   type, which must be captured rather than inferred.

The correct response is to treat the open-chat trigger as the leading fact and
shift the investigation from "is this coincidence?" to "at which boundary does
the trigger release the updates?" Team B recommends adding boundary timestamps
and targeted TDLib diagnostics before any behavior-changing fix.

**Disposition: CHANGES REQUESTED until Team A incorporates the revised TDLib
semantics, cross-session evidence boundary, and stage-by-stage experiment into
the task's facts, model, and implementation gates.**
