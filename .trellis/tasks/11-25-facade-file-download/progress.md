# Progress — facade gRPC file download (GetMessageFile + DownloadFile)

## Loop goal

给 facade gRPC 面补齐"下载文件"能力:增量新增 `GetMessageFile`(消息 → 下载后
本地路径)与 `DownloadFile`(file_id → 本地路径)两个 RPC;走 Design Reviewer
设计关卡(approval.md 锁 design_sha256)、Implementer 实现、独立 Code Reviewer
审查。不传字节流、不做进度推送、不改既有 RPC。

## Controller

- mode: state-only; state file: this file
- single writer: one Implementer at a time
- checker: separate Paseo agent (Reviewer profile), sees diff + gates
- worktree: `D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download`
  (branch `feat/facade-file-download`, base `fbc22cc`)
- paseo workspace: `wks_f2273f16466b0ea3`
- environment: Windows box. go-tdlib (cgo, linux/darwin only) cannot be built
  or tested here — NO local `go build/vet/test` on touched packages.
  Authoritative verification = GitHub Actions `ci-test.yml` via a PR to main
  (branch push alone does not trigger it: on: push[main] + pull_request[main]
  + workflow_dispatch). `gh` CLI unauthenticated; `git push origin` works.
  debian Linux host unreachable (2026-11-25).

## Phase plan

```xml
<phases>
  <phase id="P0" owner="main" files=".trellis/tasks/11-25-facade-file-download/**">
    <goal>Worktree + planning artifacts (prd/design/implement/research/jsonl) prepared.</goal>
    <gate>artifacts present on branch; design.md lockable; records committed at P4.</gate>
  </phase>
  <phase id="P1" owner="design-reviewer" files="review/design-findings.json">
    <goal>Design gate: is design.md lockable as the single ruler (D1-D8)?</goal>
    <gate>verdict PASS, D1-D8 met, no blocking finding -> approval.md (design_sha256).</gate>
  </phase>
  <phase id="P2" owner="implementer" files="proto, internal/**, docs">
    <goal>Implement design.md §5-§9 + §11 within its non-goals.</goal>
    <gate>gofmt clean; proto byte-identity (additive contract); responses.json current.</gate>
  </phase>
  <phase id="P3" owner="reviewer" files="review/findings.json">
    <goal>Independent architecture/contract review of git diff fbc22cc.</goal>
    <gate>verdict PASS, no blocking finding.</gate>
  </phase>
  <phase id="P4" owner="main" files="task artifacts + branch">
    <goal>Present commit plan; commit after confirmation; publish and validate when authorized.</goal>
    <gate>commit approval; branch publication + PR/CI evidence; merge only on explicit go-ahead.</gate>
  </phase>
</phases>
```

## Status

| Phase | Owner | Gate | Status |
| --- | --- | --- | --- |
| P0 setup | main | worktree + artifacts | ✅ artifacts included in the approved P4 task-record commit |
| P1 design gate | design-reviewer | D1-D8 met, PASS | ✅ round 4 PASS (agent 36594bbf) |
| P2 implement | implementer | scoped local checks + responses.json | ✅ round 2 aligned; fixture correction verified |
| P3 independent review | reviewer | no blocking finding | ✅ round 1 PASS (agent aa54f5bf) |
| P4 closeout | main | commit approval + publication + Linux CI | 🔵 owner approved separate CI repair commit; implementation/review underway |

## Evidence
- P0: task dir .trellis/tasks/11-25-facade-file-download; artifacts prd.md, design.md,
  implement.md, research/tdlib-file-download-api.md, implement.jsonl, check.jsonl,
  task.json, progress.md; loop prompts review/{design-reviewer,implementer,reviewer}-prompt.md + brief.md.
  Worktree @ fbc22cc (branch feat/facade-file-download, wks_f2273f16466b0ea3). Task dir
  intentionally untracked until P4 (11-05 commit-boundary convention: task records
  commit at closeout).
- P0 proto toolchain parity: PROVEN — protoc libprotoc 34.2 (installed at
  .trellis/.runtime/protoc/bin) + protoc-gen-go v1.36.11 + protoc-gen-go-grpc
  v1.5.1 (go/bin) regenerating the UNCHANGED baseline proto reproduced
  facade.pb.go + facade_grpc.pb.go byte-identically (hash-compared).
