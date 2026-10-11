# Internal event obligations

## Scope and conclusion

Phase 1 static research only: no implementation, tests, CI, or network access. The user-confirmed requirement is that facade preserves existing request services with zero business-update subscribers. Table routes are source facts; listener effects are deductions, not experimental results. SDK lifecycle safety is not re-audited here.

Omitting facade's business queue while retaining Repo's internal dispatcher preserves the inspected internal paths. Retention is a bounded design choice for this change, not proof that the current mechanism is a permanent product requirement.

## Three separate layers

Standard TDLib reception and request-response processing must remain operational. Repo creates the standard client before starting its extra listener (`internal/infra/telegram/repo.go:327`, `internal/infra/telegram/repo.go:344`). Repo's `GetListener` delegates to that client (`internal/infra/telegram/client_adapter.go:334`). Its additional application output, `r.updates`, is currently allocated unconditionally with capacity 100 (`internal/infra/telegram/repo.go:82`). Facade's dependency interface has no `Updates`; handler's does (`internal/app/facade/service.go:17`, `internal/app/handler/service.go:22`).

Disabling business publication is therefore distinct from deleting the extra listener or stopping standard reception. Effects below concern the particular Repo instance; engine's subscription is separate.

## Responsibility table

| Event / responsibility | Actual purpose and consumer: source facts | Through `r.updates`? | Retain dispatcher without business output / delete extra listener |
| --- | --- | --- | --- |
| Asynchronous request responses | Standard client serves request callers; `GetMe` delegates directly (`internal/infra/telegram/client_adapter.go:316`). | No | Neither choice may stop standard reception. The extra Repo listener is not the response-correlation mechanism. |
| Authorization: phone/code/password, Ready, Closed | Repo maps these states (`internal/infra/telegram/repo.go:457`). Authorizer state goes to `AuthStates`, then auth.Service (`internal/infra/telegram/repo.go:325`, `internal/infra/telegram/repo.go:363`, `internal/app/auth/service.go:58`). HTTP reads state; terminal subscribes for prompts (`internal/transport/http/transport.go:69`, `internal/transport/term/transport.go:75`). | No | This separate path remains in either choice. Auth.Service ignores Closed and returns at Ready (`internal/app/auth/service.go:60`); it is not a continuous lifecycle monitor. |
| Session initialization / `ClientDone` | Repo swaps the readiness table, assigns the client and closes the startup latch before launching the extra listener (`internal/infra/telegram/repo.go:340`). Handler waits on the latch (`internal/app/handler/service.go:138`). | No | Preserve this initialization in either choice; it is not business subscription. |
| `UpdateNewChat` | Signals current-table readiness waiters and records convergence, then continues before business filtering (`internal/infra/telegram/repo.go:391`). | No | Retention preserves both. Deletion removes event-driven wakeups and convergence recording; ticker-driven retries remain (`internal/infra/telegram/warmup.go:290`). Deadline equivalence is unproved. |
| `UpdateMessageSendSucceeded` | Private pending-send delivery (`internal/infra/telegram/repo.go:410`); separately, handler records temporary/permanent-ID mappings (`internal/app/handler/service.go:279`). | Business copy only: yes | Retention preserves private delivery without a business queue. Deletion removes that instance's dispatcher delivery; engine still requires its mapping feed. |
| `UpdateMessageSendFailed` | Private pending-send error delivery (`internal/infra/telegram/repo.go:412`). Not business-whitelisted (`internal/infra/telegram/repo.go:441`). | No | Retention preserves error delivery. Deletion leaves registered waiters without this outcome path, subject to their timeout/cancellation (`internal/infra/telegram/repo.go:230`). |
| `UpdateNewMessage`, `UpdateMessageEdited`, permanent `UpdateDeleteMessages` | Handler dispatches forwarding, edit synchronization and deletion synchronization (`internal/app/handler/service.go:157`, `internal/app/handler/service.go:306`, `internal/app/handler/service.go:255`, `internal/app/handler/service.go:273`). | Yes | Neither facade choice provides these copies. No facade business-processing obligation is established; engine must retain its own delivery. |
| Non-permanent deletions / other updates | Business filter rejects them (`internal/infra/telegram/repo.go:447`). Other internal duties above are handled before filtering. | No | No additional Repo business consumer identified. This does not authorize suppressing their handling inside TDLib. |

## Design constraints and open items

1. **Separate publication, not internal reception.** For this design retain the internal dispatcher, authorization route, session initialization and standard client. Skip allocation and publication of facade's business output. The publication decision must follow send-result and NewChat handling (`internal/infra/telegram/repo.go:383`, `internal/infra/telegram/repo.go:401`). Merely assigning a nil channel is insufficient: an unconditional send to it blocks forever. No caller-side drain is required.

2. **Preserve existing request semantics.** Facade SendMessage returns the initial call's error, not final delivery (`internal/app/facade/service.go:48`). A verified `SendMessageAndWait` consumer is the live fixture (`internal/test/support/live_stack.go:367`), not facade's interface. Keeping its private route does not add a facade delivery guarantee. Warmup's bounded retry window and event/ticker wakes are documented intent (`.trellis/spec/backend/telegram-tdlib-guidelines.md:61`); the actual operation remains the success oracle (`internal/infra/telegram/warmup.go:304`). Keeping the event path avoids an unnecessary timing change. Deleting it would require separate behavioral justification; polling alone does not establish equivalence.

3. **Clarify observability without inventing obligations.** Convergence remains event-driven; backlog measures the business channel (`internal/infra/telegram/metrics.go:93`, `internal/infra/telegram/metrics.go:100`). With no queue, decide whether backlog is absent or explicitly zero; neither should imply an active subscription. Production dashboard dependence is unverified. Metric existence alone does not justify retaining a mechanism indefinitely, and comments about no-op instrumentation do not prove production inactivity. Preservation here establishes neither race freedom nor full-pipeline liveness.
