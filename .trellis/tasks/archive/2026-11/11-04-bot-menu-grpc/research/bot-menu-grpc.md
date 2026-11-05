# Bot menu over gRPC — research (2026-11-04)

状态：**调研完成，未实现**。本文回答两件事：
1. Budva 的 gRPC 能不能实现 bot menu 操作；
2. 怎么实现才不是“新轮子自实现”。

---

## 0. 结论先行

**能实现，而且不需要自己实现协议层。**

> **2026-11-04 用户已确认语义 = A2**：用户账号在某个 bot 的私聊里操作菜单。
> 因此**选定路线是 A2**；B/C/D/E 保留在本文仅作背景与对照（详见 §8）。
> 结论：只动 Budva 自己的传输层（proto + facade + 一处 clientAdapter），
> 复用已有 `go-tdlib`，不加依赖、不加 token、不加会话。

下面先区分几种“bot menu”，说明为什么必须这样选：

| 解释 | 典型操作 | 当前 gRPC | 推荐轮子 | 可行性 |
|---|---|---|---|---|
| **A. 读 bot 菜单（用户会话视角）** | 读某个 bot 的 commands / menu button | ❌ | `go-tdlib`（已是依赖） | ✅ 最小改动 |
| **A2. 操作用户会话里的 bot 菜单** ← **已选定** | 取 inline 按钮、点 callback、发 `/start` | ❌（`Message` DTO 丢了 replyMarkup） | `go-tdlib`（已是依赖） | ✅ 小改动 |
| **B. 管理自己 bot 的菜单（bot 账号视角）** | set/get/deleteMyCommands、set/getChatMenuButton | ❌ | Bot API HTTP + `go-telegram/bot`（或 `gotgbot`） | ✅ 推荐 |
| **C. 用 TDLib bot 会话管理菜单** | TDLib `setCommands/getCommands/...` | ❌ | `go-tdlib` | ⚠️ 可行但重，不推荐 |
| **D. 用纯 Go MTProto 管理菜单** | MTProto `bots.*` | ❌ | `gotd/td` | ✅ 可行，但不如 B 直接 |
| **E. 插件 ABI 的“操作码”** | 数字/字符串 opcode 分发 | 仓库无此概念 | 无 | ❓需 ABI 规格 |

**关键约束（决定一切）**：
- Budva 的 TDLib 会话是**用户账号**（手机号 + code + password 登录），见
  `internal/infra/telegram/repo.go:362`（`runAuthLoop`）、`internal/config/config.go:24-26`。
- TDLib 的 `setCommands` / `getCommands` / `deleteCommands` / `setMenuButton` /
  `getMenuButton` 官方注释全部是 **“for bots only”**（`go-tdlib@v0.7.6/client/function.go`
  中五个函数的注释；TDLib 文档同）。**在现在的用户会话上调它们必然失败**，不是“补个 RPC”就行。
- 因此“管理 bot 菜单”必须引入 **bot 身份**：bot token（Bot API 或 TDLib bot 登录），
  或者只做用户会话能做的读/操作（A / A2）。

---

## 1. 当前状态（证据）

### 1.1 gRPC 只有 10 个消息类 RPC

`internal/transport/grpc/pb/facade.proto:8-19`：

```
GetMessages, GetChatHistory, SendMessage, SendMessageAlbum, ForwardMessage,
GetMessage, UpdateMessage, DeleteMessages, GetMessageLink, GetMessageLinkInfo
```

没有 menu / command / callback 相关 RPC。生成的 `facade_grpc.pb.go:22-31` 与 proto 一致。

### 1.2 gRPC 的 Message DTO 丢掉了 replyMarkup

`internal/transport/grpc/transport.go:210-239` 的 `clientMessageToProto` 只映射
`id/chat_id/text/forward`。TDLib 的 `Message.ReplyMarkup`（inline keyboard 按钮）
**没有出现在 proto 里**（`facade.proto:21-27` 的 `Message` 也没有该字段）。
所以外部客户端即使只想“看到 bot 菜单按钮并点它”，当前 gRPC 也做不到。

### 1.3 内部其实已经有部分能力

- `clientAdapter` 已含 `GetCallbackQueryAnswer`（`internal/infra/telegram/client_adapter.go:18-52`），
  用于 `AutoAnswer` 转换（`internal/service/transform/service.go:141-155`），
  即“点 callback 按钮并拿回答”的底层调用已经存在。
