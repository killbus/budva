# CI scope addendum — owner approved

## 1. Authorization and baseline

After PR #16 failed during TDLib configuration, the owner asked whether the
repair should be included or made as a new commit. The proposed answer was
to include the repair in this task and append the independent commit
`fix(ci): support current CMake for pinned TDLib` to PR #16. The owner then
said "推进", authorizing implementation, the separate commit, push, and CI
verification. No further scope or commit approval is needed for this repair.

Baseline for this extension: `3ba0cd6f78b3881fdc0db00c850794d1900a2a2b`.
The earlier facade design remains locked at SHA256
`e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75`.
This owner-approved addendum supersedes its §11 exclusion only for the
specific CI files and clauses below. The existing facade P3 PASS stands.

## 2. Contract and responsibility

The CI workflow owns preparation of the pinned TDLib dependency on the
hosted Linux runner, followed by `go vet ./...` and `go test ./internal/...`.
It depends on upstream TDLib and CMake; application code must not acquire
CI-specific behavior. Docker image construction is unchanged.

The failed run is 45613676314; see `ci-evidence.md`. Its CMake diagnostic
explicitly suggests `-DCMAKE_POLICY_VERSION_MINIMUM=3.5`. Add exactly this
flag to the TDLib configure command. Keep the pin `22d49d5`, cache keys,
`--parallel 2`, source splitting, install/archive steps, triggers,
permissions, and vet/test commands unchanged.

## 3. Authorized files

- `.github/workflows/ci-test.yml`: the configure flag only. Existing
  comments that describe the runner-only deviations may be corrected for
  consistency; no new inline comments are required.
- `.trellis/spec/backend/docker-ci-guidelines.md`: record the observed
  diagnostic and runner-only CMake policy override; preserve the existing
  dependency-pin/cache/parallelism contracts.
- This task's metadata and `review/` records: authorization, evidence,
  implementer response, independent review, PR/CI links, progress.

Non-goals: no Dockerfile change, dependency upgrade, new abstraction,
application/proto/test change, build parallelism/cache change, test skip,
workflow permission expansion, or automatic merge.

## 4. Acceptance and loop

1. Inspect the diff against the extension baseline: only the configure
   flag and explanatory docs/records change. Validate YAML if a parser is
   available, and run a whitespace check on the workflow.
2. The existing Implementer profile writes the workflow change and
   `review/ci-responses.json`; the orchestrator writes spec/task records.
3. The existing Reviewer profile independently reviews this extension and
   writes `review/ci-findings.json`. These separate files preserve the
   original facade review evidence. The authorized spec edit is part of
   this extension and is not an unauthorized change to the review ruler.
4. Commit independently with the approved subject and push to PR #16.
   This triggers a new Linux run for the new head SHA. Configuration must
   pass, TDLib must build/install, then vet and internal tests must pass.
   Record actual results; static review is not evidence that CI passed.
5. Subsequent diagnostics in this build path are handled within this
   narrow compatibility repair only if they preserve the non-goals.
   Stop for a new scope decision if a dependency/version/cache contract
   would need to change. Merge remains an owner gate.
