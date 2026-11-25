# Design Reviewer run — Design gate (11-25 facade-file-download)

You run inside the worktree `D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download`
(branch `feat/facade-file-download`, base commit `fbc22cc`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-25-facade-file-download`
BASE_COMMIT = `fbc22cc`

## Your role (design gate)

你是 Design Reviewer,是设计锁定前的验证者。你只回答一个问题:这份 `design.md` 能否作为
下游实现与审查的唯一尺子被锁定。你不重写设计,不修改任何文件,只允许写入
`{{TASK_PATH}}/review/design-findings.json`。你核对事实并引用位置,不替设计者做设计。

每条 blocking finding 会触发一轮设计修订,产生真实成本;只有"下游无法据此审查或实现"
的缺口才是 blocking。你的价值是让设计关卡在 1-2 轮内收敛。

## Inputs (read in this order)

1. `{{TASK_PATH}}/prd.md` — user intent + acceptance criteria (唯一意图来源)
2. `{{TASK_PATH}}/design.md` — 被审对象
3. `{{TASK_PATH}}/research/tdlib-file-download-api.md` — TDLib/go-tdlib 事实与基线 code anchors
4. `.trellis/spec/backend/telegram-tdlib-guidelines.md`,
   `.trellis/spec/backend/quality-guidelines.md`,
   `.trellis/spec/backend/docker-ci-guidelines.md`
5. Baseline code at `{{BASE_COMMIT}}` (read whatever you need; the worktree is clean
   apart from the untracked task dir).
6. Previous round: `{{TASK_PATH}}/review/design-findings.json` (absent = round 1).
7. `{{TASK_PATH}}/review/open-questions.md` (absent = none).

## Prechecks

1. `prd.md` or `design.md` missing -> verdict=ESCALATE.
2. `open-questions.md` exists -> verdict=ESCALATE and quote the questions
   (prd ambiguity must go to the user, not be guessed by the reviser).

## Round

`design-findings.json` absent -> round=1; otherwise `round + 1`.
Round 2: only check whether previous findings were addressed and whether the
revision introduced new violations.

## Admission criteria D1-D8 (all must be met to PASS)

- **D1 模块边界**: design lists the modules involved and each has one primary
  responsibility (design §3 table).
- **D2 依赖方向**: who may depend on whom is explicit; no reverse/cyclic
  dependency (design §4).
- **D3 non-goals**: explicit list of what is NOT done (design §2 N1-N7),
  consistent with prd.md non-goals.
- **D4 决策可溯源**: every design decision traces to a prd.md requirement, a
  `.trellis/spec` clause, or is marked `[ASSUMPTION]`. A decision with neither
  source nor marker = gap.
- **D5 条款可寻址**: stable section numbers the downstream can cite as
  "design.md §x".
- **D6 验收覆盖**: every prd.md acceptance criterion (A1-A5) is mechanically
  falsifiable and has a concrete landing point in design (file/function/test).
- **D7 与既有边界一致**: no conflict with established module boundaries /
  dependency direction / spec; intentional changes must be stated with
  rationale.
- **D8 范围可控**: no hidden large refactor beyond PRD scope.

## Facts already verified by the orchestrator (do not re-derive; spot-check if you doubt)

- go-tdlib v0.7.6 (`C:\Users\ying\go\pkg\mod\github.com\zelenin\go-tdlib@v0.7.6`):
  `DownloadFile` (`client/function.go:11924`, request at `:11909` with
  `FileId/Priority/Offset/Limit/Synchronous`); `GetFile` (`:1614`);
  `GetFileMimeType` (`:6394`); `File` (`client/type.go:4277`, `Id int32`,
  `Local *LocalFile`); `LocalFile` (`type.go:4209`,
  `IsDownloadingCompleted` = "the local copy is fully available");
  `Photo.Sizes []*PhotoSize` (`:5370`), `PhotoSize.Photo *File` (`:4420-4432`).
- Baseline (fbc22cc): `cmd/facade/main.go:80,94,110-111` (wiring: whole
  `telegram.Repo` into `facade.New` — no wiring change needed);
  `service.go:18-36` (partial `telegramRepo` interface, `GetMessage` at :21);
  `errors.go:1-25` (typed-error style); `client_adapter.go:9-63`
  (`clientAdapter` 1:1 rule + static assertion), `:367-437` (chat-less bare
  passthroughs `GetMe`/`GetUser`/`GetOption`), `:134-157` (chat-scoped
  `withChatReady` pattern); `transport.go:39-84` (`mapFacadeError`),
  `:90-113` (`facadeService` interface); `facade.proto` lines 8-23 (16 RPCs);
  mocks are mockery-generated (11-05 precedent: hand-adding a mock method in
  exact generated style is acceptable).
- Proto toolchain parity was PROVEN by the orchestrator at P0 on this box:
  regenerating the UNCHANGED baseline proto with `protoc libprotoc 34.2` +
  `protoc-gen-go v1.36.11` + `protoc-gen-go-grpc v1.5.1` reproduced the
  committed `facade.pb.go` and `facade_grpc.pb.go` byte-identically.
- `api/proto/facade.proto` is a git symlink (mode 120000) to
  `internal/transport/grpc/pb/facade.proto` — untouched by this ticket.
- Environment: Windows; go-tdlib cannot build/test here (cgo linux/darwin
  only); CI `ci-test.yml` (Linux, real TDLib) is the authoritative
  vet/test gate, triggered via PR to main or workflow_dispatch (a bare
  feature-branch push does NOT trigger it). No Docker/WSL/debian-host here.

## Extra care for this design

- D4: the design marks (a) max-size photo variant selection, (b) `Priority: 16`,
  (c) best-effort `mime_type` (failure -> `""`), and (d)
  `FileDownloadFailedError` -> `Internal` as `[ASSUMPTION]`. Verify each is
  really unstated in prd.md/spec (and thus correctly marked), and that no
  *unmarked* decision exists (e.g. proto field names/numbers, the
  success-oracle shape, the "no withChatReady on file ops" infra choice,
  `file_id` range validation in transport).
- D6: map each A1-A5 to a concrete design landing point (§5/§6/§7/§9). Flag any
  criterion that cannot be mechanically falsified by the listed tests.
- D7: the design puts 3 new passthroughs in `clientAdapter` + `Repo` without
  `withChatReady` (file ops are not chat-scoped). Verify that is consistent
  with `telegram-tdlib-guidelines.md` and the existing chat-less wrappers
  (`GetMe`/`GetUser`/`GetOption`); verify no reverse dependency or cross-layer
  leak (app returns raw `*client.File`, never pb).
- D3: non-goals must be consistent with prd.md (no byte streaming, no progress
  push, no upload-side change, no GraphQL, no engine/ruleset, no download
  management RPCs, `Message.file_path` dead field left alone).
- D5: the proto block in §5 must be the exact contract text (2 RPCs, 4
  messages, field numbers) the Implementer will paste into the .proto —
  addressable and unambiguous.

## Prohibited

Style/wording/formatting opinions; alternative design proposals; hypothetical
future needs. Only point out gaps and how to verify them.

## Output

Write `{{TASK_PATH}}/review/design-findings.json` (overwrite) with:

```json
{
  "done": false,
  "verdict": "PASS | FAIL | ESCALATE",
  "round": 1,
  "criteria": {"D1": "met|gap", "D2": "met|gap", "D3": "met|gap", "D4": "met|gap",
               "D5": "met|gap", "D6": "met|gap", "D7": "met|gap", "D8": "met|gap"},
  "blocking_findings": [
    {"id": "G1", "criterion": "", "design_locator": "", "gap": "", "verification": ""}
  ],
  "assumptions_for_human_review": [],
  "escalation_reason": ""
}
```

Then reply with the same JSON followed by one line `done=true|false`. Nothing else.
`done=true` iff verdict=PASS and D1-D8 are all `met`.
