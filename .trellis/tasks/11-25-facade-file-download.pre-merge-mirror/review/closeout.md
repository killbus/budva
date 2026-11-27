# Local commit record — facade file download

## Delivered behavior

The implementation is in the `feat/facade-file-download` worktree, based on
`fbc22cc`. The owner confirmed the two local commit batches. Implementation
commit: `d22b3072f4e1b96a8f62e4c5d61b3acff55feac7`. The owner subsequently
authorized publication. The branch was pushed to origin at
`3ba0cd6f78b3881fdc0db00c850794d1900a2a2b` (task-record commit); no merge has
occurred. PR https://github.com/killbus/budva/pull/16 is open against main.
Linux ci-test failed at
https://github.com/killbus/budva/actions/runs/45613676314 for that head:
the pinned TDLib build's CMake configuration was rejected before Go
checks could run. See `review/ci-evidence.md` for the failure and implemented
CI compatibility patch. Independent CI review passed; repair commit
`518bfcc3b167512f089403d1709fb5bd38a6c2e0` is published to PR #16.
Run https://github.com/killbus/budva/actions/runs/45630679946 completed
successfully for that head: cold TDLib build/cache, vet and tests passed.
The owner subsequently requested an acceptance image. Docker publication
run https://github.com/killbus/budva/actions/runs/45640542028 was cancelled
when the owner questioned whether the path-only behavior met the intended
download contract. Both push steps were skipped; no test image published.

- `GetMessageFile(chat_id, message_id)` resolves one of eight media types,
  downloads the file synchronously, and returns `file_path`, `size`,
  `file_id`, and content-derived `mime_type`.
- `DownloadFile(file_id)` downloads a known TDLib file and returns
  `file_path` and `size`.
- Returned paths belong to the facade server's filesystem. These RPCs do
  not transfer file bytes to the calling machine. Photos use the largest
  variant; client-selectable variants remain outside scope.
- Success requires a non-nil file and local state, a completed download,
  and a non-empty local path.

## Review and verification

- Design gate: PASS, round 4; hash
  `e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75`.
- Implementation alignment: round 2, including the invalid fixture-field
  correction verified before P3.
- Independent code review: PASS, round 1, eight criteria met, no blocking
  findings; report: `review/findings.json`, reviewer
  `aa54f5bf-a47e-48ee-95b7-6ab0cdf91770`.
- Local evidence: clean gofmt, generated protobuf byte identity, source
  field checks against go-tdlib v0.7.6, and `git diff --check`.
- CI extension review: PASS, round 1, no blockers, recorded in
  `review/ci-findings.json`. CI-N1's header comment clarification is applied;
  CI-N2 notes the existing CRLF convention. YAML parsing and workflow/spec
  whitespace checks passed; this does not establish Linux test success.
- Task-record staging preserves CRLF/mixed line endings from the reviewed
  artifacts. Check these with `core.whitespace=cr-at-eol`; the locked
  `design.md` also retains its reviewed trailing blank line at EOF, so
  only that file uses `cr-at-eol,-blank-at-eof`. No design bytes or hash
  changed. These formatting exceptions do not apply to implementation code.
- Test coverage and red paths were inspected statically. No Go build, vet,
  test suite, executed mutation test, or live Telegram download was run in
  this Windows environment. The first Linux run failed during TDLib
  configuration; the second Linux run 45630679946 passed the cold build,
  `go vet ./...` and `go test ./internal/...` on the published repair head.

### Non-blocking review follow-ups

- F1: the 21 unit-count rows sum to 446, while the summary said 445.
  Fixed by the Implementer in `docs/TEST-MATRIX.md`, with a
  `responses.json` evidence note. The orchestrator independently recomputed
  all 21 rows (446) and verified the declared total and clean diff check.
- F2: the baseline has 15 RPCs and this implementation has 17. References
  to 16 existing RPCs in the planning documents are factual typos. The
  reviewed design stays byte-for-byte locked; use these verified counts
  in any subsequent design revision. The additive contract is unaffected.

### Spec-sync assessment (workflow §3.3)

The facade implementation required no spec changes. The existing
`telegram-tdlib-guidelines.md` already requires an adapter limited to live
consumers, preserves plain wrappers for operations outside chat warmup,
and documents the Windows cgo limitation. `quality-guidelines.md` already
requires strict mock expectations and a known red path. This implementation
uses those rules without introducing a new cross-project pattern.

The later owner-approved CI extension adds the observed CMake compatibility
failure and runner-only policy override to `docker-ci-guidelines.md`. See
`ci-scope-addendum.md`; the original facade design and review stay locked.

Task-specific lessons are retained in the research/revision records:
verify dependency fields in both production code and test fixtures against
the pinned source; `GetFileMimeType` accepts a filename, not a file ID;
prefer existing content MIME fields and avoid an unused adapter method.

The referenced `../level85/AGENTS.md` and `.agents` directory are absent
from this checkout; available local workflow/spec files supplied the
project guidance. `.trellis/scripts` is also absent, so metadata is updated
manually and the task remains `in_progress`.

## Approved local commits

The owner replied "确认" to this batch under `.trellis/workflow.md` §3.4.
Both commits belong to the feature worktree, with explicit staging paths.

1. `feat: expose facade file download RPCs (11-25)`
   - Commit: `d22b3072f4e1b96a8f62e4c5d61b3acff55feac7`.
   - `internal/transport/grpc/pb/facade.proto`
   - `internal/transport/grpc/pb/facade.pb.go`
   - `internal/transport/grpc/pb/facade_grpc.pb.go`
   - `internal/app/facade/service.go`
   - `internal/app/facade/errors.go`
   - `internal/app/facade/mocks/telegram_repo.go`
   - `internal/app/facade/service_test.go`
   - `internal/infra/telegram/client_adapter.go`
   - `internal/infra/telegram/mocks/client_adapter.go`
   - `internal/infra/telegram/repo_test.go`
   - `internal/transport/grpc/transport.go`
   - `internal/transport/grpc/mocks/facade_service.go`
   - `internal/transport/grpc/transport_test.go`
   - `docs/TEST-MATRIX.md`
2. `chore(task): record facade file download review (11-25)`
   - `.trellis/tasks/11-25-facade-file-download/**` (planning, research,
     review evidence, current task metadata, and this closeout plan).
   - Metadata records the implementation hash. This file is part of the
     second commit; its own hash is obtained from Git history.

All dirty feature-worktree paths belong to this task; there are no
unrecognized dirty files in the proposed batch. The main checkout's
`.trellis/.runtime/` contains local tools/validation scratch and is excluded.
Do not stage the main checkout, the proto symlink, or scratch worktrees.

## Remaining gates

Authentication is resolved: after disclosure of the initial auto-review
rejection, the owner explicitly authorized GCM. Its GitHub credential was
used only in the process environment to authenticate gh, without logging
or saving the token. PR #16 was created and triggered ci-test.

1. Resolve the contract mismatch: gRPC supports file-byte server streaming,
   while this implementation returns only server-local paths. Publication
   is stopped; the previous path-based acceptance guide is a retained draft.
   Any streaming implementation needs revised requirements/design/review,
   then CI and image acceptance before merge readiness can be reassessed.
2. Merge only with explicit owner approval, then archive/journal the task.

Do not mark the task completed or claim the full suite passed before these
gates are resolved.
