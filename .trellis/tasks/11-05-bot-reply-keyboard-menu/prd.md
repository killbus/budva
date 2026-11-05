# Chat reply-keyboard menu over gRPC

## Goal

在现有 `FacadeGRPC.GetInlineMenu` 上**增量**暴露 bot 私聊里"输入框下方"的快捷菜单
（Telegram reply keyboard / TDLib `replyMarkupShowKeyboard`），让调用方能够读取该菜单的
按钮、布局和键盘属性；同时修正在 `message_id = 0` 时取错消息的问题。

Owner 场景（2026-11-05，附截图）：用户账号进入 bot 私聊后，底部显示一排按钮
（"Check-in" / "Top Up" / "Redeem Code" / …），输入框占位符为 "Choose a menu…"。
bot 消息正文是普通文本（"🏠 Main Menu … Select an option below:"），消息下方没有
inline 按钮。当前 `GetInlineMenu` 对这条消息返回 `rows: []`，无法表达底部菜单。

## Requirements

- **R1 — 读取底部菜单**：`GetInlineMenu` 的响应必须能表达 reply keyboard：按行列返回
  按钮文本与按钮类型；`web_app` 类型额外返回 URL。
- **R2 — 区分菜单类型**：响应必须明确告诉调用方这是 `INLINE`、`REPLY` 还是没有菜单
  （`NONE`），调用方不再靠 `rows` 是否为空猜测。
- **R3 — 键盘属性**：返回 reply keyboard 的 `is_persistent`、`resize_keyboard`、
  `one_time`、`input_field_placeholder`（截图中的 "Choose a menu…" 即该字段）。
- **R4 — `message_id = 0` 取当前生效菜单**：优先使用 TDLib 的
  `chat.reply_markup_message_id`（"Identifier of the message from which reply markup
  needs to be used"），而不是"最新一条历史消息"；该字段为 0 时保留现有
  `GetChatHistory(limit=1)` 回退行为。
- **R5 — 纯增量兼容**：不新增 RPC；不改现有 RPC/消息的字段编号、名称与语义；
  既有 inline `rows` 行为不变；现有调用方在忽略新增字段时行为不变。

## Non-goals（本任务不做）

- 不实现 Web App 执行（只返回 `web_app` 按钮的类型与 URL）。
- 不代用户点击 reply-keyboard 按钮（点击语义 = 以按钮文本 `SendMessage`，由调用方用
  现有 `SendMessage` 完成）。
- 不做 Bot API / bot token / per-chat menu button 管理。
- 不把 `reply_markup` 塞进 `Message` DTO（不改变 `GetMessage` / `GetMessages` /
  `GetChatHistory` 的响应体积与语义）。
- 不新增轮询、订阅或 stream；`GetChat` 是一次同步查询。
- 不新增 Go 依赖、不新增必需环境变量、不改 ruleset schema、不新增 TDLib 会话。

## Acceptance Criteria

- [ ] A1 对携带 `ReplyMarkupShowKeyboard` 的 bot 消息，`GetInlineMenu` 返回
      `menu_kind = MENU_KIND_REPLY`，`reply_rows` 的行列/按钮文本与 TDLib 数据一致，
      并回填 `message_id`。
- [ ] A2 `web_app` 类型按钮返回 `type = keyboardButtonTypeWebApp` 且 `web_app_url` 非空；
      其余类型 `type` 返回 TDLib class 名、`web_app_url` 为空。
- [ ] A3 `is_persistent` / `resize_keyboard` / `one_time` / `input_field_placeholder`
      与 TDLib `ReplyMarkupShowKeyboard` 对应字段一致。
- [ ] A4 对携带 `ReplyMarkupInlineKeyboard` 的消息，`menu_kind = MENU_KIND_INLINE`，
      既有 `rows` 内容与数量不变（冻结现有测试）。
- [ ] A5 无 `reply_markup`（含 `replyMarkupRemoveKeyboard` / `replyMarkupForceReply`）
      时返回 `menu_kind = MENU_KIND_NONE`，`rows` 与 `reply_rows` 均为空。
- [ ] A6 `message_id = 0` 且 `chat.reply_markup_message_id != 0` 时，实际读取的是该 id
      对应的消息（断言 `GetMessage` 收到该 id，且不会用 `GetChatHistory` 的最新消息）。
- [ ] A7 `message_id = 0` 且 `chat.reply_markup_message_id == 0` 时，保持原行为：
      使用 `GetChatHistory(limit=1)` 的最新消息；历史为空仍返回 `MessageNotFoundError`。
- [ ] A8 显式传入 `message_id != 0` 时不调用 `GetChat`（保持一次 `GetMessage` 的既有路径）。
- [ ] A9 `facade.proto` 只新增字段/枚举，既有字段编号不变；生成文件可复现。
- [ ] A10 受影响包的 build / vet / test / gofmt 通过；既有测试断言未被删除或放宽。

## Notes

- 手工验收：真实账号 + 截图中的 bot（8862641954）。步骤：`ResolveBot` →
  `GetInlineMenu(chat_id, 0)`，应返回 7 个按钮（"Gemini Plus (12 Months)" 独占一行，
  其余两两一行）与 `input_field_placeholder = "Choose a menu…"`。验收不得用于刷第三方 bot。
- 回复内容仍不轮询（沿用 11-04 owner 约束）；本任务只读菜单，不取 bot 后续回复。
