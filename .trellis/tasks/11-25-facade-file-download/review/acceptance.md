# Facade 文件下载镜像验收

> 暂停使用：用户指出仅返回服务端路径可能不符合下载预期。镜像任务
> `45640542028` 已取消，两个架构均未执行 push，manifest 发布未执行。
> 本次未更新 `dispatch-test`。下面保留的是旧路径接口的验收草案，
> 不能作为客户端接收文件字节的验收标准。

## 镜像与版本

- 源提交：`518bfcc3b167512f089403d1709fb5bd38a6c2e0`，PR #16。
- 该提交已通过 Linux TDLib 冷构建、`go vet ./...`、`go test ./internal/...`。
- 镜像构建：https://github.com/killbus/budva/actions/runs/45640542028
- 预期 tag：`ghcr.io/killbus/budva:dispatch-test`。
- 预期版本输出：`dev-run22-518bfcc`。
- 当前发布状态：已取消，未发布；该 tag 不代表本次文件下载功能。
- `dispatch-test` 会被下一次 dispatch 覆盖。验收时记录 manifest digest，
  需要固定版本时使用 `ghcr.io/killbus/budva@sha256:...`。

## 返回路径的含义

下载方向是 Telegram → facade 所在服务器或容器。RPC 成功后只传回元信息，
包括文件在服务端的位置和大小；RPC 响应不包含文件字节，也不是 HTTP 下载 URL。

例如返回 `filePath=/app/.data/tdlib-files/documents/a.pdf`，表示 PDF 位于
facade 容器里。调用者在其他机器上，不能直接打开自己机器上的这个路径。
可以使用 `docker cp`、共享文件卷或已有的文件服务取得文件内容。

默认下载目录由 `TELEGRAM_FILES_DIR` 控制，默认值 `.data/tdlib-files`；
镜像工作目录为 `/app`。仓库 compose 将 `/app/.data` 挂载到 `facade-data`
命名卷。以实际 RPC 返回的路径为准，不要自行拼接或假定文件名。

如果验收目标是“远端客户端调用 gRPC 后直接接收文件字节”，当前这两个 unary
RPC 尚未覆盖该目标，需要另行增加文件传输接口。

## 1. 拉取并核对版本

以下命令在具有 Docker 的验收机器执行。发布成功后再执行：

```sh
docker pull ghcr.io/killbus/budva:dispatch-test
docker run --rm ghcr.io/killbus/budva:dispatch-test /app/facade --version
docker buildx imagetools inspect ghcr.io/killbus/budva:dispatch-test
```

版本应为 `dev-run22-518bfcc`。manifest 的 revision annotation 应为完整源提交，
并包含 `linux/amd64` 和 `linux/arm64`。版本不符时不要用旧镜像验收本功能。

## 2. 启动 facade

在已有部署的 compose 文件旁创建 `compose.acceptance.yml`：

```yaml
services:
  facade:
    image: ghcr.io/killbus/budva:dispatch-test
```

```sh
docker compose -f docker-compose.yml -f compose.acceptance.yml up -d --no-deps facade
docker compose exec facade /app/facade --version
```

保留原 `.env` 和 facade 数据卷，使用自己的 Telegram 账户。首次授权通过已有
HTTP 授权接口完成：查询 `GET /api/auth/telegram/state`，按状态向
`POST /api/auth/telegram/phone`、`/code` 或 `/password` 提交对应 JSON 字段。
健康检查成功不等于 Telegram 已授权；下载需要账户已授权且能够访问目标消息。
facade 和 engine 的 TDLib 数据库仍须使用各自的数据卷。

## 3. 调用 GetMessageFile

使用该提交的 proto，而非尚未合并的 main 版本。可直接下载：

```sh
curl -fsSL -o facade-file-download.proto https://raw.githubusercontent.com/killbus/budva/518bfcc3b167512f089403d1709fb5bd38a6c2e0/internal/transport/grpc/pb/facade.proto
```

用 grpcurl 调用；下面 JSON 中的占位符需替换为实际 TDLib ID：

```sh
grpcurl -plaintext -max-time 300 -proto facade-file-download.proto -d '{"chat_id":"<chat_id>","message_id":"<tdlib_message_id>"}' localhost:50051 pb.FacadeGRPC/GetMessageFile
```

不要直接把 Telegram 网页链接末尾的数字当作 TDLib message ID。可先通过已有
`GetMessageLinkInfo` 解析消息链接，再用响应中的 `message.chatId` 和 `message.id`：

```sh
grpcurl -plaintext -proto facade-file-download.proto -d '{"link":"<Telegram 消息链接>"}' localhost:50051 pb.FacadeGRPC/GetMessageLinkInfo
```

也可从 `GetChatHistory` 的响应取得消息 ID。大文件需要根据实际网络情况调整客户端
等待时间；客户端超时不能直接证明 TDLib 已取消下载。

成功响应应包含非空 `filePath`、`size` 和 `fileId`。grpcurl 使用 protobuf JSON
字段名，int64 通常显示为字符串。`mimeType` 来自消息内容，缺失时可为空或省略。
照片固定选择最大版本；不支持由调用方选择缩略图。

## 4. 取出文件并校验

```sh
docker compose cp facade:<RPC 返回的 filePath> ./downloaded-file
```

该命令在 Docker 主机上执行时，文件复制到该主机的当前目录；若 Docker 主机与
你的电脑不同，还需用已有的文件传输方式取回。文件名包含空格时给整个源参数加引号。

检查复制出的文件字节数与 `size` 相同，并实际打开文件。再次以响应中的 `fileId`
调用 `DownloadFile`，应返回完整文件的路径和大小：

```sh
grpcurl -plaintext -max-time 300 -proto facade-file-download.proto -d '{"file_id":"<GetMessageFile 返回的 fileId>"}' localhost:50051 pb.FacadeGRPC/DownloadFile
```

`fileId` 是当前 TDLib 实例的标识，使用同一个 facade 会话进行该项验收。

## 5. 失败路径与验收边界

- 文本消息调用 `GetMessageFile`：预期 gRPC `NotFound`，不返回伪造路径。
- `DownloadFile` 的 `file_id=0`：预期 gRPC `InvalidArgument`。
- 下载失败或文件不完整：应返回错误，不能成功返回部分文件路径。
- 建议至少验证一条图片消息和一条文档消息，并检查重复下载。

镜像冒烟测试覆盖版本、facade 健康接口、engine 启动，不连接真实 Telegram
账户。真实文件下载由以上验收步骤确认，不能用冒烟成功代替。