- `SendMessage` 可以发任意文本，发 `/command` 触发 bot 命令无需新能力。
- `clientAdapter` **没有** `GetUserFullInfo`；读 bot commands/menu button 需要加一个方法。

### 1.4 gRPC 目前没有鉴权拦截器

`cmd/facade/main.go:108-112` 用 `agrpc.NewDefault` 注册，未加 auth interceptor。
现有 `SendMessage` 已经是写操作，所以“写”本身不是新风险；但 bot menu 是
**账号级配置**，一旦暴露公网，任何能连上 :50051 的调用方都能改 bot 菜单。
新增写 RPC 前需要明确网络边界（内网/本机）或加拦截器。

---

## 2. 规范操作集与三套协议映射

### 2.1 标准 bot 菜单 CRUD（Bot API 语义，5 个操作）

| 操作 | Bot API（HTTP） | TDLib（bot 会话） | MTProto（bot 登录） |
|---|---|---|---|
| 设置命令 | `setMyCommands` | `setCommands` | `bots.setBotCommands` |
| 读取命令 | `getMyCommands` | `getCommands` | `bots.getBotCommands` |
| 删除命令 | `deleteMyCommands` | `deleteCommands` | `bots.resetBotCommands` |
| 设置菜单按钮 | `setChatMenuButton` | `setMenuButton` | `bots.setBotMenuButton` |
| 读取菜单按钮 | `getChatMenuButton` | `getMenuButton` | `bots.getBotMenuButton` |

Bot API 语义（官方文档已核对）：
- `setMyCommands`：最多 100 条命令；`scope` + `language_code` 可选。
- `setChatMenuButton`：只对**私聊**或默认按钮生效；`MenuButton` 三选一
  `default | commands | web_app{text,url}`。
- `BotCommandScope` 七种：`default`、`all_private_chats`、`all_group_chats`、
  `all_chat_administrators`、`chat`、`chat_administrators`、`chat_member`。
  TDLib 侧同名 scope 类型齐全（`BotCommandScopeDefault` … `BotCommandScopeChatMember`）。
- 运行时还有一个相关但**不属于菜单 CRUD** 的操作：`answerCallbackQuery`
  （TDLib：`getCallbackQueryAnswer`）——这是“点菜单按钮后回答”，不是“配置菜单”。

### 2.2 用户会话能做的“菜单操作”

用现有用户账号可以：
- `getUserFullInfo(user_id)` → `UserFullInfo.BotInfo`
  （`ShortDescription/Description/MenuButton/Commands/...`）读任意 bot 的菜单；
- 从 `Message.ReplyMarkup` 读 inline 按钮（callback/url/webapp/switch_inline…）；
- `getCallbackQueryAnswer(chat_id, message_id, payload)` 点 callback 按钮；
- `sendMessage` 发 `/start` 或任意 `/command`；
- `sendBotStartMessage(bot_user_id, chat_id, parameter)` 邀请并 `/start`。

这些都**不需要 bot token**，是当前架构的自然延伸。

### 2.3 “操作码”一词

当前 Budva 仓库（main `609f181`）里没有 `opcode` / `BotMenu` / `MenuButton`
任何业务实现或 ABI 定义。全仓（排除 `.trellis/.runtime`）grep 无命中。
如果“操作码”来自外部插件 ABI（例如另一仓库的 plugin ABI），那它属于
**调用方协议映射**，不是 gRPC 传输层能力；不要为了“操作码”在 Budva 里
发明一个 `Execute(opcode, payload)` 通用 RPC（见 §6 反模式）。

---

## 3. 现有轮子盘点（避免自实现）

### 3.1 协议/客户端层：成熟，直接用

| 轮子 | 覆盖 | 状态（2026-11-04） | 评价 |
|---|---|---|---|
| `github.com/zelenin/go-tdlib` v0.7.6（已在 go.mod） | 5 个 menu 函数 + `UserFullInfo.BotInfo` + `GetCallbackQueryAnswer` + `CheckAuthenticationBotToken` | 543★，2026-05 更新 | **首选（用户会话 / TDLib bot 会话）**；已编译进项目 |
| `github.com/gotd/td` | `BotsSet/Get/ResetBotCommands`、`BotsSet/GetBotMenuButton`；`auth.Bot(token)` | 2376★，2026-10 更新 | 纯 Go MTProto，无 CGO；适合“不能走 Bot API HTTP 且不想加第二个 TDLib”的场景 |
| `github.com/go-telegram/bot` | Bot API 5 个方法全支持（`methods.go:561-631`） | 1875★，2026-11-02 更新 | **首选（Bot API CRUD）**；活跃、context-aware |
| `github.com/PaulSonOfLars/gotgbot` | Bot API 全量 | 722★，2026-10 更新 | 备选 |
| `github.com/go-telegram-bot-api/telegram-bot-api` | Bot API 全量 | 6409★，但 **2024-08 后未更新** | 能用但偏陈旧，不建议新代码引入 |
| `PaulSonOfLars/telegram-bot-api-spec` | Bot API 的机器可读 JSON 规格 | 150★，2026-08 更新 | 只在需要自生成类型/文档时用 |

