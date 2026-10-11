# Adversarial implementation review

Date: 2026-09-11. Role: independently dispatched Trellis check reviewer; no subdelegation. Scope: the supplied implementation against baseline `84a9df228944a72876e08cf4555c69c423440078`. Only this report and `agent-anti-implementation-goal.md` are reviewer writes.

## Disposition

No code blockers found in the inspected state. The source removes the request-only business-publication wait while retaining internal dispatch and the subscribed outlet's existing delivery behavior. The available evidence does **not** yet establish behavioral/native acceptance: no tests or mutations were executed by this reviewer, and the inspected worker report still marks verification in progress. This is not release approval or a whole-runtime liveness claim.

No scoped code correction is requested. The outstanding evidence gates below must remain open.

## Adversarial checks

- **Ownership and assembly.** `internal/infra/telegram/repo.go:89` requires and validates the typed mode; only `BusinessUpdates` allocates the capacity-100 outlet (`:101`), and disabled access panics (`:143`). T1 inspects allocation before access/start (`repo_test.go:224`). Actual callers select request-only at `cmd/facade/main.go:80` and `cmd/stand/main.go:107`, enabled at `cmd/engine/main.go:91` and `internal/test/support/live_stack.go:98`. Engine launches the handler using that Repo (`cmd/engine/main.go:122`, `:163`); LiveStack consumes its outlet (`live_stack.go:189`). Scoped import/constructor/literal searches found no omitted caller. An incorrectly enabled facade could pass isolated Repo tests; it is rejected here by source assembly review, not by claimed startup execution.
- **Listener and internal work.** The production launch remains unconditional at `repo.go:370`. The guard at `:423` follows real send-result dispatch (`:408`) and NewChat readiness/convergence (`:417`). The SDK/auth construction and warmup implementation are not disabled. The direct fixture invokes `listenUpdates` at `updates_test.go:67`: deleting only the launch in `runAuthLoop` would evade these tests. `clientDone` closes before launch (`repo.go:368`), so it is not a listener-registration barrier.
- **Zero-reader progress.** T2 (`updates_test.go:141`, `:171`) injects 4096 relevant messages followed by a NewChat marker through an unbuffered concrete listener. It requires complete input handoff and the real readiness effect, with no business reader or allocation shortcut. Static prediction: restoring both unconditional allocation and publication stalls before producer completion; a pre-dispatch guard loses the marker. Neither failure was executed.
- **Actual waiter effects.** After the burst, T3 calls actual `SendMessageAndWait` and checks `pendingSends.Load` after quiescence (`updates_test.go:152`, `:165`), not merely entry into the mock. Success/failure assertions check the real returned message/error and pending removal (`:184`, `:225`). Readiness uses actual `withReady`, verifies a registered unclosed same-chat entry, and changes an independent atomic materialization oracle before injecting NewChat (`:232`, `:257`, `:273`). Its 1-second verdict precedes the 1-minute ticker; an unexpected LoadChats call fails the strict mock. Helper-only dispatch and call-count-only success are not substituted for these effects.
- **Full-capacity delivery.** T4 completes the unbuffered handoff of event 101 with no consumer, then waits for quiescence and observes 100 buffered events (`updates_test.go:341`). With the inspected production loop, this reaches the 101st publication attempt while full. Only then does consumption resume. The oracle includes a trailing internal marker, drains remaining buffered output, and checks exact count, independently recreated contents/order, original identities, filtering, and the private send result (`:285`, `:303`, `:350`, `:360`). Static prediction: default/drop loses at least event 101; duplicated publication fails progress/count; in-place content changes fail the independent snapshot. No such mutation has been run here.
- **Failure cleanup.** The deadline calls Fatal before deferred rescue (`updates_test.go:113`). Cleanup cancels work and joins producers/operations before canceling the listener, drains the actual private outlet even for a mutated request-only Repo, then joins loop/drainer (`:90`). Only the loop closes the fixture input. This ordering supports the required old-allocation/full-send and early-guard red paths without turning a failed assertion green. The joins themselves have no timeout; bounded termination must still be observed on each exact native mutation. Specify a loop-bypass mutation's location: entry-before-listener-acquisition is not equivalent to removing production launch or returning after installing deferred Close. The optional nil-outlet/bare-send mutant is not rescuable by this drainer and requires isolation if attempted. T5 (`:373`) covers normal receive cancellation only.

## Evidence limits and unresolved gates

1. **Native red/green remains pending.** On the supported pinned Linux/TDLib environment, run `go vet ./...` and `go test ./internal/...`, then record the required behavioral controls: old allocation plus publication, bypass/early guard, and drop-on-full. Record exact patches/revision, failing assertions, and bounded cleanup; restore mutants. An omitted constructor mode is compile evidence only. Test comments beginning “Red” are predictions, not execution receipts.
2. **Actual binary acceptance remains pending.** The literal `client.Listener{Updates: f.input}` (`updates_test.go:60`) is inactive and unregistered. Pinned SDK `client.go:128` creates/registers the real active, buffered listener; its receiver and TDLib pump are absent from the fixture. Even a Linux-linked green test would not prove SDK fan-out, request catchers, send/close interleavings, or facade startup. Separately authorized acceptance must use the actual assembled facade, zero business consumers, declared relevant burst/load and latency criteria, correct successful existing-request contents, and cold-readiness behavior at the recorded revision. Health/fast error responses are insufficient.
3. **Parent recheck remains necessary.** Gibbs was finishing formatting/reporting. This review applies to the source reads and fingerprints below, not an asserted final tree. Recheck later semantic deltas, final worker evidence, formatting, branch/status, and inclusion of `updates_test.go` in the eventual deliverable. Final fingerprint refresh and formatting verification were denied by the approval service; neither was bypassed.

