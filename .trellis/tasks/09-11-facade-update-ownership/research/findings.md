# Evidence ledger

## Authority and scope

- User accepted the zero-business-subscriber facade invariant and requested design plus review through Trellis.
- `.trellis/workflow.md` separates task creation/design approval from implementation approval. The user subsequently authorized the reviewed implementation with “开始推进” on 2026-09-11.
- Existing unrelated dirty files are user-owned and will be preserved. The bootstrap-guidelines task is not this task.

## Prior verified facts to re-check at the design seam

- `telegram.New` creates an updates channel of capacity 100 unconditionally. `Updates()` returns that channel, not a registered subscription.
- Facade has no production consumer of that channel. Engine constructs a separate Repo and starts `handler.Run`; it cannot drain facade memory.
- Facade exposes request APIs; current gRPC/GraphQL schemas have no business-update subscription endpoint.
- Repo internal send-result dispatch and warmup notification precede the sole business publication in `listenUpdates`.
- The user-provided production dump identifies blocking publication as the first edge of a cascade into the SDK receiver and global receive pump. Dump timings are incident evidence from the handoff, not newly reproduced here.
- The previous discussion identified independent engine-queue and SDK close-race risks; they are not automatically closed by a facade-only fix.
- A no-op metric comment is not proof that production metrics are disabled; provider initialization and deployment configuration matter.

## Current-source checks

- Repo publication is still a bare `r.updates <- typ`; internal send-result dispatch and `UpdateNewChat` readiness/convergence handling run before it (`repo.go:383-401`).
- Construction callers include not only facade/engine but `cmd/stand/main.go:107` and `internal/test/support/live_stack.go:98`; any default change must account for all four.
- Package tests and `doc.go` also use `New`. The implementation needs an exhaustive constructor/Repo-literal inventory, not a two-binary edit.
- The historical handoff corrects beta4 to beta1. Its further claim that metrics are definitely no-op rests on a comment and is not accepted as current deployment evidence.
- Read backend TDLib, quality, and Docker/CI specs completely. Native go-tdlib verification needs Linux/CI; tests must have falsifiable red paths and virtual-time synchronization.
- Read cross-layer and code-reuse guides. Constructor responsibilities and update routing belong at their existing boundaries; avoid a new event framework.
- Independent research started after each agent supplied a successful active-goal receipt: internal obligations (Parnas) and concurrency boundaries (Hoare).

## Assembly and verification checks

- First-principles cross-check: README.md describes Budva as a message-forwarding product, but its Run/Architecture sections explicitly assign forwarding to engine, HTTP/gRPC service to facade, and fixture management to stand. The Docker section describes independent facade/engine TDLib clients and session volumes. Product-wide forwarding does not imply a business-update consumer in every process; another process cannot drain this Repo channel. Package docs for app/facade and app/handler agree with that split.
- cmd/stand starts a Repo plus auth/terminal consumers, but no business-update consumer. Its request-only shape needs the same disabled outlet as facade.
- LiveStack owns a real business consumer (processUpdates, including temporary/permanent ID mappings and edit/delete handling), so it must retain an enabled outlet.
- Existing TestNew_InitializesChannels asserts an unconditional updates channel. Replace it with explicit construction-mode tests rather than silently weakening the assertion.
- There is no existing listenUpdates test. The new regression must drive the real loop, not just a helper disconnected from production. The concurrency research confirms a concrete SDK Listener literal with a non-nil channel is sufficient for the controlled adapter seam; the main reviewer checked the pinned listener source. It is not registered with the real SDK receiver and therefore cannot prove native fan-out safety.
- Current CI runs go vet ./... and go test ./internal/...; it does not run cmd tests, LiveStack account tests, a race detector, or live Telegram probes. Do not describe those as already covered.

## Reviewed choices (implementation now authorized)

- Use an explicit construction-time role/mode rather than a dynamic subscriber registry or background drain. A required typed mode forces all current constructors to make a choice; source review and the actual-binary probe must still establish that each choice is correct.
- Keep the production receive loop and all internal dispatch before the optional business publication guard. Mode is immutable for Repo lifetime.
- Preserve the subscribed path capacity, type filter, order, and blocking semantics in this bounded repair. Engine overload is a separate risk, not solved by this task.
- Distinguish a disabled outlet from a dropped subscribed event; do not add a false drop counter or claim absence of a backlog proves health.

## Independent review outcome

- Both validation and anti-audit reports approve the bounded design with no blocking design findings. V2 incorporates their two low-severity corrections and acceptance clarifications; see research/review.md.
- Corrected the draft attribution: stand does not call SendMessageAndWait; LiveStack is the verified caller. Keeping private send-result dispatch is a shared-Repo compatibility decision.
- T4 must attempt the 101st relevant publication while capacity 100 remains full. T2/T3 require observed internal effects, actual waiter registration, and pre-ticker readiness after the burst; failure cleanup must not mask the verdict.
- All four scoped research/review goals have completed and their agents are closed. No runtime implementation, native test, CI, or live-account probe occurred.

