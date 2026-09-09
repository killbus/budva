# reproxy docker-publish 模式调研

来源：`d:\Repositories\reproxy`（同开发者姊妹项目）。本文记录可移植骨架与 budva 特有差异，为 design.md 供证。

## 工作流骨架（.github/workflows/docker-publish.yml）

- 触发：`push: tags: ['v*']`（发布通道）+ `workflow_dispatch`（测试通道，只推覆盖式 `dispatch-test` tag，永不发布版本）。
- 事件判别按 `GITHUB_EVENT_NAME == "push"` 而非 ref 类型——dispatch 在 tag ref 上也必须走测试通道。
- 版本字符串：push = tag 名；dispatch = `dev-run${RUN_NUMBER}-<short SHA>`（不可能与发布 tag 碰撞）。
- 双腿矩阵（每腿原生 runner，无 QEMU）：
  - `linux/amd64` → `ubuntu-latest`
  - `linux/arm64` → `ubuntu-24.04-arm`
  - `arch` 仅作 job output key（`digest-amd64` / `digest-arm64`，各腿只写自己的 key，避免 last-writer-wins 竞态）。
- 发布顺序保证（"无 tag 可拉取直到双腿 smoke 全过"）：
  1. 每腿 build（`load: true`）→ 本地跑 `scripts/docker-smoke.sh` → 通过后以 `outputs: type=image,push-by-digest=true` 推送（digest-only，不可按名拉取）；
  2. merge job（`needs: build`）用 `docker buildx imagetools create` 合并双腿 digest 成 manifest list 并打 tag。
- OCI annotations（revision/version/source）在 build 与 push 两个 step 保持一致（annotations 是 manifest 字节的一部分，不一致会导致第二次构建缓存失效）。
- action 版本：checkout@v7.0.1、setup-buildx-action@v4.3.0、login-action@v4.6.0、build-push-action@v7.3.0。
- kill list（刻意不做，等信号）：cosign、SBOM、漏洞扫描、QEMU、更多架构。

## smoke 脚本骨架（scripts/docker-smoke.sh）

- `set -euo pipefail`，usage 检查，脚本自身定位 repo root（`cd "$(dirname …)/.."`）。
- 单一 EXIT trap 清理所有容器/临时文件（`docker rm -f … || true`）。
- 三项检查：版本 banner（`docker run --rm <image> --version` 精确匹配）、请求 round-trip、compose local-build 变体。
- 环境变量而非模板插值传 tag 名进 shell（防 shell 元字符注入）。
- readiness 探测用 if 条件包裹（`bash -e` 下短路 `&&` 会留下非零状态）。

## Dockerfile 骨架

- `ARG VERSION=dev` + `-ldflags "-s -w -X main.version=${VERSION}"`。
- `main.go` 里 `var version = "dev"` + `--version` 早退（在 flag 解析之前检查 `os.Args[1]`），banner 到 stdout（CI 断言捕获的就是它）。

## budva 差异（移植时必须处理的点）

| 维度 | reproxy | budva |
|---|---|---|
| CGO | 无（零依赖纯 Go，distroless static） | 有（TDLib C++ 源码编译，debian 基底） |
| 二进制 | 1 个 | 3 个（facade/engine/stand） |
| 构建时长 | 分钟级 | TDLib 源码编译 ~30 分钟/腿（无并行时） |
| 版本注入点 | 1 个 main | 2 个 main（facade/engine；stand 不进发布验收路径） |
| smoke 依赖 | 真实公网 https round-trip | 假凭据 + 本地 health 端点（已验证可行，无外网依赖） |
| 基础镜像源 | docker.io | dockerhub.timeweb.cloud（已探测有 amd64+arm64 manifest） |
| 仓库 | killbus/reproxy | killbus/budva（PUBLIC fork，arm64 runner 免费） |

## budva CI-smoke 可行性验证（代码级证据）

- `internal/infra/telegram/repo.go:71-77` — `Start` 只做日志配置 + `go runAuthLoop(ctx)`，立即返回；TDLib 客户端创建在 goroutine 内，失败只记日志不杀进程。
- `internal/transport/http/health/controller.go:44-59` — `/ready` 与 `/health` 遍历 pingers，facade 的 main（cmd/facade/main.go:80）`health.New()` 无参调用 → 零 pinger → 固定 200；`/live` 无条件 200。
- `cmd/engine/main.go:156` — `logger.Info("Engine started, waiting for shutdown signal")` 在所有 repo Start 之后、信号等待之前打出；这些 Start 不依赖网络成功。
- `internal/transport/term/transport.go:108-111` — stdin EOF 时 `ReadLine` 返回错误，goroutine 返回，主进程继续运行（`-i` 不 attach stdin 即触发 EOF）。
- `aenv.InitConfig` — 内部 `godotenv.Load(".env")`（不覆盖已有 env），故显式 `-e` 环境变量优先于镜像内烘焙的 `.env`。
- config.go:22-24 — `TELEGRAM_API_ID/HASH/PHONE` 为 `required:"true"`，smoke 需传假值（任意非空即可过 envconfig）。

## 杂项发现

- budva 无任何 git tag；reproxy 约定 `v0.x.y` 语义化小版本。
- `.gitattributes` 无 `*.sh` eol 规则；Windows 开发 + 新增 .sh 脚本 → 必须 `text eol=lf`，否则 CI bash 报 `\r` 错。
- `.dockerignore` 已排除 docs/、test/、.claude 等，但未排除 `.trellis/`（新增目录，占 build context 体积）与 `.github/`。
- 现有 Dockerfile 的运行时镜像把 `.env.example` 拷成 `/app/.env`（空凭据）——facade 容器凭 dummy env 也能起。
- 现有 compose（test/smoke/testdata/docker-compose.yml）通过 `entrypoint:` 覆盖入口并 `build:` 本地 Dockerfile，与新增 `--version` 早退不冲突（stand 的 `--up/--down` 互斥校验在 flag.Parse 之后，不受影响——不动 stand）。
