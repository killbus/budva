# Implementation Plan: ci-test workflow

## Preconditions

- [ ] Task status in_progress (`task.py start`)
- [ ] Working tree: only `.trellis` + AGENTS.md/.claude local noise on top of
      origin/main (0aea49b)

## Steps

### 1. Write `.github/workflows/ci-test.yml`

- [ ] `name: ci-test`; `on: push: branches: [main]`, `pull_request: branches:
      [main]`, `workflow_dispatch`
- [ ] `permissions: contents: read`
- [ ] Single job `test` on `ubuntu-latest`
- [ ] Steps: checkout → setup-go (`go-version-file: go.mod`, `cache: true`) →
      apt install (libssl-dev zlib1g-dev php-cli g++ cmake git make
      ca-certificates) → TDLib restore-or-build with actions/cache
      (`tdlib-22d49d5-amd64`, tarball `/tmp/tdlib.tar.zst`) →
      `go vet ./...` → `go test ./internal/...`
- [ ] TDLib build recipe copied verbatim from Dockerfile lines 23-27
      (`--parallel 2`, `prepare_cross_compiling`, `SplitSource.php`, install)
- [ ] LF endings, no BOM (Write tool on Windows produces LF for `\n` content)
- [ ] Style: comment density/narrative comments matching docker-publish.yml

### 2. Local validation (no CI run possible yet)

- [ ] `actionlint` if available, else YAML parse check (python yaml.safe_load)
- [ ] Eyeball diff against docker-publish.yml for style consistency

### 3. CI validation (the only true validation)

- [ ] Commit + push to main → watch run:
      `gh run watch -R killbus/budva` (first run is the ~20-25 min cold build)
- [ ] Green: `go vet ./...` passes, `go test ./internal/...` passes (watch
      `internal/service/transform` — see design.md "Expected first-run reality")
- [ ] Second run (any trigger) hits the TDLib cache: job time drops to
      ~4-6 min → proves the cache contract
- [ ] Gate proof: push a scratch branch with a deliberate vet error, run via
      workflow_dispatch, confirm the job FAILS (proves the channel gates),
      delete branch (this proves AC #3; the failing commit never touches main)

### 4. Spec update

- [ ] `.trellis/spec/backend/docker-ci-guidelines.md`: add ci-test section —
      trigger contract, TDLib cache key contract (`tdlib-<commit>-amd64` must
      move with the Dockerfile pin), test scope (`./internal/...` only, and
      why bdd/smoke are excluded), kill list (no arm64, no race, no lint,
      no `docker build --target builder`)

### 5. Wrap-up

- [ ] trellis-check dispatch (or inline self-check)
- [ ] safe_commit via `python ./.trellis/scripts/safe_commit.py` with narrow
      pathspec (`.github/workflows/ci-test.yml` + spec + task artifacts)
- [ ] Archive task, journal

## Rollback

- Single new file + spec/task additions; `git revert` of the workflow commit
  restores the previous state. No shared state outside the repo (the Actions
  cache entry is inert if the workflow is gone).

## Validation commands

```
python -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci-test.yml',encoding='utf-8'))"
gh run list -R killbus/budva --workflow ci-test.yml --limit 5
gh run watch -R killbus/budva <run-id>
```
