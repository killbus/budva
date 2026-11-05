# Design — bot chat menu interaction over gRPC (option A2)

状态：**设计，未实现**（owner go-ahead 后进入 implement）。
上游：`research/bot-menu-grpc.md`；范围结论由 owner 2026-11-04 确认。

---

## 1. 目标与语义

**用户账号**在某个 bot 的私聊里读取并操作它提供的菜单。

- actor = Budva 的 TDLib **用户会话**（`internal/infra/telegram/repo.go`）。
- 无 bot token、无第二个会话、无新依赖、无规则集改动。
- Web App 菜单不在范围。

## 2. 非目标（明确不做）

1. **不轮询、不拉取 chat 里的回复**（owner 2026-11-04）：本设计不新增任何
   “取 bot 回复 / 等待 bot 回复 / 订阅更新”的 RPC，也不加 stream。
   `GetMessage`/`GetChatHistory` 仍存在，但那是既有能力，本任务不为其增加
   任何“等回复”语义；调用方若想自己轮询，那是调用方的事，Budva 不提供。
2. 不做 Web App 菜单操作（需要服务端 WebView + JS bridge）。
3. 不做 bot 账号自身的菜单配置（Bot API `setMyCommands` 等，属方案 B）。
4. 不实现通用 `Execute(opcode, payload)` RPC。

## 3. 决策：新增 5 个 RPC，加进现有 `FacadeGRPC`

语义上是用户账号的 chat 操作，与 `FacadeGRPC` 一致，**不新开 service**。

| RPC | 类型 | 作用 | TDLib 调用 |
|---|---|---|---|
| `ResolveBot` | 读 | `@username` → 私聊 `chat_id` + `bot_user_id` | `searchPublicChat` + `getUser` |
| `GetBotInfo` | 读 | 读 bot 的命令菜单 / menu button | `getUserFullInfo` |
| `GetInlineMenu` | 读 | 读某条消息里的 inline 按钮（含 callback data） | `getMessage` |
| `ClickCallbackButton` | 写 | 点 callback 按钮并拿同步回答 | `getCallbackQueryAnswer` |
| `SendBotCommand` | 写 | 在 bot chat 发 `/command`（`/start` 支持 deep-link 参数） | `sendMessage` / `sendBotStartMessage` |

**为什么不直接给 `Message` 加 `reply_markup`**（另一可行方案）：那会改变
`GetMessage`/`GetMessages`/`GetChatHistory` 既有响应体积，影响所有现有调用方；
用独立的 `GetInlineMenu` 让新能力保持**纯增量**。若 owner 想要“消息自带按钮”，
再单独加 Message 字段即可，不冲突。

## 4. 数据流

```
① 解析 bot
   client --ResolveBot(username)--> FacadeGRPC --> facade.Service
        --> telegram.Repo.SearchPublicChat(username)
        --> Chat.Type.(*ChatTypePrivate).UserId
        --> GetUser(user_id): UserTypeBot? 是 -> {chat_id, bot_user_id}
                                            否 -> FailedPrecondition
② 读菜单（命令）
   client --GetBotInfo(bot_user_id)--> ... --> GetUserFullInfo(user_id).BotInfo
        --> {commands[], menu_button{text,url}, description, short_description}
③ 读菜单（inline 按钮）
   client --GetInlineMenu(chat_id, message_id)--> ... --> GetMessage
        --> ReplyMarkup.(*ReplyMarkupInlineKeyboard) -> rows[][]
        message_id=0 -> GetChatHistory(limit=1) 取最新一条（一次调用，非轮询）[ASSUMPTION]
④ 点按钮（写）
   client --ClickCallbackButton(chat_id, message_id, data)--> ... 
        --> GetCallbackQueryAnswer(chat_id, message_id, &CallbackQueryPayloadData{Data: data})
        --> {text, show_alert, url}
⑤ 发命令（写）
   client --SendBotCommand(chat_id, command[, bot_user_id, parameter])--> ...
        --> command == "/start" && parameter != "" :
              SendBotStartMessage(bot_user_id, chat_id, parameter)
            否则:
              SendMessage(chat_id, InputMessageText{"/command"})
        --> EmptyResponse
```

## 5. proto 变更（`internal/transport/grpc/pb/facade.proto`）

