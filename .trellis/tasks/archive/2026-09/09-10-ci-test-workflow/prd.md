# CI: go vet + go test workflow on push to main

## Goal

Add a GitHub Actions workflow that runs `go vet ./...` and `go test ./internal/...`
on every push to `main`, closing the platform-verification gap: Windows dev
machines cannot build go-tdlib (cgo-only, linux/darwin build tags only), so the
spec-required vet+test validation (`.trellis/spec/backend/telegram-tdlib-guidelines.md`
§6) currently has no CI channel.

## Background / Problem

- The repo's only workflow, `docker-publish.yml`, triggers on `v*` tags and
  `workflow_dispatch` only — a push to main runs nothing.
- Its pipeline only compiles (`go build` inside the Docker image); it never runs
  `go vet` or `go test`.
- The warmup-lazy-loadchats task (commit 070e2fe) explicitly deferred
  "go vet ./... + go test in CI/Linux before merge" — this task delivers that
  missing channel.
- Hard constraint: compiling any package importing `internal/infra/telegram`
  (→ go-tdlib v0.7.6, cgo-only) requires the TDLib C++ static libraries
  installed at `/usr/local/include` + `/usr/local/lib` (exactly where the
  Dockerfile's tdlib-builder stage installs them) plus `libssl-dev`/`zlib1g-dev`.
  A bare runner cannot compile the module graph.

## Requirements

1. New workflow file `.github/workflows/ci-test.yml` (name: `ci-test`).
2. Triggers: `push` to `main` (branches filter) + `pull_request` targeting
   `main`. `workflow_dispatch` for manual runs.
3. Single job, single runner (ubuntu-latest, amd64 only — the product binaries
   are amd64+arm64 but vet+test only needs one Linux platform; multi-arch test
   coverage would double CI cost for no new information).
4. TDLib availability: prebuilt TDLib artifact for cgo. Options (to be settled
   in design):
   - a. Run tests inside the already-built budva Docker image (reuse the
     Dockerfile's tdlib-builder stage as the test environment);
   - b. Build TDLib from source in the job (~5 min with `--parallel 2`);
   - c. Prebuilt TDLib tarball cached in the Actions cache.
   Constraint for all options: TDLib commit pinned at `22d49d5` (same as
   Dockerfile), matching go-tdlib's expected install layout
   (`/usr/local/include/td/…`, `/usr/local/lib/libtd*`).
5. Test scope: `go test ./internal/...` only. `test/bdd/**` requires a real
   Telegram account (LiveStack) — out of scope. `test/smoke` has a `smoke`
   build tag and requires docker-compose stack — out of scope.
   `cmd/**` is main-packages with no tests.
6. `go vet ./...` must pass before tests run.
7. Go version pinned to the go.mod toolchain: `go 1.25.9`.
8. Zero changes to existing workflows, Dockerfile, or scripts — this workflow
   is purely additive.

## Acceptance Criteria

- [ ] Push to main triggers `ci-test` and it runs green on the current HEAD
      (warmup commits 070e2fe..0aea49b already on origin/main).
- [ ] The workflow runs `go vet ./...` and `go test ./internal/...` and both
      pass on a clean run (cold cache).
- [ ] A deliberately broken commit (e.g. vet error or failing test) run via
      workflow_dispatch on a scratch branch fails the workflow — proves the
      channel actually gates.
- [ ] No changes to `docker-publish.yml`, `Dockerfile`, `scripts/`, or any
      Go source files.
- [ ] `git ls-files -s .github/workflows/ci-test.yml` — not applicable (YAML,
      no exec bit), but the file must be committed with LF endings and no BOM.
- [ ] Spec updated: `.trellis/spec/backend/docker-ci-guidelines.md` gains a
      section on the ci-test workflow contract (trigger, TDLib sourcing
      decision, test scope, kill list).

## Out of scope

- Coverage reports, race detector, lint (golangci-lint) — no signal yet for
  adding cost.
- BDD (test/bdd) and smoke (test/smoke) integration suites.
- arm64 test runner.
- Any change to make CI blocking on PRs (branch protection rules are a repo
  settings action, not a file).
