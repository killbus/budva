# Docker / CI Build Guidelines

> Conventions for the Docker image build and the `docker-publish` workflow.

Content below comes from real CI failures (Sept 2026, the docker-publish
task). Every entry was paid for with a dead runner or a red run.

---

## Scenario: Building the budva image in GitHub Actions

### 1. Scope / Trigger

- Trigger: any change to `Dockerfile`, `.github/workflows/docker-publish.yml`,
  or `scripts/docker-smoke.sh`.
- The image builds TDLib (C++) from source with CGO on 16 GB runners.

### 2. Signatures

```dockerfile
# TDLib build — parallelism is capped, NOT full-core (see lesson 1)
cmake --build . --parallel 2 --target prepare_cross_compiling
cmake --build . --parallel 2 --target install
```

```bash
# Executable scripts from a Windows checkout must set the git mode bit
git update-index --chmod=+x scripts/docker-smoke.sh   # verify: git ls-files -s
```

### 3. Contracts

- `VERSION` build arg: declared globally (before first `FROM`) and
  re-declared inside the builder stage; baked via
  `-ldflags "-X main.version=${VERSION:-dev}"`. `${VERSION:-dev}` guards
  against a lost ARG (an empty `-X` wipes the "dev" default to empty string).
- Smoke gate: nothing pullable by name exists until BOTH arch legs' smokes
  pass — build jobs push by digest only
  (`outputs: type=image,push-by-digest=true`); the merge job applies tags.
- Event discrimination is `GITHUB_EVENT_NAME == "push"` (never ref type):
  this workflow's push trigger fires only for `v*` tags; a dispatch ON a
  tag ref must still take the test channel (dispatch never publishes a
  release).

### 4. Validation & Error Matrix

| Symptom | Root cause | Fix |
|---|---|---|
| Runner dies ~87% into TDLib compile, "runner has received a shutdown signal", run ~4-5 min | full-core `cmake --build` (no `--parallel`) OOMs 16 GB VM: heavy template files (`telegram_api*.cpp`) peak 1-2 GB per g++ process | `--parallel 2` (costs ~5 min extra) |
| Build step hangs ~48 min, VM killed, log blobs 404 | buildx gha cache exporter (`mode=max`) uploading GB-scale TDLib layer blobs | remove `cache-from`/`cache-to` entirely |
| `scripts/docker-smoke.sh: Permission denied` in CI | file committed from Windows as `100644` (no exec bit on NTFS) | `git update-index --chmod=+x` + `.gitattributes` `*.sh text eol=lf` |

### 5. Good/Base/Bad Cases

- Good: `--parallel 2`, no gha cache, `100755` in `git ls-files -s` →
  build + smoke + publish all green (~31 min full run, TDLib from scratch).
- Base: local `docker build -t budva:local .` — same Dockerfile, `VERSION`
  defaults to `dev`.
- Bad: re-adding gha cache "to speed things up" — the TDLib layer is only
  ~5 min from scratch; the cache has twice cost 48-minute hangs. Revisit
  only on a real signal.

### 6. Tests Required

- `scripts/docker-smoke.sh <image> <expected-version>` is the contract
  owner: version banner exact-match for facade+engine, facade `/live`
  `/ready` `/healthcheck` all 200, engine logs `Engine started`. CI runs
  it per arch before any push; local builds run the same file.
- Verify exec bit before pushing: `git ls-files -s scripts/` shows `100755`.

### 7. Wrong vs Correct

#### Wrong

```dockerfile
# Full-core parallelism — OOMs the 16GB runner at ~87% of TDLib compile
cmake --build . --target install
```

```yaml
# gha cache — stalled builds twice for ~48 minutes on GB-scale blobs
cache-from: type=gha
cache-to: type=gha,mode=max
```

#### Correct

```dockerfile
# -j2 keeps peak memory in budget; ~5 minutes slower, never OOMs
cmake --build . --parallel 2 --target install
```

```yaml
# No cache keys at all — see the workflow's kill-list comment
```

---

## Scenario: The ci-test workflow (go vet + go test on Linux)

### 1. Scope / Trigger

- Trigger: any change to `.github/workflows/ci-test.yml` or to Go code it
  compiles. Runs on push to `main`, PRs targeting `main`, and
  `workflow_dispatch`.
- The channel that executes `go vet ./...` + `go test ./internal/...` on
  Linux against real TDLib static libraries.

