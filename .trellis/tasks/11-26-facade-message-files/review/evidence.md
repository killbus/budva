# Implementation evidence: facade message file enumeration

## Source identity

Baseline: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`. This document records
Implementer-stage checks and repairs. The orchestrator's subsequent exact-source
native results and final mechanical corrections are recorded separately in
`linux-validation.md`; that record supersedes the pending runtime list below.

## Runtime repair origin

The orchestrator ran both new package test sets in a prepared native TDLib
Linux environment (snapshot manifest
`0e10b077959d7f7f4e5850b1e6399bd50eaa927416d30071d9d1db86f8f124c9`). Those
tests compiled but failed, exposing these repair requirements:

- app video fixture emitted four descriptors, not five;
- photo default selection lost because fixture sizes were incoherent;
- per-file prefix/read state leaked across selected file IDs;
- A3 tests did not extract the descriptor from the listing response and did
  not fully assert terminal completion.
This implementer did not execute those tests and makes no independent Linux
pass claim.

## Executed static checks

- `gofmt -l internal/app/facade internal/transport/grpc` produced no output.
- `git diff --check -- internal docs .trellis/tasks/11-26-facade-message-files`
  produced no output after the repair.
- Windows native compile is unavailable because `github.com/zelenin/go-tdlib`
  v0.7.6 is cgo-only. No fake SDK or stub test was used.

## Proto compatibility and hashes

- `api/proto/facade.proto` remains the unchanged relative symlink to
  `../../internal/transport/grpc/pb/facade.proto`.
- Scratch generation used pinned protoc 34.2, protoc-gen-go 1.36.11, and
  protoc-gen-go-grpc 1.5.1 with `paths=source_relative`.
- The generated file header and raw descriptor symbol retain
  `internal/transport/grpc/pb/facade.proto`; all 17 legacy RPCs and stream
  descriptors remain unchanged apart from additive entries.
- SHA256 for this repair turn:
  - `6EF7B1DB23B2F1E59E0439A3C6F17D7A67642B9F74D665775E85A683CA75B810` `internal/transport/grpc/pb/facade.proto`
  - `A38E8D82A5A04DA196CA25878E00E4029B358AC7667FD584A086CB518601DF67` `internal/transport/grpc/pb/facade.pb.go`
  - `47F38D2410F9F71748411CB1448E9F0B87866277782BF619F70A141279B726B8` `internal/transport/grpc/pb/facade_grpc.pb.go`
  - `A7FB0C334A020566120711E7751817C39E719952BD184F643541A2A630EB0015` `internal/app/facade/message_files.go`
  - `24B38E3B070B246AA981D25CEF455CB0DE358E40E859371BF1A38E0FBBD0B12E` `internal/transport/grpc/message_files.go`
  - `7749272F38B286AF54F2F768923E6CEA5E1A43CFFB6398114509F611CE6F7D29` `internal/transport/grpc/transport.go`
  - `97F233D3A2F8888FBC53DCBFB74C5A045573FD35DB266A3F3B8CCC685D0A52F1` `internal/transport/grpc/mocks/facade_service.go`
  - `A1FDA72C35062A986DC5D6F80543223CFB1A8D492ED72C26DDD418D3E94A1403` `internal/app/facade/message_files_test.go`
  - `55B3A23C09641010C52EDAF23E9D654FAE6BAAA3128318988AE3531486F9DFEF` `internal/transport/grpc/message_files_test.go` (after orchestrator mechanical correction)
  - `2DE9C26DC8F9E504FD0D4CA6E7100D00D2ED26766E19C2EB5AD741E71E5CBC42` `docs/TEST-MATRIX.md`

## Test inventory and strict behavior

Counts are top-level `Test` functions, with table subtests noted separately.
They are a source inventory, not runtime pass/fail claims.

- `internal/app/facade/message_files_test.go`: 15 top-level tests, 36 total
  including table subtests.
- `internal/transport/grpc/message_files_test.go`: 6 top-level tests, 12 total
  including table subtests; two tests exercise real bufconn A3 flows for
  nondefault photo and alternative-video selection.
- Changed package totals: `internal/app/facade` 51 top-level tests,
  `internal/transport/grpc` 71 top-level tests.

A3 repair behavior now binds:

1. the intended descriptor is selected from the actual ListMessageFiles
   response, with role/source checked and non-default identity asserted;
2. the selected descriptor ID is passed to DownloadFile;
3. metadata identity/size precedes the first chunk;
4. the first chunk has offset zero, expected payload, and
   `upstream_complete=false` while the completion barrier remains closed;
5. the download gate starts closed; it is opened only immediately before the
   explicit DownloadFile RPC, and every DownloadFile/GetFile/prefix/read call
   returns an error when the gate is closed, proving forbidden listing-side
   calls without asynchronous phase ordering assumptions;
6. fake prefix/read state is per selected file and models the current
   incomplete prefix or completed full location;
7. snapshot active/complete state is taken under mutex, and synchronous
   requests wait for the release barrier before re-reading and returning the
   current active/complete state;
8. the receive loop requires nonempty chunk data and exact offsets, and rejects
   metadata, unexpected events, and any event after terminal completion;
9. each stream receives exactly one terminal complete event with exact size;
   the assertion loop then requires clean EOF and rejects any event after
   completion;
10. completion releases only after the client receives first payload.

## Mutation proof scope

The four previously listed recipes were invalid because the relevant source
files are untracked, the duplicate paid-slot recipe would not compile, and the
A3 completion recipe was not runnable as written. They have been removed.

For design section 9, Linux remains the execution point for exactly one
meaningful enumeration omission/flattening mutation using byte backup and a
trap-based restore, plus the existing streaming mutation script. No extra
mutation recipes are claimed here.

## Pending runtime gates

Required on Linux/native TDLib against this repair head, owned by the
orchestrator:

1. `gofmt -l .` remains clean.
2. `go vet ./...`.
3. `go test ./internal/...` (or the prepared isolated native equivalent).
4. `bash scripts/verify-facade-streaming.sh`.
5. Execute the one enumeration mutation proof with backup/restore and record
   exact red output.
6. Fresh acceptance source/image flow per `acceptance/message-files.md`;
   historical image `dev-run23-2af222b` does not contain this endpoint.

## Final closeout addendum (2026-11-27)

- Independent architecture review round 1 returned `PASS` (`done: true`, 0 blocking findings) in `review/findings.json` (SHA256 `45c97bc31cd16e1ad6d83466dda64cc9e480fa4c51d8831dca58561180a7218a`).
- Post-review test-only strengthening updated only `internal/app/facade/message_files_test.go` (SHA256 `FAA16561F4736FA5898DD700F23EC7BE6AE7CD2C5B464EEA1F196F759DD8981B`) and `docs/TEST-MATRIX.md` (SHA256 `BF7178B13538A2CF13088EE5F9D56438C22BD659878DEBE7E766DE3CDD3B0069`); zero production or mock files changed after review PASS.
- Updated source test inventory: `internal/app/facade/message_files_test.go` has 16 top-level tests (40 total including subtests), `internal/app/facade` totals 52 top-level tests, `internal/transport/grpc` remains 71 top-level tests (6 top-level / 12 including subtests in `message_files_test.go`), and the 21 displayed `docs/TEST-MATRIX.md` rows sum to 551.
- Fresh native Linux validation passed all checks (`focused-final.log`, `vet-final.log`, `suite-final.log`, `progressive-final.log`, selection mutation, enumeration mutation, and post-mutation 230-file checksum verification) against exact manifest `16af0463d22c707a438034e7fe4cb4867c0223260f16c4218cc8fd6d53329e3b`; see `review/linux-validation.md`.
- Out-of-scope / awaiting owner authorization: commit, push, PR #16 update, GitHub CI run, new `dispatch-test` image build/publish, and live Telegram acceptance.
