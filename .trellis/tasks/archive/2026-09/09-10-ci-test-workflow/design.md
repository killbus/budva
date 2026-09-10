# Design: ci-test workflow

## Decision: TDLib sourcing (the one real decision)

go-tdlib v0.7.6 (default build tags, no `libtdjson` tag) requires STATIC TDLib
libs at the fixed cgo paths `-I/usr/local/include -L/usr/local/lib` and links
`-ltdjson_static -ltdjson_private -ltdclient -ltdcore …` (verified in
`go-tdlib@v0.7.6/client/tdjson_static.go`). Exactly the layout the Dockerfile's
tdlib-builder stage produces. Options:

| Option | Cold run | Warm run | Verdict |
|---|---|---|---|
| a. Test inside `docker build --target builder` | ~20+ min every run (no gha cache — kill list) | same | rejected: rebuilds TDLib every push |
| b. Build TDLib in job, no cache | ~20-25 min every run | same | rejected |
| **c. Build TDLib once, `actions/cache` the installed artifacts** | ~20-25 min once | ~4-6 min | **chosen** |

Cache design:

- Key: `tdlib-22d49d5-amd64` (TDLib commit pin + arch; bump the pin when the
  Dockerfile's `git checkout 22d49d5` changes — one file, one key, greppable).
- Cached object: a zstd tarball of the INSTALLED tree (`include/td/` +
  `lib/libtd*` only — not the ~15GB build dir), path `/tmp/tdlib.tar.zst`.
  actions/cache caches a file path fine; tarball avoids globbing and keeps the
  runner's own `/usr/local/lib` content out of the cache.
- Cold path: clone td at `22d49d5`, build with the Dockerfile's exact recipe
  (`--parallel 2` + `prepare_cross_compiling` + `php SplitSource.php` +
  `install`; OOM lesson documented in docker-ci-guidelines), `make install`
  into `/usr/local`, then tar `/usr/local/{include/td,lib/libtd*}`.
- Warm path: `tar -I zstd -xf` into `/`. No `ldconfig` needed for static
  linking (libs are `.a`, resolved by the linker, not the loader).
- Repo is public → actions/cache quota is not a concern (10 GB repo limit,
  tarball ≈ 0.5-1 GB compressed).

## Decision: everything else

- **Runner**: single `ubuntu-latest` amd64 leg. arm64 adds nothing for vet+test
  (no arch-specific code in repo — verified: only `smoke` build tag exists).
- **Go**: `actions/setup-go@v6` with `go-version-file: go.mod` (pins 1.25.9
  via the toolchain line). setup-go's default module cache (`cache: true`,
  keyed on go.sum) is kept — zero-config, self-healing.
- **Steps**: apt headers (`libssl-dev zlib1g-dev php-cli g++ cmake git make
  ca-certificates` — the Dockerfile's tdlib-builder apt list minus gperf) →
  TDLib cache restore-or-build → `ldconfig` no-op skip → `go vet ./...` →
  `go test ./internal/...`.
- **Scope**: `./internal/...` only. `test/bdd` needs a live Telegram account
  (LiveStack), `test/smoke` is `//go:build smoke` + compose stack, `cmd/` has
  no tests. Plain `go test` (no `-short`): no internal test skips on
  `testing.Short()` (verified by grep — zero hits).
- **Expected first-run reality**: `internal/service/transform`'s
  `TestAddText_ValidMarkdown` calls static `client.ParseTextEntities`, which
  the Windows shadow stubbed into failure. Real TDLib executes it locally
  (no client, no network) — it should pass here; if it does not, that is
  exactly the platform gap this channel exists to surface, and it becomes a
  follow-up task, not a workflow defect.
- **No changes** to docker-publish.yml / Dockerfile / scripts / Go sources.
  `permissions: contents: read`. No concurrency group, no fail-fast matrix —
  nothing to cancel or fan out.

## Risks

- Runner disk during TDLib build: same recipe fits the same runner class in
  docker-publish today; build happens in `/tmp` (outside the workspace, no
  interference with `actions/cache` paths).
- Cache eviction (10 GB LRU): worst case = one cold rebuild. Acceptable.
- `SplitSource.php` drift vs upstream td recipe: recipe is copied verbatim
  from the Dockerfile — if the Dockerfile pin moves, both must move together
  (enforced by the spec note: same commit, one cache key).
