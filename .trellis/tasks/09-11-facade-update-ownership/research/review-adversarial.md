# Adversarial artifact review — v1

Date: 2026-09-11. Scope: independent review of the proposed facade update-ownership contract, scope, assembly, and falsification gates. This is not an implementation or a simulated chatroom transcript.

Paths below are relative to `D:/Repositories/budva`. Unqualified `prd.md`, `design.md`, and `implement.md` refer to `.trellis/tasks/09-11-facade-update-ownership/`. Line references identify the inspected v1 artifacts, not future revisions.

## Recommendation

**Approve the bounded design. No blocking design findings.** There is one low-severity factual correction below. Approval here means the proposed boundary is defensible; it does not authorize implementation or establish that any acceptance gate has passed. User confirmation, implementation review, Linux validation, and separately authorized native acceptance remain required by `implement.md:5`, `implement.md:48`, and `implement.md:72`.

## Finding ADV-1 — Incorrect attribution of a send-completion obligation to stand

- **Severity:** Low; evidence/provenance error.
- **Artifact evidence:** `design.md:74` states, “Stand and direct Repo users still need SendMessageAndWait.”
- **Source evidence:** Stand dispatches to fixture creation or deletion at `cmd/stand/main.go:138`; its actual Telegram operations are `CreateNewBasicGroupChat` at `:213`, `CreateNewSupergroupChat` at `:226`, `SetSupergroupUsername` at `:245`, and `DeleteChat` at `:270`. Its terminal dependency exposes only `ClientDone`, `GetMe`, and `GetOption` (`internal/transport/term/transport.go:23`). A verified caller of `SendMessageAndWait` is LiveStack's test-support method `PutMessageReply` (`internal/test/support/live_stack.go:367`). The task's own research correctly identifies that caller at `research/internal-obligations.md:32`.
- **Concrete consequence:** The design attributes a final-ID waiting responsibility to the wrong executable. That can turn a deliberate shared-Repo compatibility choice into a supposed stand requirement and misdirect acceptance tests. It does not show that stand needs a business outlet, nor that facade must wait for final delivery.
- **Smallest adequate correction:** Replace the attribution with: “Keep the existing private send-result route as Repo compatibility; LiveStack is a verified SendMessageAndWait caller. Stand performs fixture-management requests, and facade's existing sends retain their initial-call return semantics.” No extra consumer, new API, or stand behavior change is warranted.
- **Blocks design approval:** No. The dispatch-preservation decision has an independent compatibility rationale, and the outlet removal remains valid. Correct the claim when incorporating review feedback.

## Scope and contract conclusions

### Omitting the outlet does not erase an identified facade or stand duty

This conclusion rests on inspected responsibilities, not merely on the absence of a reader. README assigns request serving to facade and fixture creation/deletion to stand (`README.md:61`, `README.md:67`). The actual gRPC service declares request/response methods (`internal/transport/grpc/pb/facade.proto:8`); GraphQL exposes a status query (`internal/transport/http/graphql/schema.graphqls:6`). Facade's dependency interface has no `Updates` (`internal/app/facade/service.go:17`), and its send operation returns the initial Repo call's error (`internal/app/facade/service.go:48`). Stand's operations are identified in ADV-1.

By contrast, handler explicitly requires and reads `Updates` (`internal/app/handler/service.go:22`, `:141`), and LiveStack reads it to maintain message mappings and process edits/deletes (`internal/test/support/live_stack.go:184`). Their required enabled modes are therefore substantive, not cosmetic.

A concrete counterexample would be a documented facade-owned callback or mapping operation that must receive an event from this exact outlet. No such obligation was found in the inspected interfaces and assembly. Discovery of one would activate the existing replanning condition at `implement.md:83`; it would not justify assuming a companion consumer today. Not creating an unrequested business copy is distinct from discarding an event already owed to an enabled consumer.

### Retaining internal processing is a bounded compatibility choice

The current loop routes private send outcomes at `internal/infra/telegram/repo.go:383`, then chat readiness at `:391`, before the business publication at `:401`. The proposed guard at `design.md:59` preserves that separation. Authorization and client initialization remain separate routes (`internal/infra/telegram/repo.go:325`, `:340`).

Removing the whole listener is not shown equivalent: `withReady` wakes both on a ticker and on a readiness event (`internal/infra/telegram/warmup.go:269`, `:290`, `:293`). For example, in a configurable 700 ms window with a 500 ms ticker, readiness becoming true at 600 ms can be observed through the event before the deadline but not through the next tick. This is a source-consistent counterexample, not an executed experiment. It supports retaining the current event path for this bounded change; it does not prove that the current listener or every existing metric must exist permanently. The proposed T3 already targets the relevant distinction (`design.md:95`).

### The narrowed fix is honest about what remains broken

