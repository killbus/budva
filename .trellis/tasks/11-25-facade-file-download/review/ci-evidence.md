# PR #16 — first Linux CI run

- PR: https://github.com/killbus/budva/pull/16
- Workflow: `ci-test`, event `pull_request`.
- Run: https://github.com/killbus/budva/actions/runs/45613676314
- Head SHA: `3ba0cd6f78b3881fdc0db00c850794d1900a2a2b`.
- Job: https://github.com/killbus/budva/actions/runs/45613676314/job/139203426653
- Result: failure, completed at `2026-11-25T12:26:57Z`.

## Observed failure

The runner restored no TDLib cache and entered `Build TDLib (cold cache)`.
Checkout of TDLib commit `22d49d5b8` succeeded. CMake configuration failed:

```text
CMake Error at CMakeLists.txt:1 (cmake_minimum_required):
  Compatibility with CMake < 3.5 has been removed from CMake.

  Or, add -DCMAKE_POLICY_VERSION_MINIMUM=3.5 to try configuring anyway.
```

The subsequent archive command failed because `include/td` and `lib/libtd*`
had not been installed. That is downstream of the CMake configuration
failure. `go vet` and `go test` were both skipped; this run provides no
compile or test result for the facade implementation.

The workflow is unchanged from the baseline. This is an existing TDLib
build-recipe compatibility problem exposed by the current runner/cache
state, not a failure produced by the new RPC code. No rerun with identical
inputs was requested because the diagnostic is deterministic.

## Implemented compatibility fix (Linux validation pending)

The Implementer added `-DCMAKE_POLICY_VERSION_MINIMUM=3.5` to the TDLib
configuration command in `.github/workflows/ci-test.yml:103`, as suggested
by the CMake diagnostic. The TDLib pin, cache key, parallelism, dependency
versions, and vet/test steps remain unchanged. Existing comments now name
the runner-only policy override. The original candidate patch is saved at:

`D:/Repositories/budva/.trellis/.runtime/facade-ci-cmake-policy.patch`

`review/ci-responses.json` records clean workflow whitespace and successful
YAML parsing with yq. Independent CI review passed (round 1, no blockers;
see `ci-findings.json`). CI-N1's comment wording is corrected to attach
the root-ownership explanation only to sudo. CI-N2 records existing CRLF
in task metadata, which the whitespace check recognizes. Static checks do
not establish that the compatibility adjustment works: a fresh Linux
cold-cache run must configure/build TDLib and then run vet/tests. The local
Windows environment cannot perform that validation.

## Scope decision (resolved)

Locked design §11 originally excluded `Dockerfile/CI changes`. The owner
approved the narrow extension and separate `fix(ci)` commit by replying
"推进" to that plan. `ci-scope-addendum.md` is the scope authority for this
extension. Continue the existing implement/review profiles, document the
runner-only recipe deviation in the CI guidelines, and obtain actual Linux
results before claiming PASS. The facade design/code remain unchanged.

Authentication no longer blocks work: the owner explicitly authorized
GCM after the prior auto-review denial. GCM authenticated gh successfully
with a process-only token; PR creation and CI inspection completed without
printing or saving credentials. No merge has occurred.