The inspected `research/implementation-runtime.md` is explicitly in progress and contains no native or mutation success receipt; `research/progress.md` records dispatch, not implementation acceptance. Their historical/planning text is not proof of current execution.

SDK close races, subscribed-consumer overload, authentication/session shutdown, and general resource boundedness remain accepted exclusions. No concrete newly caused regression or lost responsibility was established. Unchanged lifecycle code is not evidence of unchanged race exposure, and normal fixture cancellation does not close those risks.

## Checks actually performed

- Read the entire workflow, `check.jsonl` and all nine referenced documents: backend index, TDLib, quality, Docker/CI specs; task findings, internal obligations, concurrency boundaries, design review, and validation review. Then read the complete PRD, design v2, and implementation plan.
- Read all nine explicitly assigned source files in full (listed below). Additionally inspected the full adapter, metrics, handler service, pinned SDK `listener.go`/`client.go`/`tdlib.go`, `go.mod`, CI workflow, and current progress/runtime report. Focused reads covered the warmup-test constructor helper, generated mock constructor/GetListener/SendMessage, facade request interface/GetChatHistory, gRPC Telegram-type uses, SDK update types and native build constraints.
- Successful read-only commands: `git log -1 --format='%H %s'`; the scoped baseline diff; constructor/import/consumer/mock/SDK searches; initial SHA-256 capture; scoped `git diff --check` (exit 0, no output). The latter included the warmup test migration. The ordinary diff did not include `updates_test.go`; its full content and fingerprint were inspected separately.
- Two initial regex searches failed to parse; corrected fixed-string searches succeeded. Windows sandbox process creation repeatedly failed with `SetNamedSecurityInfoW` error 5; normal approval was used where required. Broad instruction discovery, git status, read-only `gofmt -l`, and final SHA refresh were rejected because automatic approval review was rate-limited. No alternate route was used to obtain those denied results. The exact Trellis check skill path was absent; review followed the documented workflow and this explicit assignment.
- **Not executed:** Go build/vet/test, mutation tests, native or live-account probes, race/BDD tests, CI dispatch, network writes, installations, commits/push/merge, or ACL changes. No source edits, formatting writes, or unrelated-file changes were made by this reviewer.

The inspected CI workflow runs vet over `./...` and tests over `./internal/...` (`.github/workflows/ci-test.yml:121`, `:124`). It does not execute command startup, live-account BDD, or a race detector.

## Inspected-state record

HEAD was observed as `84a9df228944a72876e08cf4555c69c423440078`, matching the supplied baseline. Branch `fix/facade-update-ownership` is supplied by the assignment/worker records; independent status verification was denied. These are the successful initial SHA-256 fingerprints (tool receipt `4f63c1`), captured before the full source reads. They are not a post-formatting or atomic-final-tree attestation; final refresh was denied.

| File | SHA-256 |
| --- | --- |
| internal/infra/telegram/repo.go | 3D488415968C4D5F002B03EADC7C1D8934316FB747C31DFC9FEA347C605C22FB |
| internal/infra/telegram/repo_test.go | 16BD0FEC1587A304934D4A2DECE5C66A920654901D792EBCCC414E729226242D |
| internal/infra/telegram/updates_test.go | A1713C2CB0C2F8766AB8D1F0D4F387C5E2254BEA78031549F61D62996F927D0F |
| internal/infra/telegram/warmup.go | 6C99363D3F65B5814AF6A13BB5315DA63E4D0799A8FD0531E8BC9E367B416BDE |
| internal/infra/telegram/doc.go | B13D902CEDC3FACD8F2C723F17930D59442DF821202BB04BA5D77EC804C87A77 |
| cmd/facade/main.go | C9A4D5B20BE31F31EBCFD5E8017D903EE2605BD42C47079E46198C0F1FF7EBDF |
| cmd/engine/main.go | 42BA1289580412523F45B4FFEB1D4CC0D746FEF398E2FB9172D32D70DA98E07F |
| cmd/stand/main.go | 4CFEA2242F764D3C3DDD418C79B477371C3ADBF15D7AA87ACC5FD6B1DEF72DDF |
| internal/test/support/live_stack.go | 167E66021291647AF53C9E173AB1729DCAD7554C50386E15F29E5398B9306562 |

## Scoped goal

The mandatory successful create/active receipts were persisted before review in `agent-anti-implementation-goal.md`. Only this review goal is eligible for completion after report verification; its actual completion receipt belongs in that file. Task implementation/native acceptance remains pending regardless of this scoped goal's completion.

