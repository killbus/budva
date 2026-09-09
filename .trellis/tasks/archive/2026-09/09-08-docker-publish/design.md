# Design: Docker image publishing to GHCR

参照 `.trellis/tasks/09-08-docker-publish/research/reproxy-pattern.md` 的 reproxy 骨架；本文只记录 budva 特有的技术设计。

## 架构与边界

```
v* tag push / workflow_dispatch
        │
        ▼
┌─ build job (matrix: amd64@ubuntu-latest, arm64@ubuntu-24.04-arm) ─┐
│ checkout → buildx → GHCR login → 版本计算                          │
│ → docker build (load:true, VERSION build-arg, gha cache)          │
│ → scripts/docker-smoke.sh <image> <version>   ← 发布门禁            │
│ → digest-only push (不可按名拉取)                                    │
│ → 导出本腿 digest 到 job output                                     │
└──────────────────────────────────────────────────────────────────┘
        │ needs: build (双腿绿才继续)
        ▼
┌─ merge job (ubuntu-latest) ────────────────────────────────────────┐
│ imagetools create: 双 digest → manifest list                       │
│ push tag: v<tag> + :latest（push 事件）/ :dispatch-test（dispatch）  │
└──────────────────────────────────────────────────────────────────┘
```

边界：不新增应用代码行为（`--version` 早退除外）；不动 stand；不动现有 test/smoke compose 冒烟。

## 组件设计

### 1. Dockerfile（改动，保持三阶段结构）

- **版本注入**：`ARG VERSION=dev` 顶层声明（build/builder 阶段可用）；三个 `go build` 的 `-ldflags` 追加 `-X main.version=${VERSION}`。facade 与 engine 消费；stand 也注入（同一 build-arg 顺手，无行为差异——它不打印，但注入无害且免特判）。
- **TDLib 提速**：`cmake --build . --target prepare_cross_compiling` 与主构建均加 `--parallel`（不写死核数，让 cmake 用 BuildKit 提供的所有核）。TDLib commit `22d49d5` 已钉死 → 该层缓存稳定。
- **`go build -a` 移除**：`-a` 强制重建所有依赖，3 个二进制重复 3 次全量编译。移除后共享同一 layer 内的构建缓存，一次编译。`-trimpath -ldflags "-s -w"` 保留。
- **`EXPOSE 50051`**：与 gRPC 端口对齐（`GRPC_PORT` 默认 50051，config.go:37）。
- **runtime 层不动**：debian:bookworm-slim + libtd* + ca-certificates 不变——TDLib 是动态链接，distroless static 不可用。

### 2. cmd/facade + cmd/engine 的 `--version`（新增，reproxy 模式）

每 个 main 增加约 25 行：

```go
var version = "dev"   // build 时 -ldflags -X main.version=<tag>

func versionRequested(args []string) bool {
    return len(args) > 0 && args[0] == "--version"
}
```

`main()` 顶部：`if versionRequested(os.Args[1:]) { fmt.Println(version); os.Exit(0) }`——在 envconfig 之前早退（`--version` 不需要也不应触发配置加载/required 校验）。banner 到 stdout。

### 3. scripts/docker-smoke.sh（新增）

`scripts/docker-smoke.sh <image> <expected-version>`，`set -euo pipefail`，单一 EXIT trap 清理。

- **检查 1 版本 banner**：`docker run --rm <image> /app/facade --version` 与 `docker run --rm <image> /app/engine --version` 均精确等于 expected。
- **检查 2 facade 健康**：假凭据（`TELEGRAM_API_ID=1 TELEGRAM_API_HASH=smoke TELEGRAM_PHONE=+10000000000`）起 detached 容器，映射随机高位端口 → 轮询 `/live`（无限条件包裹，30×1s）→ 依次断言 `/live` `/ready` `/healthcheck` 均 200。env 显式 `-e` 传入，优先于镜像内 `.env`。
- **检查 3 engine 启动**：同假凭据起 engine 容器（`-i` 不 attach → stdin EOF，终端 transport goroutine 安全退出），轮询 `docker logs` 出现 `Engine started`（30×1s）。
- TDLib 数据目录指向容器内可写路径（`/app/.data` 已由 WORKDIR/chown 归 appuser）——无需 volume，冒烟即弃。

### 4. .github/workflows/docker-publish.yml（新增，reproxy 骨架）

差异于 reproxy 的点：

- `IMAGE: ghcr.io/killbus/budva`。
- build step 增加 `cache-from: type=gha` / `cache-to: type=gha,mode=max`（reproxy 无依赖故意不用缓存；budva 的 TDLib 层 ~30 分钟/腿，commit 钉死 → 缓存近乎全额命中，这是把双腿构建拉回可接受时长的关键）。
- smoke 调用 `scripts/docker-smoke.sh budva:smoke-${arch} <version>`。
- 其余原样移植：事件判别（push vs dispatch）、版本字符串、digest-only + merge、OCI annotations 一致性、per-arch job output key。

### 5. 杂项

- `.gitattributes`：追加 `*.sh text eol=lf`（Windows 开发者 + CI bash）。
- `.dockerignore`：追加 `.trellis/`、`.github/`、`*.md` 之外保持现状（README 等 md 已含在 `docs/` 排除之外？不——README.md 在根目录未被排除，但 `.dockerignore` 追加 `.trellis/` 与 `.github/` 即可，其余不动）。

## 数据流 / 契约

- 契约 1：`<binary> --version` → stdout 一行版本字符串，exit 0，不触发配置加载。
- 契约 2：`scripts/docker-smoke.sh <image> <version>` → 全过 exit 0；任一失败 exit 1 并打印失败项。
- 契约 3：`v*` tag push → GHCR 上 `ghcr.io/killbus/budva:v<tag>` + `:latest`，amd64+arm64 manifest list；dispatch → 仅 `:dispatch-test`。

## 兼容性

- 现有 `task test:smoke`（test/smoke compose 冒烟）：compose 用 `entrypoint:` 覆盖 + `build:` 本地 Dockerfile——Dockerfile 的 ENTRYPOINT 本就未定义（镜像无 ENTRYPOINT，只有 WORKDIR），行为不变。
- `--version` 早退先于 flag.Parse 与 envconfig → 不影响正常启动路径与 stand 的 `--up/--down`。
- `-a` 移除与 `--parallel` 追加只影响构建时长，不影响产物字节语义（同为 Release 优化构建）。

## 权衡记录

- **缓存用 gha 而 reproxy 不用**：reproxy 零依赖无缓存可吃；budva TDLib 层是 30 分钟级的重复成本，且 Dockerfile 若不动则层缓存键稳定。取舍：gha 缓存有 10GB 限额，TDLib 层 + Go module 层会占用，但单项目足够。
- **stand 不加 `--version`**：发布验收路径不包含 stand；少动一个 main 降低回归面。
- **smoke 不做 compose 变体**（reproxy 有）：budva 的 compose 冒烟已由 `task test:smoke` 覆盖（testcontainers），CI 再做一次是重复；docker-smoke.sh 只做镜像级检查（版本 banner + 假凭据运行）。

## 回滚

全部为新增/追加型改动（新 workflow、新脚本、main 加早退、Dockerfile 参数化），无数据迁移。回滚 = revert 对应 commit；GHCR tag 保留（reproxy 约定：tag 只增不改）。
