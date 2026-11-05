# Design — chat reply-keyboard menu over the existing GetInlineMenu RPC

状态：**设计，待 Design Reviewer 锁定**。
上游：`prd.md`；owner 场景与截图（2026-11-05）；TDLib 事实见
`research/reply-keyboard-tdlib.md`。
基线：`ab172b7`（v0.5.0）。

---

## 1. 目标与语义

在**不新增 RPC**的前提下，让现有 `FacadeGRPC.GetInlineMenu` 能回答：

> 这个 bot 私聊当前"输入框下方"的快捷菜单是什么？

即 TDLib 的 `replyMarkupShowKeyboard`（reply keyboard），以及现有的
`replyMarkupInlineKeyboard`。响应必须显式区分两者，并提供调用方渲染底部键盘所需的
按钮与属性。actor 仍是 Budva 的 TDLib 用户会话（`internal/infra/telegram`）。

## 2. 非目标（明确不做）

1. **不新增 RPC、不新增 service**；只做 `GetInlineMenu` 的增量字段。
2. **不执行 Web App**：`web_app` 按钮只返回类型与 URL，调用方自行处理。
3. **不代用户点击 reply-keyboard 按钮**：点击语义是"把按钮文本作为普通消息发送"，
   由调用方使用现有 `SendMessage` 完成（见 §9）。
4. 不做 Bot API / bot token / per-chat menu button 管理。
5. 不把 `reply_markup` 放进 `Message` DTO；`GetMessage` / `GetMessages` /
   `GetChatHistory` 的响应不变。
6. 不新增轮询/订阅/stream；`GetChat` 是一次同步调用。
7. 不改现有 15 个 RPC 的请求/响应字段、编号与语义（`prd.md` R5）。

## 3. 决策：扩展现有 `GetInlineMenu`（而非新 RPC）

| 方案 | 结论 |
|---|---|
| 新增 `GetChatMenu` RPC | 否决：与 `GetInlineMenu` 职责重叠，且违背 `prd.md` R5 的"不新增 RPC" |
| 改 `GetInlineMenu`（本设计） | 采纳：同一语义（读菜单）、同一入参（chat_id + message_id）、纯增量字段 |

依据：`prd.md` R1–R5；`design.md §2`。现有 `rows` 字段保持原义（inline 按钮），
新增字段只描述 reply keyboard 与菜单类型。

## 4. 数据流

```
GetInlineMenu(chat_id, message_id)
  ├─ message_id != 0 → GetMessage(chat_id, message_id)          （既有路径，不调 GetChat）
  └─ message_id == 0
       ├─ GetChat(chat_id) → chat.reply_markup_message_id
       ├─ != 0 → GetMessage(chat_id, reply_markup_message_id)    ← 新增的权威来源
       └─ == 0 → GetChatHistory(chat_id, limit=1) → messages[0]  （保留既有回退）
            └─ 空 → MessageNotFoundError                        （既有行为）
  → transport 按 msg.ReplyMarkup 类型映射：
       *ReplyMarkupInlineKeyboard → menu_kind=INLINE, rows[]
       *ReplyMarkupShowKeyboard   → menu_kind=REPLY, reply_rows[] + 键盘属性
       其它 / nil                 → menu_kind=NONE, rows/reply_rows 均空
```

`chat.reply_markup_message_id` 的语义来自 TDLib 的 `chat` 定义原文：
"Identifier of the message from which reply markup needs to be used; 0 if there is no
default custom reply markup in the chat"（`td_api.tl`，见 `research/reply-keyboard-tdlib.md`）。
它正是客户端渲染"当前生效键盘"时使用的消息指针；因此 `message_id = 0` 用它而不是
"最新一条历史消息"（`prd.md` R4）。

`GetChat` 在冷 DB 上走既有的 `withChatReady` 等待窗口（`client_adapter.go:290`），
与 `GetChatHistory` / `GetMessage` 的冷启动语义一致；本设计不改变该包装器。

## 5. proto 变更（`internal/transport/grpc/pb/facade.proto`，纯增量）