### 3.2 gRPC 层：**没有可复用的轮子**

全网检索结果（GitHub API + Sourcegraph，2026-11-04）：

| 项目 | 结果 |
|---|---|
| `x0rium/tdlib-grpc` | 0★，仓库 size=0（空），无 README |
| `StromenkoNikita/telegram_bot_grpc` | 0★，Java/Maven，5 KB 玩具工程 |
| `sb-elliot-7s/grpc-telegram-bot` | 5★，是“bot 去调别的 gRPC 服务”，不是 Telegram gRPC facade |
| Sourcegraph `"setChatMenuButton" lang:protobuf` | **0 命中** |
| Sourcegraph `"getMyCommands" lang:protobuf` | **0 命中** |
| Sourcegraph `"rpc SetMyCommands"` | **0 命中** |
| Sourcegraph `file:.*\.proto "BotCommandScope"` | **0 命中** |

结论：**gRPC 这一层不存在成熟可依赖的项目**。因此“写一个薄 gRPC adapter”
不是在造轮子——轮子在客户端库里；adapter 只是把已有库的 5 个方法映射成
proto，属于本项目本来就必须有的传输层代码。反过来，**任何自己实现 Bot API
HTTP 客户端 / TDLib JSON 协议 / MTProto 序列化**才是重复造轮子，必须禁止。

---

## 4. 方案对比（含落点与成本）

### 方案 A — 只读：用户会话读 bot 菜单

**做什么**
- `clientAdapter` 加 `GetUserFullInfo`（1 行接口 + `Repo` 转发）。
- `internal/app/facade` 加 `GetBotInfo(userID)`，把 `BotInfo` 映射成 DTO。
- proto 加 `GetBotInfo` RPC + `BotInfo/ BotCommand / BotMenuButton` 消息。
- 不动鉴权、不加 token、不加会话。

**成本**：0.5–1 天（含 proto 生成 + 测试）。
**风险**：低。只读，账号语义不变。
**适用**：只需要“看见某个 bot 的菜单是什么”。

### 方案 A2 — 用户会话操作菜单（读按钮 + 点按钮）

**做什么**
- `Message` proto 增加 inline keyboard 结构（或单独 `GetMessageMenu` RPC）。
- 新增 `ClickMessageButton(chat_id, message_id, data)` → 复用已有
  `GetCallbackQueryAnswer`（`transform` 已在用）。
- 可选 `SendBotStartMessage`（或直接让调用方用 `SendMessage` 发 `/start`）。
- 同 A，不需要 token。

**成本**：1–2 天。
**风险**：低–中（新增写操作；`GetCallbackQueryAnswer` 有 502 超时语义，
需要映射成 gRPC `DeadlineExceeded/Unavailable`）。
**适用**：要“驱动 bot 的 inline 菜单”，而不是“配置 bot 菜单”。
**注意**：`GetCallbackQueryAnswer` 要求消息未被删除且 callback 未过期，
长轮询语义与现有 gRPC 的同步模型兼容，但要设超时。

### 方案 B — Bot API HTTP：管理自己 bot 的菜单（推荐）

**做什么**
- 新 infra 包（例如 `internal/infra/botapi`）包 `go-telegram/bot`，只暴露 5 个方法。
- 新 gRPC service（建议独立 `BotMenuGRPC`，不要塞进 `FacadeGRPC`：
  `FacadeGRPC` 的语义是“用户账号的消息操作”，bot 菜单是“bot 账号的配置”）。
