# Journal - killbus (Part 1)

> AI development session journal
> Started: 2026-09-08

---



## Session 1: docker-publish: multi-arch GHCR image via workflow, v0.1.0 released
<!-- trellis-session: v=2 fp=fca6526339313ee1 -->

**Date**: 2026-09-09
**Task**: docker-publish: multi-arch GHCR image via workflow, v0.1.0 released
**Branch**: `main`

### Summary

Full docker-publish pipeline: multi-arch (amd64+arm64) image to ghcr.io/killbus/budva via two native matrix legs, smoke gate (version banner, facade health endpoints, engine startup log) before digest-only pushes, merge job tags manifest list. Fixed in flight: TDLib --parallel 2 (full-core OOMs 16GB runner at 87%), gha cache removed (48-min blob-upload hangs), Windows exec bit via update-index --chmod=+x. Deployment docker-compose.yml added (per-service volumes for TDLib SQLite locks, ruleset bind for fsnotify hot-reload, stdin/tty for interactive auth). Squash-merged to main as ca992e3, tagged v0.1.0; GHCR :v0.1.0 and :latest verified as amd64+arm64 manifest lists. CI lessons captured in spec/backend/docker-ci-guidelines.md.

### Git Commits

| Hash | Message |
|------|---------|
| `ca992e3` | feat(docker): publish multi-arch image to GHCR via workflow |

### Status

[OK] **Completed**


## Session 2: TDLib chat warmup: lazy LoadChats-on-miss + single retry
<!-- trellis-session: v=2 fp=088ee90b3140a462 -->

**Date**: 2026-09-10
**Task**: TDLib chat warmup: lazy LoadChats-on-miss + single retry
**Branch**: `main`

### Summary

Fixed cold-database 400 Chat not found on facade gRPC: TDLib does not load the chat list after authorization, so the nine chat-scoped adapter wrappers (SendMessage, SendMessageAlbum, ForwardMessages, GetMessage, GetMessages, GetChatHistory, GetMessageLink, GetMessageLinkInfo, GetChat) now run a LoadChats bootstrap on that error and retry once. Singleflight (mutex+cond) plus a 30s min-interval rate limit prevent LoadChats storms; non-matching errors pass through; zero caller changes. Key discoveries: go-tdlib v0.7.6 returns ResponseError by value (errors.As must target the value type, code/message in nested *Error; errors.Is compares the inner pointer so tests share one error instance) and mockery typed-call chains must be RunAndReturn(...).Times(N) since Times/Maybe return plain *mock.Call (compile blocker caught by trellis-check, proven via pure-Go repro). warmup_test.go covers miss/retry, failure, passthrough, 10-goroutine singleflight, rate-limit window, interval resumption, and table-driven all-9-wrappers. Spec captured in .trellis/spec/backend/telegram-tdlib-guidelines.md (7 sections incl. Windows cgo validation constraint). go vet/go test not run locally: go-tdlib is cgo-only, CGO_ENABLED=0, no gcc/Docker/WSL on this machine; verified statically (gofmt, module-cache source reads, compile repro). Needs go vet ./... + go test ./internal/infra/telegram/... in CI/Linux before merge, plus optional manual smoke (fresh TDLib DB + facade GetChatHistory).

### Git Commits

| Hash | Message |
|------|---------|
| `070e2fe` | fix(telegram): warm chat cache on "Chat not found" and retry once |
| `32af7b0` | docs(spec): add telegram/tdlib integration guidelines |

### Status

[OK] **Completed**
