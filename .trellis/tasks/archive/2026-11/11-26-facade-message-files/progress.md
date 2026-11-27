# Planning progress

## 2026-11-26

- Owner requested entering design after confirming one message may expose multiple file IDs.
- Created a separate follow-up task; previous streaming task/design and published image remain the baseline, not rewritten requirements.
- Verified pinned SDK direct file fields, paid-media cases, album semantics, and existing application/transport/repository boundaries.
- Drafted PRD, design, execution plan and curated context manifests. Proposed schema/scope policies are explicitly assumptions for review.
- Manual Trellis document fallback: task.py/brainstorm unavailable; no task activation or hook execution claimed. Current status planning; source untouched.
- Dispatched configured Paseo Design Reviewer (codex/remote-magpie/auto/qwen3.8-27b, auto-review, xhigh), agent 05673b68-3109-4b91-89db-94c74991964d, on the feature workspace against source 2af222b. Only its new review/design-findings.json is writable.
- Mechanically checked the proposed proto in a runtime scratch directory with libprotoc 34.2: syntax valid, all 17 baseline RPC declarations intact, 18 total with the new unary endpoint. Production proto unchanged. Both context manifests resolve against the feature workspace. See review/draft-validation.md.
- Independent Design Reviewer completed round 1: PASS, done=true, D1-D8 met, zero blocking findings. SDK fields, baseline resolver behavior, proto symbols, module boundaries and A1-A7 coverage were independently checked. No design revision required.
- Recomputed design and PRD SHA256 in both planning mirror and feature workspace; all match the reviewed draft. Validated the actual findings artifact, not only the notification. See review/design-gate.md for the review record and remaining owner-facing assumptions.
- Design deliverable is complete as a reviewed proposal. Task remains planning; implementation is not activated or dispatched. Source, production proto, image, PR16 and old task records remain unchanged by this follow-up. Next stage requires the owner's implementation decision on this concrete design.

## Implementation activation

- Owner replied "确认" to the reviewed proposal. Approved assumptions and scope are now the implementation ruler; no design/PRD edits were made.
- Recorded approval.md and activated task status in_progress manually (task.py unavailable). Existing feature worktree at 2af222b is the integration baseline; no branch or planning commit is created. Preserve all prior dirty 11-25 task records.
- Main session dispatches the configured Implementer as the trellis-implement sub-agent under workflow.md section 2.1, with explicit child-side context loading and no recursive implement/check dispatch. Independent Reviewer follows after source completion.
- Local environment has no Docker command or usable installed WSL distribution; go-tdlib native checks still require Linux. Validation feasibility will be checked separately while implementation proceeds. Do not claim static checks as executed Linux tests.
- Dispatched Implementer agent 5349c03c-47ca-40e8-ba15-07a1a05ae205 (configured codex/remote-magpie/auto/glm-5.3-flash, auto-review, xhigh) with immutable rulers, controlled file set, explicit context loading, no commits/publication and no recursive sub-agent dispatch.
- Read-only Linux feasibility check: debian is reachable via the existing trusted SSH configuration outside the sandbox. Go is 1.21.4 at /usr/local/go/bin/go (project requires 1.25.9); TDLib headers/static libraries are not installed at the project's expected paths. Docker exists, but the account cannot access its socket and sudo -n requires a password. No remote files/packages/containers were changed. Existing Linux CI remains the native vet/test gate.

## Implementation preflight and native validation preparation

