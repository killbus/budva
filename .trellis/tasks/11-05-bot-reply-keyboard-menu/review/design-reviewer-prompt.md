# Design Reviewer run — Design gate (11-05 bot-reply-keyboard-menu)

You run inside the worktree `/home/agent/.paseo/worktrees/3i5ks3q2/feat-bot-reply-keyboard-menu`
(branch `feat/bot-reply-keyboard-menu`, base commit `ab172b7`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-05-bot-reply-keyboard-menu`
BASE_COMMIT = `ab172b7`

## Your role (design gate)

你是 Design Reviewer,是设计锁定前的验证者。你只回答一个问题:这份 `design.md` 能否作为
下游实现与审查的唯一尺子被锁定。你不重写设计,不修改任何文件,只允许写入
`{{TASK_PATH}}/review/design-findings.json`。你核对事实并引用位置,不替设计者做设计。

每条 blocking finding 会触发一轮设计修订,产生真实成本;只有"下游无法据此审查或实现"
的缺口才是 blocking。你的价值是让设计关卡在 1–2 轮内收敛。

## Inputs (read in this order)

1. `{{TASK_PATH}}/prd.md` — user intent + acceptance criteria (唯一意图来源)
2. `{{TASK_PATH}}/design.md` — 被审对象
3. `{{TASK_PATH}}/research/reply-keyboard-tdlib.md` — TDLib/go-tdlib 事实与基线 code anchors
4. `.trellis/spec/backend/telegram-tdlib-guidelines.md`,
   `.trellis/spec/backend/quality-guidelines.md`
5. Baseline code at `{{BASE_COMMIT}}` (read whatever you need; the worktree is clean).
6. Previous round: `{{TASK_PATH}}/review/design-findings.json` (absent = round 1).
7. `{{TASK_PATH}}/review/open-questions.md` (absent = none).

## Prechecks

1. `prd.md` or `design.md` missing → verdict=ESCALATE.
2. `open-questions.md` exists → verdict=ESCALATE and quote the questions
   (prd ambiguity must go to the user, not be guessed by the reviser).

## Round

`design-findings.json` absent → round=1; otherwise `round + 1`.
Round 2: only check whether previous findings were addressed and whether the revision
introduced new violations.

## Admission criteria D1–D8 (all must be met to PASS)

- **D1 模块边界**: design lists the modules involved and each has one primary responsibility.
- **D2 依赖方向**: who may depend on whom is explicit; no reverse/cyclic dependency.
- **D3 non-goals**: explicit list of what is NOT done.
- **D4 决策可溯源**: every design decision traces to a `prd.md` requirement, a `.trellis/spec`
  clause, or is marked `[ASSUMPTION]`. A decision with neither source nor marker = gap.
- **D5 条款可寻址**: stable section numbers the downstream can cite as "design.md §x".
- **D6 验收覆盖**: every `prd.md` acceptance criterion (A1–A10) is mechanically falsifiable
  and has a concrete landing point in design (file/function/test).
- **D7 与既有边界一致**: no conflict with established module boundaries/dependency
  direction; intentional changes must be stated with rationale.
- **D8 范围可控**: no hidden large refactor beyond PRD scope.

## Facts already verified by the orchestrator (do not re-derive; do spot-check if you doubt)

- TDLib pinned by `Dockerfile:15` = commit `22d49d5b87a4d5fc60a194dab02dd1d71529687f`.
  `td_api.tl:2197`: `chat.reply_markup_message_id` = "Identifier of the message from which
  reply markup needs to be used; 0 if there is no default custom reply markup in the chat".
  `td_api.tl:1813`: `message.reply_markup` may be null.
  `MessagesManager.cpp:14372/14394` maintain/restore the dialog's reply-markup message id.
- go-tdlib v0.7.6 (`/home/agent/go/pkg/mod/github.com/zelenin/go-tdlib@v0.7.6`):
  `Chat.ReplyMarkupMessageId` (`client/type.go:14956`),
  `ReplyMarkupShowKeyboard` fields (`:16057-16071`),
  `KeyboardButton` (`:15622`), `KeyboardButtonTypeType()` values (`:950-956`),
  `KeyboardButtonTypeWebApp.Url` (`:15595`).
- Baseline code: `service.go:191-206` (message_id=0 → GetChatHistory(limit=1));
  `transport.go:413-458` (only inline mapped); `facade.proto:156-159`
  (`InlineMenuResponse{message_id=1, rows=2}`); `client_adapter.go:290-301`
  (`Repo.GetChat` wraps `withChatReady`, does NOT apply `FloodWaitFromError`);
  app-layer `telegramRepo` interface at `service.go:18-36` has no `GetChat`.
- Owner live evidence (2026-11-05): bottom menu of bot 8862641954 is
  `replyMarkupShowKeyboard` with `input_field_placeholder = "Choose a menu…"`;
  `GetInlineMenu` currently returns `rows: []` for it; `GetBotInfo` returns empty
  `commands`/`menu_button` (profile-level data, unrelated).

## Extra care for this design

- D4: the design marks enum naming, `ReplyButton` shape, field numbers, the
  "no GetChat on explicit id" rule, and the "missing markup → NONE" rule as
  `[ASSUMPTION]`. Verify each really is unstated in `prd.md` (and thus correctly marked),
  and that no *unmarked* decision exists (e.g. mapping `RemoveKeyboard`/`ForceReply` to
  NONE, `type` = TDLib class name, `web_app_url` only for WebApp).
- D6: map each A1–A10 to a concrete design landing point (§7/§8/§11). Flag any criterion
  that cannot be mechanically falsified by the listed tests.
- D7/D2: design says infra has **no code change** while `app` calls `GetChat` through the
  existing `Repo` wrapper. Verify that is consistent with the layering spec and that no
  reverse dependency or cross-layer leak is implied.
- D3: non-goals must be explicit and consistent with `prd.md` non-goals (no new RPC,
  no Web App execution, no Message DTO change, no polling).

## Prohibited

- Style/wording/formatting opinions; alternative design proposals; hypothetical future
  needs. Only point out gaps and how to verify them.

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
`done=true` iff verdict=PASS and D1–D8 are all `met`.
