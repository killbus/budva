# Independent design gate record

Date: 2026-11-26. Task: `11-26-facade-message-files`.

## Reviewed artifacts

- Source baseline: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`.
- design.md SHA256: `6f99747c01261442760108e8377199410eea48e535980a8918e74b41b1849059`.
- prd.md SHA256: `8d0dd16fa2a12420d635cd044c8b21c8a2bdf31a4051693d8eaaa062bb5068a7`.
- Profile: Design Reviewer, `codex/remote-magpie/auto/qwen3.8-27b`, auto-review, xhigh.
- Reviewer agent: `05673b68-3109-4b91-89db-94c74991964d`, server `srv_mJRvlG6BX3ck`.
- Report: `review/design-findings.json`.
- Result: round 1 PASS, done=true, D1-D8 met, no blocking findings. No revision round required.

The orchestrator read and validated the report and recomputed both artifact hashes in the feature workspace and main planning mirror. Both copies match the reviewed draft. The reviewer checked the pinned SDK fields, actual baseline interfaces/resolver behavior and proto symbols, existing spec boundaries, and mechanically falsifiable A1-A7 coverage. Separate scratch proto compilation is recorded in draft-validation.md.

## Owner-facing proposed decisions

1. Add metadata-only ListMessageFiles for one message, grouped by logical media item; reuse DownloadFile for one explicitly selected file. Preserve existing GetMessageFile semantics.
2. Include all photo sizes, video alternatives and paired HLS files, file-backed thumbnails/covers, and regular-sticker premium animation within the explicitly supported types. HLS listing does not implement playback or segment retrieval.
3. Paid media preserves one group per original slot. Accessible photos/videos expose files; locked previews expose no file IDs. No purchase/unlock operation. Albums remain separate messages; only the current message's album ID is returned.
4. Retain duplicate file-ID occurrences so their media/role associations remain visible. Positions are snapshot indices, not persistent identities. File IDs are scoped to the current TDLib namespace.
5. Return only known metadata; no guessed file names, no server-local paths, no approximate sizes presented as exact. List metadata is not transferred into the existing ID-based stream's MIME field.
6. Keep the scoped error/context policy and compatibility rules in design sections 7-8; preserve the global mapper and all 17 baseline RPC contracts.

These choices remain explicitly marked assumptions in the reviewed proposal; the gate checks implementability, not that the owner individually selected each choice.

## Phase boundary

The requested design work is complete. Task status remains planning. This record is not approval.md and does not authorize implementation, commit, push, image publication, or merge. No Implementer was dispatched; no production proto/source was changed and no Go/runtime test execution is claimed. The current dispatch-test image contains the previous streaming implementation, not ListMessageFiles.

When the owner selects implementation, record that decision and the reviewed hashes in approval.md and resolve the integration baseline relative to PR16 before activating the Implementer -> Reviewer loop. Do not reinterpret this technical PASS as runtime acceptance or merge authorization.