```proto
service FacadeGRPC {
  // ... 现有 10 个 RPC 不变 ...
  rpc ResolveBot          (ResolveBotRequest)          returns (ResolveBotResponse);
  rpc GetBotInfo          (GetBotInfoRequest)          returns (BotInfoResponse);
  rpc GetInlineMenu       (GetInlineMenuRequest)       returns (InlineMenuResponse);
  rpc ClickCallbackButton (ClickCallbackButtonRequest) returns (CallbackAnswerResponse);
  rpc SendBotCommand      (SendBotCommandRequest)      returns (EmptyResponse);
}

message ResolveBotRequest  { string username = 1; }        // 不带 @
message ResolveBotResponse { int64 chat_id = 1; int64 bot_user_id = 2; }

message BotCommand        { string command = 1; string description = 2; }
message BotMenuButtonInfo { string text = 1; string url = 2; }   // url 空 = 默认/命令菜单
message GetBotInfoRequest  { int64 bot_user_id = 1; }
message BotInfoResponse {
  int64 bot_user_id = 1;
  repeated BotCommand commands = 2;
  BotMenuButtonInfo menu_button = 3;   // 无自定义按钮时缺省
  string description = 4;
  string short_description = 5;
}

message GetInlineMenuRequest  { int64 chat_id = 1; int64 message_id = 2; }  // 0 = 最新一条
message InlineButton {
  string text = 1;
  oneof action {
    bytes  callback_data = 2;    // 可点
    string url           = 3;    // 需调用方自行打开
    WebAppButtonInfo web_app = 4;  // 仅返回 url；v1 不执行
    string other_type = 5;         // [ASSUMPTION] 不可执行类型的 TDLib class 名，如实返回
  }
}
message WebAppButtonInfo { string text = 1; string url = 2; }  // [ASSUMPTION] 仅返回按钮类型；v1 不执行
message InlineButtonRow  { repeated InlineButton buttons = 1; }
message InlineMenuResponse {
  int64 message_id = 1;                 // 实际读取的消息（请求为 0 时回填）
  repeated InlineButtonRow rows = 2;    // 空 = 该消息没有 inline 键盘
}

message ClickCallbackButtonRequest {
  int64 chat_id = 1; int64 message_id = 2;
  bytes callback_data = 3;              // 显式传 data，不传按钮序号
}
message CallbackAnswerResponse {
  string text = 1; bool show_alert = 2; string url = 3;
}

message SendBotCommandRequest {
  int64 chat_id = 1;                    // 与 bot 的 private chat
  string command = 2;                   // "/start"（含前导斜杠）
  int64 bot_user_id = 3;                // 仅 /start + parameter 时需要
  string parameter = 4;                 // 仅 /start deep-link 参数
}
```

兼容性：纯增量（新 RPC + 新 message），不改现有字段 → 向后兼容；
gRPC reflection 已启用（`internal/transport/grpc/doc.go`），客户端可自省。

## 6. 分层落点（薄改）

| 层 | 文件 | 改动 |
|---|---|---|
| infra adapter | `internal/infra/telegram/client_adapter.go` | 加 `SearchPublicChat`、`GetUserFullInfo`、`GetUser`、`SendBotStartMessage`（`CreatePrivateChat` 可选）。已在：`GetMessage`、`GetChatHistory`、`SendMessage`、`GetCallbackQueryAnswer`（`client_adapter.go:18-52`） |
| infra repo | `internal/infra/telegram/repo.go` | 透传（`clientAdapter` 嵌入已自动获得）。对本任务新增 5 个 RPC 的 TDLib 调用返回处用导出 helper 包装 flood-wait；本轮不要求既有调用路径生效 |
| app | `internal/app/facade/service.go` | 加 `ResolveBot/GetBotInfo/GetInlineMenu/ClickCallbackButton/SendBotCommand`，返回 raw TDLib 类型或 domain DTO |
| transport | `internal/transport/grpc/transport.go` | `facadeService` 接口加 5 方法；加 5 个 handler；加 raw→proto 映射；扩 `mapFacadeError` |
| proto | `api/proto`（真源 `internal/transport/grpc/pb/facade.proto`）+ 生成文件 | 见 §5 |

无新增 env；无 ruleset 变更；无新依赖。

## 7. 错误映射（扩 `mapFacadeError`，`transport.go`）

| 场景 | TDLib 表现 | gRPC |
|---|---|---|
| username 不存在 | error | `NotFound` |
| 目标不是 bot | `UserTypeBot` 判定失败 | `FailedPrecondition` |
| 消息没有 inline 键盘 | 正常，空 rows | `OK`（空） |
| 按钮已失效/消息被删 | `getCallbackQueryAnswer` 4xx | `FailedPrecondition`（附 TDLib 消息） |
| bot 未及时回答 callback | 502 | `DeadlineExceeded`（明确不是 Internal） |
| flood wait | TDLib 错误文本含 `retry after N` | `ResourceExhausted` + `RetryInfo`（见下方落点与时长来源） |
| callback data 为空 | 参数校验 | `InvalidArgument` |

**flood-wait 检测落点**：在 `internal/infra/telegram` 定义导出 typed error
`FloodWaitError{ Delay time.Duration }`（名字可按包内风格微调），并提供导出
helper `FloodWaitFromError(err error) error`；新增 5 个 RPC 的 TDLib 调用返回处
调用该 helper。内部复用现有包私有 `parseFloodWait`（`repo.go:299`）的解析逻辑，
不重复实现正则。Transport 不解析错误文本。