- proto：5 个 RPC + `BotCommand`、`BotCommandScope`（oneof）、`MenuButton`（oneof）。
- 配置：`TELEGRAM_BOT_TOKEN`（**可选**，为空则服务返回 `FailedPrecondition`，
  不影响现有部署）；`TELEGRAM_BOT_API_BASE_URL`（默认 `https://api.telegram.org`，
  测试/自建 Bot API Server 可覆盖）。
- `cmd/facade/main.go` 注册新 service；token 为空时跳过注册或注册报错桩。

**成本**：1–2 天（含 proto 生成、httptest 假 Bot API、错误映射）。
**风险/注意**
- 需要 **HTTPS 出网到 `api.telegram.org`**；现有 Budva 走 MTProto DC，网络策略
  可能不同，部署前要确认。若不允许 HTTPS，改走方案 D。
- Bot API 有 429 `retry_after`；5 个方法都是幂等配置写，重试安全。
- 命令上限 100；`scope`/`language_code` 组合多，proto 必须用显式结构，
  不要 `map<string,string>` 糊弄。
- 与用户会话完全隔离：不会碰 TDLib 的 `.data` 目录和排他锁。

**为什么推荐**：Bot API 是“bot 菜单”的**规范来源**；`go-telegram/bot` 是活跃轮子；
gRPC 层只是薄映射；没有第二个 TDLib CGO 客户端的内存/存储/鉴权负担。

### 方案 C — TDLib bot 会话

**做什么**：再加一个 `Repo` 实例，用独立 `DatabaseDirectory`，认证走
`CheckAuthenticationBotToken`（`go-tdlib` 已支持），然后调 5 个 bot-only 函数。

**成本**：2–4 天。
**风险**：中–高。
- 同进程两个 CGO TDLib 客户端（本机有 OOM 约束，见项目环境说明）。
- 需要改造现有 `runAuthLoop`（当前只处理 phone/code/password）。
- 独立 SQLite 目录 + 独立排他锁 + 独立生命周期。
- 收益只是“复用 go-tdlib”，但 Bot API 已经更直接。

**结论**：除非**硬性禁止 HTTPS Bot API 且不想引入 gotd**，否则不做。

### 方案 D — gotd/td bot 会话

**做什么**：新依赖 `gotd/td`，`auth.Bot(token)` 登录，调 5 个 `Bots*` 方法，
薄 gRPC adapter。纯 Go，无 CGO，不占 TDLib 存储。

**成本**：2–3 天。
**风险**：中（新依赖、新会话生命周期、MTProto 错误/flood 语义要映射）。
**适用**：Bot API HTTP 被网络策略禁止，且不希望增加第二个 TDLib。

### 方案 E — 插件 ABI 操作码

如果外部 ABI 要求“opcode → 操作”的分发，正确做法是：
1. 先在 ABI 规格里定义 opcode ↔ 上述 5 个操作的映射；
2. Budva gRPC 仍然暴露**强类型 RPC**；
3. 由 ABI 宿主（或调用方）做 opcode 到 RPC 的翻译。
不要在 Budva 里做 `rpc Execute(ExecuteRequest{opcode:int32, payload:bytes})`。

---

## 5. 决策树

```
你要的是哪一种？
├─ 读某个 bot 的菜单（不修改）───────────────→ A（最小，无 token）
├─ 驱动用户会话里的 inline 菜单（点按钮）──────→ A2 ← 【用户已选定】
├─ 修改“自己的 bot”的命令/菜单按钮
│   ├─ 允许 HTTPS 到 api.telegram.org ───────→ B（推荐）
│   ├─ 不允许 HTTPS，但可加纯 Go 依赖 ───────→ D（gotd/td）
│   └─ 只能用 TDLib ─────────────────────────→ C（不推荐，重）
└─ 外部插件 ABI 的 opcode ───────────────────→ E（先定 ABI 映射，传输层仍用 A/B）

选定 = A2：用户账号在 bot 私聊里的菜单交互（读命令菜单 / 读 inline 按钮 / 点 callback /
发命令）。B/C/D/E 不在本轮范围。
```

---

## 6. 反模式（明确不要做）

1. **不要自己写 Bot API HTTP 客户端**（手拼 URL/JSON/错误码）：
   已有 `go-telegram/bot` / `gotgbot`。
2. **不要自己实现 MTProto/TDLib JSON 协议**：`go-tdlib` 已封装；`gotd/td` 是纯 Go 备选。
3. **不要发明通用 `Execute(opcode, payload)` RPC**：丢掉类型安全，偏离现有
   “一操作一 RPC”的 facade 风格；opcode 映射放调用方/ABI 层。
