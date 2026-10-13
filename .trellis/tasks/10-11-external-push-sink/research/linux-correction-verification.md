# P10-P12 Linux Correction Verification

## Scope And Status (2026-10-12)

The user authorized real Linux validation after the Windows handler cgo build
limitation. P10/P11 and AC10/AC11 pass. P12's isolated runtime gate is complete;
AC8's new GitHub CI requirement and replacement publication remain pending.
No commit, push, image, PR, merge, production access or task archive occurred
during validation. After subsequent commit-plan confirmation, corrective code
and specs were committed locally as `4243ddf8cffc70ef50420bd196f56f83ced2cbea`;
the second approved commit records this evidence. No push or publication occurred.
This does not implement the linked rich-content/media protocol planning task.

## Reproducible Environment

- Host alias: `debian`; new isolated root:
  `/home/ying/budva-push-validation-20261011/correction-20261012-2iYCAb`.
- Go: `go1.25.9 linux/amd64`; real gcc/cgo, `CGO_ENABLED=1`, `libtdjson` tag.
- Existing dependency root: `/home/ying/budva-push-validation-20261011/linux-deps`.
  No substituted TDLib stubs and no production configuration or session state.
- Real `libtdjson.so` SHA256:
  `11fa9154f9b0146b4c8e7abccb316b3117de28d570ccae8faa277c55189ddc98`.
- `include/td/telegram/td_json_client.h` SHA256:
  `cdf6190b0264417c69c108d192d1f3381f3a1315ddb309798dbefc5036be1eb3`.
- The existing C ABI header is pinned to TDLib commit
  `c15d3f5a5de6e3ba5839822c451152e5e18bb700`.
- `linux-correction.tgz` contains 157 source files, required embed/testdata and
  validation scripts, not credentials. Bundle SHA256:
  `8de3c0aa4a1fc11e529d0782995fe92ed6d41ba0652df0806018c6a839c97458`.
- Current `internal/app/handler/service.go` SHA256:
  `94d2c0fba6b76d5f8e3e55549ef9f0dc87c24d2dd0cd8fc34bc497c5b038f573`.
  All 157 entries in `correction-files.sha256` match before mutations, after
  restoration, and after all green commands. Existing Telegram tests unchanged.

## Behavioral Reds

`linux_correction_mutation.py` mutates only the isolated handler copy and
restores its original bytes in `finally` after each run. Every case exits 1,
contains the named test failure, and has no `[build failed]` marker. These are
post-implementation mutation proofs, not a claim that handler reds were run
before local implementation. The restored source is then tested green.

| Mutation | Named test | Observed behavioral failure |
| --- | --- | --- |
| Reinstate no-copy early return | `TestPush_EditWithoutCopies` | Expected one event, got zero; strict text expectations unmet |
| Resolve recipients only from copies | `TestPush_SourceEditRoutingIndependentOfCopies/mapped_and_unmapped_rules` | Recipient elements differ |
| Omit album dispatcher hook | `TestPush_AlbumPipelineInvariant/forward/success` | Expected accepted submission, got nil; member text/filter expectations unmet |
| Timestamp album after collection | `TestPush_AlbumNewDoesNotSupersedeEditsOrDeletes` | Shared first-ingress timestamp equality fails |
| Replace quiet-period wait with zero | `TestPush_AlbumPipelineInvariant/forward/success` | Unexpected third `LastReceivedAge` call before the virtual deadline |

Exact failures and restored source hash: `linux-correction-evidence/*-red.log`
and `linux-correction-evidence/mutation-results.json`.

## Green Commands And Results

Run from the isolated root; `linux_correction_validate.sh` sets the real library
and header paths, shared Go caches, and source working directory.

```sh
sh linux_correction_validate.sh test -tags libtdjson -p 2 -count=1 -json ./internal/app/handler -run TestPush
python3 linux_correction_mutation.py
sha256sum -c correction-files.sha256
sh linux_correction_validate.sh test -tags libtdjson -p 2 -count=1 -json ./internal/...
sh linux_correction_validate.sh test -tags libtdjson -race -p 2 -count=1 -json ./internal/service/push/... ./internal/app/handler ./internal/infra/ruleset
sh linux_correction_validate.sh vet -tags libtdjson -p 2 ./...
sh linux_correction_validate.sh build -tags libtdjson -p 2 ./...
sh linux_correction_validate.sh build -tags libtdjson -p 2 -o "$PWD/engine" ./cmd/engine
sha256sum -c correction-files.sha256
```

| Green scope | Tested packages | Top-level tests | Pass records including subtests |
| --- | ---: | ---: | ---: |
| Handler `TestPush` | 1 | 14 | 45 |
| All internal packages | 22 | 509 | 853 |
| Push, handler, ruleset race | 3 | 140 | 211 |

Packages with no tests are excluded from tested-package counts; top-level test
counts exclude names containing `/`. All green commands exit 0. No failed test
actions, race reports or stderr output occur. Whole-repository vet/build and
engine build also exit 0; their logs are empty on success.

Engine SHA256:
`b0c90dc35dfd6bb131bc73a886b3d99793b6d7e1b09d67b76f50b3ff6420affa`.
The binary remains in the isolated validation root; it is not deployed.

## Evidence And Remaining Gates

Raw JSON test streams, stderr, five red logs, mutation results, final source
checks and vet/build logs are saved in `linux-correction-evidence/`.
Downloaded `validation-evidence.tgz` SHA256:
`5ed29bf3b471a37e416ee07dddba7652702086994de69dbbb09f0ea41d8bacf7`.

Raw evidence is committed without whitespace cleanup: the downloaded source
manifest retains CRLF and Go assertion logs contain trailing tabs/spaces.
The local top-level manifest is normalized to LF; hash entries are unchanged.
The commit gate checks authored files and excludes only the raw evidence
directory from whitespace validation to preserve its original bytes.

This run is not GitHub CI or actual-account facade acceptance. The historical
`ghcr.io/killbus/budva:dispatch-test` / `dev-run12-d58909f` image is unchanged.
After subsequent push authorization, both approved commits were pushed and
`ci-test.yml` was dispatched at `2026-10-12T06:31:08Z` on
`6d7daa750fb708b86c83b12f5d5c19bc10f93eb6`:
https://github.com/killbus/budva/actions/runs/38212912036
Observed status: `queued`; the user requested no waiting. The isolated evidence
above does not substitute for this run's result. Require green GitHub CI before
deciding a replacement test image, then record its immutable digest and embedded
version. No Docker workflow was dispatched. Main merge and production deployment
remain separate gates.