**RetryInfo 时长来源**：`FloodWaitError.Delay` 来自 `parseFloodWait` 的既有结果，
即 `N 秒 + 500ms`（`repo.go:309`），不是恰好 `N 秒`。`mapFacadeError` 用 `errors.As`
识别 `FloodWaitError`，映射 `codes.ResourceExhausted`，并把 `Delay` 放入
`errdetails.RetryInfo{RetryDelay: delay}`。

回调应答的超时：`getCallbackQueryAnswer` 阻塞至 bot 应答或超时。给该 RPC 一个服务端
deadline，常量名 `callbackAnswerTimeout = 10 * time.Second` [ASSUMPTION]；超时映射
`DeadlineExceeded`。这是服务端 deadline，不是轮询。该值依据 go-tdlib v0.7.6 对
`getCallbackQueryAnswer` 的 502 超时语义（`client/function.go:7458` 注释原文：Returns an error with code 502 if the bot fails to answer the query before the query timeout expires），但 10s 是实现
便利选择，prd 未指定。

## 8. 测试策略

- **unit**：`app/facade` 用 telegramRepo mock（仓库已用 mockery v3）；
  `transport/grpc` 用 `facadeService` mock（`internal/transport/grpc/mocks`），
  断言 raw→proto 映射与错误码。
- **映射单测**：`ReplyMarkupInlineKeyboard` → proto rows 的行列保持；`BotInfo` →
  `BotInfoResponse`；`CallbackQueryAnswer` → proto。
- **flood-wait 机械验收**：注入含 `retry after 7` 的 TDLib 错误，断言 infra helper
  产出 `FloodWaitError.Delay = 7.5s`；再断言 gRPC 状态为 `ResourceExhausted` 且
  `RetryInfo.RetryDelay = 7.5s`。
- **proto 兼容性**：生成后跑现有 `grpc reflection`/`transport_test`，确认现有 10 RPC 不变。
- **live 验收（手工，不能进 CI）**：需要一个带 inline 键盘的真实 bot。
  步骤：`ResolveBot` → `GetInlineMenu` → `ClickCallbackButton` → 观察 bot 侧；
  以及 `SendBotCommand("/start")`。注意：这是**在真实账号上代用户点击**，验收不要
  拿去刷第三方 bot（风控），只用自建/可信 bot。
- BDD：不适合（无法自动化创建带键盘的 bot），在 `docs/TEST-MATRIX.md` 记手工验收项。

## 9. 前置工程项

仓库当前**没有** proto 生成脚本（`facade.pb.go` 头：`protoc v7.34.1`、
`protoc-gen-go v1.36.11`、`protoc-gen-go-grpc v1.5.1`；本机全都没有）。
实现第一步必须先补一条可复现命令（`buf generate` 或 `Taskfile.yml` 的 `proto` task +
`go install` 两个 plugin + protoc 二进制/Docker），否则不能改 proto。

## 10. 风险与边界

- **代用户点击**：`getCallbackQueryAnswer` = 用户在该 chat 点按钮，是账号级写操作，
  与 `SendMessage` 同级；gRPC 目前**无鉴权拦截器**（`cmd/facade/main.go:108-112`），
  暴露范围需 owner 明确（内网/本机）。
- **data 会失效**：bot 编辑消息后旧 `callback_data` 无效；调用方应每次经
  `GetInlineMenu` 重新取，不能缓存。
- **不保证按钮类型可点**：url / web_app / switch_inline / callback_game / buy /
  login_url / copy_text / `InlineKeyboardButtonTypeUser`（go-tdlib v0.7.6
  `type.go:15903`；reviewer 原文写作 `UserShared`）等不可执行类型放入
  `InlineButton.other_type`，用 TDLib class 名如实返回；v1 只执行 `callback_data`。
- **bot chat 不能进转发规则**：否则 bot 的回复会被 engine 当普通消息转发（既有行为，
  非本设计引入；在文档/验收里提示）。
- **回复不可见**：点击/发命令后 bot 的新消息本设计不返回（owner 要求），调用方若需要
  自行用既有 `GetMessage`/`GetChatHistory` 处理，与本接口无关。

## 11. 交付顺序（implement 阶段）

1. proto 生成脚本（§9）。
2. `clientAdapter` 四个方法（`SearchPublicChat`、`GetUserFullInfo`、`GetUser`、
   `SendBotStartMessage`）+ 编译期断言。
3. `app/facade` 五个方法 + 单测。
4. proto 五个 RPC + 生成 + `transport` handler/映射/错误码 + 单测。
5. `docs/TEST-MATRIX.md` 手工验收项；`README`/`doc.go` 补文档（沿用 doc.go 约定）。
