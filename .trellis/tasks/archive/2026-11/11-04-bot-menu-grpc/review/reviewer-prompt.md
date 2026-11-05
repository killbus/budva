# Reviewer run — P3 (bot-menu-grpc), independent gate

You run inside the worktree `/home/agent/.paseo/worktrees/3i5ks3q2/bot-menu-grpc`
(branch `feat/bot-menu-grpc`, base commit `609f181`). Confirm `pwd` first.

TASK_PATH = `.trellis/tasks/11-04-bot-menu-grpc`
BASE_COMMIT = `609f181`

Read, in this order:

1. your role rules (already in your system prompt),
2. `{{TASK_PATH}}/approval.md` (locks `design_sha256`),
3. `{{TASK_PATH}}/design.md` and `{{TASK_PATH}}/prd.md`,
4. `{{TASK_PATH}}/brief.md` (the 8 task-specific blocking criteria),
5. `{{TASK_PATH}}/review/responses.json` (implementer's report).

Then review the change on its own merits. `git diff {{BASE_COMMIT}}` is the object,
never the normative basis; trace every finding through real production entry points.

## Independent verification (this box is equipped; run it yourself)

Export the env block described in
`{{TASK_PATH}}/review/implementer-prompt.md` (section "Environment"), then run:

```bash
go test -p 2 -count=1 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
go vet  -p 2 -tags libtdjson ./internal/transport/grpc/ ./internal/app/facade/ ./internal/infra/telegram/
```

Do not run `go test ./...`, do not use Docker. Test green is **not** architecture proof —
judge the assertions themselves per your role's test-evidence rule. Deleting, weakening,
or reshaping an existing assertion to accommodate the implementation is a blocking
finding even if every test passes.

## What to check with extra care (task-specific)

- Additivity: the 10 existing RPCs, their request/response messages and fields are
  unchanged (proto diff + handler/`mapFacadeError` diff).
- Non-goals: no reply fetching/polling/stream RPC, no Web App execution, no Bot API
  client/bot token, no generic `Execute(opcode, payload)`, no second TDLib session,
  no new Go module, no new required env, no ruleset schema change (design §3).
- Layering per design §6: `transport` never touches TDLib directly, `app/facade` stays a
  thin wrapper, `infra/telegram` owns TDLib specifics and the flood-wait typed error.
  Verify the dependency direction used for the new typed errors.
- Flood-wait: detection in infra reusing `parseFloodWait` (not a duplicated regex),
  delay = N s + 500 ms, transport maps via `errors.As` only, and the required test
  (`retry after 7` → infra 7.5 s; gRPC `ResourceExhausted` + `RetryInfo` 7.5 s) exists and
  actually encodes the contract.
- Error mapping matches design §7 (not-found, not-a-bot, callback 502 → DeadlineExceeded,
  other callback 4xx → FailedPrecondition, empty data/command → InvalidArgument).
- Existing 10-RPC tests untouched and green.
- Scope: no unrelated refactor; `Taskfile.yml proto` task + `docs/TEST-MATRIX.md` entry +
  `doc.go` docs only as design §11 requires.

## Output

Write `{{TASK_PATH}}/review/findings.json` in your role's schema (verdict, round,
system_context, spec_conformance_summary, test_assertion_review, blocking_findings,
resolved_previous, withdrawn, follow_ups, discarded_count, escalation_reason), then reply
with the same JSON followed by `done=true|false`. You may not modify implementation files.
