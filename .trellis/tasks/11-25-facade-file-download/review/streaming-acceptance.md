# 渐进下载验收

状态：流式实现进行中，尚未发布对应验收镜像。旧 CI run 45630679946
只验证过路径接口；已取消的镜像 run 45640542028 没有更新 dispatch-test。
不要把当前同名 tag 当成本次流式实现。镜像发布后由编排者补充 SHA、digest、version。

## 文件如何到达客户端

TDLib 在 facade 内部缓存下载内容；app 读取可用的连续区间，gRPC 将字节
逐块发送给远程客户端。客户端直接写本机 `<out>.part`，无需访问 facade
的文件系统。只有收到完成帧、干净的 RPC EOF、且可选 SHA256 校验通过后，
才将文件重命名为 `<out>`。失败会保留 `.part` 并返回非零退出码。

客户端不会覆盖已有目标或 `.part`。重试时选择新文件名，或自行检查/处理
原有部分文件。它不实现断点续传。Ctrl+C 和 `-timeout` 结束当前读取；
共享 TDLib 缓存任务可能继续下载，避免取消同一文件的其他消费者。

## 可运行客户端

在该验收镜像对应源提交的仓库根目录运行。客户端只依赖生成的 gRPC
绑定，不依赖 TDLib，Windows/Linux 均可在 CGO_ENABLED=0 下构建运行。
将 facade 的 plaintext gRPC 端口通过现有可信网络或 SSH/容器端口转发
暴露到本机；这里不改变现有服务的认证和网络部署。地址按实际映射填写。

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

实时 Telegram 验收需要可访问的已登录 facade、消息/文件与原始 digest。
在这些输入不可用时，明确保持“实时验收待执行”，不推断通过。

## 发布记录（待补）

- reviewed source SHA：pending
- Linux full CI + A1 red/green run：pending
- amd64/arm64 image smoke run：pending
- dispatch-test manifest digest / dev-run version：pending
- live Telegram first-payload and integrity evidence：pending

发布只走既有 docker-publish workflow，两种架构 smoke 成功后才更新
`ghcr.io/killbus/budva:dispatch-test`。该 tag 可被后续 dispatch 覆盖；
验收和部署应记录 digest 并按 digest 固定镜像。合并需 owner 另外确认。