- P0 environment: go-tdlib unbuildable on Windows (cgo linux/darwin only,
  verified: 'undefined: JsonClient'); no Docker/WSL; debian host unreachable
  (192.168.216.3:22 timeout). CI ci-test.yml via PR to main = authoritative gate.
  git push origin works (credential manager verified); gh CLI unauthenticated.
- P1: Design Reviewer agent 36594bbf started 2026-11-25 (profile: codex/
  remote-magpie/auto/qwen3.8-27b, auto-review, xhigh).

- P1 r1: Design Reviewer verdict=FAIL, single blocking G1 (D6): prd R1 said
  "optional variant selection" but design only implements the default (largest)
  variant. Orchestrator resolution: PRD is the intent source and the user
  requirement was download capability only - variant selection is YAGNI. prd.md
  updated (R1 narrowed to chat_id+message_id; variant selection moved to
  Non-goals). Design Reviser agent 270e40fa dispatched to align design.md
  (expect: new N8 non-goal + wording alignment, minimal diff). Reviewer
  36594bbf retained for round 2 (findings.json round=1, FAIL).
- P1 r1.5: Design Reviser (270e40fa) finished — design.md aligned to option (b):
  new N8 non-goal + §1/§3/§5/§6/§9/§11 wording alignment; only design.md
  touched (orchestrator spot-check verified). review/revision-notes.md written.
  Design Reviewer (36594bbf) re-dispatched for round 2 (G1-only verification).
- P1 r2: Design Reviewer verdict=PASS (D1-D8 met, G1 resolved, no new violations).
  Design locked: review/approval.md, design_sha256=
  653f4a18122e3bd6acc95ddcad5119290cdb89de3d9ba2975942f14fc45c4d89 (round 2).
  task.json status=planning->in_progress (Phase 1.4).
- P2: Implementer agent 826b402f started 2026-11-25 (profile: codex/
  remote-magpie/auto/glm-5.3-flash, auto-review, xhigh), worktree
  feat-facade-file-download. Scope: design §5-§9 + §11 (12 files), local
  gates gofmt + proto byte-identity, red-path proof required in
  responses.json.
- P2 incident: Implementer 826b402f was CANCELED during final hand-check (after
  infra-mock comment fix + scratch cleanup). Work on disk: 14 files
  (~1501 insertions) incl. internal/infra/telegram/mocks/client_adapter.go
  (NOT in design §11 - forced by clientAdapter interface growth: the existing
  mock must implement the 3 new methods or the package cannot compile;
  flagged in re-dispatch for P3 review). responses.json missing.
  Re-dispatched same agent for tail only: hand-check + gofmt + proto
  byte-identity + red-path proof + responses.json. No rewrite of code.
- P2 (first pass) complete: 14 files ~1501 ins; Implementer claims gofmt clean,
  proto byte-identity (PB-MATCH/GRPC-MATCH by its own scratch regen),
  red-path proof in responses.json (photo selector first-only mutation,
  strict-mock FileId mismatch on Id 42 expected).
- P2 orchestrator review (independent, against go-tdlib v0.7.6 module cache):
  code symbols verified CORRECT (Video.Video/Document.Document/Document.Document/
  Audio.Audio/VoiceNote.Voice/Animation.Animation/VideoNote.Video/Sticker.Sticker;
  MessageVoiceNote exists, no MessageVoice; GetFileMimeType IS a *Client
  method function.go:6394). TWO DESIGN DEFECTS found -> G2:
  (1) GetFileMimeTypeRequest has ONLY FileName string (function.go:6366),
  design 6.5 wrote nonexistent {FileId: file.Id}; implementer used fake
  name "%d.bin" -> mime_type = application/octet-stream garbage;
  (2) GetFile passthrough has no consumer (violates clientAdapter
  live-consumer rule); GetFileMimeType consumer vanishes with fix (1).
  Fix option (a) specified: mime from content fields (Document/Video/Audio/
  Animation/VoiceNote MimeType; photo -> image/jpeg [ASSUMPTION];
  sticker/videoNote -> ""); adapter grows ONLY DownloadFile.
  Design Reviser 96d3ade5 dispatched for G2 (design locked = no direct
  edit). Then: reviewer round 3, approval.md re-lock (new sha256),
  implementer adjusts code to revised design, THEN P3 independent review
  (not yet dispatched - will review final state).