4. **不要在现有用户会话上调 TDLib 的 bot-only 5 函数**：必然失败，且错误信息
   容易误导（会被当成“功能没实现”）。
5. **不要引入 0★ 的 Telegram gRPC 项目**：检索到的三个要么空仓库、要么玩具、
   要么不是 facade。
6. **不要把“bot 账号配置”（方案 B）混进 `FacadeGRPC`**：那是 bot 账号语义，
   与 `FacadeGRPC` 的用户账号语义不同，需要独立 service + 独立 token 开关。
   注意：**本任务选定的 A2 是用户账号语义，反而应当加进 `FacadeGRPC`**。

---

## 7. 如果选 B：proto 草图与测试策略

```proto
service BotMenuGRPC {
  rpc SetBotCommands   (SetBotCommandsRequest)   returns (EmptyResponse);
  rpc GetBotCommands   (GetBotCommandsRequest)   returns (BotCommandsResponse);
  rpc DeleteBotCommands(DeleteBotCommandsRequest) returns (EmptyResponse);
  rpc SetBotMenuButton (SetBotMenuButtonRequest) returns (EmptyResponse);
  rpc GetBotMenuButton (GetBotMenuButtonRequest) returns (BotMenuButtonResponse);
}

message BotCommand { string command = 1; string description = 2; }

message BotCommandScope {
  oneof scope {
    bool default = 1;
    bool all_private_chats = 2;
    bool all_group_chats = 3;
    bool all_chat_administrators = 4;
    int64 chat_id = 5;              // scope=chat
    int64 chat_administrators_id = 6;
    ChatMemberScope chat_member = 7;
  }
}

message BotMenuButton {
  oneof button {
    bool default = 1;
    bool commands = 2;
    WebAppButton web_app = 3;
  }
}
```

实现要点：
- `language_code` 原样透传（空串 = 默认）。
- 错误映射：参数非法 → `InvalidArgument`；token 缺失 → `FailedPrecondition`；
  Bot API 401 → `Unauthenticated`；429 → `ResourceExhausted` + `RetryInfo`；
  网络/5xx → `Unavailable`。
- 测试：`httptest.Server` 假 Bot API（断言 JSON body + 返回 fixture），
  CI 不需要真 token；与现有 `transport_test.go` 风格一致。
- 生成工具：仓库当前**没有** protoc 脚本（`protoc v7.34.1`、`protoc-gen-go v1.36.11`、
  `protoc-gen-go-grpc v1.5.1`，见 `facade.pb.go` 头）。落地时要先补一条可复现的
  generation 命令（`go install` 两个 plugin + protoc 二进制，或用 Docker），
  否则手改生成文件不可接受。

---

## 8. 已选定语义（2026-11-04 用户确认）：用户侧 bot chat 菜单交互

> 用户明确：**“我作为一个用户，需要在一个提供了操作 menu 的 bot 的 chat 中进行
> menu 交互。”**

即：**用户账号**在自己的 TDLib 会话里，和某个 bot 的私聊中读取并操作它提供的菜单。
**不需要 bot token，不需要第二个会话，不需要 Bot API。** 对应 §4 的方案 **A2**。

### 8.1 “menu” 的三种形态（必须分开处理）

| 形态 | 用户看到的 | TDLib 读法 | TDLib 操作 |
|---|---|---|---|
| **命令菜单**（输入框旁的菜单按钮 / `/` 列表） | bot 提供的命令列表 | `getUserFullInfo(user_id).BotInfo.Commands`；`BotInfo.MenuButton` | 没有“按菜单按钮”的 API；要触发就 `sendMessage` 发 `/command`（或 `sendBotStartMessage` 发 `/start`） |
| **消息内 inline keyboard** | 消息下方的按钮行 | `Message.ReplyMarkup.(*ReplyMarkupInlineKeyboard)` | callback 按钮：`getCallbackQueryAnswer(chat_id, message_id, data)`；url 按钮：只返回 URL，由调用方打开 |
| **Web App 菜单**（menu button 或 web_app 按钮） | 打开的 WebView | `BotInfo.MenuButton.Url` / `InlineKeyboardButtonTypeWebApp.Url` | `openWebApp` / `getMainWebApp` 只返回 `WebAppInfo{LaunchId, Url}`；**真正的交互需要 WebView + JS bridge，gRPC 服务器无法代替** |

