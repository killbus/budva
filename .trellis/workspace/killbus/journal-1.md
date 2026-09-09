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
