# Implement — facade gRPC file download (execution plan)

Active task: `.trellis/tasks/11-25-facade-file-download`
Design ruler: `design.md` (locked at `approval.md` `design_sha256`). Baseline: `fbc22cc`.

## Ordered checklist

1. **Toolchain parity check** (design §10): install `protoc` v34.2 +
   `protoc-gen-go v1.36.11` + `protoc-gen-go-grpc v1.5.1` (see §Environment).
   Regenerate the UNCHANGED baseline proto in the worktree; diff must be empty.
   If not empty: stop, write `review/spec-gap.md` (toolchain parity failure).
2. **Proto edit** (design §5): add the 2 RPCs + 4 messages to
   `internal/transport/grpc/pb/facade.proto`; preserve
   `api/proto/facade.proto` as the existing symlink (no copy or rewrite).
   Regenerate (design §10 command). Verify the proto contract is additive
   and generated output matches scratch regeneration byte-for-byte;
   descriptor/index shifts in `facade.pb.go` are expected.
3. **Infra passthrough** (design §8, G2-revised): `clientAdapter` interface +1
   method (`DownloadFile` only — `GetFile`/`GetFileMimeType` are NOT added, see
   design §1 Revision 2 fact note); `Repo.DownloadFile` is a bare passthrough (no
   `withChatReady`) wrapping with `FloodWaitFromError`.
4. **App layer** (design §6/§7): `errors.go` +2 typed errors; `service.go`:
   `telegramRepo` interface +1 method (`DownloadFile`); `GetMessageFile`
   (resolution table in §1, max-size photo variant [ASSUMPTION], synchronous
   download, success oracle §6.4, MIME from content fields per §6.5);
   `DownloadFile` (same download+oracle core).
5. **App mocks**: add `DownloadFile` to
   `internal/app/facade/mocks/telegram_repo.go` (regenerate if toolchain
   works, else hand-add in exact generated style).
6. **App tests** (design §9): all A1/A2/A3 cases, strict mocks, red-path
   comment per quality-guidelines R3.
7. **Transport** (design §5/§7): `facadeService` interface +2 methods; 2 RPC
   impls (request validation: `file_id` range → `InvalidArgument`; raw→proto
   mapping only); `mapFacadeError` +2 branches (`MessageFileNotFoundError` →
   NotFound, `FileDownloadFailedError` → Internal).
8. **Transport mocks + tests**: hand-add `GetMessageFile`/`DownloadFile` to
   `internal/transport/grpc/mocks/facade_service.go`; new transport tests
   (design §9: validation, error mapping, success mapping).
9. **Docs**: `docs/TEST-MATRIX.md` rows under App / Facade and Transport /
   gRPC with actual test counts; update summary table totals.
10. **Red-path evidence** (design §13): document which strict matcher or
    assertion rejects an incorrect photo selector or success oracle.
    Windows evidence is static reasoning only; any executed mutation test
    must run in a supported Linux environment, restore the implementation,
    and record its actual command and result separately. Do not describe
    predicted failures as observed test executions.
11. **Final local pass**: `gofmt -l` on all changed files (must be empty);
    re-run proto byte-identity on the final proto (additive diff only).
12. **Write `responses.json`** for the current implementation round
    (scope_summary, changes, test_notes incl. red-path evidence; final
    alignment is round 2). Code-review rounds are tracked separately in
    `review/findings.json`. No implementation verdict wording.

## Rollback points

- If a phase must be rolled back, inspect and preserve existing work first;
  restore only that phase's changes. Never discard the entire worktree.
- Reverting published work requires an explicit owner decision and a scoped
  revert commit.

## Validation commands (normative)

```
# local (Windows) — everything that can run here
gofmt -l <changed .go files>                      # expect: empty
# proto byte-identity (design §10) on a scratch copy of the baseline
#   + on the final proto (expect additions-only)

# CI (authoritative; after authorized publication AND a PR targeting main,
# or an explicit workflow_dispatch; a feature-branch push does not trigger it)
gh --repo killbus/budva run list --branch feat/facade-file-download -L 3
# gate: ci-test succeeds for the reviewed PR head (or workflow-dispatch SHA)
```

Do NOT run `go build` / `go vet` / `go test` locally (go-tdlib cgo is
linux/darwin only on this box — design §13).

## Environment (Windows, this box)

- Go 1.25.9 (`go version`), `GOPATH C:\Users\ying\go`.
- `protoc` v34.2: install via `scoop install protobuf` (extras bucket) or
  extract the `protoc-34.2-windows-x86_64.zip` from the protobuf GitHub
  releases into `D:\Apps\protoc` (PATH). Report `protoc --version` =
  `libprotoc 34.2`.
- Plugin versions are pinned by the `Taskfile.yml` `proto` task
  (`protoc-gen-go@v1.36.11`, `protoc-gen-go-grpc@v1.5.1`) — `go install`
  both; the Taskfile task does exactly that, then runs the protoc command.
- git: `origin` push works (Windows credential manager, verified 2026-11-25
  with `git ls-remote origin HEAD`); `gh` CLI is NOT authenticated — use raw
  `git push origin feat/facade-file-download` and read CI through
  `gh --repo killbus/budva run ...` only if a token becomes available,
  otherwise the orchestrator (owner) watches CI.
