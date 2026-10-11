# Native CI channel and proposed validation commit

Date: 2026-09-11. Task remains in_progress on fix/facade-update-ownership.
This is a pre-validation checkpoint proposal, not task completion or release approval.

## Confirmed channel (read-only observations)

- GitHub repository lookup resolves to killbus/budva, default branch main.
  Access capability does not authorize remote writes.
- The ci-test workflow is active: ID 354474075, path
  .github/workflows/ci-test.yml. Its checked-in configuration uses ubuntu-latest,
  Go from go.mod (1.25.9), and real TDLib static libraries pinned to 22d49d5.
  The Dockerfile pin agrees. The cache key is tdlib-22d49d5-amd64.
- Native commands are go vet ./... and go test ./internal/... (no live account
  suites). The workflow supports manual
  workflow_dispatch, pushes to main, and PRs targeting main. A push only to this
  task branch does NOT automatically trigger the workflow.
- The latest reported run is completed/success on the unchanged main baseline
  84a9df228944a72876e08cf4555c69c423440078:
  https://github.com/killbus/budva/actions/runs/34462381435
  This run does not contain the current uncommitted fix or regression tests.
- All nine current Go file SHA-256 values still match research/review-takeover.md.
  The existing source review/formatting evidence therefore applies to this
  snapshot; it is not native behavioral evidence.

No new CI run, source upload, build, test, behavioral mutation, or account action
was performed. A local Linux runner is still unavailable, but an existing remote
native validation channel is now confirmed; no local environment rebuild is needed.

## Proposed single local checkpoint commit

Message: fix: make Telegram business update ownership explicit

Confirmation must explicitly include the inherited task changes below, which
were not authored in this takeover. Nothing is currently staged. Stage only the
31 enumerated files, never the whole worktree. The new untracked regression file
must be included; normal tracked diff statistics omit it.

### AI-edited task records in this takeover (7)

- .trellis/tasks/09-11-facade-update-ownership/task.json
- .trellis/tasks/09-11-facade-update-ownership/implement.md
- .trellis/tasks/09-11-facade-update-ownership/research/task_plan.md
- .trellis/tasks/09-11-facade-update-ownership/research/progress.md
- .trellis/tasks/09-11-facade-update-ownership/research/findings.md
- .trellis/tasks/09-11-facade-update-ownership/research/review-takeover.md
- .trellis/tasks/09-11-facade-update-ownership/research/native-ci-handoff.md

### Inherited source/spec changes requested for the same commit (11)

These are inspected task changes, not automatically included by a general
permission to continue. The requested confirmation covers their inclusion.

- cmd/engine/main.go
- cmd/facade/main.go
- cmd/stand/main.go
- internal/infra/telegram/repo.go
- internal/infra/telegram/repo_test.go
- internal/infra/telegram/warmup_test.go
- internal/infra/telegram/updates_test.go
- internal/infra/telegram/doc.go
- internal/test/support/live_stack.go
- .trellis/spec/backend/index.md
- .trellis/spec/backend/telegram-tdlib-guidelines.md

### Inherited task artifacts requested for the same commit (13)

- .trellis/tasks/09-11-facade-update-ownership/prd.md
- .trellis/tasks/09-11-facade-update-ownership/design.md
- .trellis/tasks/09-11-facade-update-ownership/implement.jsonl
- .trellis/tasks/09-11-facade-update-ownership/check.jsonl
- .trellis/tasks/09-11-facade-update-ownership/research/agent-anti-implementation-goal.md
- .trellis/tasks/09-11-facade-update-ownership/research/agent-runtime-goal.md
- .trellis/tasks/09-11-facade-update-ownership/research/concurrency-boundaries.md
- .trellis/tasks/09-11-facade-update-ownership/research/implementation-runtime.md
- .trellis/tasks/09-11-facade-update-ownership/research/internal-obligations.md
- .trellis/tasks/09-11-facade-update-ownership/research/review-adversarial.md
- .trellis/tasks/09-11-facade-update-ownership/research/review-implementation-adversarial.md
- .trellis/tasks/09-11-facade-update-ownership/research/review-validation.md
- .trellis/tasks/09-11-facade-update-ownership/research/review.md

### Unrecognized/unrelated dirty paths: excluded from this commit

This is the current dirty-path snapshot outside the candidate set. Directory
entries exclude every descendant; they do not authorize cleanup or staging.

- AGENTS.md
- .trellis/workspace/killbus/index.md
- .trellis/.gitignore
- .trellis/.template-hashes.json
- .trellis/.version
- .trellis/config.yaml
- .trellis/scripts/
- .trellis/spec/backend/database-guidelines.md
- .trellis/spec/backend/directory-structure.md
- .trellis/spec/backend/error-handling.md
- .trellis/spec/backend/logging-guidelines.md
- .trellis/spec/guides/
- .trellis/tasks/00-bootstrap-guidelines/
- .trellis/workspace/killbus/pipeline-freeze-brief-team-b.md
- .trellis/workspace/killbus/research-history-open.md
- .trellis/workspace/killbus/research-pipeline-freeze.md
- .trellis/workspace/killbus/warmup-brief-r1.md
- .trellis/workspace/killbus/warmup-convergence.md
- .zvec-grep/
- log.txt

## Approval and remaining gates

1. Ask for one-shot confirmation of the local commit and its complete file set.
   Recheck the index and source snapshot before staging explicit paths. No amend.
   This checkpoint enables later native validation; it does not waive a test gate.
2. Ask separately before pushing the task branch or dispatching CI. Verify origin
   points to the intended repository. Do not push main, open a PR, publish an image,
   or trigger docker-publish. A possible approved dispatch is:

   gh workflow run ci-test.yml --repo killbus/budva --ref fix/facade-update-ownership

3. Correlate any resulting run with the new commit SHA and retain actual logs and
   exit results. A baseline green or a run on a different ref is not patch evidence.
4. The existing workflow does not itself apply the required negative controls.
   Execute them only on an isolated, authorized validation snapshot/ref, recording
   the exact mutations, behavioral failures, and bounded cleanup, then restore all
   mutations and obtain final full-suite green. Do not silently expand the permanent
   workflow or mutate the user's working source. See research/review-takeover.md.
5. Actual-facade idle-burst/request acceptance still requires separate explicit
   account/session authorization and declared load, successful request data,
   cold-readiness preconditions, latency/error bounds, and exact revision.

This proposal requests only the local commit. Push/dispatch, validation-branch
creation, live-account actions, merge, deployment, and task archival are not
authorized by confirmation of this commit plan.