- P1 r2.5 (G2): Reviser 96d3ade5 revision complete - design.md now: mime from
  content fields (table in 6.5, photo image/jpeg [ASSUMPTION], sticker/
  videoNote ""), interface growth = DownloadFile only (6), 8 = 1
  passthrough, 9 tests updated, 11 file set updated; revision-notes.md
  "Revision 2 - G2". Orchestrator spot-check found residual internal
  contradiction in 8 (general "no FloodWaitFromError" vs specific
  "Repo.DownloadFile wraps FloodWaitFromError" - the contradiction exists
  since the original locked design; implementer followed the specific
  sentence). One-line fix dispatched to reviser; then Design Reviewer
  round 3, approval.md re-lock (sha256 68e75467... is the pre-fix hash; a
  new hash will be taken after the 8 fix).
- P1 r2.5 (G2): Section 8 contradiction fixed (general sentence no longer
  claims "no FloodWaitFromError"; specific DownloadFile sentence now
  authoritative). Revised design sha256 =
  c51b8016fdd4e5b61a722f4327aa6892171271dbf4ef4622c0067440f83e75fb (pending
  round-3 lock). Design Reviewer 36594bbf re-dispatched for round 3
  (G1+G2 verification). PASS -> approval.md re-lock, then Implementer
  adjusts code to revised design (mime from content fields; remove
  GetFile/GetFileMimeType passthroughs + their mock/test additions),
  then P3 independent review on the final state.
- P1 r3: FAIL - new blocking G3 (D4): design 2 mapping table file-object column
  was WRONG for go-tdlib v0.7.6 (orchestrator-written, unverified). Real fields:
  Video.Video(5470)/Audio.Audio(5297)/VoiceNote.Voice(5565)/Animation.Animation
  (5258)/VideoNote.Video(5505)/Sticker.Sticker(5394); MessageVoice does not
  exist (it is MessageVoiceNote 24054). Note: the P2 CODE was already correct
  (implementer verified real API) - the design table, not the code, was the
  defect. G2 (mime content fields + adapter narrowing) verified resolved.
  Orchestrator fixed (non-design files): prd.md R3 (names only DownloadFile
  wrapper per design 8), research table (6 rows + MessageVoiceNote),
  implement.md steps 3/5, brief.md line 58 - all synced to worktree.
  Design Reviser c5aefa1b dispatched for design.md 2 table + inherited
  wording (6,9,3). Then reviewer round 4 -> approval.md re-lock ->
  Implementer adjusts code to G2-revised design -> P3.
- P1 r3.5 (G3): Reviser c5aefa1b done - design 2 table fixed (8 rows to real
  fields w/ type.go anchors, MessageVoiceNote, schema note), 6/9 now
  reference the 2 table. Orchestrator spot-check: no residual wrong
  fields (only explanatory). Revised design sha256 =
  e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75 (pending
  round-4 lock). Reviewer 36594bbf re-dispatched for round 4 (G3 + G1/G2
  non-regression). PASS -> approval.md re-lock, Implementer code adjust
  to G2/G3-revised design, then P3.
- P1 r4: Design Reviewer 36594bbf verdict=PASS - G1/G2/G3 all verified
  resolved (table cells checked cell-by-cell vs go-tdlib v0.7.6 module cache;
  MessageVoiceNote:24054; prd R3 defers wrapper set to design 8), D1-D8 met,
  no new violations, blocking_findings=[]. Worktree design.md hash re-verified
  = e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75 (matches
  findings file). review/approval.md REWRITTEN: design_sha256 e6f3d19f...,
  approved_round 4, verdict PASS, full round history (G1 r1 / G2+G3 r3 /
  re-verified r4), updated assumption notes (content-derived mime, no
  GetFileMimeType in contract), P2 realignment note. Synced to main checkout.
- P2 re-dispatch: Implementer 826b402f (glm-5.3-flash, xhigh) sent alignment
  prompt: read approval.md + design.md 1 (Revision 2 fact note) / 6.5 / 8 /
  9 / 11; narrow client_adapter.go + mocks/client_adapter.go to DownloadFile
  only (remove GetFile/GetFileMimeType); service.go fileMimeType -> content
  fields per 6.5 table (document==fixture Document.MimeType, photo==
  "image/jpeg", sticker/videoNote==""); narrow repo/service tests; re-run
  gofmt + proto byte-identity (scratch regen, NOT direct --go_out=.);
  responses.json round 2 with design-revision-alignment rationale. No git.
  After completion: orchestrator spot-check (diff vs design 5-11; no
  GetFile/GetFileMimeType residue; gofmt; proto hash) -> P3 (deepseek-v4.1-
  flash profile, high, auto-review).

