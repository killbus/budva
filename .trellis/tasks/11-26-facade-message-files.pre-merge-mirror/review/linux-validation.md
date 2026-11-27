# Native Linux validation

Date: 2026-11-27 (supersedes the 2026-11-26 pre-strengthening run `faee6509e5e667bcfe657d3739b2e2fb24af0fd0142d523bb4d9516bf2cbb5b5`). Executed by the orchestrator against the uncommitted feature-worktree source at baseline `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`.

## Source and environment

- 230 source inputs: manifest SHA256 `16af0463d22c707a438034e7fe4cb4867c0223260f16c4218cc8fd6d53329e3b`; archive SHA256 `fdb13804a739743bfc06621e8165aa808dd447158fbf29d1d182ca9e052ec0aa`. The manifest includes tracked cmd/internal/test/scripts/go.mod/go.sum and all four new `message_files` source/test files. Documentation is outside the execution snapshot.
- Native Debian amd64, Go 1.25.9, `CGO_ENABLED=1`, `GOMAXPROCS=2`, build parallelism 1. Authentic pinned TDLib 1.8.40 static libraries and headers, no SDK mocks/replacements at the linker boundary. See `validation-environment.md` for immutable image/layer and toolchain provenance.
- Isolated directory `/tmp/budva-message-files-aQCDMb`. Per-file SHA256 verification succeeded before execution. No live Telegram account/session, service deployment, system installation, sudo, Docker daemon, or image publication.
- Post-review test-only strengthening (2026-11-27): after independent Reviewer round 1 PASS (`review/findings.json`, SHA256 `45c97bc31cd16e1ad6d83466dda64cc9e480fa4c51d8831dca58561180a7218a`), zero production files changed. Only `internal/app/facade/message_files_test.go` (SHA256 `faa16561f4736fa5898dd700f23ec7be6ae7cd2c5b464eea1f196f759dd8981b`, 16 top-level / 40 total including subtests) was strengthened to add thumbnail + dimension assertions (`Animation.Thumbnail`, `VideoNote.Thumbnail`, `PaidMediaVideo.Video.Thumbnail`, plus `Document.Thumbnail` and `Sticker.Thumbnail` dimensions), `MessageVideo` with `Video: nil` preserving `AlternativeVideos` and `default_file_id=0`, duplicate-ID coverage across both roles in one item and two separate paid items (closing Reviewer's A5 nit), and `TestService_GetMessageFile_PaidMediaUnsupported` (A4 regression coverage without modifying tracked `*_test.go` files). `internal/transport/grpc/message_files_test.go` remains unchanged (`55b3a23c09641010c52edaf23e9d654fae6baaa3128318988ae3531486f9dfef`, 6 top-level / 12 total including subtests).

## Executed checks

| Check | Result | Evidence |
| --- | --- | --- |
| `go test -v -p 1 -count=1 -timeout=120s ./internal/app/facade ./internal/transport/grpc -run 'ListMessageFiles\|PaidMediaUnsupported'` | Exit 0 (app 16 top-level / 40 with subtests; transport 6 top-level / 12 with subtests) | `runtime/focused-final.log` |
| `go vet -p 1 ./...` | Exit 0 | `runtime/vet-final.log` |
| `go test -p 1 -count=1 ./internal/...` | Exit 0 (all 23 testable internal packages pass) | `runtime/suite-final.log` |

The real bufconn photo and alternative-video flows select the returned descriptor, receive payload before releasing upstream completion, validate the chosen bytes/offsets/metadata, and require exactly one terminal complete followed by clean EOF. Listing itself uses only one permitted `GetMessage` in strict app mocks.

## Executed mutation proofs and restoration

All three checks ran sequentially on the disposable Linux source after the full suite succeeded. Temporary changes never reached the feature worktree. Each mutation was restored before the next check; final `sha256sum -c source.sha256` verified all 230 files unchanged (`runtime/restored-source-check.log`).

1. Existing `bash scripts/verify-facade-streaming.sh`: exit 0. Original progressive test passed for `GetMessageFile` and `DownloadFile`; changing the sole starter to `Synchronous:true` failed the named payload-before-completion test with `PROGRESSIVE_PAYLOAD_BEFORE_COMPLETION`; exact restoration passed again. No build/setup/panic/timeout failure was accepted. See `runtime/progressive-final.log`.
2. Selection-flow proof: the same synchronous-starter mutation caused both `TestListMessageFiles_RealBufconnPhotoThenSelectNonDefaultDownload` and `TestListMessageFiles_RealBufconnAlternativeVideoThenSelectDownload` to fail with the causal payload diagnostic (exit 1). Restoring bytes made both pass (exit 0). Starter source SHA256 before/after: `4beed166d2ae1a9399afe2b2e8f0386a4f6bf507c534b45562a28dc56a2a2e78`. See `runtime/selection-mutation-evidence.json`, `runtime/selection-red.jsonl`, and `runtime/selection-restored-green.jsonl`.
3. Enumeration omission proof: insert `result.Items = nil` after the unique real extraction assignment. Both `TestService_ListMessageFiles_PhotoMutationBindsExtraction` and `TestService_ListMessageFiles_PhotoSizesMetadataOnly` failed literal item/default assertions (exit 1), then passed after byte restoration (exit 0). Enumerator SHA256 before/after: `a7fb0c334a020566120711e7751817c39e719952bd184f643541a2a630eb0015`. See `runtime/enumeration-mutation-evidence.json`, `runtime/enumeration-red.log`, and `runtime/enumeration-restored-green.log`.

Runtime scripts, manifest and logs are retained under `review/runtime`. Original source manifest after all mutation checks remains `16af0463d22c707a438034e7fe4cb4867c0223260f16c4218cc8fd6d53329e3b`.

## Limits and next gate

Native validation and independent architecture review (round 1 PASS, zero blocking findings, zero production changes after review) are complete for this exact snapshot. GitHub CI, commit/push, PR #16 update, a new acceptance image, and live Telegram acceptance have not been executed for this follow-up. The historical `dispatch-test` image (`dev-run23-2af222b`) does not contain `ListMessageFiles`.
