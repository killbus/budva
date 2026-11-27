# 渐进下载验收

状态：流式实现、Linux 验证、双架构构建/smoke 和镜像发布均已通过。
GHCR index 与 amd64/arm64 manifest 的源码/版本标注已核验，index 的
实际字节 SHA256 与 digest 一致。版本为 `dev-run23-2af222b`。
旧 CI run 45630679946 只验证过路径接口；真实 Telegram 验收仍待执行。

## proto 是否更新

已随 `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4` 推送到 PR #16，
分支为 `feat/facade-file-download`，尚未合并 main。源文件是
`internal/transport/grpc/pb/facade.proto`；`api/proto/facade.proto` 是指向
它的 Git 符号链接，链接本身无需改动。两个 Go 生成文件也已同步。

```proto
rpc GetMessageFile (GetMessageFileRequest) returns (stream FileDownloadEvent);
rpc DownloadFile (DownloadFileRequest) returns (stream FileDownloadEvent);
```

消息顺序为 metadata、零个或多个 chunk、complete、成功 EOF。
每个 chunk 有 offset、bytes data、upstream_complete；每块最多 256 KiB。
后者是读取前的上游完成状态采样。其他语言客户端应按新版 proto 重新
生成绑定，消费 server stream，而非等待旧版路径响应。原有 Message 的
file_path 字段为兼容保留，不是这两个下载接口的结果。

仅获取对应镜像的 proto（PowerShell）：

```powershell
Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/killbus/budva/2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4/internal/transport/grpc/pb/facade.proto' -OutFile facade.proto
```

## 文件如何到达客户端

TDLib 在 facade 内部缓存下载内容；app 读取可用的连续区间，gRPC 将字节
逐块发送给远程客户端。客户端直接写本机 `<out>.part`，无需访问 facade
的文件系统。只有收到完成帧、干净的 RPC EOF、且可选 SHA256 校验通过后，
才将文件重命名为 `<out>`。失败会保留 `.part` 并返回非零退出码。

客户端不会覆盖已有目标或 `.part`。重试时选择新文件名，或自行检查/处理
原有部分文件。它不实现断点续传。Ctrl+C 和 `-timeout` 结束当前读取；
共享 TDLib 缓存任务可能继续下载，避免取消同一文件的其他消费者。

## 启动验收 facade

将以下内容保存为 `compose.acceptance.yml`，与原 docker-compose.yml 同目录：

```yaml
services:
  facade:
    image: ghcr.io/killbus/budva@sha256:9e4cd5de3f3cacde1110b304921ebf31f0d5d9bec4843d9c9910b6ac4e2221ac
```

沿用仓库 compose 和自己的 `.env`、facade 数据卷：

```sh
docker compose -f docker-compose.yml -f compose.acceptance.yml pull facade
docker compose -f docker-compose.yml -f compose.acceptance.yml up -d --no-deps facade
docker compose exec facade /app/facade --version
```

版本应为 `dev-run23-2af222b`。首次登录沿用现有授权流程；facade
和 engine 的 TDLib 数据库各自使用独立数据卷。客户端连接原有 gRPC
端口 `50051`，无需挂载或复制 facade 内部下载目录。

## 用消息链接取得正确 ID

使用 grpcurl 和刚下载的 facade.proto，替换为该 facade 账户可访问、包含
文件的消息链接与实际服务地址。PowerShell 通过 stdin 传 JSON，避免引号转义：

```powershell
'{"link":"https://t.me/your_channel/123"}' | grpcurl -plaintext -import-path . -proto facade.proto -d '@' localhost:50051 pb.FacadeGRPC/GetMessageLinkInfo
```

取响应的 `message.chatId` 和 `message.id`。它们通常以 JSON 字符串表示，
传给客户端时保留完整整数，不用浮点数转换。若返回 ID 为零或接口错误，
先检查账户访问权限和消息链接；不要直接用链接末尾的 123 替代 TDLib ID。
若已知正确的 chat ID，也可用已有 GetChatHistory 取得 message.id。

## 可运行客户端

在该验收镜像对应源提交的仓库根目录运行。客户端只依赖生成的 gRPC
绑定，不依赖 TDLib，Windows/Linux 均可在 CGO_ENABLED=0 下构建运行。
将 facade 的 plaintext gRPC 端口通过现有可信网络或 SSH/容器端口转发
暴露到本机；这里不改变现有服务的认证和网络部署。地址按实际映射填写。

本机现有功能 worktree（主目录 D:/Repositories/budva 仍是 main）：

```powershell
Set-Location 'D:\Users\ying\.paseo\worktrees\1fum0qdo\feat-facade-file-download'
```