```proto
enum MenuKind {                       // [ASSUMPTION] 命名与取值
  MENU_KIND_UNSPECIFIED = 0;
  MENU_KIND_NONE = 1;
  MENU_KIND_INLINE = 2;
  MENU_KIND_REPLY = 3;
}

message ReplyButton {                 // [ASSUMPTION] 字段集合
  string text = 1;
  string type = 2;                    // TDLib class 名：keyboardButtonTypeText /
                                      // keyboardButtonTypeRequestPhoneNumber / ... /
                                      // keyboardButtonTypeWebApp
  string web_app_url = 3;             // 仅 type=keyboardButtonTypeWebApp 时非空
}
message ReplyButtonRow { repeated ReplyButton buttons = 1; }

message InlineMenuResponse {
  int64 message_id = 1;               // 既有字段，语义不变
  repeated InlineButtonRow rows = 2;  // 既有字段，语义不变：inline 键盘
  MenuKind menu_kind = 3;             // 新增
  repeated ReplyButtonRow reply_rows = 4;   // 新增
  bool is_persistent = 5;             // 新增
  bool resize_keyboard = 6;           // 新增
  bool one_time = 7;                  // 新增
  string input_field_placeholder = 8; // 新增；截图中 "Choose a menu…"
}
```

`GetInlineMenuRequest` 不改（`chat_id` / `message_id`，`0 = 当前`）。
字段编号 1/2 不动，新增字段只使用 3–8，保证旧客户端/旧生成代码兼容。

`ReplyButton.type` 直接返回 go-tdlib 的 `KeyboardButtonTypeType()` 字符串，
与既有 `InlineButton.other_type` 返回 `InlineKeyboardButtonTypeType()` 的约定一致
（`transport.go:445` 基线事实），调用方不需要第二套枚举。

## 6. 分层落点（薄改）

| 层 | 文件 | 改动 |
|---|---|---|
| proto | `internal/transport/grpc/pb/facade.proto` + 生成文件 | §5 的枚举/消息/字段 |
| transport | `internal/transport/grpc/transport.go` | 私有映射函数改为按 `msg.ReplyMarkup` 类型分派（inline / showKeyboard / 其它）；`GetInlineMenu` handler 不变式保持 |
| app | `internal/app/facade/service.go` | `GetInlineMenu`：`message_id=0` 时先 `GetChat` 取 `ReplyMarkupMessageId`，为 0 再走既有 `GetChatHistory(limit=1)` 回退；`telegramRepo` 接口新增 `GetChat(*client.GetChatRequest) (*client.Chat, error)` |
| app mocks | `internal/app/facade/mocks/telegram_repo.go` | 手工补 `GetChat` 方法（沿用现有 mockery 风格） |
| infra | `internal/infra/telegram/*` | **无代码改动**（`Repo.GetChat`/`GetMessage`/`GetChatHistory` 已存在） |
| docs | `docs/TEST-MATRIX.md` | 更新 `GRPC-U-002` / `LIVE-BOT-*` 手工验收项 |

依赖方向保持 `transport/grpc → app/facade → infra/telegram`；transport 不接触 TDLib
类型以外的映射逻辑（既有边界），app 只做请求构造与消息选择，infra 不感知菜单语义。
无新增依赖/env/ruleset；无第二个 TDLib 会话。

## 7. 错误映射（不新增映射规则）

| 场景 | 行为 |
|---|---|
| `message_id != 0` 且消息不存在 | 既有 `MessageNotFoundError` → `NotFound`（不变） |
| `message_id = 0`、`reply_markup_message_id != 0`、该消息已删除 | `GetMessage` 的 TDLib 400 → 既有 `Internal`（与显式 id 路径一致） |
| `chat_id` 不存在/不可达 | `GetChat` 的既有错误（冷 DB 走 `ChatNotReadyError` → `Unavailable` + RetryInfo(2s)；其它 400 → `Internal`），与既有 `GetChatHistory` 路径一致 |
| flood-wait on `GetChat` | `Repo.GetChat` 当前不调用 `FloodWaitFromError`（`client_adapter.go:290-301`），因此该一步的 429 映射为 `Internal`。为使本任务不改变既有 `GetChat` 调用方（transform/source_opener）的语义，**本设计不修改该包装器**；后续步骤 `GetMessage(FloodWaitFromError)` 仍正常映射。`[ASSUMPTION]` → 可接受的已知限制 |
| 无键盘 / removeKeyboard / forceReply | `OK` + `MENU_KIND_NONE`（`prd.md` A5） |