**v1 范围建议**：命令菜单 + inline callback 按钮；Web App 菜单明确**不做**（或只返回
“需要 WebView”的信息），否则等于在服务端实现一个浏览器，严重跑题。

### 8.2 现有内部能力与缺口

已有：
- `GetCallbackQueryAnswer` 已在 `clientAdapter`（`internal/infra/telegram/client_adapter.go:18-52`），
  且 `AutoAnswer` 已在用（`internal/service/transform/service.go:141-155`）。
- `SendMessage` 已可发 `/command`；`GetMessage` 可读单条。
- 点击按钮的“回答”由 `GetCallbackQueryAnswer` **同步返回**；点击引起的消息变化
  （bot 改消息/发新消息）走 TDLib update：`UpdateMessageEdited` / `UpdateNewMessage`
  ——前者在 `isRelevantUpdate`（`repo.go:515-530`）白名单里，但 facade 以
  `NoBusinessUpdates` 启动（`cmd/facade/main.go:80`），`updates` 通道为 nil，
  所以 facet 侧目前**没有任何途径**把点击结果推给调用方。
  （注：`UpdateNewCallbackQuery`（`type.go:55448`）是发给 **bot** 的更新，用户会话收不到。）

缺口（都在 A2 范围内，无需 token）：
1. `Message` proto 丢失 `ReplyMarkup`（`transport.go:210-239`）→ 外部看不见按钮。
2. 没有“取 bot 信息/命令列表”的 RPC（`GetUserFullInfo` 未进 `clientAdapter`）。
3. 没有“点 callback 按钮”的 RPC（底层已有）。
4. 没有“在 bot chat 里发命令”的语义化 RPC：现在只能裸 `SendMessage`，调用方必须
   自己知道 `chat_id`。而 bot 私聊的 `chat_id` 并不是 `bot_user_id`（`User.Type ==
   UserTypeBot` 只给 user_id），需要 `searchPublicChat(username)` 或
   `createPrivateChat(user_id)` 先解析出私聊 chat。缺一个 `ResolveBot` 入口。
5. callback 按钮的 `ShowAlert` 语义（`CallbackQueryAnswer.ShowAlert`）在
   gRPC 返回里没有承载；`GetCallbackQueryAnswer` 对未及时回答的 bot 返回 502
   需要映射成 gRPC 状态。

### 8.3 关键风险（实测前已知）

- **`getCallbackQueryAnswer` 的语义**：这是 TDLib 给用户侧客户端的官方接口
  （源码注释：“Sends a callback query to a bot and returns an answer”），
  内部替客户端发 MTProto `messages.getBotCallbackAnswer`。它等价于“用户在这个
  bot chat 里点了这个按钮”，属于普通账号动作（和 `SendMessage` 同级），不需要
  额外特权、也不算伪装；风险只在 BDD/验收层面：它是**代用户点击**，自动化测试里
  不要用它去刷别人的 bot（可能触发 bot 侧限流或风控）。
- **按钮只能点一次**（`data` 指向 bot 侧状态）：连续点击同一按钮、或消息已被
  bot 编辑（`UpdateMessageEdited` / `UpdateMessageContent`）后，旧 `data` 会失效。
  设计要允许“按 message_id + 当前 ReplyMarkup 重新取按钮”，不能缓存 data。
- **必须在 bot chat 里调用**：`getCallbackQueryAnswer` 对非 bot chat 返回错误；
  且 `chat_id` 必须是用户与 bot 的 **private chat** id（`createPrivateChat(BotUserId)`，
  如未建立可先 `searchPublicChat(@username)`）。不要把 `bot_user_id` 当 `chat_id` 传。
- **callback 数据没有“按钮索引”概念**：TDLib 只认 `data []byte`。若 bot 在不同
  消息/版本里复用同一 data，语义由 bot 决定——API 应**显式**要求调用方传 `data`，
  而不是传按钮序号去“猜”。
- **无读回保证**：点击后 bot 的回答可能是一张新消息（`UpdateNewMessage`），
  也可能是就地编辑原消息（`UpdateMessageEdited`），也可能只是 alert
  （`GetCallbackQueryAnswer` 的返回）。**不能假设一定是哪一种**。
- **与 ruleset / webhook 的交互**：点击 RPC 本身只是同步请求，不注入事件；但点击
  引发的 bot 回复是普通 TDLib update，engine 按现有 ruleset 处理——若该 bot chat
  恰好在 `sources`/`forwardRules` 里，这些回复会被正常转发/推送。这是规则配置问题，
  不是新机制；设计时要在文档里提示“别把交互用的 bot chat 加进转发规则”。

