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


## Session 3: ci-test workflow: go vet + go test on Linux
<!-- trellis-session: v=2 fp=a3f8c21e7b9d40f1 -->

**Date**: 2026-09-10
**Task**: ci-test workflow: go vet + go test on Linux
**Branch**: `main`

### Summary

Added .github/workflows/ci-test.yml: go vet ./... + go test ./internal/... on Linux for push/PR on main and dispatch. TDLib static libs built once with the Dockerfile recipe (pin 22d49d5, --parallel 2) and cached as a zstd tarball (34 MB) under tdlib-<commit>-amd64; cache split restore/save so red runs still populate it. Verified end-to-end: cold run green in 21m44s (20 packages ok, transform TestAddText_ValidMarkdown passes against real TDLib — confirming the Windows shadow-module failure was a stub artifact, not a code defect), warm run green in 44s, gate proof green-then-red: a deliberate fmt.Printf vet error on scratch branch ci-gate-proof fails the job with the exact diagnostic (run 34422196933); first gate-proof attempt passed because println (builtin) is not in vet's analyzer set — printf-mismatch needs fmt.Printf. User feedback folded in: commit message rewritten from gap-narrative ("Windows cannot compile go-tdlib, so vet+test had no channel") to positive deliverable framing (amend + force-push, saved to memory). Spec updated in docker-ci-guidelines.md (ci-test contracts: cache-key lockstep, save-before-test, runner sudo deviations, glob-cwd pitfall). Push to main: 02908b4, 834e806.

### Git Commits

| Hash | Message |
|------|---------|
| `02908b4` | ci: add go vet + go test workflow on push to main |
| `834e806` | docs(spec): add ci-test workflow contracts to docker-ci guidelines |

### Status

[OK] **Completed**


## Session 4: warmup wait-for-ready: lazy subscription instead of retry-once
<!-- trellis-session: v=2 fp=warmup-wait-ready -->

**Date**: 2026-09-10
**Task**: warmup-wait-ready
**Branch**: `warmup-wait-ready` (iteration) → `main` (squash)

### Summary

Delivered wait-for-ready warmup replacing retry-once: withReady(ctx, chatID, fn) window on "400 Chat not found" with updateNewChat-edge + LoadChats-bootstrap acceleration, fn retry as sole oracle; per-session certificate table behind atomic.Pointer swapped at clientAdapter overwrite; ChatNotReadyError → gRPC Unavailable + RetryInfo(2s); 9 wrappers routed; config WarmupDeadline/Ticker (5s/500ms); tests on testing/synctest virtual time. Design converged over 7 dbs-chatroom rounds. Two production-grade defects found during CI iterations: (1) certificates refuted by the oracle (miss while closed) must be deleted — otherwise the select wakes instantly every iteration and hot-loops fn calls until deadline (chat removed/left after certification); (2) ctx/deadline closeFailed must target the current table, not the one captured before a possible session swap. Test discipline incidents (3 CI round-trips): assert.Zero on *mock.Mock (cannot pass), missing LoadChats expectation under strict mock, success gated on call counter masking a skipped bootstrap (flake landed in CI at 50%) — bootstrap-gated success fixed it; discipline codified in quality-guidelines.md (F1-F3 with incident anchors, red-path proof rule). Structure-audit reviewer agent (step 2.2b) added to workflow; its first finding (duplicated 9-entry wrapper tables) resolved in the same pass. Commit boundary per user policy: branch carried 7 iteration commits; main received one squash (reset --hard 27ea95c + merge --squash + force-with-lease), PR #1 closed with pointer to squash; PR CI green on 5fb285d/dc2ee1b, main CI green on 4f3a805. Note: gh CLI default repo resolved to upstream pure-golang/budva — set-default killbus/budva.

### Git Commits

| Hash | Message |
|------|---------|
| `4f3a805` | feat: TDLib chat warmup wait-for-ready with lazy subscription (squash of 7 branch commits) |

### Status

[OK] **Completed**
