# Docker image publishing to GHCR via GitHub workflow

## Goal

发布 budva 的 Docker 镜像到 GitHub Container Registry（ghcr.io/killbus/budva），通过 GitHub workflow 在 `v*` tag push 时自动构建、smoke 测试、发布。模式参照 `../reproxy` 的 docker-publish（单一构建路径、原生 runner 无 QEMU、digest-only 推送 + manifest 合并、smoke 门禁先于发布）。

## Background / Confirmed facts

仓库现状（侦察已确认）：

- budva 是 CGO + TDLib 项目：Dockerfile 从源码编译 TDLib C++（commit `22d49d5` 钉死），3 个二进制（facade/engine/stand）在 runtime 阶段拷贝。
- 镜像基础来自 `dockerhub.timeweb.cloud`（部署目标是 timeweb VPS，amd64）。已探测该镜像源的 debian:bookworm 与 golang:1.25.9-bookworm 均有 amd64+arm64 manifest list。
- 现有 Dockerfile 无 VERSION 注入、TDLib `cmake --build` 无 `--parallel`（单线程）、`go build -a` 重复全量编译 3 次、缺 `EXPOSE 50051`（gRPC 端口）。
- `cmd/facade` 与 `cmd/engine` 的 main 无 `--version` 早退。
- CI smoke 可行性已验证：
  - `telegramRepo.Start` 立即返回，auth loop 在后台 goroutine（internal/infra/telegram/repo.go:75）；
  - facade 的 `/live` `/ready` `/healthcheck` 为无依赖固定 200（internal/transport/http/health/controller.go:31-59）；
  - engine 打出 `Engine started` 日志不依赖真实 Telegram 连接（cmd/engine/main.go:156）；
  - 终端 transport 在 stdin EOF 时 goroutine 安全退出、主进程存活（internal/transport/term/transport.go:108-111）。
  - → 假凭据（dummy API_ID/HASH/PHONE）即可确定性 smoke：断言本身（health 200、启动日志、版本 banner）不依赖任何外部服务。
- `.env` 处理：`aenv.InitConfig` 内部 `godotenv.Load(".env")`（不覆盖已有 env），镜像内烘焙 `.env.example` → `/app/.env`。smoke 时用显式环境变量即可优先于镜像内 `.env`。
- reproxy 参考实现：`.github/workflows/docker-publish.yml`（双腿矩阵 + digest-only + merge job）、`scripts/docker-smoke.sh`（版本 banner + round-trip + compose）、Dockerfile（`-ldflags -X main.version`）。
- 仓库 killbus/budva 为 PUBLIC fork（arm64 runner 免费，`ubuntu-24.04-arm` 可用）。
- reproxy 的 tag 约定：`v0.2.1` … `v0.5.0`；budva 当前无任何 tag，本次发布需打出第一个 `v*` tag。
- 用户在 Windows 上开发；`.gitattributes` 当前只有 journal union-merge 规则，无 `*.sh` eol 规则 —— 新增 shell 脚本若无 LF 规则，CI bash 会因 CRLF 失败。

## Requirements

- R1: Dockerfile 支持 `VERSION` build-arg，经 `-ldflags -X` 注入二进制（未注入默认 `dev`），`facade --version` / `engine --version` 打印该版本。
- R2: Dockerfile 构建提速修正：TDLib `cmake --build --parallel`；移除 `go build -a`（一次全量编译）；补 `EXPOSE 50051`。
- R3: `scripts/docker-smoke.sh <image> <expected-version>`：容器级 smoke，含 (a) 版本 banner 断言（facade 与 engine 都查）、(b) facade 假凭据起容器后 `/live` `/ready` `/healthcheck` 均 200、(c) engine 假凭据起容器后日志出现 `Engine started`。确定性、无外网依赖、无真实 Telegram 凭据。
- R4: `.github/workflows/docker-publish.yml` 按 reproxy 模式：
  - 触发：`v*` tag push（发布 `:v<tag>` + `:latest`）与 `workflow_dispatch`（仅 `dispatch-test` 测试通道）；
  - 双腿原生 runner 矩阵（amd64 = ubuntu-latest，arm64 = ubuntu-24.04-arm），无 QEMU；
  - 每腿 build → load → smoke 通过 → digest-only 推送；merge job 合并 manifest 并打 tag——任一腿 smoke 失败则无任何可拉取 tag；
  - OCI annotations（revision/version/source）；
  - BuildKit 缓存 `cache-from/to: type=gha`（TDLib 层由 commit 钉死，命中率近乎 100%）。
- R5: `.gitattributes` 增加 `*.sh text eol=lf`；`.dockerignore` 补 `.trellis/`、`.github/`。
- R6: 三个二进制的入口不破坏现有 `docker-compose`（test/smoke/testdata/docker-compose.yml 走 entrypoint 覆盖，`--version` 早退在 flag 解析之前不冲突——stand 的 `--up/--down` 不动）。

## Acceptance Criteria

- [ ] AC1: 本地 `docker build -t budva:local .` 成功，`docker run --rm budva:local /app/facade --version` 与 `docker run --rm budva:local /app/engine --version` 输出注入的 VERSION（未注入时为 `dev`），且 `--version` 不触发配置加载/required 校验。
- [ ] AC2: 本地 `bash scripts/docker-smoke.sh budva:local dev` 三项检查全部通过（版本 banner ×2、facade 三端点 200、engine `Engine started` 日志）。
- [ ] AC3: `workflow_dispatch` 触发的 CI run 全绿，且 GHCR 上出现 `ghcr.io/killbus/budva:dispatch-test`（覆盖式测试通道，无版本 tag）。
- [ ] AC4: 打 `v*` tag 后 CI 全绿，GHCR 出现 `ghcr.io/killbus/budva:v<tag>` 与 `:latest`，`docker buildx imagetools inspect` 显示 amd64+arm64 manifest list，且两个架构的 smoke 均通过（任一失败则无 tag）。
- [ ] AC5: 冒烟流程不使用真实 Telegram 凭据（假凭据仅满足 envconfig required 校验）；TDLib 可能发起连接尝试，但无有效 api_id/hash，不可能完成认证或产生真实账号操作。
- [ ] AC6: 现有 `task test:smoke`（test/smoke compose 冒烟）行为不变。

## Out of scope

- cosign / SBOM / 漏洞扫描（reproxy kill-list 同款排除项）。
- 386 / arm(v7) 等更多架构。
- `stand` 二进制的 `--version`（它是本地测试夹具工具，不进发布验收路径）。
- CI 单元测试 workflow（已有 reproxy ci.yml 模式，但本次范围只有 docker-publish）。
- 镜像自动部署到 timeweb VPS（发布即止，部署另行任务）。

## Open questions

（无——架构范围已定 amd64 + arm64，其余决策见 Key Decisions。）

## Key Decisions

- 架构范围：amd64 + arm64 双腿原生 runner（用户选定；尽管部署目标 VPS 是 amd64，fork 公开仓库 arm64 runner 免费且 TDLib 层有 gha 缓存兜底）。
- BuildKit gha 缓存开（reproxy 因零依赖不用；budva TDLib 层 30 分钟级，必须吃缓存）。
- stand 不加 `--version`（不在发布验收路径）。
- smoke 不做 compose 变体（`task test:smoke` 已覆盖 compose 冒烟，CI 不重复）。
- 发布通道纪律照搬 reproxy：tag 只增不改、dispatch 永不发布版本、任一腿 smoke 失败则无 tag。