### 8.4 落地改动点

`getCallbackQueryAnswer` / `getUserFullInfo` 在 go-tdlib 里都是**同步 RPC**
（`client.Send` 内部即发请求并等响应），不需要 `pendingSends`/`sendMessageAndWait`
那套临时 message-id 机制（那套是为 `SendMessage` 这种“结果晚到”的调用准备的）。
因此改动很薄：
- `clientAdapter`（`client_adapter.go:18-52`）加 `GetUserFullInfo`；
  `SendMessage`、`GetCallbackQueryAnswer`、`GetChatHistory`、`GetMessage` 已在。
- `Repo` 只需最薄的转发（可选：复用 `parseFloodWait` 做 flood-wait 重试）。
- `app/facade.Service` 加 `GetBotInfo`、`GetInlineMenu`、`ClickCallbackButton`、
  `SendBotCommand`（`internal/app/facade/service.go`）。
- `transport/grpc.Transport` 加对应 RPC；因为这是**用户账号自己的会话**，
  与现有 `FacadeGRPC` 语义一致，**默认加进 `FacadeGRPC`**（不必新开 service）。
- proto 真实文件只有 `internal/transport/grpc/pb/facade.proto`；
  `api/proto/facade.proto` 是一个 45 字节的**指针文件**（内容就是那行相对路径），
  改 proto 只动前者。生成工具见 §8.6。

### 8.5 对外接口建议（草图，供 owner 审阅，不实现）

以下 RPC 加进现有 `FacadeGRPC`（用户会话语义一致）；若 owner 更想分组，也可
单独 `BotChatGRPC`，但不改变实现。

```proto
// 追加到 service FacadeGRPC
rpc ResolveBot         (ResolveBotRequest)         returns (ResolveBotReply);
rpc GetBotInfo         (GetBotInfoRequest)         returns (BotInfoReply);
rpc GetInlineMenu      (GetInlineMenuRequest)      returns (InlineMenuReply);
rpc ClickCallbackButton(ClickCallbackButtonRequest) returns (CallbackAnswerReply);
rpc SendBotCommand     (SendBotCommandRequest)     returns (EmptyResponse);

message ResolveBotRequest { string username = 1; }   // 不带 @
message ResolveBotReply {
  int64 chat_id = 1;      // 与 bot 的 private chat id（用于 SendMessage/GetChatHistory）
  int64 bot_user_id = 2;  // 用于 GetBotInfo / SendBotStartMessage
}

message BotCommand { string command = 1; string description = 2; }
message BotMenuButtonInfo { string text = 1; string url = 2; }
message BotInfoReply {
  int64 bot_user_id = 1;
  repeated BotCommand commands = 2;
  BotMenuButtonInfo menu_button = 3;
  string description = 4;
  string short_description = 5;
}
message GetBotInfoRequest { int64 bot_user_id = 1; }   // 或 string username = 1;

message GetInlineMenuRequest {
  int64 chat_id = 1;      // 必须是与 bot 的 private chat
  int64 message_id = 2;   // 0 = 最近一条 bot 消息
}
message InlineButton {
  string text = 1;
  oneof action {
    bytes callback_data = 2;
    string url = 3;
    WebAppButton web_app = 4;    // 仅返回 url，v1 不执行
  }
}
message InlineButtonRow { repeated InlineButton buttons = 1; }
message InlineMenuReply { repeated InlineButtonRow rows = 1; }   // 保留行列结构

message ClickCallbackButtonRequest {
  int64 chat_id = 1;
  int64 message_id = 2;
  bytes callback_data = 3;   // 显式传 data，不传按钮序号
}
message CallbackAnswerReply {
  string text = 1;
  bool show_alert = 2;
  string url = 3;
  string message_text = 4;   // 可选：click 后“原消息新的文本”，无则空
}

message SendBotCommandRequest {
  int64 bot_user_id = 1;   // 或 string username
  string command = 2;      // "/start"
  string parameter = 3;    // 仅 /start 的 deep-link 参数，其余传空
}
```

**注意**：inline keyboard 是**行 × 按钮**的二维结构，`InlineMenuReply` 必须按
`rows → buttons` 保留，不能压平成一维列表（否则调用方无法还原布局）。