- Implementer's first notification was incomplete. Source/tests and responses.json exist, but evidence/acceptance documents were absent despite being listed as changes. Main session requested explicit closeout, without treating that notification as completion.
- Preflight found invalid multi-value helper calls, an int/int32 mismatch, unused test import, an incorrect raw-error identity assertion after gRPC status mapping, and a cancellation fixture canceled before its claimed after-return point. The initial bufconn test began with upstream already complete and lacked the alternative-video case. These are concrete implementation/test corrections under the existing design; Implementer is repairing them. TEST-MATRIX arithmetic also needs correction.
- Prepared an isolated Linux validation directory `/tmp/budva-message-files-aQCDMb` on the existing debian host without sudo/Docker/system installation. Real TDLib static libraries came from the already verified published amd64 manifest; Go 1.25.9 was downloaded and checksum-verified. Pinned native headers plus CMake-generated visibility macros complete the build inputs. Details in review/validation-environment.md.
- Baseline source archive (published 2af222b) was copied solely to prewarm dependencies and verify native link viability while implementation continues. Its no-new-tests run must not count as ListMessageFiles acceptance. Later tests must use a hash-verified snapshot of the changed source.
- Isolated Linux environment checks completed: real TDLib static link and local getOption(version) probe returned 1.8.40; baseline facade/grpc packages compiled and linked (no ListMessageFiles tests existed in that baseline). New implementation execution still pending Implementer repair completion.
- Orchestrator's descriptor compatibility check succeeded: 17 legacy RPCs, 38 legacy messages and 1 legacy enum unchanged by name/definition; exactly one unary RPC, four messages and three enums added. Descriptor hashes and corrected declaration-order comparison are recorded in review/preflight-evidence.md. This is separate from runtime validation and independent architecture review.

## First implementation runtime check

- Uploaded and verified all 230 source inputs, manifest SHA256 `0e10b077959d7f7f4e5850b1e6399bd50eaa927416d30071d9d1db86f8f124c9`, archive SHA256 `581993a68f80cd33c4205766005de0c6cdb252b4841e29ef7fad8c2b0a281408`.
- Actual Linux `go test -p 1 -count=1 -timeout=120s ./internal/app/facade ./internal/transport/grpc -run ListMessageFiles` compiled/linked but failed three tests: app VideoAlternativesAndHLS expected five descriptors from a four-file fixture; bufconn photo default expected 103 but file 101 had largest Size; both bufconn downloads received Internal after release because the fake prefix summed all files' payload lengths.
- Returned concrete repairs to the same Implementer. Also required selection from actual listing descriptors, complete/EOF assertions for both cases, per-file prefix bounds, mutex-safe fake snapshots, and accurate mutation instructions/evidence. No production behavior change is inferred from these fixture failures. Full suite and independent code review remain pending the corrected source snapshot.
- Second native snapshot `1a70d33ac153c2b3850f6ad6f765df1d1bfc1722eb0d1dd041df8344f1963e19`: all focused app tests passed, but transport tests failed to compile at two int64-to-int32 helper arguments. Inspection also found an unrecorded list phase, a prefix-offset calculation error, absent barrier cleanup, and incomplete terminal-event ordering assertions. Sent precise test-only fixes to the Implementer, plus the stale TEST-MATRIX top-table count and acceptance-guide artifact wording. No full-runtime or architecture PASS claimed.

## Final source validation and independent review

- Implementer's last closeout still placed a defer before its repo declaration, compared int64 against int32 via assert.Equal, and discarded the appended receive-buffer length inside a helper. Orchestrator corrected those three mechanical test issues directly and required nonempty first chunks/terminal completion. No production code or locked ruler changed. Final new transport test SHA256 `55b3a23c09641010c52edaf23e9d654fae6baaa3128318988ae3531486f9dfef`.
- Final 230-file source manifest `faee6509e5e667bcfe657d3739b2e2fb24af0fd0142d523bb4d9516bf2cbb5b5`, archive `d0cfb5ab14637bdaf143cb11175fe88e1e711deef8d915dc9a05b8454fb4c3fa`. All inputs verified remotely before native execution. Focused app/transport ListMessageFiles tests passed, then `go vet -p 1 ./...` and `go test -p 1 -count=1 ./internal/...` both exited 0.
- Dispatched configured independent Reviewer agent `542fe521-65ff-434d-b9e8-8cd8bdaa8bff` (codex/remote-magpie/auto/deepseek-v4.1-flash, auto-review, high), reading all changed and untracked files plus production call chain against the locked design. Only review/findings.json is writable. Source frozen during review; main continues runtime and documentation evidence.
- Recounted changed packages: app facade 51 and gRPC 71 top-level tests. Corrected stale bottom app row and arithmetic: the 21 displayed TEST-MATRIX inventory rows total 550, not 551; this is not a whole-repo executed test count.
- Existing progressive green-red-green script passed. Both new A3 selection flows also failed correctly for a synchronous-starter mutant and passed after restoration. Enumeration omission failed both named tests and passed after restoration. Full 230-file manifest verified unchanged after all mutations. Runtime logs/scripts and manifest are recorded under review/runtime; see review/linux-validation.md. Await independent Reviewer result; publication/live acceptance remain separate.