其他机器使用分支 `feat/facade-file-download` 或上述固定源码提交。
需要仓库 go.mod 指定的 Go 工具链；此客户端在宿主机运行，不在 facade
镜像内，也不依赖本机 TDLib。

PowerShell，按消息下载：

```powershell
$env:CGO_ENABLED = '0'
go run ./cmd/facade-file-download-client -addr localhost:50051 -chat-id -1001234567890 -message-id 123456 -out ./download.bin -timeout 10m -expected-sha256 <原始文件的64位SHA256>
```

TDLib 的 message ID 以服务器消息对象返回值为准；不要直接假定 Telegram
链接中的序号就是该 ID。file_id 来自同一 facade TDLib 会话的文件信息。

按 file_id 下载（可省略尚未知的 `-expected-sha256`）：

```powershell
go run ./cmd/facade-file-download-client -addr localhost:50051 -file-id 42 -out ./download-by-id.bin -timeout 10m
```

Linux/macOS 可在命令前加 `CGO_ENABLED=0`；其余参数相同。
程序先输出 file_id/MIME/size_known，然后报告首个有效数据块的耗时、
首块已写字节数以及 `upstream_complete_sample`，最后报告总字节与 SHA256。

如果只想用 grpcurl 检查协议（适合小文件），在含 facade.proto 的目录运行：

```powershell
'{"chat_id":"-1001234567890","message_id":"123456"}' | grpcurl -plaintext -max-time 600 -import-path . -proto facade.proto -d '@' localhost:50051 pb.FacadeGRPC/GetMessageFile
```

grpcurl 输出的是各帧的 JSON，bytes data 显示为 base64；重定向该输出不会
得到原文件。完整性和渐进落盘验收使用上面的专用客户端。

## 验证内容与证据边界

1. 选择本 facade 未缓存的大型普通 Telegram document，先取得其原始字节的
   SHA256。小文件、完整缓存、或在验证前不可读的加密内容，不适合证明首块时序。
2. 下载过程中观察 `.part` 增长，且服务端仍在下载。首块数据必须是真实非空
   payload；仅收到 metadata 不算。检查客户端的首块诊断与独立服务端观察。
3. 完成后检查退出码为 0、目标文件存在、`.part` 消失、字节数和独立 SHA256
   一致。取消或中途失败必须保留部分输出，不能报告完整成功。
4. 首块的 upstream_complete_sample 是服务端读取前的采样，不能单独证明
   远端收到该块的瞬间 TDLib 仍未完成。自动化 A1 另以 fake upstream 的
   完成屏障，穿过真实 app/transport/generated client，确定性验证因果顺序。
   自动化测试不是实测 Telegram；镜像启动 smoke 也不是流式验收。

建议补测：用新输出名重下同一文件以验证完整缓存；下载未缓存大文件时
Ctrl+C，确认只留下 .part 且不报告成功；使用纯文本消息确认返回 NotFound。
可在第二个 PowerShell 窗口用 `Get-Item ./download.bin.part | Select-Object Length`
观察增长，结束后用 `Get-FileHash ./download.bin -Algorithm SHA256` 对比原文件。
原文件请以 Telegram document 方式发送，避免照片/视频压缩改变原始字节。

实时 Telegram 验收需要可访问的已登录 facade、消息/文件与原始 digest。
在这些输入不可用时，明确保持“实时验收待执行”，不推断通过。

## 发布记录

- reviewed source SHA：`2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`
- Linux full CI + A1 red/green run：PASS，https://github.com/killbus/budva/actions/runs/45733805744
- amd64/arm64 image smoke run：PASS，https://github.com/killbus/budva/actions/runs/45734082328
- manifest publication：SUCCESS，2026-11-26T01:46:01Z
- dispatch-test manifest digest：`sha256:9e4cd5de3f3cacde1110b304921ebf31f0d5d9bec4843d9c9910b6ac4e2221ac`
- verified dev-run version：`dev-run23-2af222b`
- linux/amd64 manifest：`sha256:3fd626f51797bc8815725a078139970a05cf852dcf450d635b965d831c4dc8e0`
- linux/arm64 manifest：`sha256:48c56d392f548c4980873e616f639c751d53220471557bf9eaf50c11dcfd9589`
- live Telegram first-payload and integrity evidence：pending

发布只走既有 docker-publish workflow，两种架构 smoke 成功后才更新
`ghcr.io/killbus/budva:dispatch-test`。该 tag 可被后续 dispatch 覆盖；
验收和部署应记录 digest 并按 digest 固定镜像。合并需 owner 另外确认。