**响应式读取的缺口**：`ClickCallbackButton` 只返回 callback answer；bot 之后改消息
/发新消息不会推给 gRPC。v1 让调用方用现有 `GetMessage`/`GetChatHistory` 轮询；
若确需服务器推送，再单开 `WatchChat(chat_id) returns (stream ChatEvent)`，把 facade
切成 `BusinessUpdates` 模式并做单通道 fan-out（属独立设计，不在本任务）。

### 8.6 proto 生成工具（当前缺失，落地前必须补）

仓库没有 `protoc` 脚本（`facade.pb.go` 头写 `protoc v7.34.1`、`protoc-gen-go v1.36.11`、
`protoc-gen-go-grpc v1.5.1`，本机 **全都没有**）。落地前必须先补一条可复现的生成命令：
- 用 `buf generate`（加 `buf.gen.yaml`）；或
- 在 `Taskfile.yml` 加 `proto` task：`go install` 两个 plugin + 安装 `protoc`
  二进制（或走 Docker）；
然后才能改 `internal/transport/grpc/pb/facade.proto` 并重新生成。
真源只有 `internal/transport/grpc/pb/facade.proto`；`api/proto/facade.proto` 是指针文件
（内容即 `../../internal/transport/grpc/pb/facade.proto`），不要把它当第二个真源。

---

## 9. 参考

- Bot API 官方方法：<https://core.telegram.org/bots/api#setmycommands>、
  `#getmycommands`、`#deletemycommands`、`#setchatmenubutton`、`#getchatmenubutton`
  （本地抓取核对：`setChatMenuButton` 仅私聊/默认按钮；命令上限 100）。
- TDLib `setCommands/getCommands/deleteCommands/setMenuButton/getMenuButton`
  文档注释均为 “for bots only”；`go-tdlib@v0.7.6/client/function.go`。
- TDLib `UserFullInfo.BotInfo`（`BotMenuButton`、`Commands`）：
  `go-tdlib@v0.7.6/client/type.go`（`BotInfo`、`UserFullInfo`）。
- `gotd/td`：`tg/tl_bots_set_bot_commands_gen.go`、`tg/tl_bots_get_bot_commands_gen.go`、
  `tg/tl_bots_reset_bot_commands_gen.go`、`tg/tl_bots_set_bot_menu_button_gen.go`、
  `tg/tl_bots_get_bot_menu_button_gen.go`；`telegram/auth/bot.go`（`Bot(token)`）。
- `go-telegram/bot`：`methods.go:561-631`（5 个方法）、`models/bot_commands_scope.go`、
  `models/menu_button.go`。
- 仓库现状：`internal/transport/grpc/pb/facade.proto:8-19`、
  `internal/transport/grpc/transport.go:210-239`、
  `internal/infra/telegram/client_adapter.go:18-52`、
  `internal/config/config.go:24-26`、`cmd/facade/main.go:108-112`。
- gRPC 层检索：GitHub API（`x0rium/tdlib-grpc` 空仓库 0★；
  `StromenkoNikita/telegram_bot_grpc` 0★ Java 玩具；
  `sb-elliot-7s/grpc-telegram-bot` 5★ 非 facade）+ Sourcegraph 多组 proto 查询 0 命中。

选定路线（A2）用到的 TDLib 接口，均在 `go-tdlib@v0.7.6/client`：

- `searchPublicChat`（function.go:1727）、`createPrivateChat`（:8304）——解析 bot 私聊 chat。
- `getUserFullInfo`（:1124）→ `UserFullInfo.BotInfo`（type.go:9532、`BotInfo` :9410）。
- `getCallbackQueryAnswer`（:7459）→ `CallbackQueryAnswer{Text, ShowAlert, Url}`（type.go:35853）。
- `sendBotStartMessage`（:3885）、`sendMessage`（已在用）。
- `ReplyMarkupInlineKeyboard`（type.go:16094）、`InlineKeyboardButton`（:15957）及按钮类型
  （callback :15751、url :15666、web_app :15724、switch_inline :15830、callback_game :15805…）。
- Web App 菜单（v1 不做，供完整性）：`openWebApp`（:7337）→ `WebAppInfo{LaunchId, Url}`（:16336）、
  `getMainWebApp`（:7234）、`sendWebAppData`（:7289）。
- 点击后 TDLib 侧更新：`UpdateMessageEdited`（type.go:51119）、`UpdateNewMessage`（:50951）。
