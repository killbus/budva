# Design Reviser run — round 1

You run inside the worktree `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc`
(branch `feat/bot-menu-grpc`, base commit `609f181`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-04-bot-menu-grpc`
BASE_COMMIT = `609f181`

你是设计修订者。你不能向用户提问,只按 Design Reviewer 的 findings 修订设计。

## 输入

- `{{TASK_PATH}}/review/design-findings.json`（必读；本轮 round=1，1 条 blocking G1）
- 只读依据：`{{TASK_PATH}}/prd.md`、`.trellis/spec/`、基线 `{{BASE_COMMIT}}`
- 你要改的唯一文件：`{{TASK_PATH}}/design.md`
- 允许写：`{{TASK_PATH}}/review/` 下的说明文件

## 规则

- 只允许修改 `design.md` 以及写入 `review/`。不得修改 `prd.md`、`.trellis/spec/`、任何源码、`task.json`。
- 补充的内容必须来自 `prd.md`、`.trellis/spec/` 或基线；凭空新增的设计决定必须标注 `[ASSUMPTION]`。不要把假设写成既定事实。
- 缺口需要用户回答才能补时，不要猜：在 `review/open-questions.md` 写明问题并停止。
- 每条 finding 只做最小补充,不借机扩大设计范围,不重写未被指出的章节。
- 禁止 git commit / push / merge。

## 本轮必须处理的 blocking finding：G1（D6 验收覆盖）

Design Reviewer 指出 `design.md §7` flood-wait 行只写了结果（ResourceExhausted + RetryInfo），
但没写检测发生在哪一层、RetryInfo 的时长从哪来；`parseFloodWait` 是
`internal/infra/telegram/repo.go:299` 的包私有函数，transport 不可见，且现有
`mapFacadeError` 只有 `ChatNotReadyError` 一个 typed-error 先例、RetryInfo 固定 2s。

**按以下 orchestrator 决定修订（可追溯到基线，不是新发明）：**

1. **检测/解析落点 = infra**：在 `internal/infra/telegram` 新增并导出 typed error
   （例如 `FloodWaitError{ Delay time.Duration }`，名字可按包内风格微调），由一个
   导出 helper（例如 `FloodWaitFromError(err error) error`）在 TDLib 调用返回处包装；
   内部复用现有 `parseFloodWait`（`repo.go:299`）的解析逻辑，不重复实现正则。
   说明它作用于本任务新增的 5 个 RPC 的 TDLib 调用（以及可选地既有调用，但本轮只
   要求在新增路径生效，避免扩大范围）。
2. **时长来源 = `parseFloodWait` 的结果**：即 `N 秒 + 500ms`（`repo.go:311` 的既有语义）。
   明确写出这一点，避免 checker 按“恰好 N 秒”误判。
3. **transport 只做映射**：`mapFacadeError` 用 `errors.As` 识别该 typed error →
   `codes.ResourceExhausted` + `errdetails.RetryInfo{RetryDelay: delay}`；transport
   不解析错误文本。
4. **机械验收**：补一条测试要求：注入含 `retry after 7` 的 TDLib 错误，断言
   （a）infra helper 产出 delay = 7.5s；（b）gRPC 状态为 `ResourceExhausted` 且
   `RetryInfo.RetryDelay = 7.5s`。写进 §7 或 §8 的测试策略。

## 同时处理以下非阻塞 assumptions（它们来自 Reviewer 的 assumptions_for_human_review）

- **message_id=0 → 最新一条**（§4③）：保留，但在该处标注 `[ASSUMPTION]`（prd 未要求，
  是实现便利；并说明它是一次 `GetChatHistory(limit=1)`，不是轮询）。
- **ClickCallbackButton 服务端上限**：把“如 10s”改成锁定的命名常量（例如
  `callbackAnswerTimeout = 10 * time.Second`），并标注 `[ASSUMPTION]`（依据：TDLib
  502 超时语义，`function.go:7458` 注释）。说明它是服务端 deadline，不是轮询。
- **WebAppButtonInfo 字段**：保留并标注 `[ASSUMPTION]`，理由：如实返回按钮类型，
  v1 不执行（§10）。
- **按钮类型映射（事实修正）**：§10 中 `user` 在 go-tdlib v0.7.6 的实际类名是
  `InlineKeyboardButtonTypeUserShared`（`type.go:15903`）。并在 §5 的 `InlineButton`
  oneof 中补一个字段（例如 `string other_type = 5;`）把所有不可执行类型
  （login_url / callback_game / buy / copy_text / switch_inline / user_shared 等）
  用 TDLib class 名如实表示；明确 v1 只执行 `callback_data`。
- 不要改 `task.json`（主会话负责同步）。

## 输出

- 更新 `design.md`（保持现有章节编号稳定；新增内容可放在对应小节内）。
- 写 `{{TASK_PATH}}/review/design-revision.md`：逐条列出 G1 的修法、每个 assumption 的
  标注位置、以及未处理项（若有）。
- 最终回复只写 3-6 行摘要：改了什么、G1 是否已覆盖、是否产生 open-questions。
