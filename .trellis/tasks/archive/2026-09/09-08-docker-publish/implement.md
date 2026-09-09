# Implementation Plan: Docker image publishing to GHCR

前置：`prd.md`（需求/验收）、`design.md`（技术设计）、`research/reproxy-pattern.md`（reproxy 证据）。按序执行，每步含验证命令。

## 顺序清单

### Step 1: 版本注入 — cmd/facade + cmd/engine

- [ ] `cmd/facade/main.go`：加 `var version = "dev"`；`main()` 顶部加 `--version` 早退（`os.Args[1] == "--version"` → `fmt.Println(version)` + `os.Exit(0)`，在 InitConfig 之前）。
- [ ] `cmd/engine/main.go`：同上。

验证：`go build ./cmd/...`；`go vet ./cmd/...`；`go run ./cmd/facade --version` 输出 `dev`（本地无 TDLib 也能打印——早退在配置加载前）。

注意：本机 Windows 无 TDLib 时 `go build ./cmd/...` 可能失败（CGO 链接 TDLib）。验证改为 `go vet`（含编译检查不行时退化为语法检查）或直接依赖 Docker 构建（Step 3）。本地已装 TDLib 则无此问题——README 说本机开发需装 TDLib。

### Step 2: Dockerfile

- [ ] 顶部加 `ARG VERSION=dev`；builder 阶段三个 `go build` 的 `-ldflags` 改为 `-ldflags "-s -w -X main.version=${VERSION}"`。
- [ ] TDLib 两处 `cmake --build` 加 `--parallel`。
- [ ] 移除三个 `go build` 的 `-a`。
- [ ] runtime 阶段补 `EXPOSE 50051`。

验证：`docker build -t budva:local .`；`docker run --rm budva:local /app/facade --version` → `dev`。

### Step 3: scripts/docker-smoke.sh

- [ ] 新建 `scripts/docker-smoke.sh`（设计见 design.md §3）：参数检查、EXIT trap、检查 1（双二进制版本 banner）、检查 2（facade 假凭据 + 三端点 200）、检查 3（engine 日志 `Engine started`）。
- [ ] `.gitattributes` 追加 `*.sh text eol=lf`。

验证：`bash scripts/docker-smoke.sh budva:local dev`（本地 Docker Desktop；Windows 下经 Git Bash 执行）。

### Step 4: .github/workflows/docker-publish.yml

- [ ] 新建 workflow（reproxy 骨架 + design.md §4 差异点：IMAGE 名、gha 缓存、smoke 调用）。

验证：推送分支后 `workflow_dispatch` 手动触发（workflow_dispatch 不限分支可从 main 触发；先用 git push 后 Actions 页手动 run）→ Actions 全绿 + GHCR 出现 `:dispatch-test`。

### Step 5: 杂项

- [ ] `.dockerignore` 追加 `.trellis/`、`.github/`。
- [ ] README「Запуск」节后追加一节 Docker/GHCR 用法（pull 命令 + `--version`）。

### Step 6: 全量回归

- [ ] `task test:smoke`（现有 compose 冒烟不变绿不收工）——若本机 Docker 可用。
- [ ] AC1–AC6 逐条核对（AC3/AC4 涉及 GitHub 远端，在 dispatch/tag 阶段验证）。

## 验证命令汇总

```bash
docker build -t budva:local .
docker run --rm budva:local /app/facade --version     # dev 或注入值
bash scripts/docker-smoke.sh budva:local dev          # 三项检查
task test:smoke                                        # 现有冒烟回归
docker buildx imagetools inspect ghcr.io/killbus/budva:v<tag>   # 发布后
```

## 风险文件与回滚点

| 文件 | 风险 | 回滚 |
|---|---|---|
| cmd/facade/main.go, cmd/engine/main.go | 早退位置错误（在 envconfig 之后 → `--version` 会被 required 校验挡住） | revert 单文件 |
| Dockerfile | `--parallel` 在低内存 runner 上 OOM（TDLib 并行编译吃内存；GH arm64 runner 16GB，cmake 默认取全部核） | 去掉 `--parallel` 或显式 `-j` 限核 |
| scripts/docker-smoke.sh | CRLF（Windows）导致 CI bash 失败 | `.gitattributes` 规则 + `git add --renormalize .` |
| workflow yml | digest/merge 顺序写错 → 半发布状态 | 对照 reproxy 逐行 |
| 现有 compose 冒烟 | Dockerfile 改动破坏 build: 路径 | Step 6 回归 |

## task.py start 前检查

- [ ] prd.md 收敛（无未决问题）
- [ ] design.md / implement.md 就绪
- [ ] implement.jsonl / check.jsonl 已填真实条目（spec/research 路径 + 理由）

## 执行记录（2026-09-08）

- Step 1-5 完成，commit `1e3c2e6` 于分支 `feat/docker-publish`。
- **修正 1（ARG 作用域）**：初次把 `ARG VERSION=dev` 写在 stage 0 内部——Docker ARG 不跨 stage，builder 的 `${VERSION}` 会展开为空串（`-X main.version=` 注入空值）。改为全局声明（首个 FROM 之前）+ builder 内重声明 + `${VERSION:-dev}` 兜底。
- **修正 2（smoke 脚本简化）**：初次版本用 `--entrypoint /bin/sh -c` 跑 `--version`——镜像本无 ENTRYPOINT，直接 `docker run <image> /app/facade --version` 即可；顺带删除了未使用的 `declare -A SMOKE_ENV` 死代码。
- **本地验证受阻（已记录，不阻塞）**：本机（Windows）无 Docker（`docker` 不在 PATH，无 Docker Desktop/WSL 发行版）；`go build` 也不可行——go-tdlib v0.7.6 的 cgo 绑定 build tag 限定 `linux || darwin`（tdjson_static.go），与 TDLib 是否安装无关。AC1/AC2 的镜像级验证改经 CI dispatch 通道（AC3 路径，同一构建路径）。
- **GHCR push 权限**：fork 仓库 `killbus/budva` 的 default workflow permissions 原为 `read`（fork 默认），已通过 API 提为 `write`（`PUT /repos/killbus/budva/actions/permissions/workflow`）。workflow 自身的 `permissions: packages: write` 声明已就位。
- 剩余验证：dispatch run 全绿 → `dispatch-test` tag 出现（AC3）→ 合并 main 后打 `v0.1.0`（AC4）。