## 2026-11-27

- Independent Reviewer agent `542fe521-65ff-434d-b9e8-8cd8bdaa8bff` completed round 1: `PASS`, `done=true`, 0 blocking findings (`review/findings.json`, SHA256 `45c97bc31cd16e1ad6d83466dda64cc9e480fa4c51d8831dca58561180a7218a`).
- Post-review audit strengthened `internal/app/facade/message_files_test.go` only (zero production or mock edits): added thumbnail + dimension assertions (`Animation`, `VideoNote`, `PaidMediaVideo`, plus `Document`/`Sticker` dimensions), `MessageVideo` with `Video: nil` preserving `AlternativeVideos` and `default_file_id=0`, split duplicate-ID coverage across both roles in one item and two separate paid items (closing Reviewer's A5 nit), and added `TestService_GetMessageFile_PaidMediaUnsupported` (A4 regression coverage). Final app test SHA256: `faa16561f4736fa5898dd700f23ec7be6ae7cd2c5b464eea1f196f759dd8981b` (16 top-level / 40 including subtests).
- Re-ran full native Linux validation in `/tmp/budva-message-files-aQCDMb` on exact 230-file manifest `16af0463d22c707a438034e7fe4cb4867c0223260f16c4218cc8fd6d53329e3b` (archive `fdb13804a739743bfc06621e8165aa808dd447158fbf29d1d182ca9e052ec0aa`): focused app/transport tests, `go vet -p 1 ./...`, `go test -p 1 -count=1 ./internal/...` (all 23 testable packages), `bash scripts/verify-facade-streaming.sh`, selection-flow synchronous mutation red/green, enumeration omission mutation red/green, and post-mutation 230-file checksum check all passed. All logs pulled into `review/runtime/`.
- Updated `docs/TEST-MATRIX.md` (`internal/app/facade` 52, `internal/transport/grpc` 71, 21 displayed inventory rows sum 551, manifest `16af0463...`) and synced all Trellis closeout artifacts.
- Delivery is ready for owner review. Commit, push, PR #16 update, GitHub CI run, new `dispatch-test` image build, and live Telegram acceptance remain explicitly awaiting owner confirmation.
- Owner live Telegram acceptance PASS on 2026-11-27 against dispatch-test
  dev-run24-d52d683 (source d52d683c81cbc34c2a85aeda332a1ce281a460cf, index
  digest sha256:73f962f8ce570152c2a0b6dd4e3e9172d5df0d96bb35636ee58fd847d04f7278):
  ListMessageFiles -> descriptor selection -> DownloadFile verified on real Telegram.
- Delivery commit d52d683c81cbc34c2a85aeda332a1ce281a460cf pushed to
  origin/feat/facade-file-download (PR #16); PR title/body updated for the
  final scope. Merge remains the owner's explicit gate.

## Archive (2026-11-27)

Merged to main via PR #16 squash commit 6313eb32c26339ca8ded28dc6c9d73fbeff72fbf;
task status completed and records archived under .trellis/tasks/archive/2026-11/.
Release tag v0.6.0 follows this close-out commit.