不新增 `InvalidArgument` 校验：`chat_id`/`message_id` 的既有非法输入行为保持不变
（`prd.md` R5 的"不改既有语义"优先于防御性校验）。

## 8. 测试策略

- **app/facade（mock `telegramRepo`）**
  - `message_id != 0`：只调 `GetMessage`，断言 `GetChat` 未被调用（A8）。
  - `message_id = 0` 且 `ReplyMarkupMessageId = 777`：`GetMessage` 收到 777，
    断言 `GetChatHistory` 未被调用（A6）。
  - `message_id = 0` 且 `ReplyMarkupMessageId = 0`：走 `GetChatHistory(limit=1)`（A7）。
  - 历史为空仍返回 `MessageNotFoundError`（A7）。
- **transport/grpc（mock `facadeService`）**
  - `ReplyMarkupShowKeyboard`：`menu_kind=REPLY`、行列/文本/type/web_app_url/四个属性
    逐项断言（A1–A3）；行内 nil 按钮被跳过。
  - `ReplyMarkupInlineKeyboard`：既有 `rows` 映射测试保持通过，并新增
    `menu_kind=INLINE` 断言（A4）。
  - nil / `ReplyMarkupRemoveKeyboard` / `ReplyMarkupForceReply`：
    `menu_kind=NONE`、两个 rows 均为空（A5）。
- **proto**：重新生成后，既有字段编号/名称不变；生成文件可复现（A9）。
- **机械验证**：受影响包 build / vet / test / gofmt（A10）。
- 测试断言必须能失败（quality-guidelines F1/F2）：reply keyboard 用例在映射被删除时
  必须红；`GetChat` 未调用断言用 strict mock（未声明期望即失败）。
- **手工 live 验收**（不能进 CI）：截图中的 bot 8862641954；
  `ResolveBot` → `GetInlineMenu(chat_id, 0)`，期望 7 个按钮（1 + 2 + 2 + 2）、
  `is_persistent`、`input_field_placeholder="Choose a menu…"`。

## 9. 交互语义（调用方契约）

- `MENU_KIND_REPLY` 的按钮：点击 = 用按钮 `text` 调既有 `SendMessage`
  （`{message:{chat_id,text}}`）；`web_app` 类型需调用方自行打开 `web_app_url`；
  `request_*` 类型需要客户端交互。
- `MENU_KIND_INLINE` 的按钮：沿用 `ClickCallbackButton`。
- 本设计不新增"点击按钮"RPC，也不取 bot 后续回复（沿用 11-04 的 owner 约束）。

## 10. 风险与边界

- **TDLib 可能没有该消息的 markup**：`chat.reply_markup_message_id` 指向的消息若
  markup 未加载/已被 TTL 清理，则返回 `MENU_KIND_NONE`——如实反映 TDLib 状态，
  不猜测、不回退到别的消息（`[ASSUMPTION]`）。
- **只在 reply keyboard 变化的会话里生效**：`GetChat` 是单次读取，不订阅键盘变更；
  调用方需要最新菜单时重新调用。
- **bot 消息文本不含结构化菜单**：截图中的 "Main Menu" 正文是普通文本，不能从文本
  推断菜单；必须以 `reply_markup` 为准。
- **不做 Web App**：`web_app` 按钮只给 URL。

## 11. 交付顺序

1. 更新 `facade.proto`（枚举 + 消息字段）并用既有 `Taskfile.yml proto` 重新生成。
2. `app/facade`：接口加 `GetChat`、`GetInlineMenu` 解析逻辑 + 单测 + mock 方法。
3. `transport/grpc`：映射函数分派 + 单测（reply/inline/none）。
4. `docs/TEST-MATRIX.md` 更新手工验收项。
5. 受影响包 build/vet/test/gofmt 全绿。