### 2. Signatures

```yaml
# TDLib cache — split restore/save, not the combined action
- uses: actions/cache/restore@v6.1.0
  with: { path: /tmp/tdlib.tar.zst, key: tdlib-22d49d5-amd64 }
# ... build TDLib only on cache miss ...
- uses: actions/cache/save@v6.1.0
  with: { path: /tmp/tdlib.tar.zst, key: tdlib-22d49d5-amd64 }
```

### 3. Contracts

- **Cache key lockstep**: `tdlib-<commit>-amd64` MUST move together with
  the Dockerfile tdlib-builder stage's `git checkout <commit>` — both
  build the same library; when one pin moves, both move and the cache key
  moves with them. `22d49d5` is the current pin.
- **Save placement**: `actions/cache/save` runs right after the tarball
  exists, BEFORE vet/test — the combined action saves only in its
  post-step on green jobs, which would make every red run re-pay the cold
  build. A red run must still populate the cache.
- **Runner-only deviations from the Dockerfile recipe**: `sudo` on the
  cmake install target and the tarball extract (`/usr/local` is
  root-owned on hosted runners; the Dockerfile runs as root and needs
  neither). The build recipe itself (pin, `--parallel 2`,
  `prepare_cross_compiling`, `SplitSource.php`) is verbatim Dockerfile.
- **The tarball stores the INSTALLED tree** (`include/td/` + `lib/libtd*`
  only, ~34 MB zstd) — never the ~15 GB build dir. Tar the tree with
  `cd /usr/local && tar ... lib/libtd*`: the glob expands against the
  shell's cwd, not tar's `-C`.
- **Test scope**: `./internal/...` only — `test/bdd` needs a live
  Telegram account (LiveStack), `test/smoke` is a `//go:build smoke`
  compose stack, `cmd/` holds main packages without tests. No `-short`:
  no internal test gates on `testing.Short()`.

### 4. Validation & Error Matrix

| Condition | Behavior |
|---|---|
| Push to main / PR / dispatch | vet then test; cold TDLib build ~22 min once, then ~44 s per run (cache hit) |
| Cache miss on a red run | tarball still saved (save runs before vet) |
| vet error or failing test | job fails — the gate is real (proven by a deliberate vet error on a scratch branch via dispatch) |
| Dockerfile TDLib pin changes without a cache-key change | go-tdlib compiles against a stale TDLib — key lockstep is a spec contract |

### 5. Good/Base/Bad Cases

- Good: cache hit run = 44 s total; TDLib tarball 34 MB under
  `tdlib-22d49d5-amd64`.
- Base: cold run ~22 min (TDLib from source, `--parallel 2`).
- Bad: switching to the combined `actions/cache` action (red runs lose
  the cold-build investment); tar without `cd /usr/local` first (glob
  expands against the wrong dir); adding an arm64 leg / race detector /
  lint without a signal (kill list).

### 6. Tests Required

- The workflow IS the test runner; its own gate is proven by the
  deliberate-vet-error dispatch (branch `ci-gate-proof`, deleted after).

### 7. Wrong vs Correct

#### Wrong

```yaml
# Combined action: post-job save only on green — red runs re-pay cold builds
- uses: actions/cache@v6.1.0
```

```bash
# Glob expands against cwd (the workspace), matches nothing, fails
tar -I zstd -cf /tmp/tdlib.tar.zst -C /usr/local include/td lib/libtd*
```

#### Correct

```yaml
# restore early, save as soon as the artifact exists
- uses: actions/cache/restore@v6.1.0
# ... build ...
- uses: actions/cache/save@v6.1.0
```

```bash
cd /usr/local && tar -I zstd -cf /tmp/tdlib.tar.zst include/td lib/libtd*
```

---

## Common Mistakes

### Committing shell scripts from Windows

**Symptom**: CI fails with `<script>.sh: Permission denied` at the step
that runs it; the file exists and is well-formed.

**Cause**: NTFS carries no POSIX exec bit; git stores the file as `100644`.
A Linux checkout then cannot execute it directly.

**Fix**: `git update-index --chmod=+x <script>` (mode only — the blob is
unchanged). Also keep `.gitattributes` rule `*.sh text eol=lf` so CRLF
does not corrupt the shebang line.

**Prevention**: after adding any script on Windows, check
`git ls-files -s <script>` shows `100755` before pushing.
