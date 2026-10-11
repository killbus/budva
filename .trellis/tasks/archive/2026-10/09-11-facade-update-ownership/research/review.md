# Design review record

Status: design review complete; recommend proceeding to the separate user implementation-authorization gate. Both independent reports approve the bounded design, with no blocking design findings. This is not implementation or release approval.

## Review basis

- User-confirmed requirement: facade serves its existing request API with no business-update subscriber and no caller-side drain.
- Reviewed artifacts: prd.md, design.md v1/v1.1, and implement.md. The main agent integrated the nonblocking findings and verification clarifications into v2 without changing the reviewed construction API, runtime scope, or acceptance boundary. This records incorporation, not a separate claim that both reviewers reread v2.
- First-principles source checks: README.md, application responsibilities, all production/support Repo constructors, internal event routing, pinned SDK listener implementation, and existing Linux CI commands.
- Source baseline: main at 84a9df228944a72876e08cf4555c69c423440078. A scoped diff check found no modifications under cmd, internal, go.mod, go.sum, or .github during this planning turn.
- Scoped agents received their assignments only after successful active-goal handshakes. Both source researchers and both independent reviewers returned real completed-goal receipts; all four agents were closed after their assigned work completed.

## Completed source research

| Research | Result | Main integration |
| --- | --- | --- |
| Parnas: internal-obligations.md | No facade business-output consumer was identified; send-result and readiness/convergence routes are separate internal obligations. | Keep the listener and internal dispatch; make only the extra business outlet optional. |
| Hoare: concurrency-boundaries.md | Guard publication after internal work; nil alone still blocks. SDK closure risks predate the patch. A non-nil SDK Listener literal is feasible for the adapter seam. | Keep construction immutable; use the concrete fixture without a new production interface; do not claim unchanged shutdown risk or native pump coverage. |

## Independent review and disposition

| Finding / review | Severity and recommendation | Disposition in v2 |
| --- | --- | --- |
| ADV-1, review-adversarial.md: stand send-completion attribution | Low, nonblocking; approve bounded design. | Corrected design section 5. Stand performs fixture requests; LiveStack is the verified SendMessageAndWait caller. Preserve the private route as Repo compatibility, not an invented stand duty. Main checked the call sites. |
| V1, review-validation.md: fullness is not overflow | Low, nonblocking design clarification; approve bounded design. It blocks implementation acceptance if the mandated mutation does not fail. | T4 explicitly attempts event 101 while all 100 slots remain full, using input handoff plus quiescence/barriers before resume. Require exact content/order/count and no extras after processing. |
| Validation preconditions and failure cleanup | Acceptance precision, not an additional design finding or new runtime mechanism. | T2 must observe the real tail effect; T3 runs after the burst with actual waiter registration and stateful readiness before polling. Teardown preserves the failed verdict, stops producers, drains only a non-nil private outlet if needed, and joins workers. Optional unrescuable nil-send controls require bounded isolation. |
| Assembly and native falsifiers from both reviewers | Preserve existing review/probe gates; do not mistake type checks for assembly proof. | Inspect actual constructor choices. The disabled getter detects access misuse, not pre-authentication startup or an erroneously enabled facade. The actual-binary probe must return correct successful data under predeclared load/latency/cold-state conditions. |

The main agent accepts these corrections. No additional consumer, production hook, queue, hierarchy, SDK patch, or registration lifecycle was added to the proposal. The first-principles boundary remains facade/stand request-only assembly versus engine/LiveStack business consumption. Retaining internal dispatch is a bounded compatibility choice, not a claim that every existing mechanism must be permanent.

Source-referenced detailed reviews: review-adversarial.md and review-validation.md. Static schedules in those reports are counterexamples and test-design reasoning, not executed mutations.

## Checks executed in the planning turn

| Check | Observed result and limit |
| --- | --- |
| Trellis task/current pointer | Points to this task for the current Codex session; task lifecycle remains planning. |
| task.py validate | Final context manifests passed with nine real entries each, including the review record and validation protocol. |
| Task record / scope | task.json parses successfully, status is planning, and branch/commit are unset. Runtime/CI paths remain unmodified. |
| Task-directory trailing-whitespace scan | No matches. The task is untracked, so ordinary git diff does not validate its new files. |
| Repository-wide git diff --check | Fails on preexisting .trellis/workspace/killbus/index.md trailing whitespace at lines 23-25 and 46. This task did not edit that user-owned file. |
| Native Go tests / CI / live Telegram acceptance | Not run; this turn is planning only. |

## Remaining gates

The planning artifact set is validated and all scoped completion receipts are collected. The next required decision is user implementation authorization; task status remains planning. Do not run task.py start, switch branches, or edit runtime code in this planning turn. Native CI, behavioral red/green evidence, isolated-account acceptance, commits, and rollout remain future gates, not completed checks. Engine overload, SDK send/close races, and full session/shutdown safety remain open risks; completing this task cannot close them.