`prd.md:7`, `prd.md:27`, and `prd.md:28` explicitly exclude engine overload and SDK lifecycle repair. `design.md:77` expressly admits that an enabled consumer can still stall its process's pump. The current business publication is blocking (`internal/infra/telegram/repo.go:401`); pausing an engine consumer long enough is a concrete counterexample to any claim of general pipeline liveness after this fix. It is not a counterexample to removal of facade's unused outlet.

The task research also records that removing a stall can expose cancellation/closure that the stall previously prevented (`research/concurrency-boundaries.md:27`). Thus “same lifecycle code” must not become “proven unchanged race exposure.” V1 does not make that safety claim: it retains residual-risk wording and requires replanning for a newly caused lifecycle regression (`design.md:136`, `implement.md:86`). I found no evidence requiring an engine or SDK redesign as a prerequisite to approving this local design. I did not independently re-audit the SDK interleavings or validate shutdown safety.

### Typed mode and panic are limited guards, not an assembly proof

Concrete wrong assemblies remain possible: facade can be constructed with `BusinessUpdates`, recreate the unused outlet, and never call `Updates`; engine can be constructed with `NoBusinessUpdates` and still satisfy the handler interface. Both are type-correct under the proposed signature; this is static reasoning, not a compilation result. In the latter case the proposed panic occurs only when `Updates` is called; handler first waits for `ClientDone` (`internal/app/handler/service.go:134`). It is not a pre-authentication startup check. In the former case the accessor panic provides no protection at all.

These are not unacknowledged holes in v1: `design.md:53` explicitly rejects compile success as proof of correct mode, and `implement.md:17`, `:46`, `:66`, and `:67` require inspection of the actual construction sites. The required argument catches omitted migration; the table review establishes the intended role; the panic makes an incorrectly accessed disabled outlet loud. They serve different purposes. Do not report a generic constructor unit test as verification of production assembly.

The proposed mechanisms are proportionate: one fixed construction choice, private nil/non-nil state, and one misuse check, with no registry, worker, or mutable subscription lifecycle (`design.md:36`, `:40`, `:42`, `:68`). A nil return could silently park the handler; dynamic inference from whether someone has called `Updates` could suppress startup events. Those concrete failure modes justify explicit selection and misuse detection. They do not justify adding separate Repo hierarchies or a subscription framework. No additional runtime mechanism is recommended.

## Falsifiers to retain during implementation

These clarify the existing gates; they are not findings that those gates are absent, and none was executed in this review.

1. **Exercise overflow, not just fullness.** T4 at `design.md:96` must reach an attempted relevant publication while the 100-slot outlet is full and the reader remains paused. Feeding exactly 100 events and then resuming permits a drop-on-full mutation to preserve every ID. Use a synchronized attempted 101st event and subsequent marker, then resume and check exact identities/order. The required mutation at `design.md:107` must actually lose an event and fail. If it stays green, reject the test evidence, not automatically the design; no larger buffer or new production queue is needed.

2. **Bind internal success to the real loop.** Feed send-success, send-failure, and NewChat through the controlled listener and verify the result/readiness state. Existing send-result unit tests invoke the helper directly (`internal/infra/telegram/repo_test.go:565`, `:727`), so their green result alone cannot reject removal of listening. T2/T3 and the bypass mutation already address this at `design.md:94`, `:95`, and `:106`. Reject evidence that remains green when internal dispatch is bypassed.

3. **Verify the assembled facade, not only a correctly configured fixture.** A test-created `NoBusinessUpdates` Repo can pass while the executable still selects `BusinessUpdates`. Retain the construction-site review and the actual-binary idle-burst gate (`design.md:44`, `:116`). The native probe must produce the expected successful response for a valid existing request, such as GetChatHistory against known test data, after the known relevant burst; fast validation errors or health 200s are not request-response progress. Record the revision and result. If native acceptance is unavailable, leave that acceptance pending as already required by `implement.md:76`.

## Validation limits and stopping rule

- Reviewed v1 PRD/design/implementation plan; task findings, internal-obligations and concurrency-boundaries research; the project workflow; applicable TDLib, quality, Docker/CI and cross-layer/reuse guidance. Focused source checks covered public schema bodies, facade/stand responsibilities, consumer assembly, Repo dispatch, readiness, and existing send-result tests. Other reviewers' conclusions were not used as proof.
- No runtime code, other task artifact, git state, or CI configuration was changed. This report is the sole write. No subdelegation or network access occurred.
- No Go build, vet, test, race detector, mutation, native listener fixture, Linux CI, live-account run, or BDD scenario was executed. Static reachability and hypothetical schedules are not test results. This review does not verify production exporter configuration, final delivery, general resource boundedness, or SDK shutdown safety.
- Stop reopening architectural selection once the bounded contract is accepted and the existing behavioral, assembly, and native gates have the evidence they claim. Reopen for a concrete lost responsibility or demonstrated regression, not because a larger rewrite might appear less indebted. Conversely, completion of this task must not close the separately recorded engine or SDK risks.