## Execution resumption

- Sandbox startup still fails, but explicitly approved outside-sandbox commands now work. The inspected installed apply_patch wrapper invokes `codex.exe --codex-run-as-apply-patch`; use that same editor through normal approval without changing ACLs or project tooling.
- Current branch is `fix/facade-update-ownership` at `84a9df228944a72876e08cf4555c69c423440078`, not main as in the historical handoff. Both 9-entry manifests revalidated successfully; task.py set-branch and start succeeded.
- The CLI patch entrypoint can partially update earlier files before a later hunk fails. Verify every target after a patch error.

## Implemented contract (supersedes the baseline source observations above)

- New requires an immutable typed UpdateMode. Invalid values panic; disabled construction allocates no business outlet, and disabled Updates access panics as a programming error. Enabled access returns the same capacity-100 outlet; it is not registration or broadcast.
- All four assembly choices were verified by source inspection and constructor inventory: facade/stand disabled, engine/LiveStack enabled. No external subscriber, drain goroutine, dynamic registry, drop-on-full policy, or SDK lifecycle change was introduced.
- The production listener still dispatches private send results and NewChat readiness/convergence before skipping absent business publication. Enabled filtering, contents, order, and blocking semantics are retained.
- Tests drive the actual Repo loop with an inactive SDK Listener fixture, not the native receiver. They cover a 4096-event no-consumer burst, real send waiters and pre-ticker readiness after the burst, attempted 101st publication while full, exact subscribed delivery, and normal receive-wait cancellation. Behavioral/mutation execution is pending.
- A listener-bypass negative control must return at function entry before GetListener. Do not close synthetic input underneath producers or add an unjoinable nil-channel-send mutation. Cleanup must preserve the failed verdict before rescue-draining the actual private outlet.
- Formatter success is established by the runtime worker's exit-0 receipt. Native compilation, liveness, mutation discrimination, SDK fan-out, and actual-binary behavior are separate unexecuted gates.

## Takeover verification (2026-09-11)

- Revalidated the current task and all constructor choices; the nine-file source implementation remains inherited, with no runtime edit during takeover. The source snapshot and review disposition are recorded in research/review-takeover.md.
- Re-ran nine-file gofmt list mode, task-scoped tracked diff checking, and a separate trailing-whitespace scan for the untracked update-loop tests. All passed as static checks only.
- A fresh independent structure audit returned no finding above threshold. Rechecked the existing TDLib ownership spec and backend index; spec sync is present and consistent, contrary to older pending notes.
- No usable local native runner was established: Docker command absent, wsl --list --quiet returned installation/help text with exit 1, and wsl --status returned no output with exit 1. No installation or remote CI dispatch was attempted.
- Native test execution, actual red-control failures, restored final green, and isolated-account facade acceptance remain open. Formatting, source review, and inactive-listener tests cannot substitute for these gates.

## Existing native CI channel confirmed (2026-09-11 continuation)

- Read the exact ci-test workflow and Dockerfile. Both retain TDLib pin 22d49d5; ci-test uses ubuntu-latest, Go 1.25.9 from go.mod, the existing installed-library cache, go vet ./..., and go test ./internal/.... No workflow change is needed for the first native run of the patch.
- Approved read-only GitHub queries confirm killbus/budva, default main, and active workflow ID 354474075 at .github/workflows/ci-test.yml. The latest run 34462381435 passed for baseline 84a9df228944a72876e08cf4555c69c423440078, not for this uncommitted patch.
- A push to fix/facade-update-ownership alone will not trigger CI; the workflow supports pushes to main, PRs to main, and explicit workflow_dispatch. Commit, push, and dispatch authorization are distinct from the observed GitHub access capability.
- All nine Go hashes match the takeover review. Prepared one task-only checkpoint commit plan with 31 explicit candidate paths and 20 excluded dirty path/directory entries in research/native-ci-handoff.md. This explicitly requests inclusion of inherited task work rather than treating it as new session edits.
- Only remote metadata was read. Native patch tests, required negative controls/restored final green, and actual-facade isolated-account acceptance are still unexecuted. Do not use baseline CI success to close this task.

## Resources

- `README.md` (Run, Docker, and Architecture sections)
- `.trellis/workspace/killbus/research-pipeline-freeze.md`
- `.trellis/workspace/killbus/pipeline-freeze-brief-team-b.md`
- `.trellis/spec/backend/index.md`
- `internal/infra/telegram/repo.go`
- `cmd/facade/main.go`, `cmd/engine/main.go`
- `internal/app/facade/service.go`, `internal/app/handler/service.go`
