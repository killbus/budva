# Runtime implementation handoff

## Status and ownership

The scoped implementation is ready and source ownership is released to the parent/full-scope checker. No further code edits or diagnostic retries are planned. Parent owns task records, manifests, spec sync, independent reviews, and validation integration. Native/CI gates are outside this scoped implementation completion.

The original eight Go files and both reports were assigned exclusively to this agent. Parent subsequently transferred `internal/infra/telegram/updates_test.go` and its T2-T5 draft; that file was confirmed absent before it was added. No agents were spawned. Unrelated dirty files were preserved. Branch: `fix/facade-update-ownership`; supplied baseline: `84a9df228944a72876e08cf4555c69c423440078`.

## Changed paths and contract

- `internal/infra/telegram/repo.go`: required `New(cfg, mode)`, `UpdateMode uint8`, `NoBusinessUpdates = iota` / `BusinessUpdates`, and an immediate explanatory invalid-mode panic. The private nil/non-nil outlet encodes immutable mode; enabled allocation is capacity 100 before Start, with no extra mutable mode field.
- `cmd/facade/main.go`, `cmd/stand/main.go`: explicitly request-only. `cmd/engine/main.go`, `internal/test/support/live_stack.go`: explicitly business-enabled.
- `internal/infra/telegram/repo_test.go`: T1 covers both allocations, capacity, stable accessor identity, auth/clientDone initialization, warm state, invalid modes 2/255, and disabled-access panic. All 21 other existing constructors explicitly select request-only responsibility.
- `internal/infra/telegram/warmup_test.go`: shared constructor explicitly request-only.
- `internal/infra/telegram/doc.go`: both caller roles, competing readers/no broadcast, immutable configuration, consumer duty, and unchanged shutdown limits.
- `internal/infra/telegram/updates_test.go`: transferred T2-T5 draft applied in full. It drives real Repo.listenUpdates through an inactive SDK listener: 4096-event progress, registered private send waits, materialized readiness before ticker rescue, actual 101st full-queue handoff and complete ordered/content/identity drain, and ordinary receive-cancellation cleanup in both modes.
- Task reports: `research/agent-runtime-goal.md` and this file.

The existing SDK pump and Repo listener are retained. Private send-result dispatch and NewChat readiness/convergence precede the absent-outlet guard. Enabled filtering and ordered blocking sends are unchanged. Updates keeps its signature and panics when disabled; it is an accessor, not subscription registration. One structured mode log is emitted per construction. Backlog is observed only when the queue exists; there is no invented drop metric, extra queue/worker, or closure change.

Scoped constructor/literal inspection found the four runtime/support callers, package-local test constructors/helper, doc examples, and the constructor-owned Repo literal. No additional required site was identified. Parent independently confirmed the current four caller modes. The final broad alias/literal diagnostic batch was interrupted without a result, so no completed exhaustive final scan is claimed.

## Executed evidence and limits

- Required workflow, nine manifest references, PRD, reviewed design, and implementation plan were read. Static source self-review checked handler ordering, nil-outlet placement, unchanged enabled publication, explicit caller roles, and the supplied fixture's cancellation/producer ordering. Planning-with-files kept persistence inside these two reports; simplify guided only scoped readability review, with no redesign.
- Approved installed apply_patch writes succeeded for all nine Go files. An earlier repo_test patch failed a hunk match; inspection showed the file unchanged, then ordered smaller patches succeeded. Retrieved successful receipts: T1 `ee8eeb`, remaining test callers `f04078`, doc `6687ff`, transferred tests `38ee7c`, all exit 0.
- `go version` executed: exit 0, output `go version go1.25.9 windows/amd64` (receipt `8ef18d`).
- `gofmt -w internal/infra/telegram/repo.go internal/infra/telegram/repo_test.go internal/infra/telegram/warmup_test.go internal/infra/telegram/doc.go internal/infra/telegram/updates_test.go cmd/facade/main.go cmd/stand/main.go cmd/engine/main.go internal/test/support/live_stack.go` executed: exit 0, empty output (receipt `d3a0e6`). This is formatter/parser evidence, not package type-checking or behavioral proof.
- Subsequent `gofmt -l` and task-scoped `git diff --check` were submitted in one read-only batch, then interrupted during the approval-sensitive call (tool returned only `aborted by user after 442.6s`). Neither command returned an exit code/output; neither is counted as executed or passing. The parent's separately timed-out checks are not counted either.
- No go test, go vet, race, mutation-control, live-account, SDK-pump, or CI run was executed here. TDLib-dependent build/test/vet requires the Linux/TDLib environment; the available host is Windows. Do not infer successful compilation or behavioral red/green results from formatting or static review.

## Handoff blockers and tool state

Normal sandbox process creation has the known ACL error 5. Writes used the installed `D:/Applications/Scoop/apps/codex/current/codex.exe --codex-run-as-apply-patch` entrypoint with normal explicit approval; no ACL changes, approval bypass, alternate writer, dependency/workflow changes, commits, pushes, or remote actions occurred.

All source-writing patch/formatter cells have returned exit 0; no source-writing command remains outstanding. No exec session ID was returned for the interrupted read-only batch, and its underlying command completion is unconfirmed. Any such command can only read source. Stop at this checkpoint; parent handles remaining validation rather than awaiting diagnostic retries.

Retain all T1-T5 cases for Linux validation. For the bypass negative control, return at listenUpdates entry before GetListener, avoiding premature SDK listener closure against a producer. Other red controls include old unconditional outlet allocation/publication, guarding before internal effects, lost NewChat dispatch, full-queue drop/default, changed filtering/content/order, and omitted listener Close. These are intended sensitivity checks, not executed evidence. Cleanup drain occurs after the verdict; the fixture does not test native SDK fan-out.

Only the unused request-only business-publication wait edge is repaired. Engine consumer stalls, SDK close races, auth backpressure, and full-pipeline liveness remain unclaimed. The remaining native checks, parent spec/task gates, and independent integration review do not expand this agent's completed implementation scope.
