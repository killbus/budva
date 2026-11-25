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

## Published repair and second Linux run

- Repair commit: `518bfcc3b167512f089403d1709fb5bd38a6c2e0`,
  `fix(ci): support current CMake for pinned TDLib`.
- Pushed to the existing PR #16; the PR head matches the repair commit.
- Run: https://github.com/killbus/budva/actions/runs/45630679946
- Job: https://github.com/killbus/budva/actions/runs/45630679946/job/139262701945
- Status observed: `in_progress`; `Build TDLib (cold cache)` started at
  `2026-11-25T14:08:09Z`. Cache unpack was skipped, so the cold path is
  exercised. `go vet` and `go test` remain pending, with no success claim.
- CI-N1 was corrected before commit; final YAML parse, explicit nine-file
  staging manifest, CRLF-aware staged whitespace check, and design hash
  validation all passed. Original code/design review records are intact.
- `gh pr edit` requested organization fields beyond the token's scopes;
  the PR body was successfully updated through the repository REST API
  using existing permissions. No additional token scopes were requested.
- A proposed repair-and-publication heartbeat was rejected by automatic
  approval review as too broad. The accepted alternative `e9c14965` is
  limited to read-only status checks for this run, every three minutes,
  at most 15 times / 45 minutes. It cannot edit files, commit, push, edit
  PRs, launch repairs, merge, or trigger workflows.

## Final Linux result

Run `45630679946`, exact head `518bfcc3b167512f089403d1709fb5bd38a6c2e0`,
completed successfully at `2026-11-25T14:30:49Z`. Cold TDLib build succeeded
at `14:29:35Z`, cache save at `14:29:36Z`, `go vet ./...` at `14:30:12Z`,
and `go test ./internal/...` at `14:30:43Z`. Read-only monitoring was deleted
after completion. The subsequent user request to publish an acceptance
image resumes record maintenance; the monitor itself made no file edits.

## Acceptance image publication

The owner requested a dispatch-test-like image for live acceptance.
Existing `docker-publish.yml` was dispatched on `feat/facade-file-download`
after checking its remote head equals the successful CI source SHA.

- Run: https://github.com/killbus/budva/actions/runs/45640542028
- Run number: 22; source: `518bfcc3b167512f089403d1709fb5bd38a6c2e0`.
- Expected binary version: `dev-run22-518bfcc`.
- Destination tag: `ghcr.io/killbus/budva:dispatch-test` (mutable).
- Current status: building; both native architectures must build and pass
  smoke before the workflow updates the multi-architecture tag.
- No workflow/source edits, release tag, latest-tag publication or merge.
- Live acceptance procedure: `review/acceptance.md`. Downloaded files live
  on the facade server/container; the RPC returns metadata, not bytes.

### Publication stopped for requirement alignment

The owner said "等等" and questioned whether downloading only to the
facade container was the intended contract, asking whether gRPC supports
streaming. gRPC does support server streaming; the path-only behavior was
a design scope choice, not a protocol restriction. No streaming revision
has been implemented or reviewed yet.

Cancellation of run `45640542028` was confirmed: status `completed`,
conclusion `cancelled`, both architecture jobs and merge job cancelled.
Both push steps were skipped. This run did not update `dispatch-test`.
The earlier CI PASS remains valid only for the existing path-only code;
it is not evidence that client-side file download is implemented.