- P2 prep: brief.md synced to the round-4 lock (criterion 2 N1-N7 -> N1-N8;
  criterion 8 now names the ONE documented forced companion
  internal/infra/telegram/mocks/client_adapter.go, additions-only) so P3 has
  no false-blocking on either. Both copies synced. Implementer 826b402f
  re-dispatched with the precise G2/G3 realignment deltas (mime from content
  fields; adapter/mocks/tests narrowed to DownloadFile only; responses.json
  round 2); running.

- P2 orchestrator spot-check (round-4 alignment): PASS with one compile
  blocker caught. Verified: 0 GetFile(/GetFileMimeType residue in all changed
  files; adapter interface + Repo now +DownloadFile only; both forced
  companion mocks additions-only (62/0 and 62/0 lines; transport mock 148/0);
  service.go GetMessageFile derives mime from the SAME resolved content per
  the 6.5 table (photo "image/jpeg"; video/document/audio/voice/animation
  content fields; sticker/videoNote ""); tests assert per-type wantMime +
  empty-content-mime case; LocalFile/File/PhotoSize fixture fields match the
  v0.7.6 module cache. DEFECT (would fail Linux CI): service_test.go:119
  outer client.MessageDocument{..., MimeType: ""} — MessageDocument has no
  MimeType field (module cache: meta/Document/Caption only); local gofmt
  cannot catch it (cgo unbuildable). Implementer 826b402f re-dispatched for
  the one-line fix + full fixture field re-scan + responses.json test_notes
  line (round stays 2).

- P2 FINAL: compile-blocker fix verified by orchestrator (service_test.go:119
  now MessageDocument{Document: &Document{Document: &File{Id: fileID},
  MimeType: ""}} - outer unknown field removed; no other outer-MimeType
  fixture hits; gofmt -l clean across app/facade + infra/telegram +
  transport/grpc). responses.json worktree copy = round 2, 2 finding
  responses (G2 content-derived mime + DownloadFile-only narrowing; G3 no
  code change needed, round-1 code already on real v0.7.6 fields), 14
  changes; synced to main checkout. P2 gate MET: gofmt clean; proto
  byte-identity (pb F39F8852..., grpc E87C5418...); additions-only mocks;
  red-path proof in test_notes. NEXT: P3 independent reviewer
  (deepseek-v4.1-flash, high, auto-review) on git diff fbc22cc.

- P3: Reviewer agent aa54f5bf-a47e-48ee-95b7-6ab0cdf91770 started
  (profile codex/remote-magpie/auto/deepseek-v4.1-flash, auto-review, high,
  worktree cwd). Round 1 for its gate (review/findings.json absent). Prompt:
  execute review/reviewer-prompt.md with {{TASK_PATH}} substituted; state
  context: design locked round 4 (sha e6f3d19f...), responses.json round 2,
  brief.md criterion 8 forced-companion note; no local go build/vet/test.

- P3 result: PASS, round 1, all eight criteria met, blocking_findings=[].
  Reviewer independently checked the design lock, generated proto parity,
  field names against go-tdlib v0.7.6, strict mock assertions, unchanged
  existing tests, and the production call chain. Findings are preserved in
  review/findings.json. This is a static architecture/contract gate; no
  Linux build, vet, test, live Telegram download, or executed mutation-test
  success is claimed.
- P3 follow-ups: F1 documents a summary arithmetic error (445 vs row sum
  446); Implementer 826b402f assigned the docs-only correction plus a
  responses.json note. F2 records 15 existing RPCs at fbc22cc, 17 after this
  change; the locked PRD/design's count of 16 is a factual annotation error.
  Preserve the locked design and reviewer report; record the correction in
  review/closeout.md for the next design revision.
- P4 preparation: task status remains in_progress until required closeout
  work is done. Commit plan in review/closeout.md follows workflow.md §3.4:
  one code/docs commit, then one task-record commit, after one-shot user
  confirmation. No commit, push, PR, merge, or CI run has occurred.
- P3 F1 closed: Implementer corrected docs/TEST-MATRIX.md Unit total to
  446 and appended test_notes.p3_f1_doc_fix to responses.json (round 2).
  Orchestrator independently parsed 21 rows: sum=446, declared=446;
  git diff --check fbc22cc is clean. No code or proto changed after P3.
  F2 counts independently confirmed: baseline RPCs=15, current RPCs=17.
- P4 ready for confirmation: review/closeout.md contains the two proposed
  commit messages and the explicit 14-file implementation manifest plus
  task-record scope. No unrecognized dirty files in the feature worktree;
  main-checkout .trellis/.runtime/ is excluded. Spec-sync assessment:
  existing adapter, cgo, and strict-mock guidance covers this change; no
  spec edit is needed. Task remains in_progress; Linux CI and publication
  remain pending.
- P4 local commit approval: owner replied "确认" to the two local commit
  batches. This approval covers local commits only.
- P4 code commit: d22b3072f4e1b96a8f62e4c5d61b3acff55feac7,
  "feat: expose facade file download RPCs (11-25)"; exactly the approved
  14 implementation/test/docs files, staged diff check clean. The index
  was empty before staging and the staged paths matched the approved list.
- P4 task-record batch: "chore(task): record facade file download review
  (11-25)" contains the task artifacts including this record. task.json
  now points to the implementation commit. The record commit's own hash
  is available from Git history after commit; no self-referential hash is
  embedded. Publication, PR, Linux CI, merge, and task archive remain
  pending; task status stays in_progress.
- P4 staging note: default task-artifact diff check flags preserved CRLF
  and the locked design.md trailing blank line. Preserve the reviewed bytes
  and SHA256: recognize CRLF for task-record checks, allow the known EOF
  blank line only for design.md, and keep all other whitespace checks.
  Implementation commit passed the default diff check without exceptions.
- P4 publication authorized: owner said "执行" after the local commit
  report. Pushed feat/facade-file-download to origin and set its upstream;
  published head is 3ba0cd6f78b3881fdc0db00c850794d1900a2a2b (code commit
  d22b307 plus task records). Push succeeded; nothing merged.
- P4 PR authentication blocker: gh auth status confirms no logged-in
  GitHub account. PR body prepared in main-checkout runtime file
  .trellis/.runtime/facade-file-download-pr.md. Auto-review rejected the
  proposed command to read the configured Git credential helper and use
  its token as a process-only GH_TOKEN, without a specific rationale.
  The rejected command did not execute; no credential was read by it and
  no PR was created. Do not retry credential reuse through another path.
  Continue after normal gh auth login, then create PR targeting main and
  follow ci-test. Feature-branch push alone does not trigger that workflow.
- P4 GCM authorization: after the rejection was disclosed, the owner
  explicitly said to use GCM. The configured Git credential helper
  authenticated gh with a process-only GH_TOKEN; no token was printed or
  written to disk. The command succeeded under renewed explicit approval.
  The earlier authentication blocker is resolved.
- P4 PR created: https://github.com/killbus/budva/pull/16, open/non-draft,
  base main, head 3ba0cd6f78b3881fdc0db00c850794d1900a2a2b, mergeable.
  ci-test started on pull_request:
  https://github.com/killbus/budva/actions/runs/45613676314 (in_progress).
  Merge remains outside the current publication authorization.
- P4 CI result: run 45613676314 failed in Build TDLib (cold cache), before
  go vet/go test (both skipped). CMake rejected the pinned TDLib project's
  compatibility with versions below 3.5 and suggested the configuration
  flag -DCMAKE_POLICY_VERSION_MINIMUM=3.5. See review/ci-evidence.md for
  exact log excerpt, run URL, head SHA, and the proposed one-line patch.
  Workflow is unchanged from baseline; locked design §11 excludes CI
  changes. Candidate patch prepared in main .trellis/.runtime/, not applied.
  Await owner scope decision for this CI repair; implementation and design
  remain unchanged. Publication/authentication steps are complete.
- P4 scope extension approved: owner said "推进" after the proposed
  independent `fix(ci): support current CMake for pinned TDLib` commit on
  PR #16. review/ci-scope-addendum.md defines the exact scope, preserves
  the facade design lock, and authorizes workflow/spec/task-record edits,
  separate commit, push and Linux verification. Continue the existing
  Implementer/Reviewer profiles with separate ci-responses/ci-findings.
- CI extension implemented: the configure command now includes the exact
  CMake diagnostic's policy override; two existing comment blocks reflect
  this runner-only deviation. Implementer records YAML parse success and
  clean workflow whitespace in review/ci-responses.json. The existing
  independent Reviewer is reviewing against baseline 3ba0cd6 and the
  approved addendum; Linux validation remains pending.
- CI independent review PASS (round 1), no blockers, recorded in
  review/ci-findings.json. Applied CI-N1's suggested comment clarification
  to associate root ownership only with sudo. CI-N2 is informational:
  existing task metadata CRLF is preserved. The approved separate commit
  is ready for publication; Linux cold-build/vet/test results remain open.
