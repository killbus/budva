# Business-outlet concurrency boundaries

Phase 1 static verification, based on this task's `research/task_plan.md`, `research/findings.md`, and `.trellis/spec/backend/telegram-tdlib-guidelines.md`. No constructor API is selected here. No implementation, tests, CI, or network access occurred; static interleavings are not production reproductions.

Project anchors are repository-relative. `SDK/` denotes `C:/Users/ying/go/pkg/mod/github.com/zelenin/go-tdlib@v0.7.6/client/`; the dependency pin is `go.mod:17`.

## 1. Publication, startup, and cancellation requirements

Required invariant: **disabled business outlet implies no reachable business-channel send**, not merely a nil channel. Fix the choice before `Start` launches goroutines and never mutate it at runtime (`internal/infra/telegram/repo.go:77`, `internal/infra/telegram/repo.go:94`). A bare nil-channel send blocks forever; selecting between that send and cancellation merely waits for cancellation. A later assignment to nil cannot release a send already waiting on the old channel.

Guard only business publication, after `dispatchSendResult` and `UpdateNewChat` readiness/convergence handling (`internal/infra/telegram/repo.go:383`, `internal/infra/telegram/repo.go:391`, `internal/infra/telegram/repo.go:401`). Do not guard listener acquisition or the whole update loop. `Updates()` returns the channel; it does not register or activate a subscriber (`internal/infra/telegram/repo.go:120`). A nil return is not end-of-stream: receiving or ranging waits forever, so disabled-mode assembly must not launch such a consumer. Sending results uses a capacity-one channel and `LoadAndDelete`, independently of business consumption (`internal/infra/telegram/repo.go:217`, `internal/infra/telegram/repo.go:425`).

Preserve auth channels, warmup initialization, and session-table replacement. `clientDone` closes before the update goroutine is launched, so it is not a listener-registration barrier (`internal/infra/telegram/repo.go:340`, `internal/infra/telegram/warmup.go:404`). `NewClient` receives no caller context, auth publication remains a bare send, and `Repo.Close` only clears the adapter without cancelling/joining workers (`internal/infra/telegram/repo.go:327`, `internal/infra/telegram/repo.go:363`, `internal/infra/telegram/repo.go:102`). These are unchanged limitations, not new guarantees. Never close a nil outlet or hold a lifecycle lock across a blocking send.

## 2. Exactly which incident edge disappears

The existing capacity-100 outlet can strand `listenUpdates` at publication. With sufficient subsequent traffic, its SDK listener fills; the SDK receiver blocks on listener delivery, stops draining capacity-1000 `responses`, and blocks the process-wide receive pump (`internal/infra/telegram/repo.go:82`, `internal/infra/telegram/repo.go:401`; `SDK/client.go:49`, `SDK/client.go:89`, `SDK/client.go:133`; `SDK/tdlib.go:42`, `SDK/tdlib.go:72`).

The candidate deletes the facade publication wait edge while retaining standard reception and internal dispatch. It does not prove that other listeners, authentication, or SDK lifecycle paths cannot block or fail. The SDK's 60-second request timeout releases that caller, not the blocked publication or pump (`SDK/client.go:55`, `SDK/client.go:118`). Timeout also has an existing catcher interleaving: receiver loads a catcher, timeout deletes/closes it, receiver sends (`SDK/client.go:75`, `SDK/client.go:111`). No whole-runtime liveness claim follows.

## 3. SDK closure risks and the implementation gate

Existing listener counterexample: receiver observes `IsActive()`, cancellation makes Repo execute deferred `Close`, then receiver sends to the closed channel (`internal/infra/telegram/repo.go:371`; `SDK/client.go:88`; `SDK/listener.go:53`, `SDK/listener.go:61`). The listener mutex does not cover the sender's check-and-send sequence. Extending a lock across a potentially blocking send would obstruct cancellation, not establish safe ownership.

Existing responses counterexample: the client receiver processes `AuthorizationStateClosed` and closes `responses`; the global pump subsequently sends another response (`SDK/client.go:98`; `SDK/tdlib.go:72`). The Closed documentation excludes later updates but explicitly allows query responses with error 500 (`SDK/type.go:4044`). Repo's close bypass does not eliminate real termination: authentication errors call SDK Close, and Repo still forwards LogOut (`SDK/authorization.go:51`; `internal/infra/telegram/client_adapter.go:325`; `SDK/function.go:523`).

Both hazards predate this candidate. Removing the publication stall can make deferred listener closure reachable where the stalled loop previously could not observe cancellation; unchanged race frequency is therefore not established. Nevertheless, retaining the same listener and lifecycle adds no required close operation. There is no source evidence making an SDK repair a prerequisite for implementing this bounded guard. New listener churn, changed close ownership, or a demonstrated new regression would reopen that judgment. This is not shutdown-safety or release approval.

## 4. Acceptance boundary and a production-loop fixture

Acceptance must cover the actual facade update loop with zero business readers, retained internal dispatch, and real asynchronous requests under pressure. Engine retains capacity 100, filtering, and blocking publication (`internal/infra/telegram/repo.go:82`, `internal/infra/telegram/repo.go:398`, `internal/infra/telegram/repo.go:401`). Pausing its reader can still stall the chain; cancellation cannot interrupt its bare publication. Record this as unsolved, not a passing whole-pipeline claim. No unbounded buffer or default event discard is authorized.

A fake adapter can return `&client.Listener{Updates: ch}` to drive real `listenUpdates`: the interface and wrapper return that concrete pointer (`internal/infra/telegram/client_adapter.go:51`, `internal/infra/telegram/client_adapter.go:334`). `Close` needs no private initialization: the mutex's zero value works, and Close sets `isActive` itself (`SDK/listener.go:47`, `SDK/listener.go:53`). Require a non-nil channel, one closer, and no concurrent producers at closure. Stop/join synthetic producers before cancelling the loop; let its defer own Close. Do not close the input channel to terminate the loop, which would cause deferred double-close. For enabled-mode saturation, resume business draining before awaiting cancellation.

The literal starts inactive and is not registered with the SDK receiver; this fixture verifies Repo wiring, not SDK fan-out/closure safety (`SDK/client.go:130`). No fixture was compiled or run; native verification remains a Linux gate.
