# Implementation plan

Baseline: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`. Owner confirmed the independently reviewed proposal on 2026-11-26. Implementation is activated in the existing `feat/facade-file-download` worktree; approval.md records the design lock. No new branch or commit is created by activation.

1. Completed: independent Design Reviewer round 1 PASS, all D1-D8 met, zero blocking findings; owner confirmed the proposal. Preserve the reviewed design and PRD hashes.
2. Record approval and actual baseline; use the existing PR16 worktree without changing its published head. Verify no ruler drift and no blocked/spec-gap artifact. Read implement.jsonl. Since task rulers are not yet committed, validate their recorded hashes and explicitly inventory untracked source files in addition to the baseline diff; do not manufacture a planning commit without authorization.
3. Append proto and generate bindings with the baseline toolchain; keep the api/proto symlink. Add app DTOs and metadata-only enumeration using existing GetMessage and pure default resolver (design sections 4-7).
4. Add unary transport mapping and partial service mock method. Implement exact extraction/metadata/error fixtures and real bufconn list-select-download verification; preserve old streaming behavior (sections 8-10).
5. Run gofmt/diff checks and descriptor compatibility; execute Linux `go vet ./...`, `go test ./internal/...`, and `bash scripts/verify-facade-streaming.sh`. Record exact source and real results. Use strict mocks/literal expectations and a temporary omission mutation for the new enumeration behavior as section 9 requires. Windows static checks do not constitute Linux runtime evidence.
6. Independent Code Reviewer profile checks the locked design against baseline diff; Implementer addresses only findings. Update actual test matrix and acceptance guide with list/select/download examples; separate mock tests from live Telegram evidence.
7. Commit/push/image/merge only under the applicable owner authorization for that implementation stage. For new image acceptance, pin source/digest and test a nondefault photo or video file; verify chosen ID, bytes, complete/EOF and progressive behavior. Do not recycle prior image results as enumeration proof.

Rollback points: before source edits, discard only this ticket's implementation changes; before publication, hold/revert additive endpoint commit; after publication, redeploy prior immutable streaming image with explicit new-RPC Unimplemented impact. Preserve all unrelated working changes and prior task evidence.
