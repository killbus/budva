# Takeover review and validation handoff

Date: 2026-09-11. Active task: `09-11-facade-update-ownership`.
Branch: `fix/facade-update-ownership`. HEAD remains the reviewed baseline
`84a9df228944a72876e08cf4555c69c423440078`; the implementation is uncommitted.

## Disposition

No blocking source finding in the inspected task changes. The inherited
implementation, source reviews, structure audit, and spec consistency check are
complete. This is **not** native test acceptance, live acceptance, or release
approval. The task remains `in_progress`.

- Rechecked actual constructors: facade and stand use `NoBusinessUpdates`
  (`cmd/facade/main.go:80`, `cmd/stand/main.go:107`); engine and LiveStack use
  `BusinessUpdates` (`cmd/engine/main.go:91`,
  `internal/test/support/live_stack.go:98`). Import, package-constructor, and
  Repo-literal inventories found no omitted assembly site.
- The required typed mode, disabled allocation/accessor contract, unconditional
  listener startup, and internal dispatch before the publication guard match
  design v2. Enabled filtering, capacity 100, ordering, and blocking sends remain.
- Rechecked the saved adversarial implementation review against current source.
  The new tests exercise the production Repo loop, real pending-send/readiness
  effects after 4096 events, and the attempted 101st publication before draining.
  Source reasoning about these oracles is not an executed red/green result.
- The existing TDLib ownership section and backend index already implement spec
  sync. Their constructor table, evidence limits, and residual-risk wording agree
  with source; no duplicate section or runtime change was needed during takeover.

## Executed evidence in this takeover

| Check | Observed result | Scope / limitation |
| --- | --- | --- |
| `task.py validate` | Exit 0; nine entries per manifest | Context references only |
| `task.py start` | Exit 0; rebound existing task to this session | Still `in_progress` |
| `gofmt -l` on all nine Go deliverables | Exit 0; no output | Formatting, not native type/link/test proof |
| Task-scoped `git diff --check` | Exit 0; no output | Eight tracked Go files and two specs; does not include untracked files |
| Trailing-whitespace scan of `internal/infra/telegram/updates_test.go` | `rg` exit 1; no matches | Separately checks the untracked test file |
| Constructor/import/literal searches and source/diff reads | Modes and dispatch ordering agree with design | Assembly inspection, not command startup execution |
| Fresh independent structure audit | No findings above threshold | Read-only; no test execution or source edits |
| `git status --short --branch` and recent log | Task branch, unchanged HEAD; no staged files | Unrelated dirty work preserved |

Structure reviewer: Dirac (`01a08feb-8d25-7fa2-9e58-4f2fec157735`).
The reviewer inspected all nine Go deliverables and the complete new fixture.
Repo responsibilities remain preexisting; the added helper is test-local rather
than a generalized event framework. The completed reviewer was closed.

## Available environment

The checkout is Windows; `go.mod` requires Go 1.25.9 and go-tdlib v0.7.6. The
native build constraint is documented in the TDLib spec: this dependency cannot
build on Windows merely by disabling cgo. Do not introduce stubs or dependency
changes to manufacture a green result.

- Command discovery found Go, gofmt, gh, and the WSL executable, but no Docker
  executable. Presence of `wsl.exe` alone is not a Linux runner.
- `wsl --list --quiet` exited 1 and returned installation/help text rather than
  installed distributions. `wsl --status` exited 1 with no output. No usable
  local Linux/TDLib runner was established; no WSL or Docker installation was
  attempted. These observations do not diagnose the OS feature configuration.
- The Windows sandbox ACL startup error recurred. Important failed operations
  used normal scoped escalation; no permissions, ACLs, or tooling were changed.

**Not run:** Go build/vet/test, behavioral mutations, race tests, CI dispatch,
live-account tests, actual facade requests, or deployment. No native failures or
successes can be inferred from the environment probes or formatting results.

## Next validation gate (requires a supported Linux/TDLib environment)

