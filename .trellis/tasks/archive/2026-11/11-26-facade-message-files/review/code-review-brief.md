# Independent code-review brief

Task: `.trellis/tasks/11-26-facade-message-files`.
Baseline: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4` on the existing feature worktree.
Use the configured Reviewer profile and its full architecture-review rules. This brief supplies task-specific placement and evidence limits; it does not replace the profile.

## Ruler and scope

Read approval.md, design.md, prd.md, check.jsonl and referenced spec/research files. Design SHA256 is `6f99747c01261442760108e8377199410eea48e535980a8918e74b41b1849059`; PRD SHA256 is `8d0dd16fa2a12420d635cd044c8b21c8a2bdf31a4051693d8eaaa062bb5068a7`. The orchestrator mechanically verifies these before dispatch. They are untracked because no planning commit was authorized; untracked does not mean mutable or absent.

Enumerate both tracked changes and new files. In particular these four source files are not in git diff:

- internal/app/facade/message_files.go
- internal/app/facade/message_files_test.go
- internal/transport/grpc/message_files.go
- internal/transport/grpc/message_files_test.go

Read actual production wiring from cmd/facade through the registered generated server and Transport to Service and the existing Telegram repository. Design section 10 is the closed source-file set. Historical dirty documents under 11-25 predate this task and are outside its scope; do not rewrite or misattribute them.

## Review focus

Check metadata-only enumeration, one existing GetMessage call, grouping and occurrence identity, pinned SDK fields, no album expansion, locked/nil/unsupported paid slots, default resolver compatibility, and no inferred local paths or auxiliary MIME. Confirm the new handler maps app data without taking on media extraction, handles cancellation locally, and does not widen the repository surface or change stream orchestration.

Compare the additive proto and generated bindings with all 17 legacy RPCs, existing messages and stream flags. Inspect the assertions in new tests, including real List -> selected returned descriptor ID -> progressive DownloadFile flows for photo and alternative video. No test-only production behavior or weakened old assertion is acceptable.

## Evidence and ownership

Read review/responses.json and evidence.md as Implementer reports, not independent facts. Orchestrator runtime results are in preflight-evidence.md, validation-environment.md and later linux-validation.md when present; distinguish authentic native tests from static checks and prior published-image evidence. Test success is not architecture compliance.

Only write this task's review/findings.json. Do not change implementation, rulers, specs, older reports, publication state or git history. Apply the Reviewer profile's reachability/clause/scope requirements and round-convergence rules; output the specified findings JSON and done flag. No recursive delegation.