Use the existing native setup and pinned TDLib revision `22d49d5`, including the
untracked regression file when transferring the exact task snapshot. Record its
revision/diff and the commands, outputs, and exit codes. Do not push or dispatch
remote CI without separate authorization.

1. Run the targeted constructor/update-loop tests without cache, e.g.
   `go test ./internal/infra/telegram -run '^(TestNew_|TestUpdates_|TestListenUpdates_)' -count=1 -timeout=60s`.
2. In an isolated task snapshot, execute each required negative control and
   record the exact patch plus the actual behavioral failure and bounded cleanup:
   - Restore both unconditional outlet allocation and publication under the
     migrated constructor API. T2 must fail on blocked burst/tail progress, not
     just the T1 allocation assertion or an old-signature compile error.
   - Bypass listening at function entry **before GetListener**, or move the
     absent-outlet guard before internal dispatch. T2/T3 must lose actual internal
     effects. Do not close fixture input underneath producers.
   - Replace enabled blocking publication with drop-on-full. T4 must detect
     missing exact delivery, including event 101 attempted while full.
   Preserve failed verdicts before rescue; use the actual private outlet for
   cleanup. Do not add a bare send to a nil outlet, which rescue cannot unblock.
3. Restore every mutation, then run the existing gates on the final snapshot:
   `go vet ./...` and `go test ./internal/...`. Capture final green, not just a
   previous targeted pass. No permanent workflow expansion is requested.

Existing CI vets command packages but does not execute command startup or account
BDD. The inactive Listener fixture omits SDK registration/fan-out, and direct
loop invocation cannot catch removal of its production launch. Thus green native
tests still do not close the incident by themselves.

Actual-binary acceptance separately requires explicit authorization and an
isolated account/session. Declare relevant update load, real client-backed
requests and expected successful data, request count, latency/error bounds, cold
readiness precondition, and exact revision before the idle-burst/request probe.
Health 200s and fast validation errors are not acceptance.

## Source snapshot (SHA-256)

Captured after static inspection with no active source writer. These identify
the reviewed working files, not an executed native or atomic release revision.

| File | SHA-256 |
| --- | --- |
| `cmd/engine/main.go` | `42BA1289580412523F45B4FFEB1D4CC0D746FEF398E2FB9172D32D70DA98E07F` |
| `cmd/facade/main.go` | `C9A4D5B20BE31F31EBCFD5E8017D903EE2605BD42C47079E46198C0F1FF7EBDF` |
| `cmd/stand/main.go` | `4CFEA2242F764D3C3DDD418C79B477371C3ADBF15D7AA87ACC5FD6B1DEF72DDF` |
| `internal/infra/telegram/repo.go` | `3D488415968C4D5F002B03EADC7C1D8934316FB747C31DFC9FEA347C605C22FB` |
| `internal/infra/telegram/repo_test.go` | `16BD0FEC1587A304934D4A2DECE5C66A920654901D792EBCCC414E729226242D` |
| `internal/infra/telegram/warmup_test.go` | `16274453D126261D6EC78CFD68E80FE3EEE2336A7285BDBE430BD0724D6C3853` |
| `internal/infra/telegram/doc.go` | `137798A7674458367E44C4F13918A2856A5066604D88FB1594C9AA1BA2CD3ED6` |
| `internal/infra/telegram/updates_test.go` | `A1713C2CB0C2F8766AB8D1F0D4F387C5E2254BEA78031549F61D62996F927D0F` |
| `internal/test/support/live_stack.go` | `167E66021291647AF53C9E173AB1729DCAD7554C50386E15F29E5398B9306562` |

## Delivery boundaries

This takeover changes task records only. The nine Go files and two spec changes
were inherited and inspected, not authored again. Nothing is staged, committed,
pushed, archived, or deployed. A future task-only commit plan must classify these
inherited changes, list unrelated dirty paths, and receive confirmation before
staging explicit files; the current handoff does not authorize that step.

Engine overload, SDK listener/response send-close races, and full shutdown/session
safety remain unresolved. Neither unchanged lifecycle source nor this review
establishes unchanged race exposure or whole-pipeline liveness.
