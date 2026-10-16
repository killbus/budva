# Current Handoff (2026-10-13)

Status: active. M01 research is complete. The user explicitly approved the
M02 Message-only v2 design on 2026-10-13, authorizing M03. The M03 wire
schema is frozen in `research/m03-message-v2-wire-schema.md`. M03 execution
has begun: the canonical v2 serializer core plus RED/GREEN fixtures are
implemented on branch `feat/external-push-sink` (see M03 execution log
below). On 2026-10-13 the user additionally authorized a takeover of this
task with the following working rules (they supersede the older no-commit
wording below):

- Local commits on branch `feat/external-push-sink` are allowed. Record the
  commit plan in `progress.md` before committing; use positive-framing commit
  messages.
- Hard stops (record a GATE line here and stop): push, PR, merge, image
  publication, deployment, `task.py start`, and any M04/resource/Attachment/
  byte-access work. v1 stays default; v2 is per-endpoint opt-in with no silent
  fallback.
- Scope: this task (M03) plus a one-time 10-11 CI-acceptance wrap-up, which is
  done (see below).

## 10-11 correction CI: ACCEPTED (2026-10-13)

GitHub CI run `38212912036` on head SHA `6d7daa750fb708b86c83b12f5d5c19bc10f93eb6`
(`ci-test.yml`, dispatched 2026-10-12T06:31:08Z) completed with conclusion
`success`. Confirmed 2026-10-13 via `gh api` and the magpie routing log.
Acceptance is recorded in `10-11-external-push-sink/progress.md`. The 10-11
task remains in progress: replacement test-image publication, PR/merge and
deployment are still an open user decision (published `dev-run12-d58909f` is
still the text-only test build).

## Deliverables (unchanged, from 2026-10-12 handoff)

- PRD now defines Message facts without presupposing media download, hosting,
  caching or byte delivery. Attachment and optional Relation product scope must
  be decided by M01/M02 evidence before any resource transport is designed.
- `research/message-attachment-boundaries.md` is the authoritative decision:
  Message facts are separate from resources and optional relationships;
  Webhook is transport behavior, while Album is a cross-Message aggregate.
- `research/content-type-matrix.md` maps all 74 pinned MessageContent types to
  exact source fields, Message-fact projections, resource semantics, exclusions,
  support decisions and representative fixture coverage.
- `research/m01-representative-fixtures.jsonl` contains 13 non-wire-schema
  research scenarios that preserve the Message-only boundary.
- `research/m02-protocol-decision.md` records the domain, distributed-systems
  and evolutionary-contract cross-audit and the converged M02 decisions.
- `design.md` now contains only the Message v2 design candidate. Attachment,
  Relation, byte access and resource infrastructure are explicitly deferred.
- `implement.md` orders M01-M06 with research/design approval before coding.
- `research/media-gap-baseline.md` records repository-backed v1 gaps.
- `implement.jsonl` and `check.jsonl` contain real spec/research contexts.

M01 semantic validation passed on 2026-10-12: the pinned source contains 74
unique MessageContent implementations and 201 top-level fields; the matrix
contains all 74 types exactly once and reproduces their exact source fields.
All 13 JSONL fixtures parse and contain no forbidden Attachment, Relation,
Delivery, Album-completeness or resource-lifecycle keys inside Message.
Document-integrity checks also cover JSON/JSONL parsing, ASCII, final newlines,
trailing whitespace, local references and `git diff --check`. These checks
complete M01 research only; the user's 2026-10-13 approval separately authorizes
M03 but does not prove runtime behavior.

## Optional recurring loop (NOT yet created)

On 2026-10-13 a recurring Paseo schedule was designed to advance M03 in fresh
agents (cadence `5,45 * * * *`, provider `claude/remote-magpie/group/qwen3-8-27b`,
mode auto, max 20 runs / 2-day TTL, file-lock concurrency guard in
`.trellis/.runtime/loops/`). Creation was blocked by the local magpie gateway's
auto-mode safety classifier (GLM-5.3-Flash long thinking block stalling past
the harness timeout; gateway routing log records 499 client aborts; the model
endpoint itself probes fine). The full per-run prompt is at
`.trellis/.runtime/loops/m03-prompt.txt` on this machine; the state file is
`.trellis/.runtime/loops/m03-loop-state.md`. Creating the loop is optional and
non-blocking: do it only from a session not in `auto` permission mode (or after
the classifier issue is resolved), since `auto` mode routes every write through
that flaky classifier.

## Next Action And Boundaries

M03 increments in progress (see M03 execution log). Done: serializer core
plus RED/GREEN fixtures (commit `7750b19`), `PushTarget.schemaVersion`
ruleset parsing and rejection (this run). Next: per-endpoint v2 projector
wiring between handler source facts and `messagev2.MarshalNotification` at
the delivery boundary, mixed v1/v2 endpoint behavior and endpoint-isolated
failure tests. Keep v1 as the default; do not enter M04 resource access.
Task-branch pushes are limited to CI evidence; final main integration
requires explicit owner agreement followed by a pull request. `task.py start`
requires recording intent and stopping for user approval.

## Takeover execution policy update (2026-10-14)

The previous loop run exhausted local memory. From this point onward, local work
must remain lightweight: no full typecheck, whole-repository test/build/vet/race
jobs, or parallel heavy commands. Run formatting, diff checks and narrowly scoped
tests locally when safe; move necessary memory-heavy or Linux/CGO-dependent
verification to the relevant GitHub Actions workflow and record the branch, commit
SHA, workflow/run URL and result here.

Non-main iteration branches are allowed for M03 traceability. A task branch may be
pushed when needed to trigger relevant CI, but direct updates to main remain
forbidden. Final integration requires explicit owner agreement followed by a pull
request; the loop must not merge it.

`mytokens.txt` is local secret input and is excluded through `.git/info/exclude`.
Agents must never open, print, search, parse, diff, stage, commit, upload or archive
it. If GitHub authentication is required, stream it directly to the consuming
process through stdin or an equivalent non-echoing channel; never expose it in
arguments, shell tracing, environment dumps, remotes, config, scripts or artifacts.

The related correction CI is already accepted (green) as noted above; do not
re-poll it. Future 10-11 acceptance gates are publication, not CI. This task is
not archived, and the linked correction task remains in progress pending its
publication gate.

Planning/context files are local and uncommitted (deliberately, per user
instruction on 2026-10-13). Commit them only through the normal reviewed
commit-plan gate. Existing caches/tools/unrelated WIP remain untouched.

## M03 execution log

### 2026-10-13 — serializer core plus RED/GREEN fixtures (single immediate run)

The old loop run (started 2026-10-13T16:32:41Z, no RUN END) was stopped by the
user; its leftovers were audited and preserved, not reverted:

- `internal/service/push/messagev2/model.go` (untracked, left by the old run):
  the TDLib-free canonical v2 fact carrier. Kept unchanged; it matches the
  frozen wire schema.
- `go.mod` (modified, uncommitted): promotes
  `go.opentelemetry.io/otel/sdk/metric v1.43.0` from indirect to direct.
  `internal/service/push/metrics_test.go` (committed in 10-11) imports it, so
  the promotion is required for `go mod` consistency. Kept and folded into the
  serializer commit.

Discriminator strings, entity/reply/origin discriminators,
`UpdateDeleteMessages` facts and UTF-16 entity offsets were re-verified
against the pinned `go-tdlib v0.7.6` module cache (read-only); the frozen
schema needs no correction.

Implemented in this run (new files, no v1 or handler changes):

- `internal/service/push/messagev2/serializer.go`: `MarshalNotification`
  validates projected facts and renders the frozen v2 wire JSON:
  deterministic key order, UTC RFC3339 times, zero/nil omission semantics,
  `has_sensitive_content` snapshot-only, delete facts only for `deleted`,
  bounded discriminators (128 bytes, ASCII allowlist, no control
  characters), UTF-16 offsets copied verbatim, endpoint-relevant errors
  wrapping `ErrInvalidNotification` with bounded messages. No TDLib
  dependency, no v1 fallback, no raw source data in output or errors.
- `internal/service/push/messagev2/serializer_test.go`: RED/GREEN coverage
  for the serializer increment:
  - RED control 3: the four exact frozen wire fixtures (new text, edited
    photo caption, delete, unknown content) compared as decoded JSON.
  - RED control 4: nil sender/reply/origin/content handling, unsupported
    reply discriminator, entity type validation, nil notification, unknown
    event, zero occurred_at/chat_id/message_id, snapshot vs delete fact
    isolation (including a deleted-with-sensitive-flag reject).
  - RED control 5: UTF-16 offsets (65535/2) emitted verbatim; discriminator
    accepted at exactly 128 bytes, rejected at 129; control characters,
    non-ASCII and JSON-looking source_type rejected.
  - RED control 6: absence set over rendered output and the frozen
    fixtures (no raw TDLib `@type`, file_id, path, URL, credential,
    delivery or resource-cache tokens).
  - RED control 1 partial: v1 `event` package and `push` package tests
    re-run and unchanged (v1 stays default and source-compatible).

Local verification (lightweight, memory-bounded, GOPROXY=off, GOMAXPROCS=2):

- `gofmt` clean on both new files; repo-wide `git diff --check` clean.
- `go vet ./internal/service/push/messagev2/` clean.
- `go test ./internal/service/push/messagev2/` — all 8 top-level tests and
  all subtests pass (Linux/amd64; the package has no CGO dependency).
- `go test ./internal/service/push/` (v1 envelope/webhook/manager/dispatcher
  tests) — pass; no v1 regression from this increment.
- No full typecheck, whole-repo test/build/vet/race was run.

Commit plan (recorded before committing, per takeover policy):

- Branch: `feat/external-push-sink` (existing task branch; no new branch
  needed for this increment).
- Commit 1 (code), message `feat: add canonical Message v2 serializer`,
  executed as `7750b19` on `feat/external-push-sink`, including:
  - `internal/service/push/messagev2/model.go` (preserved old-run artifact,
    unchanged);
  - `internal/service/push/messagev2/serializer.go` (new);
  - `internal/service/push/messagev2/serializer_test.go` (new);
  - `go.mod` (otel sdk/metric indirect-to-direct promotion, required by the
    committed 10-11 metrics test).
- Commit 2 (docs), message `docs: record M03 serializer increment and 10-11
  correction CI acceptance`, including:
  - `.trellis/tasks/10-12-telegram-webhook-semantics/` (task directory:
    prd, design, implement, progress, task.json, research records and jsonl
    contexts) — first commit of this task's records;
  - the recorded 10-11 CI-acceptance doc updates (progress.md, implement.md,
    task.json, research/linux-correction-verification.md) folded in per the
    m03-prompt one-time wrap-up.
- Both commits end with the required `Co-Authored-By` line from
  `m03-prompt.txt`.
- Excluded from both commits (intentionally left as-is):
  - `.trellis/.runtime/` (loop prompt/state package, local-only),
    `.trellis/.developer`, `.trellis/scripts/`;
  - `go.sum` (unchanged; the version was already present);
  - the 10-11 evidence archives (`linux-correction.tgz`, `linux-files.txt`,
    `linux-source.tgz`, `validation-evidence.tgz`): untracked, not part of
    this increment;
  - `.tmp-go-cache/`, `.zvec-grep/`, `probe.js`, `ruleset.yml`,
    `mytokens.txt` (secret, excluded via `.git/info/exclude`): untouched and
    untracked.
- No push, PR, merge, image publication, deployment, `task.py start` and no
  M04 work in this run.

Next action: M03 increment 2 — per-endpoint `schemaVersion` ruleset parsing
(absent/zero/explicit-1 => v1, explicit 2 => v2, other values reject the
ruleset) plus projector wiring between the handler source facts and
`messagev2.MarshalNotification` at the delivery boundary. (Increment 2
split: ruleset parsing lands this run; projector wiring is the next
increment. Push of this task branch is for CI evidence of the Linux/CGO
packages, which cannot be compiled locally.)

### 2026-10-13 — PushTarget schemaVersion ruleset parsing (single immediate run, increment 2a)

Scope: per-endpoint webhook schema selection at the ruleset boundary.
`internal/domain` and `internal/infra/ruleset` transitively import
`go-tdlib` (cgo, linux/darwin: `domain/params.go` imports `client`), so
these packages cannot be compiled or tested on this Linux box without the
TDLib headers (`td_json_client.h` missing; `CGO_ENABLED=0` also fails with
undefined `meta`/`JsonInt64`). That is the documented environment
limitation, not a red; the focused package tests move to `ci-test.yml`.

Changes (no v1 wire change, no handler change, no Telegram behavior change):

- `internal/domain/ruleset.go`:
  - `PushTarget.SchemaVersion` field (`yaml:"schemaVersion"`);
  - `ErrInvalidSchemaVersion` sentinel;
  - `(*PushTarget).EffectiveSchemaVersion()`: absent/0 => 1 (v1 default),
    explicit 1 => 1, explicit 2 => 2 (Message-only v2 opt-in), any other
    value rejected with a bounded error wrapping the sentinel. The
    resolution is the single canonical authority used by the loader and
    (later) by the delivery boundary.
- `internal/infra/ruleset/repo.go`: `validate()` rejects a ruleset whose
  any push target requests an unsupported `schemaVersion`, naming the
  target. This makes "any other value is invalid and rejects the ruleset"
  (frozen wire schema) true at load time; a v2 failure therefore can never
  silently fall back to v1, because such a ruleset never loads.
- `internal/domain/ruleset_test.go` (new): `EffectiveSchemaVersion` table —
  absent/1/2 accepted, -1/3/99 rejected (sentinel match).
- `internal/infra/ruleset/push_schema_test.go` (new): `validate()` RED
  table — -1/3/20 reject (error names `schemaVersion`), absent/1/2 accept;
  plus a mixed-target case proving the error names the offending target
  (`"bad"`), not a valid sibling.

Local verification (lightweight, memory-bounded, GOPROXY=off):

- `gofmt` clean on all four touched files; `git diff --check` clean; new
  test files ASCII-clean.
- `go test ./internal/service/push/...` (pure Go, unaffected packages) —
  green; no regression.
- `go vet`/`go test` for `internal/domain` and
  `internal/infra/ruleset` are NOT runnable locally (TDLib cgo headers
  absent — limitation above). The RED state could not be executed locally
  for the same reason; the rejection tests are constructed to pass only
  with the new validation (absent/1/2 accept cases would also pass
  without it, so the reject cases carry the red).

Commit plan (recorded before committing, per takeover policy):

- Branch: `feat/external-push-sink` (existing task branch).
- Commit 1 (code), message `feat: add per-endpoint push schemaVersion
  selection`, including exactly:
  - `internal/domain/ruleset.go`;
  - `internal/infra/ruleset/repo.go`;
  - `internal/domain/ruleset_test.go` (new);
  - `internal/infra/ruleset/push_schema_test.go` (new).
- Push the task branch to `origin` for CI evidence (authorized for this
  task; branch only, never main). The push triggers no workflow
  automatically (`ci-test` pushes on main only, `docker-publish` on
  version tags only).
- Push the task branch to `origin` for CI evidence (authorized for this
  task; branch only, never main). The push triggers no workflow
  automatically (`ci-test` pushes on main only, `docker-publish` on
  version tags only). Executed: `git push origin feat/external-push-sink`
  advanced the remote `6d7daa7..76a6f45`. The token reached git only as the
  stdout of a `700`-mode `GIT_ASKPASS` helper that `cat`s `mytokens.txt`;
  it was not printed, placed in argv/env/trace/URL, and the helper was
  deleted immediately after.
- CI: dispatched `ci-test.yml` via `workflow_dispatch` against
  `feat/external-push-sink` (tip `76a6f45`; a SHA dispatch returned
  `422 No ref found` twice immediately after push, so the branch name was
  used — its tip is the exact code commit). The token was passed to `gh`
  through a non-echoing command substitution; it was not printed, staged,
  committed, archived or placed in any config.
  - Branch: `feat/external-push-sink`
  - Commit SHA: `76a6f45c9bad2a74152f96a5c4da19a4ba1f3008`
  - Workflow: `ci-test.yml`
  - Run: https://github.com/killbus/budva/actions/runs/38470869938
    (databaseId 38470869938, created 2026-10-13T21:28:29Z)
  - Steps: `go vet ./...` then `go test ./internal/...` on ubuntu-latest
    with cached TDLib (key `tdlib-22d49d5-amd64`).
  - Result: **success** (job `test` conclusion `success`; covers both
    `go vet ./...` and `go test ./internal/...`). Completed within ~45s of
    creation (TDLib cache hit). This is the Linux/CGO evidence that the
    `internal/domain` and `internal/infra/ruleset` packages compile and the
    new `schemaVersion` RED/GREEN tests pass.

CI acceptance (increment 2a): the focused package tests that cannot run
locally (TDLib cgo headers absent) are green on Linux CI at
`76a6f45`. The serializer increment (`7750b19`) is pure Go and was verified
locally. No v1 wire change, no handler change, no Telegram behavior change
in either increment.

Remaining M03 after this run: per-endpoint v2 projector wiring between the
handler source facts and `messagev2.MarshalNotification` at the delivery
boundary, mixed v1/v2 endpoint behavior, endpoint-isolated serialization
failure (no v1 fallback), and the remaining RED controls (7, 8, 10). This is
the next increment; M04 resource access remains forbidden.
- Commit 2 (docs), message `docs: record M03 schemaVersion increment and
  CI evidence`, including:
  - `.trellis/tasks/10-12-telegram-webhook-semantics/progress.md` (this
    entry with the CI run URL and result);
  - `.trellis/tasks/10-12-telegram-webhook-semantics/task.json` (notes:
    increment 2a status and branch/SHA/CI evidence).
- Excluded: `.trellis/.runtime/`, caches, archives, `probe.js`,
  `mytokens.txt` (secret, excluded) — untouched and untracked.
- No push to main, no PR, no merge, no image publication, no release tag,
  no deployment, no `task.py start`, no M04 work.

## Takeover Increment 3 (M03 delivery-boundary wiring) — 2026-10-14

Takeover of an interrupted context. No schedule/heartbeat created; no
worktree created or switched; the existing dirty worktree is preserved.

### Takeover review (phase 1)

Baseline confirmed: HEAD `3007040` matches expected and `origin` tracks it;
branch `feat/external-push-sink`. The prior draft was 17 tracked modifications
+ 2 new files, all consistent with M03 delivery-boundary wiring. No
unexplained or out-of-scope changes found; no M04 (Attachment/Relation/media
bytes/cache/URL/token/resource lifecycle) introduced. Draft was coherent and
compiles for the pure-Go `internal/service/push/...` packages.

Findings on the prior draft:
- `internal/service/push/webhook.go` lost its sink body and
  `classifyDeliveryError`; both were re-homed into the new `delivery.go`
  together with the `RenderPayload` delivery-boundary renderer and the
  `FailureSerialization` class. No behavior removed; this is the intended
  M03 wiring.
- `event.Sink`/`Submitter`/`RenderedEnvelope` and `push` aliases widened the
  carrier to carry both the v1 `Envelope` and the optional v2
  `*messagev2.Notification`. This is the minimal delivery-boundary interface
  spread required for per-endpoint schema selection; it is not M04.
- `messagev2.ValidateDiscriminator`/`ValidateBoundedString` exported so the
  source projection can bound optional strings before emission. Consistent
  with the frozen wire schema.
- `projection_v2.go` is a pure source projection (no TDLib in the wire, no
  endpoint config, no serialization). Matches the frozen M03 wire schema.

### Corrections applied to the draft (minimal)

1. `internal/app/handler/push_test.go:83` — fixed a leftover six-argument
   `OnDeletedMessages` call (two stray `false`s) to the correct five-argument
   signature `(chatID, ids, isPermanent, fromCache)`; this was a compile
   break.
2. `internal/app/handler/doc.go` — updated the `OnDeletedMessages` usage
   example to the five-argument signature.

No draft file was deleted, reverted or overwritten. Untracked caches,
`.trellis/.runtime`, archives, `probe.js` and `mytokens.txt` were left
untouched.

### Changes added by this increment (minimal)

- `internal/app/handler/projection_v2_test.go` (new): RED for the source
  projection — bounded v2 facts (pinned content, bounded entities, unknown
  content without raw TDLib data, delete facts without synthesized snapshot
  facts). Runs on Linux/CGO CI.
- `internal/service/push/webhook_test.go`: RED 7 (`MixedSchemasRenderPer
  Endpoint`) and RED 8 (`V2FailureIsEndpointIsolated`) — v1 keeps the exact
  default wire, explicit `schemaVersion=2` emits v2, and a v2
  projection/serialization failure fails only the v2-selected endpoint with
  the `serialization` class and never falls back to v1/raw/text-only.
- `internal/app/handler/push_test.go`: `serialization` added to the
  `TestPush_TelegramInvariance` outcome loop (RED 10 — a
  serialization-class delivery failure leaves Telegram copy/forward/delete
  untouched), plus `TestPush_MixedEndpointsDeliverSelectedSchemas` (RED 7 at
  the handler boundary) and `TestPush_V2FailureIsEndpointIsolated` (RED 8 at
  the handler boundary, real sinks + real projection).

M04 remains untouched: no Attachment, Relation, media bytes, cache, URL,
token or resource lifecycle.

### Local verification (memory-bounded, GOPROXY=off, CGO_ENABLED=0)

- `gofmt` clean on all touched files; `git diff --check` clean; new test
  files ASCII-clean.
- `go test ./internal/service/push/...` (pure Go) — green, including the new
  RED 7/8 sink tests.
- `internal/app/handler` and `cmd/engine` depend on TDLib cgo and cannot be
  compiled or executed locally (TDLib headers absent). Their RED 10 / handler
  RED 7/8 / projection tests are carried by the existing GitHub Actions
  Linux CI (cached TDLib), dispatched after push.

### Commit plan (recorded before committing)

- Branch: `feat/external-push-sink` (existing task branch; no main touch).
- Commit 1 (code), message `feat: wire v2 webhook schema at the delivery
  boundary`, exactly the M03 delivery-boundary scope:
  - `cmd/engine/main.go`
  - `internal/app/handler/projection_v2.go` (new)
  - `internal/app/handler/projection_v2_test.go` (new)
  - `internal/app/handler/service.go`
  - `internal/app/handler/push_test.go`
  - `internal/app/handler/push_coverage_test.go`
  - `internal/app/handler/service_test.go`
  - `internal/app/handler/doc.go`
  - `internal/service/push/delivery.go` (new)
  - `internal/service/push/dispatcher.go`
  - `internal/service/push/dispatcher_test.go`
  - `internal/service/push/envelope.go`
  - `internal/service/push/envelope_test.go`
  - `internal/service/push/event/envelope.go`
  - `internal/service/push/messagev2/serializer.go`
  - `internal/service/push/metrics_test.go`
  - `internal/service/push/webhook.go`
  - `internal/service/push/webhook_test.go`
  - `internal/service/push/webhook_security_test.go`
  - `internal/test/support/live_stack.go`
  - `test/bdd/suite/steps_05_sync.go`
- Excluded from the commit: `.tmp-go-cache/`, `.trellis/.runtime/`,
  `.trellis/scripts/`, `.trellis/.developer`, `.trellis/tasks/*/…tgz`,
  `.zvec-grep/`, `probe.js`, and `mytokens.txt` (secret, git-excluded) — all
  left untracked.
- Push `feat/external-push-sink` to `origin` for CI evidence (branch only),
  then dispatch `ci-test.yml` via `workflow_dispatch`. Token via non-echoing
  askpass/stdin only; never in argv/env/trace/URL/config. Then record the
  branch, code commit SHA, workflow/run URL, result and residual risk here.
- No `task.py start`, no merge, no PR, no image publication, no release tag,
  no deployment, no M04 work.

### Commit 1 landed and first CI result

- Commit 1 `8896f35` "feat: wire v2 webhook schema at the delivery boundary"
  pushed to `origin/feat/external-push-sink` (branch only).
- CI run 38515803215 (https://github.com/killbus/budva/actions/runs/38515803215)
  FAILED: `go test ./internal/...` — `internal/app/handler` timed out at the
  600s package limit. Goroutine dump: TLS `persistConn` goroutines blocked
  inside the `synctest.Test` bubble created by the two new handler tests
  (`TestPush_MixedEndpointsDeliverSelectedSchemas`,
  `TestPush_V2FailureIsEndpointIsolated`). Repo convention (see
  `TestPush_ProductionHooksDeliverCanonicalWebhook`) is real-TLS-server
  tests without synctest; synctest only with pure transports. All other
  packages (including `internal/service/push` and `messagev2`) passed;
  `go vet ./...` passed.

### Commit 2 (fix) — recorded before committing

- Commit 2, message `fix: run mixed-schema webhook tests without synctest`,
  exactly one file: `internal/app/handler/push_test.go`.
- Change: drop the `synctest.Test` wrapper from the two mixed/v2-failure
  handler tests (dedent, remove `synctest.Wait()`), and call `g.Retire()`
  (which joins all sink workers) before draining the buffered body
  channels; `Retire()` is idempotent. Bodies are recorded by each sink
  before the response returns, so after `Retire()` all three bodies per
  endpoint are in the buffered channels. Assertions unchanged.
  `TestPush_TelegramInvariance` and the synctest-only pure-transport tests
  are untouched.
- Local verification (memory-bounded, GOPROXY=off, CGO_ENABLED=0):
  gofmt clean on the touched file, `git diff --check` clean,
  `go test ./internal/service/push/...` green. Handler package still cannot
  compile locally (TDLib cgo); CI is the arbiter for the handler tests.
### Commit 2 CI result and commit 3 (fix)

- Commit 2 `d1e6a75` pushed; CI run 38518089554
  (https://github.com/killbus/budva/actions/runs/38518089554) FAILED but the
  synctest deadlock is gone: `internal/app/handler` now runs in 1.1s (previously
  a 600s package timeout). `go vet ./...` passed; `internal/service/push` and
  `messagev2` green.
- Sole failure: `TestV2Projection_DeleteCarriesOnlyDeleteFacts`
  (`internal/app/handler/projection_v2_test.go:133`) asserted a literal
  `"delete":{"is_permanent":true,"from_cache":false}` substring, but
  `encoding/json` sorts object keys alphabetically, so the wire bytes are
  `{"delete":{"from_cache":false,"is_permanent":true}}`. The frozen schema
  document (`research/m03-message-v2-wire-schema.md`, JSON example) is
  semantically identical: both delete flags always present. This is a test
  bug, not a serializer regression.
- Commit 3, message `fix: assert delete facts semantically in v2 projection
  test`, exactly one file: `internal/app/handler/projection_v2_test.go`.
  The assertion now checks `"delete":{`, `"is_permanent":true` and
  `"from_cache":false` independently (matching the CI-observed wire bytes)
  and keeps the negative `sent_at` / `content` assertions. No production
  code touched; serializer and frozen schema unchanged.
- Local verification: gofmt clean on the touched file, `git diff --check`
  clean, `go test ./internal/service/push/...` green. Handler package still
  CI-only (TDLib cgo); the fixed assertion is verified against the exact
  wire bytes emitted in run 38518089554.
### Commit 3 CI result — M03 increment 3 evidence

- Commit 3 `81a5ae9` "fix: assert delete facts semantically in v2 projection
  test" pushed to `origin/feat/external-push-sink` (branch only).
- CI run 38518657912
  (https://github.com/killbus/budva/actions/runs/38518657912) on
  `feat/external-push-sink` @ `81a5ae920908f1bd36abe6e714bc051c6e8f399a`:
  **success** (workflow_dispatch, ci-test.yml, `test` job success:
  `go vet ./...` + `go test ./internal/...` on Linux with cached TDLib).
- Validation coverage of this run:
  - `internal/app/handler`: RED 7 at the handler boundary
    (`TestPush_MixedEndpointsDeliverSelectedSchemas`), RED 8 at the handler
    boundary (`TestPush_V2FailureIsEndpointIsolated`, failing v2 transport
    records a v2 body, no v1/raw/text fallback), RED 10
    (`TestPush_TelegramInvariance` with the added `serialization` outcome and
    strict Telegram mocks for copy/forward/delete), production canonical
    webhook test, and all `projection_v2_test.go` source-projection REDs
    (unknown content without raw TDLib data, bounded discriminators, delete
    facts only).
  - `internal/service/push`: RED 7/8 sink tests, schemaVersion ruleset
    (0/empty→v1, 1=v1, 2=v2, others rejected), security/redirect/timeout
    tests; `messagev2` frozen fixtures and validation all green.
  - Handler package runtime: ~1s (synctest deadlock from run 38518089554 is
    gone; that run had proven the deadlock fix, this run is the arbiter for
    the semantic delete-fact fix).

### M03 closure assessment

M03 is CLOSED for this increment chain: the approved canonical serializer
(increment 1, `7750b19`), per-endpoint schemaVersion opt-in with rejection
(increment 2, `76a6f45`, CI 38470869938), and delivery-boundary wiring
(increment 3: `8896f35` + test-only fixes `d1e6a75`, `81a5ae9`, CI
38518657912) are all committed on the task branch and green in CI.
`implement.md` M03 checkbox checked. Residual risk: none known inside M03
scope; M04 (Attachment/Relation/media bytes/cache/URL/token/lifecycle)
remains untouched and forbidden; M05/M06 are separate plan items. No main
merge, PR, image, tag or deployment performed.

### 2026-10-14 — M04 boundary correction and next-loop handoff

- User reconfirmed the architecture: `/app/engine` passively publishes an
  honest Telegram Message view; facade is the external entry point for active
  Telegram interactions.
- Retracted the proposed `resource_ref -> TDLib file_id` mapping. Telegram
  already supplies the Message -> media -> File relationship; Budva must not
  re-invent it as Attachment, Relation or an opaque resource identity.
- Corrected the repository claim: pinned go-tdlib provides native file state
  and actions, while Budva currently does not wire `UpdateFile` or expose
  `GetFile`/`DownloadFile`/`CancelDownloadFile` through the inspected adapter
  and gRPC contract.
- M04 is now a completed no-code product decision gate, not an implementation
  milestone and not a dependency of M05.
- A future approved native-file follow-up must reuse/extend facade, preserve
  account/session scope, and use reviewed Telegram-native facts. It must not
  introduce automatic webhook-side download, duplicate identity storage,
  cache, hosting, permanent URLs, object storage or a generic resource
  lifecycle.
- M05 remains the next delivery milestone. Before code changes, its loop must
  audit RED controls so "textless media" means Message lifecycle eligibility,
  not file download or byte delivery.
- No runtime code changed in this decision update. No main merge, PR, image,
  release tag, deployment, schedule, heartbeat or `task.py start` was
  performed.

### 2026-10-14 — M05 increment: webhook-only textless media (single TDD increment)

M05 RED control "webhook-only textless media" executed as one minimal TDD
increment. Base confirmed clean at takeover: branch
`feat/external-push-sink`, HEAD `df78704`, no unexplained modifications.

Decision (user, via AskUserQuestion): a webhook-only textless media source
fact (captionless photo/video/document) is delivered as a canonical v2
content fact only; the v1 (text-only) endpoint stays silent rather than
emitting an empty or text-only envelope. No Telegram copy/forward, no
download, no file or network work is added. Eligibility is defined
independently of the Telegram copy/forward guards (PRD: "Do not accidentally
inherit forwarding guards").

Production changes:

- `internal/service/push/event/envelope.go`: `Submitter` gains
  `SubmitV2Only([]string, *messagev2.Notification) SubmitResults` — the
  webhook-only carrier entry point with no v1 representation.
- `internal/service/push/dispatcher.go`: `Generation.SubmitV2Only` enqueues a
  `RenderedEnvelope{V2: v2}` (empty `Envelope`), reusing the existing bounded
  non-blocking submit/lease/retire path unchanged.
- `internal/service/push/delivery.go`: new sentinel `ErrV1Unavailable`;
  `RenderPayload` v1 branch fails with it when the carrier has no v1
  representation (`Envelope.Event == ""`). Silence is keyed on the absent v1
  representation, never on empty text: a legitimate v1 delete (`Event` set,
  empty `Text`) still renders. A v1 endpoint given a v2-only carrier therefore
  reports a serialization-class failure and sends zero HTTP attempts.
- `internal/app/handler/service.go`: `OnNewMessage` no longer drops the whole
  update on `formattedText == nil`. Inside the rule loop, before the
  `SendCopy`/`CanBeSaved` guard, a textless non-album message with a push
  route is projected via `v2SnapshotFor` and submitted with
  `SubmitV2Only`. Albums keep the existing quiet-period path; system-message
  handling is unchanged.

Tests (TDD: the v1-silence and v2-only carrier REDs were written to fail
before the `SubmitV2Only`/`ErrV1Unavailable` wiring existed):

- `internal/service/push/dispatcher_test.go`: `SubmitV2Only` carries the v2
  fact to both endpoints with an empty v1 envelope; enqueue stays bounded.
- `internal/service/push/webhook_test.go`: a v1 endpoint given a v2-only
  captionless-photo carrier fails with `ErrV1Unavailable`/serialization class
  and sends zero bodies; a v2 endpoint renders the `messagePhoto` fact.
  `RenderPayload` v1 silence is explicit (v2-only fails; a legitimate v1
  delete still renders).
- `internal/service/push/webhook_security_test.go`, `dispatcher_test.go`
  fixtures: pre-existing throwaway v1 envelopes now carry an explicit event
  (a real v1 fact always has an event kind; the empty-event envelope is now
  reserved as the v2-only sentinel).
- `internal/app/handler/push_test.go`: `recordingSubmitter` and
  `captureSubmitter` implement the new `SubmitV2Only` so the handler package
  compiles against the widened interface.
- `internal/app/handler/textless_push_test.go` (new, CI-only cgo): one
  captionless source photo (real `file_id` present on the source) drives two
  real webhook endpoints (v1 default, v2 selected) with strict Telegram,
  state, limiter and transform mocks (zero expectations), no `SendCopy`/`To`
  destinations and `CanBeSaved: false`. Asserts exactly one v2 `messagePhoto`
  content fact with no `formatted_text` and no file identifier leaks, zero
  bodies at the v1 endpoint, an empty task queue, and no Telegram work. Real
  TLS servers run outside synctest with `Retire()` before draining.

Local verification (lightweight, memory-bounded, GOPROXY=off, CGO_ENABLED=0):

- `gofmt` clean on all touched files; `git diff --check` clean; new test file
  ASCII-clean.
- `go test ./internal/service/push/...` (pure Go) — green, including the new
  v1-silence and `SubmitV2Only` RED/GREEN tests; no v1 regression.
- `internal/app/handler` is TDLib cgo and cannot be compiled locally (TDLib
  headers absent); the new handler test and the interface-compatibility of the
  widened `Submitter` are carried by GitHub CI.

Commit plan (recorded before committing, per takeover policy):

- Branch: `feat/external-push-sink` (existing task branch; no main touch).
- Commit 1 (code + tests), message `feat: push webhook-only textless media as
  v2 facts`, exactly:
  - `internal/service/push/event/envelope.go`
  - `internal/service/push/dispatcher.go`
  - `internal/service/push/delivery.go`
  - `internal/service/push/dispatcher_test.go`
  - `internal/service/push/webhook_test.go`
  - `internal/service/push/webhook_security_test.go`
  - `internal/app/handler/service.go`
  - `internal/app/handler/push_test.go`
  - `internal/app/handler/textless_push_test.go` (new)
- Commit 2 (docs), message `docs: record M05 textless-media increment`,
  including `progress.md` (this entry + CI result) and `implement.md` (M05
  status note); the other pre-existing task-directory doc edits travel with
  them.
- Push `feat/external-push-sink` to `origin` for CI evidence (branch only),
  then dispatch `ci-test.yml` via `workflow_dispatch`. Token via non-echoing
  askpass/stdin only; never in argv/env/trace/URL/config. Then record the
  branch, code commit SHA, workflow/run URL and result here.
- Excluded from both commits (untracked, left as-is): `.tmp-go-cache/`,
  `.trellis/.runtime/`, `.trellis/scripts/`, `.trellis/.developer`,
  `.zvec-grep/`, `probe.js`, the 10-11 `…tgz` archives, and `mytokens.txt`
  (secret, git-excluded).
- No push to main, no PR, no merge, no image publication, no release tag, no
  deployment, no schedule/heartbeat, no `task.py start`, no M04 work.

Remaining M05 after this increment: albums/replies, edits/deletes of textless
media, missing predecessors, protected eligibility and unchanged
system-message actions, plus Telegram invariance under full buffers/delivery
failure/reload/shutdown for the new path — each as its own minimal TDD
increment. M04 and M06 remain untouched; main integration requires explicit
owner agreement and a pull request.

### 2026-10-14 — M05 CI evidence (run 1) and channel-read fix

Branch `feat/external-push-sink`, code commit `7108e0c`, docs commit
`e50d448` pushed; `ci-test.yml` dispatched on the branch name.

- Run 1: https://github.com/killbus/budva/actions/runs/38614765938
  (head `e50d448`) — **failure**, `go test ./internal/...` timed out at
  10m in `internal/app/handler`. Dump shows
  `TestTextlessMedia_PushesV2FactOnlyV1Silent` running for 10m: the test
  read `<-v2Bodies` twice, but the channel holds exactly one delivered
  body (one webhook-only submission), so the second read blocked forever.
  The `tls_transport`/`serialization` WARN lines in the same dump belong
  to the pre-existing M03 v2-failure-isolation test, as designed. The
  bug was in the new test's drain, not in the production path: the v2
  fact was delivered once, v1 stayed silent, and `Retire()` completed.
- Fix: `internal/app/handler/textless_push_test.go` keeps the raw body of
  the single read and reuses it for both the JSON assertions and the
  file-reference absence checks (one read of a one-element channel).
  gofmt clean; the package stays TDLib-cgo CI-only locally.
- Run 2: https://github.com/killbus/budva/actions/runs/38618093819
  (head `50e15a6`) — **success**: full `go test ./internal/...` (including
  the TDLib cgo `internal/app/handler` package with the new
  `TestTextlessMedia_PushesV2FactOnlyV1Silent` and the widened
  `SubmitV2Only` interface across all submitters) is green, plus `go vet`.
  The M05 webhook-only textless-media increment now has full CI evidence.
- GATE: no main/PR/release/deploy/tag, no schedule/heartbeat, no
  `task.py start`; M04 untouched.

### 2026-10-14 — M05 increment: textless media EDITS (RED scope + commit plan)

Chosen increment: **textless media EDITS emit a v2-only `edited` content fact;
v1 stays silent; no Telegram copy/caption work; no fabricated prior state; no
v1 empty event.** This is the minimal, least-dependent remaining item from the
M05 "textless media lifecycle" list, and it is the direct lifecycle mirror of
the completed textless-media NEW path (`OnNewMessage`).

Gap found: `internal/app/handler/service.go` `editMessages` (line 711) drops a
captionless-media edit with `if text == nil { return false }` *before* the v2
emission at line 714, so an edit of a captionless photo/video/document produces
no v2 fact at all. The delete path (`deleteMessages`) was audited and needs no
change: it already emits `v2DeleteFor` (identity + delete flags only) per
message ID, content-agnostic, so a "missing predecessor" is already handled
honestly (a delete never depends on a prior snapshot, and a v1 delete is a
legitimate identity-only event, not an empty event). "Missing predecessor" is
therefore covered inside this same edit increment: the edit projects the full
*current* message (a complete snapshot, not a patch) and, because it goes
through `SubmitV2Only`, never fabricates a v1 "edited" empty event.

Semantics protected:
- engine honestly expresses the Telegram edit fact (complete current snapshot,
  no diff/patch, no invented prior content);
- v1 behavior unchanged; textless media stays v2-only;
- no prior state required → no fabricated fact and no v1 empty event;
- no Telegram copy/forward/caption download on the webhook-only path;
- FORBIDDEN items (resource_ref, extra file_id mapping, Attachment/Relation,
  new facade API, cache/object storage) untouched;
- album exclusion mirrored (`msg.MediaAlbumId == 0`) so album scope is not
  expanded.

RED (write first, must fail before the GREEN change):
- New handler test `internal/app/handler/textless_edit_push_test.go`
  (CI-only cgo), mirroring `textless_push_test.go`: one captionless source
  photo (real `file_id` on the source) driven through `OnEditedMessage`, two
  real TLS webhook endpoints (v1 default, v2 selected), strict zero-expectation
  Telegram/State/Limiter/Transform mocks, no `SendCopy`/`To` destinations,
  `CanBeSaved: false`. The edit path is async (`taskQueue`), so the test drains
  with `q.ProcessAll()` then `g.Retire()` before reading the body channels.
  Asserts: exactly one v2 `edited` `messagePhoto` content fact carrying
  `edited_at` and no `formatted_text`, no file identifier leak, zero v1 bodies,
  empty task queue, and no Telegram work.
- FAILING reason: before the GREEN change, `editMessages` returns `false` on
  `text == nil`, so the v2 endpoint receives zero bodies (the test expects one
  `edited` fact).

GREEN (minimal, single logical change):
- `internal/app/handler/service.go` `editMessages`, in the `text == nil`
  branch, before `return false`: when `emission != nil && emission.push != nil
  && msg.MediaAlbumId == 0`, collect `names` from rules where `rule.From ==
  msg.ChatId` (same collection as the existing `submitV2` block at line 717) and
  call `emission.push.SubmitV2Only(names, v2SnapshotFor(msg,
  messagev2.EventEdited, emission.occurredAt))` when the snapshot is non-nil.
  This returns before the copies loop, so no copy/caption work runs.

Local verification (lightweight, memory-bounded, GOPROXY=off, CGO_ENABLED=0):
`gofmt` + `git diff --check` on touched files; `go test ./internal/service/push/...`
(pure Go) regression only. `internal/app/handler` is TDLib cgo and cannot
compile locally; the RED/GREEN is carried by GitHub CI.

Outcome and CI evidence:
- First GREEN landed (commit `ac6d172`), plan recorded (commit `515218f`);
  pushed to `feat/external-push-sink`. CI run `38662222906` (head `515218f`)
  was RED on one pre-existing test: `TestPush_SourceEditNilFormattedTextSkipsEvent`
  (`push_source_routing_test.go`). Its fixture is a *content-less* edit
  (`&client.Message{ChatId: 100, Id: 1}`, `Content == nil`), pinned to submit
  no event. The first guard (`text == nil && MediaAlbumId == 0`) was too broad:
  it also fired for that content-less message, producing a v2 `edited` fact with
  `content.type = unknown` -- fabricating a fact where there is no content.
  The task scope is "textless media" (content present, text absent), not "any
  nil-text message", so the over-broad guard was a scoping error, not a
  deliberate semantic change.
- Fix (commit `30f4d81`): narrow the guard in `editMessages` to
  `emission != nil && emission.push != nil && msg.Content != nil &&
  msg.MediaAlbumId == 0`. A content-bearing captionless photo/video/document
  edit still pushes the v2-only `edited` fact (satisfies the new RED test); a
  content-less edit stays silent (satisfies the pre-existing no-event test).
  No v1 empty event is fabricated in either case; the guard still returns before
  the copies loop, so no copy/caption/Telegram work runs.
- Final CI: run `38666202864` (head `30f4d81`, `feat/external-push-sink`) GREEN.
  All `go test` packages `ok`, zero `FAIL` lines, including the freshly built
  (not cached) cgo-gated `internal/app/handler` (1.111s) that carries both
  `TestTextlessMediaEdit_PushesV2EditedFactOnlyV1Silent` and
  `TestPush_SourceEditNilFormattedTextSkipsEvent`, and the pure-Go
  `internal/service/push` / `messagev2` packages.
- A workflow_dispatch without `--ref` targeted the default branch (main) -- run
  `38666048385`; it is NOT this increment's evidence and is disregarded.

Commit plan:
- Branch: `feat/external-push-sink` (no main touch).
- Commit 1 (code + tests): `internal/app/handler/service.go` and
  `internal/app/handler/textless_edit_push_test.go` (new).
- Commit 2 (docs): `progress.md` (this entry + CI result) and `implement.md`
  (M05 status note).
- Push `feat/external-push-sink` to `origin` (branch only), dispatch
  `ci-test.yml` via `workflow_dispatch`; token via non-echoing askpass/stdin
  only, never in argv/env/trace/URL/config. Record run URL + SHA here.
- Excluded (untracked, left as-is): the same protected set as the prior
  increment, including `mytokens.txt` (secret, git-excluded).
- No main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`,
  no M04 work.

### 2026-10-15 — M05 increment: scope the NEW-path textless v2 fact (mirror of 30f4d81)

Chosen increment: **a content-LESS new message (`Content == nil`) must not be
pushed as a fabricated v2 `new` fact.** The edit path was already scoped to
content-bearing textless media in commit `30f4d81` (`msg.Content != nil &&
msg.MediaAlbumId == 0`), but the new-message textless block in `OnNewMessage`
was never given the same guard, leaving a one-line asymmetry.

Gap found: `internal/app/handler/service.go` `OnNewMessage`, the `formattedText
== nil` textless block (line 252), gated only on
`rs.push != nil && len(rule.PushTo) != 0 && msg.MediaAlbumId == 0`. A
content-less message reaches this block because `message.IsSystemMessage`
returns `false` for `Content == nil` (so the system-message early-return does
not fire) and `message.GetFormattedText` returns `nil` for `Content == nil`
(so the `continue` is taken). With no `Content != nil` guard, `v2SnapshotFor`
projects `v2ContentFor(nil) == &Content{Type: UnknownContentType}` and emits a
v2 `new` fact whose `content.type` is `"unknown"` — fabricating a content fact
where the source has no content. The scope is "textless media" (content
present, formatted text absent), not "any nil-text message".

Semantics protected:
- v1 behavior unchanged; textless media stays v2-only; no v1 empty event;
- no fabricated prior state and no fabricated content fact;
- no Telegram copy/forward/download added to the serial queue/limiter;
- FORBIDDEN items (resource_ref, extra file_id mapping, Attachment/Relation,
  new facade API, cache/object storage) untouched;
- the already-green textless-media NEW delivery (`TestTextlessMedia_PushesV2FactOnlyV1Silent`,
  a captionless *photo* with `Content != nil`) is unchanged: it still pushes
  the v2 content fact and keeps v1 silent.

RED (write first, must fail before the GREEN change):
- New handler test `internal/app/handler/textless_push_test.go`
  `TestTextlessMedia_ContentlessNewMessageStaysSilent` (CI-only cgo): one
  content-less source message (`&client.Message{ChatId: 100, Id: 1}`,
  `Content == nil`, `MediaAlbumId == 0`) driven through `OnNewMessage` with a
  single rule (`From == 100`, non-empty `PushTo`), a `recordingSubmitter`
  capture, and strict zero-expectation Telegram/State/Limiter/Transform mocks.
  Strict `message.EXPECT()` binds only `IsSystemMessage -> false` and
  `GetFormattedText -> nil` (the two reads the handler performs before the
  textless branch). Asserts an empty task queue and
  `require.Empty(capture.submissions)`.
- FAILING reason: before the GREEN change the guard fires for the content-less
  message, `v2SnapshotFor` returns a non-nil `unknown`-content `new` fact, and
  `SubmitV2Only` records a submission — so `require.Empty` fails.

GREEN (minimal, single logical change):
- `internal/app/handler/service.go` `OnNewMessage`, add `msg.Content != nil` to
  the textless block guard:
  `rs.push != nil && len(rule.PushTo) != 0 && msg.Content != nil &&
  msg.MediaAlbumId == 0`. A content-bearing captionless media still pushes the
  v2-only `new` fact; a content-less message is left to the earlier no-event
  `continue`. Mirrors the edit-path guard at `editMessages`.

Local verification (lightweight, memory-bounded, GOPROXY=off, CGO_ENABLED=0):
`gofmt` on both touched files (clean) and `git diff --check` (clean); `go test
./internal/service/push/...` (pure Go) green. `internal/app/handler` is TDLib
cgo and cannot compile locally; the RED/GREEN is carried by GitHub CI.

Outcome and CI evidence:
- RED test + GREEN fix committed as `5282917`; this docs entry lands as the
  follow-up commit. Pushed to `feat/external-push-sink` and `ci-test.yml`
  dispatched via `workflow_dispatch --ref feat/external-push-sink`.
- CI run (green, verified): run `38681131316` (head `5282917c`,
  `feat/external-push-sink`) — GREEN across all packages, including the freshly
  built cgo-gated `internal/app/handler` that carries both
  `TestTextlessMedia_ContentlessNewMessageStaysSilent` and
  `TestTextlessMedia_PushesV2FactOnlyV1Silent` (handler package `ok ... 1.105s`,
  no FAIL lines in the test job).

Commit plan:
- Branch: `feat/external-push-sink` (no main touch).
- Commit 1 (code + tests, already landed `5282917`):
  `internal/app/handler/service.go` and
  `internal/app/handler/textless_push_test.go`.
- Commit 2 (docs, this commit): `progress.md` (this entry + CI result) and
  `implement.md` (M05 status note).
- Push `feat/external-push-sink` to `origin` (branch only), dispatch
  `ci-test.yml` via `workflow_dispatch --ref`; token via non-echoing
  askpass/stdin only, never in argv/env/trace/URL/config. Run SHA + number
  recorded above (`5282917c`, `38681131316`).
- Excluded (untracked, left as-is): the same protected set as the prior
  increments, including `mytokens.txt` (secret, git-excluded).
- No main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`,
  no M04 work. Remaining M05 REDs (albums/replies, protected eligibility,
  full-buffer/delivery-failure/reload/shutdown invariance for the new path)

### 2026-10-15 — M05 increment: protected-message eligibility on the new textless path + new-path delivery invariance

Chosen increment (from "Remaining M05 REDs"): the **priority-1 protected-message
eligibility** semantics for the *new* textless path, closed together with the
**pure-test invariance** of the new path under a full buffer and a ruleset
reload. This is a **lock increment** — the code already honors the required
semantics (the guard at `OnNewMessage` is
`rs.push != nil && len(rule.PushTo) != 0 && msg.Content != nil &&
msg.MediaAlbumId == 0`, deliberately independent of `CanBeSaved`/forwarding),
so the RED is the *absence of coverage* for a protected message (the existing
green textless test uses `CanBeSaved:false` but never binds the protected
`has_sensitive_content`/`restriction_reason` facts on the wire) and of the
new-path invariance. The test is written first; it must pass against the
unchanged code. No production code changes are made in this increment.

Why this closes the protected-eligibility RED with no behavior change:
- "Protected eligibility" = a source message that Telegram cannot save/copy
  (`CanBeSaved == false`, protected content, optionally flagged sensitive and
  carrying a restriction reason) must still be eligible for the webhook-only
  v2 textless fact, and that eligibility must not inherit the Telegram
  copy/forward guard (`!rule.SendCopy && !msg.CanBeSaved`).
- The new path already returns before any copy/forward/state/limiter work, so a
  protected textless photo is pushed as a canonical v2 `new` content fact with
  the v1 endpoint silent, and its protected facts (`has_sensitive_content:true`,
  `restriction_reason`) are carried verbatim by the projection. The test binds
  those facts on the wire (the gap the existing test left open) and proves no
  Telegram work runs (strict mocks), v1 stays silent, and the task queue stays
  converged (0) under success, a full push buffer, and a ruleset reload.

Semantics protected (invariant):
- v1 behavior unchanged; textless stays v2-only; no v1 empty event;
- no fabricated prior state or content fact;
- no Telegram copy/forward/download added to the serial queue/limiter;
- FORBIDDEN items (resource_ref, extra file_id mapping, Attachment/Relation,
  new facade API, cache/object storage) untouched;
- no production code change in this increment.

RED (written first; green against unchanged code):
- New handler test `internal/app/handler/textless_protected_push_test.go`:
  `TestTextlessMediaProtected_PushesV2FactV1SilentUnderOutcomes` (CI-only cgo).
  One protected captionless source photo (`Content != nil` MessagePhoto, no
  caption, `CanBeSaved:false`, `HasSensitiveContent:true`,
  `RestrictionReason:"restricted"`, `MediaAlbumId:0`) driven through
  `OnNewMessage` under three outcomes.
  - `success`: two real TLS endpoints (v1 default wire, v2 selected); asserts
    exactly one v2 body with `has_sensitive_content:true` and
    `restriction_reason:"restricted"`, `content.type = messagePhoto`, no
    `formatted_text`, no file id, and zero v1 bodies; task queue `q.Len()==0`.
  - `buffer_full`: single endpoint, `outcomeSink` (`FailureTimeout`), prefilled
    to a full buffer (album-test convention); asserts the protected v2 fact is
    still built from the source (v1 silent) and the queue converges.
  - `reload`: capture into the active generation, then
    `SetRuleSetWithPush(&domain.RuleSet{}, next)` before draining; asserts the
    new fact goes to the captured (pre-reload) generation with the v1 carrier
    absent and the post-reload submitter receives nothing; queue converges.
  Strict `message.EXPECT()` binds only `IsSystemMessage->false` and
  `GetFormattedText->nil` (the two reads the handler performs before the
  textless branch); strict TelegramRepo/StateRepo/Limiter/Transform/Album mocks
  with zero expectations fail on any copy/forward/state/limiter work.
- FAILING-as-RED rationale: the RED is coverage, not a forced failure. If a
  future change made the textless guard depend on `msg.CanBeSaved` (inheriting
  the forward guard) or routed the protected message through the album path,
  the success subtest (missing the v2 body / presence of a v1 body) and the
  strict-mock invariance assertions would fail. No change is made to turn this
  into a forced RED, per the minimal-increment constraint.

GREEN (minimal): none — the increment is test-only; the code already satisfies
the guarded semantics. The green is the test passing under CI.

Local verification (lightweight, memory-bounded, GOPROXY=off, CGO_ENABLED=0):
`gofmt` on the touched test file (clean) and `git diff --check` (clean); the
pure-Go `internal/service/push/...` packages remain green.
`internal/app/handler` is TDLib cgo and cannot compile locally; the RED is
carried by GitHub CI.

Landed scope (as-shipped, narrower than the RED sketch above): the new handler
test `internal/app/handler/textless_protected_push_test.go` covers the two
core outcomes for a protected captionless source photo through `OnNewMessage`:
- `success` — `TestTextlessMediaProtected_PushesV2FactOnlyV1Silent`: two real
  TLS endpoints (v1 default wire, v2 selected); exactly one v2 body carries
  `has_sensitive_content:true`, `restriction_reason:"restricted"`,
  `content.type = messagePhoto`, no `formatted_text`, no file id; v1 silent;
  task queue converges to 0.
- `buffer_full` — `TestTextlessMediaProtected_InvariantUnderFullBuffer`: single
  `endpoint` on an `outcomeSink` (`FailureTimeout`) prefilled to a full buffer
  (album-test convention, `BufferCapacity:1`); the protected v2 fact is still
  built from the source, the submission is rejected `buffer_full` and isolated
  to the endpoint, no v1 envelope is fabricated, and the queue converges.
The `reload` outcome named in the RED sketch is NOT in the shipped file; it
remains available as a future pure-test increment. No production file changes.

Test-authoring corrections made to close the increment (the file was a
half-finished lock test; all four are identifier/semantics fixes in the test,
still no production change):
1. `protectedTextlessService` param type `eventSubmittable` (nonexistent) ->
   `event.Submitter`.
2. `[]eventSubmitResult{eventBufferFull}` (both identifiers nonexistent) ->
   `[]event.SubmitResult{event.SubmitRejectedBufferFull}`; added the
   `internal/service/push/event` import.
3. Defined `protectedSnapshot()` in-file as
   `v2SnapshotFor(protectedTextlessMessage(), messagev2.EventNew,
   time.Now().UTC())`; added the `internal/service/push/messagev2` import.
4. `buffer_full` prefill: prefilled twice (worker holds 1 + buffer holds 1) so
   the handler's submission finds a full buffer (a single prefill would be
   drained/blocked by the worker and the event would be ACCEPT, not
   `buffer_full`); both prefills use `SubmitV2Only`. The helper also returns the
   bound `*client.Message`, now used directly (no second `protectedTextlessMessage()`
   call / gomock pointer-mismatch).
5. (Caught by CI, not in the four pre-located defects.) The buffer-full test's
   rule `PushTo` must name the generation's `endpoint` (matching
   `captureSubmitter`'s `results["endpoint"]` lookup), so the helper takes a
   `pushTo []string` argument; the success test passes `["v1","v2"]`, the
   buffer-full test passes `["endpoint"]`.

CI evidence (the RED test is CI-only cgo; local runs are limited to pure-Go):
- Local: `gofmt` clean, `git diff --check` clean, `go test
  ./internal/service/push/...` (pure Go) green. `internal/app/handler` cannot
  compile locally (TDLib cgo).
- Three code commits landed in sequence; the first two were RED on CI and
  drove the two further corrections above:
  - `5e44c5b` — CI run `38736929000` RED (go vet): `*captureSubmitter` cannot
    be passed as `*push.Generation` -> param type corrected to the
    `event.Submitter` interface.
  - `fc4e9a7` — CI run `38737479314` RED (test): buffer-full assertion expected
    `buffer_full`, actual `""` -> the `PushTo`/capture-endpoint-name mismatch
    corrected (fix 5).
  - `19736fd` — CI run `38738578362` (head `19736fd`, `feat/external-push-sink`)
    GREEN across all packages, including the cgo-gated `internal/app/handler`
    carrying `TestTextlessMediaProtected_PushesV2FactOnlyV1Silent` and
    `TestTextlessMediaProtected_InvariantUnderFullBuffer`.
- Code commit `19736fd` is what CI validated; this docs-only commit needs no
  additional CI run.

Commit plan:
- Branch: `feat/external-push-sink` (no main touch).
- Commits 1-3 (code + tests, landed `5e44c5b` -> `fc4e9a7` -> `19736fd`):
  `internal/app/handler/textless_protected_push_test.go` (new, corrected
  across the three commits). No production file changes.
- Commit 4 (docs, this commit): `progress.md` (this entry + CI result) and
  `implement.md` (M05 status note).
- Pushed `feat/external-push-sink` to `origin` (branch only); `ci-test.yml`
  dispatched via `workflow_dispatch --ref feat/external-push-sink`; token via
  non-echoing askpass/stdin only, never in argv/env/trace/URL/config.
  Run `38738578362` on head `19736fd` is GREEN.
- Excluded (untracked, left as-is): the same protected set as the prior
  increments, including `mytokens.txt` (secret, git-excluded).
- No main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`,
  no M04 work. Remaining M05 REDs (albums/replies, the reload outcome for the
  protected/textless new path, and full-buffer/delivery-failure/reload/shutdown
  invariance for the edit path) continue as separate minimal increments.

### 2026-10-15 — M05 increment: reload outcome (immutable captured route) for the new textless path

Chosen increment: the RED named "the reload outcome for the new textless path"
from the prior increment's tail. This is the **reload / immutable captured
route** semantics for the **new** (webhook-only textless) path — the last new-
path delivery RED that was left after the protected-eligibility increment.
Pure-test lock; expected zero production code changes.

Semantics protected: the handler reads the ruleset snapshot once at the
`OnNewMessage` entry and submits the textless v2 fact through the submitter
**captured with that snapshot** (immutable captured route). A ruleset reload
(`SetRuleSetWithPush`) that swaps in a new ruleset and a new (replacement)
submitter must not reroute a textless message already processed:
- the already-processed message's v2 fact still goes to the captured
  (pre-reload) generation, not the replacement submitter;
- the replacement (post-reload) submitter receives nothing;
- the v1 endpoint stays silent (no empty/text-only envelope is fabricated);
- the task queue converges to zero (a textless webhook-only delivery performs
  no Telegram copy/forward/state/limiter work).

This mirrors `push_test.go`'s reload conventions
(`TestPush_RetryUsesCapturedSnapshotAndEmitsOnce`,
`TestPush_SourceEditRetainsSnapshotAcrossReload`) but for the new-path textless
branch, which submits **synchronously** inside `OnNewMessage` via
`rs.push.SubmitV2Only(rule.PushTo, v2)` (no task-queue work).

RED (two tests, `internal/app/handler/textless_reload_push_test.go`, new file):
- `TestTextlessMedia_ReloadKeepsCapturedRoute` (live reload, real transports):
  one webhook-only rule on an **active (not pre-retired)** `push.Generation`
  with real TLS `v1` and `v2` endpoints (v2 schema-selected). Processing the
  captionless photo message synchronously submits to the captured generation;
  then `SetRuleSetWithPush` swaps in a replacement ruleset/submitter and
  `g.Retire()` joins the captured workers. Asserts: the replacement submitter
  received nothing; exactly one v2 body was delivered on the captured route
  (schema 2, event `new`, `chat_id`/`message_id`, `content.type=messagePhoto`,
  no `formatted_text`, no `file_id`/source file id on the wire); the v1 endpoint
  stayed silent; the task queue is zero. Real TLS servers run **outside**
  `synctest` (repo convention: pure transports inside, real servers outside),
  matching `textless_edit_push_test.go`.
- `TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured` (retired outcome, pure
  `synctest`): a single `endpoint` on an `outcomeSink` behind a
  `captureSubmitter{next: g}`. Processing the message records the v2-only
  submission as `SubmitAccepted` (the generation is not yet retired) with no v1
  envelope. `g.Retire()` then `SetRuleSetWithPush` to a replacement submitter:
  the replacement receives nothing and the captured fact is still the only one
  recorded. Pure transport, so it runs **inside** `synctest.Test`.

Shared helper `reloadTextlessService` mirrors `protectedTextlessService`: a
webhook-only rule (no `SendCopy`/`To`), the two bound message-service reads
(`IsSystemMessage -> false`, `GetFormattedText -> nil`), and strict
`TelegramRepo`/`StateRepo`/`RateLimiter`/`TransformService`/`AlbumService`
with zero expectations, so any copy/forward, state or limiter work fails. It
takes `pushTo []string` and the captured `event.Submitter` so each test names
its own endpoint(s) and submitter.

No production file changes. No behavior defect was exposed: the code already
implements the immutable captured route, so the tests are a lock (RED =
coverage absence, not a forced failure).

Local verification (memory-bounded, GOPROXY=off, CGO_ENABLED=0):
- `gofmt` clean on the new file; `git diff --check` clean.
- `rg`-confirmed every identifier used by the new file exists in-package or in
  its deps: the eight `mocks.New*` constructors; `captureSubmitter` /
  `recordingSubmitter` / `outcomeSink` / `v2Probe` (push_test.go);
  `queue.New`/`Len`; `push.NewGeneration`/`NewWebhookSink`/`EndpointConfig` /
  `WebhookConfig{URL,Client,Schema}` / `DeliverySchemaV2`; `event.SubmitResult` /
  `event.SubmitAccepted`; `v2SnapshotFor`; `testing/synctest` sibling usage.
- `go test ./internal/service/push/...` (pure Go) green. `internal/app/handler`
  cannot compile locally (TDLib cgo); CI is the final arbiter.

Commit plan:
- Branch: `feat/external-push-sink` (no main touch).
- Commit 1 (test): `internal/app/handler/textless_reload_push_test.go` (new).
  No production file changes.
- Commit 2 (docs, this commit): `progress.md` (this entry + CI result) and
  `implement.md` (M05 status note).
- Pushed `feat/external-push-sink` to `origin` (branch only); `ci-test.yml`
  dispatched via `workflow_dispatch --ref feat/external-push-sink`; token via
  non-echoing askpass/stdin only, never in argv/env/trace/URL/config.
- Excluded (untracked, left as-is): the same protected set as the prior
  increments, including `mytokens.txt` (secret, git-excluded).
- No main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`,
  no M04 work. Remaining M05 REDs continue as separate minimal increments.

Landed + CI evidence:
- Commit 1 (test): `585f63c` — `internal/app/handler/textless_reload_push_test.go`
  (new). No production file changes. Pushed `feat/external-push-sink` to `origin`
  (branch only).
- CI: `ci-test.yml` dispatched via `workflow_dispatch --ref feat/external-push-sink`;
  run `38750426088` on head `585f63c` (branch `feat/external-push-sink`) is
  **GREEN** (`status=completed`, `conclusion=success`) across all packages,
  including the cgo-gated `internal/app/handler` carrying
  `TestTextlessMedia_ReloadKeepsCapturedRoute` and
  `TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured`.
- Commit 2 (docs, this commit): `progress.md` (this entry + CI result) and
  `implement.md` (M05 status note). Docs-only; no additional CI run required.

### 2026-10-15 — M05 increment: delivery invariance of the EDIT textless path (RED scope + commit plan)

Chosen increment (from "Remaining M05 REDs"): **delivery invariance for the
edit textless path** — the webhook-only textless media EDIT fact (edited
v2-only, caption stripped) stays delivery-invariant under full buffer,
delivery failure (timeout / http_5xx / serialization), retired generation,
and ruleset reload: Telegram behavior unchanged (strict mocks, zero
expectations), no v1 empty event fabricated, and the task queue converges.
This is a **pure-test lock increment**; zero production code changes are
expected. The edit path differs from the new path in one structural way: the
v2 fact is submitted **inside the queue task** (`editMessagesWithRetry`,
snapshot captured at `OnEditedMessage` entry), so the test must drive
`q.ProcessAll()` (or the retired/reload ordering per `push_test.go`'s retry
convention) before `Retire()` and draining.

Why it is a lock, not a behavior change: `editMessages` already submits the
textless `edited` fact via `emission.push.SubmitV2Only` (captured at entry,
immutable captured route) inside the task, before the copies loop, so no
copy/caption/state/limiter work can run. The RED is the absence of
invariance coverage for the edit path.

RED (written first; must pass against unchanged code):
- New handler test `internal/app/handler/textless_edit_invariance_test.go`
  (new file, CI-only cgo):
  - `TestTextlessMediaEdit_InvariantUnderDeliveryOutcomes` — outcome loop
    over `buffer_full`, `timeout`, `http_5xx`, `serialization`, `retired`,
    mirroring `TestPush_TelegramInvariance`'s `outcomeSink` convention but on
    the edit path: single `endpoint` on an `outcomeSink` (class set per
    outcome; `buffer_full` prefilled `BufferCapacity:1` twice, `retired`
    pre-retires the generation), behind a `captureSubmitter{next: g}`. One
    captionless source photo through `OnEditedMessage`, then
    `q.ProcessAll()`. Asserts: exactly one v2-only submission recorded with
    no v1 envelope (`capture.events` empty), `content.type = messagePhoto`,
    `edited` event, the expected `SubmitResult` per outcome, and the queue
    converges to zero (no Telegram work re-enqueued; the textless path never
    retries, so no re-enqueue either).
  - `TestTextlessMediaEdit_ReloadRetiredOutcomeStaysCaptured` — mirror of
    `TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured` for the edit path:
    the queue task submits through the generation captured at
    `OnEditedMessage` entry; after `g.Retire()` + `SetRuleSetWithPush` to a
    replacement, the replacement receives nothing, the captured submission is
    the only one, and the queue converges. Pure transport, inside
    `synctest.Test`.
- Shared helpers in-file: `editTextlessMessage()` (captionless photo,
  `Content != nil`, `MediaAlbumId == 0`, `CanBeSaved: false`, real file id on
  the source size) and `editTextlessService()` (webhook-only rule, strict
  mocks: `GetCopiedMessageIDs -> nil`, `GetFormattedText -> nil`, zero-
  expectation TelegramRepo/StateRepo/Limiter/Transform/Album).
- FAILING-as-RED rationale: the RED is coverage. If the edit textless
  submission lost immutability (used the live submitter after reload),
  fabricated a v1 envelope, enqueued Telegram work, or retried, the
  assertions above fail.

Local verification (lightweight, memory-bounded, GOPROXY=off, CGO_ENABLED=0):
`gofmt` on the new file, `git diff --check`, `rg` identifier checks, `go test
./internal/service/push/...` (pure Go) green. `internal/app/handler` is TDLib
cgo and cannot compile locally; CI is the arbiter.

Commit plan (recorded before committing, per takeover policy):
- Branch: `feat/external-push-sink` (no main touch).
- Commit 1 (test): `internal/app/handler/textless_edit_invariance_test.go`
  (new). No production file changes. (If the test exposes a real behavior
  defect, a minimal production fix lands as a separate commit with its own
  note; scope does not expand.)
- Commit 2 (docs): `progress.md` (this entry + CI result) and `implement.md`
  (M05 status note).
- Push `feat/external-push-sink` to `origin` (branch only), dispatch
  `ci-test.yml` via `workflow_dispatch --ref feat/external-push-sink`; token
  via non-echoing askpass/stdin only, never in argv/env/trace/URL/config.
- Excluded (untracked, left as-is): the same protected set as the prior
  increments, including `mytokens.txt` (secret, git-excluded).
- No main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`,
  no M04 work. Remaining M05 REDs (albums/replies, and shutdown invariance)
  continue as separate minimal increments.

Landed + CI evidence:
- Commit 1 (test): `2f4f1de` — `internal/app/handler/textless_edit_invariance_test.go`
  (new): `TestTextlessMediaEdit_InvariantUnderDeliveryOutcomes` (outcome loop
  over `buffer_full`, `timeout`, `http_5xx`, `serialization`, `retired`, each
  inside `synctest.Test`) and
  `TestTextlessMediaEdit_ReloadRetiredOutcomeStaysCaptured` (captured-route
  mirror of the new-path retired outcome). No production file changes.
- First CI: `ci-test.yml` dispatched via `workflow_dispatch --ref
  feat/external-push-sink`; run `38764274663` on head `2f4f1de` was **RED**,
  solely in `TestTextlessMediaEdit_InvariantUnderDeliveryOutcomes/buffer_full`
  (expected `buffer_full`, got `accepted`): both prefills were enqueued before
  the worker goroutine had run, so the `synctest.Wait()` after them left one
  prefill in flight (blocked in the timeout sink) and the edit submission
  found an empty queue. Test-only sequencing flaw, no production behavior
  defect — the invariance semantics themselves held.
- Commit 2 (test fix, separate per the plan): `673bf36` — pre-fill one, join
  it in flight with `synctest.Wait`, then fill the bound (the same pattern
  `TestPush_TelegramInvariance` uses). No production file changes.
- Final CI: `ci-test.yml` dispatched via `workflow_dispatch --ref
  feat/external-push-sink`; run `38765952927` on head `673bf36` (branch
  `feat/external-push-sink`) is **GREEN** (`status=completed`,
  `conclusion=success`) across all packages, including the cgo-gated
  `internal/app/handler` carrying both new tests.
- Commit 3 (docs, this commit): `progress.md` (this entry + CI result) and
  `implement.md` (M05 status note). Docs-only; no additional CI run required.

### 2026-10-15 — M05 increment: ALBUM path scope judgment (DOCUMENTATION GATE — user decision required)

Chosen increment (from "Remaining M05 REDs"): **webhook semantics for
textless media that takes the album collection path** (a captionless media
member with `MediaAlbumId != 0`). The scope judgment the plan calls for comes
first, and it lands here as a **documentation gate, not a test or code
increment**: the album textless semantics are a genuine behavior gap that a
pure-test lock cannot honestly describe, so no code or test is written and no
album membership or v1 behavior is changed this increment.

Scope judgment (read `OnNewMessage` service.go:246-278, `processMediaAlbum`
service.go:555-628, the message service `GetFormattedText`, the existing
`TestPush_AlbumPipelineInvariant`, `TestPush_AlbumMembersEmitOnce`, the M03
`TestPush_MixedEndpointsDeliverSelectedSchemas`, and the design/PRD album
boundaries):

1. **A captionless media member never reaches `processMediaAlbum`.** `GetFormattedText`
   returns `nil` for any captionless/nil-caption media content, and the
   `OnNewMessage` guard at service.go:255 is `msg.Content != nil &&
   msg.MediaAlbumId == 0` — its comment explicitly says "albums keep their
   quiet-period path." So a captionless album member (`Content != nil`,
   `MediaAlbumId != 0`) skips the textless block, hits `continue`, is **not
   added to the album service** (`AddMessage` is only reached when
   `formattedText != nil`), and produces **no v1 and no v2 webhook fact at
   ingress**. This is the exact pre-textless no-event behavior — M05 never
   changed it for album members.
2. **Within `processMediaAlbum` (service.go:592-594) the only per-member gate
   is `text == nil`.** A captionless member that somehow appeared in the
   album collection is also dropped from the webhook submission — again with
   no v1 and no v2 fact. The two existing album tests
   (`TestPush_AlbumMembersEmitOnce`, `TestPush_AlbumPipelineInvariant`) both
   pin this: the captionless member contributes an empty-`Text` v1 submission
   **only because its mock `GetFormattedText` returns an empty (non-nil)
   caption**, never `nil`.
3. **No existing test locks the captionless-album-member v2 projection
   behavior.** There is no test asserting (or even expressing) a v2 `new`
   content fact for a captionless album member, and no mixed-endpoint album
   test asserting v1-silence vs v2-fact for that member. So the "consistent
   semantics under v1/v2" condition for the pure-test-lock branch is
   **not** met: the v2 side is simply absent, not consistent.

Why this is a real gap, not a lockable invariant: M05's established rule for
textless media (single media, `MediaAlbumId == 0`) is to push a canonical v2
content fact with the v1 endpoint silent — the webhook consumer still learns
the photo exists and its type. The album path does **not** honor that rule for
captionless members: it drops them entirely (no v2 fact). A consumer of a
webhook-only rule that sends a 3-photo album where one photo has no caption
receives v2 facts for the two captioned photos and **nothing** for the
third — the v2 projection is not the full album the rule intends to publish.
This is a semantic asymmetry between the single-media and album paths, and
deciding which is correct is a product decision (see the two candidate
semantics below). Locking the current drop with a pure test would paper over
that decision.

Two candidate semantics (no decision made this increment):

- **(A) Keep the drop (status quo; pure-test lock).** A captionless album
  member emits no v1 and no v2 fact; the album's v2 projection is partial
  (one fact per captioned member). Rationale: it is exactly the pre-M05
  v1 behavior, and the approved design deliberately does not model "album
  completeness" (PRD: "V2 only preserves the source-provided
  `media_album_id` on each member"; boundaries doc: album completeness is an
  independent aggregate out of scope). Impact: no production change; a v2
  consumer must treat an album's member set as "members that carried a
  caption." This is the conservative, no-behavior-change option and the only
  one this increment could lock honestly.
- **(B) Push a v2 content fact for captionless album members.** A captionless
  album member (content-bearing, `MediaAlbumId != 0`) emits a canonical v2
  `new` content fact (full current snapshot, `content.formatted_text`
  absent) through the same quiet-period/post-collection `SubmitV2` loop, with
  the v1 endpoint silent at the delivery boundary (the v2-only render path
  `RenderPayload` → `ErrV1Unavailable` keeps a v1-selected endpoint silent,
  the same mechanism the single-media textless path relies on). Rationale:
  makes the album v2 projection complete and consistent with the
  single-media textless rule. Impact: a genuine **behavior change** to the
  album path (new webhook deliveries the endpoint did not previously
  receive), plus a production change to `OnNewMessage` (let content-bearing
  captionless album members enter `AddMessage`/`processMediaAlbum`) and/or
  `processMediaAlbum` (emit v2 facts for `text == nil` members) — and it must
  be reconciled with the `!rule.SendCopy && !msg.CanBeSaved` forward guard,
  which currently gates album ingress too. Out of scope for this increment
  (the plan forbids changing album membership or v1 behavior without a
  decision).

Decision required from the user before any test or code: pick (A) or (B) (or
a refinement) for captionless album members. Under (A) the next increment is
a pure-test lock of the current drop; under (B) the next increment is a
production change with its own RED/lock. Either way this increment writes no
album code and changes no v1 or membership behavior.

RED / commit plan for THIS increment: none (documentation gate). Local
verification: `git diff --check` and a re-read of the cited lines only; the
`internal/app/handler` package is TDLib cgo and cannot be compiled locally,
and no test or production file changes, so there is nothing to run. Commit
plan: one docs-only commit — `progress.md` (this entry) and `implement.md`
(M05 status note marking the album increment a documentation gate pending a
user decision). Push `feat/external-push-sink` to `origin` (branch only).
Docs-only, so no `ci-test.yml` run is required this increment (the previous
increment's run `38765952927` on head `673bf36` is the last green CI).
Excluded (untracked, left as-is): the same protected set as the prior
increments, including `mytokens.txt` (secret, git-excluded). No
main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`, no
M04 work. Remaining M05 REDs (albums — blocked on this decision, and shutdown
invariance) continue as separate minimal increments.

### 2026-10-15 — M05 increment: captionless album member v2 delivery (USER DECISION — candidate B)

Chosen increment (from "Remaining M05 REDs — albums"): **v2 delivery for a
captionless album member** on the NEW path. The user decided on
**candidate B** (the push-a-v2-fact option) on 2026-10-15, in response to the
prior increment's documentation gate. This is a genuine, user-approved
production change to the album NEW path; no other path (edits, replies) is
touched this increment.

Decision source: user, 2026-10-15, candidate B from the two options recorded in
the prior increment's entry above.

Exact semantics implemented this increment:

1. **A captionless album member is v2-only delivered.** A member with
   `Content != nil`, `MediaAlbumId != 0`, and `GetFormattedText == nil` must,
   after the quiet-period/collection completes, produce one canonical v2 `new`
   content fact (full current snapshot, `content.formatted_text` absent),
   consistent with the single-media textless rule, so the album's v2
   projection is complete. The v2 endpoint receives the content fact; the v1
   endpoint stays silent. Reuses the existing v2-only carrier
   (`SubmitV2Only`: zero `Envelope`, v1 branch of `RenderPayload` returns
   `ErrV1Unavailable`), exactly the mechanism the single-media textless path
   already uses.
2. **Eligibility is independent of the forward guard.** A protected
   (`CanBeSaved == false`) captionless member is delivered the same way,
   mirroring single-media textless and the locked protected-textless behavior.
   The captionless member enters album collection from the
   `formattedText == nil` block, which precedes the
   `!rule.SendCopy && !msg.CanBeSaved` guard, so the guard never suppresses it.
3. **Captioned members are completely unchanged.** Still gated by the forward
   guard, unchanged v1 representation (including the existing "empty non-nil
   caption → v1 empty-`Text` submission" semantics) and unchanged v2 fact.
4. **No fabricated facts, no Telegram work.** Content-less
   (`Content == nil`) members stay dropped (no fact). v1 emits no (empty)
   event for a captionless member. No Telegram copy/forward/download runs;
   strict zero-expectation mocks pin that.
5. **`isFirst` / captionless-first-member.** When a captionless member is the
   first to arrive, `AddMessage` returns `isFirst == true` and
   `processMediaAlbum` is triggered; both the quiet-period flow and the
   member-level projection must handle the v2-only case.
6. **Boundary: NEW path only.** If the album EDIT path has the same gap it is
   recorded as a remaining item, not fixed here. Replies are out of scope this
   round.

Production change (two minimal edits in `internal/app/handler/service.go`):

- `OnNewMessage` (service.go:246-261), the `formattedText == nil` block: add an
  album branch so a content-bearing captionless album member
  (`msg.Content != nil && msg.MediaAlbumId != 0`) is added to album collection
  via `albumService.AddMessage`, and on `isFirst` triggers
  `processMediaAlbum` (passing `nil` formattedText, which the function already
  ignores). This sits inside the `formattedText == nil` block, before the
  forward guard, so eligibility stays independent of it. The existing
  single-media textless branch (`MediaAlbumId == 0`) is left unchanged.
- `processMediaAlbum` (service.go:592-601), member submit loop: when
  `text == nil` and the member is content-bearing (`msg.Content != nil`),
  submit the v2-only content fact via `SubmitV2Only` (when the snapshot is
  non-nil), then `continue`. Content-less members (`Content == nil`) remain
  dropped. The captioned `SubmitV2` path is untouched.

RED design (forced failure; the handler package is TDLib cgo and cannot be
compiled locally, so CI RED is the evidence): new handler test
`internal/app/handler/textless_album_push_test.go`,
`TestTextlessAlbumMember_PushesV2FactOnlyV1Silent`. It drives one captionless,
protected (`CanBeSaved == false`) album member through the real NEW-path flow:
two real TLS webhook endpoints (v1 default wire, v2 `DeliverySchemaV2`), a
webhook-only rule (`PushTo: [v1, v2]`, no `To`, no `SendCopy`) so zero Telegram
work is possible, strict zero-expectation Telegram/State/RateLimiter/Transform
mocks, and album mocks bound to the desired end state (`AddMessage("r:42") →
true` isFirst, `LastReceivedAge` ≥ 3s so the quiet loop breaks without a real
sleep, `PopMessages` → the member, `GetFormattedText` nil ×2 — ingress read +
member-loop read). It asserts, after `q.ProcessAll()` + `g.Retire()`: the v2
endpoint received exactly one `new` content fact (`content.type ==
"messagePhoto"`, `media_album_id` present, no `formatted_text`, no file
identifier leak), the v1 endpoint received zero bodies, and the task queue
converged to zero. Pre-GREEN the test is forced to fail on several assertions:
the captionless member is dropped at ingress (guard `MediaAlbumId == 0` false),
so `AddMessage` is never called (strict-mock failure), no album task is
enqueued (`q.Len()` is 0, not 1), `GetFormattedText` is called once (expected
2), and the v2 endpoint is empty (expected 1).

Commit plan: (1) RED — this test file only; (2) GREEN — the two `service.go`
edits; (3) docs — `progress.md` CI evidence + `implement.md` status note. Push
`feat/external-push-sink` (branch only) after each; dispatch
`ci-test.yml` via `workflow_dispatch --ref feat/external-push-sink` after the
RED and GREEN commits. The RED run failing is expected and recorded; the GREEN
run must be fully green.

Excluded (untracked, left as-is): the same protected set as the prior
increments, including `mytokens.txt` (secret, git-excluded). No
main/PR/release/deploy/tag, no schedule/heartbeat, no `task.py start`, no M04
work. Local verification is `gofmt`, `git diff --check`, `rg` identifier
resolution and the pure-Go `go test ./internal/service/push/...` only; the
handler package is compiled and tested by CI. Remaining M05 REDs (album EDIT
path if the same gap exists, replies, and shutdown invariance) continue as
separate minimal increments.

CI evidence (this increment; branch `feat/external-push-sink`):

- RED commit `d56aa52800b50217bcdf286030d2f99c50a47126` (test file only) — run
  `38787762004`, conclusion **failure**. This is the required forced-RED: the
  handler package compiles and every other handler test passes; only
  `TestTextlessAlbumMember_PushesV2FactOnlyV1Silent` fails. The CI log shows the
  exact forced assertions: `require.Equal(t, 1, q.Len(), ...)` → "expected: 1,
  actual: 0" (textless_album_push_test.go:113 — no album task enqueued because
  the captionless member is dropped at ingress), the album mock "0 out of 3
  expectation(s) were met" (the never-called `AddMessage` /
  `LastReceivedAge` / `PopMessages`), and the message mock `GetFormattedText`
  "1 out of 2". The package did NOT fail to compile; the failure is behavioral,
  proving the test is a genuine forced-RED and not a compile error.
- GREEN commit `3df297092648dc03100934bda60f005ad36e4cd4` (the two
  `service.go` edits) — run `38790312809`, head
  `3df297092648dc03100934bda60f005ad36e4cd4`, conclusion **success** across all
  packages. This is the final criterion: the GREEN branch run is fully green
  with the forced-RED test now passing.

## M05 increment 6 — shutdown invariance for the textless/album v2-only paths — 2026-10-15

Locked shutdown/stop invariance for the three new M05 paths (single-media
textless NEW, textless EDIT, captionless album member) with a pure-test
increment: new file
`internal/app/handler/textless_shutdown_invariance_test.go`, zero production
changes. Service shutdown is modeled as generation retirement (the
carrier's only stop mechanism), and each path is driven under it:

- No v1 event ever: the NEW "delivered" case proves v1 silence at the wire
  (a real v1-counting endpoint receives zero HTTP requests while the v2
  endpoint receives exactly one content fact through the real retirement
  drain); the buffer-full/retired cases assert `capture.events` stays empty
  (no fabricated v1 envelope).
- v2 fact submitted at most once: every subtest re-drains the queue after
  shutdown (`requireAtMostOnceReDrain`) and asserts the captured fact stays
  exactly one and no Telegram work was re-enqueued.
- No Telegram copy/forward/caption/status/network work: strict
  zero-expectation `TelegramRepo`/`StateRepo`/`RateLimiter`/`TransformService`
  (and album-path mocks bound only to collection) fail on any such work.
- Queue convergence / no hang / no worker outliving the test: `q.Len()` is
  asserted zero after the task and after the re-drain; `Retire()` (idempotent
  second call) joins the sink workers — including the one parked in the
  blocking `outcomeSink{class: FailureTimeout}` from the buffer-full prefill
  pattern ("prefill one → synctest.Wait() parks it in-flight → fill the
  bound") — before the test ends.
- Content-less messages produce nothing: per-path contentless subtests
  (`Content == nil`, including the album member, which never enters
  collection) assert no v1, no v2, and no submission outcome.

Subtests per path: NEW — delivered (real servers + retirement drain),
buffer_full, retired, contentless; EDIT — buffer_full (driven with
`q.ProcessAll()`), retired, contentless; album member — buffer_full,
retired, contentless. Helpers mirror the existing M05 fixtures:
`protectedTextlessService`/`protectedSnapshot`, `editTextlessService`,
`captureSubmitter`/`outcomeSink`, `v2Probe`; one new shared helper
(`albumShutdownService`) binds the same album end state as
`textlessAlbumFixture`.

Verification (local = LIGHT only): `gofmt -l` clean, `git diff --check`
clean, `rg` identifier resolution for every reused helper/constant,
`CGO_ENABLED=0 GOPROXY=off go test ./internal/service/push/...` green
(pure Go). The handler package (TDLib cgo) was judged by CI only.

CI evidence (this increment; branch `feat/external-push-sink`):

- Test commit `19fc75b2f1f7901b1602f208e146565af5de8e9e` — run
  `38798652816`, conclusion **failure**. Not a product RED: the failure was a
  test-harness defect in this increment's own NEW/delivered subtest, which
  used plain `httptest.NewServer` with the default (non-trusting) client, so
  the v2 webhook delivery failed TLS (`class=tls_transport`) and `v2Bodies`
  stayed empty (`require.Len(t, v2Bodies, 1)` failed at line 166). Every other
  subtest in the file passed, and no production test was touched.
- Test-only fix commit
  `fc71940721e7fade8e51581b76903e7edc5e64ca` — switched the delivered case to
  `httptest.NewTLSServer` with each sink wired to its own server's client
  (matching `textlessFixture`/`textlessAlbumFixture`), same convention as the
  other M05 wire tests. Run `38799241697`, head
  `fc71940721e7fade8e51581b76903e7edc5e64ca`, conclusion **success** across
  all packages: the full shutdown-invariance lock is green in CI.

No production change was required: RED never proved a behavioral defect —
the paths were already shutdown-invariant (retired generations reject the
captured submission, the task queue never re-enqueues push work, and
`Retire()` drains and joins).

Remaining M05 items (open, separate increments): album EDIT path, replies,
and the overall M05 closure assessment.

## M05 increment 11 — reply/thread projection lock on the textless/album v2 paths (LOCK-ONLY, no behavioral RED) — 2026-10-15

Chosen increment (from "Remaining M05 items"): **replies**. Per the
pre-agreed decision rule the item was first checked for a real behavioral
gap before any test was written.

Facts (targeted reads; the handler package is cgo and is judged by CI only):

1. `v2SnapshotFor` (`internal/app/handler/projection_v2.go`) projects the
   reply and thread facts UNCONDITIONALLY from `*client.Message`:
   `ReplyTo: v2ReplyFor(msg.ReplyTo)` (bounded discriminator, and chat +
   message id for the `messageReplyToMessage` variant),
   `MessageThreadID: msg.MessageThreadId` and
   `SavedMessagesTopicID: msg.SavedMessagesTopicId`. There is no
   `IsTopicMessage`/topic gate anywhere in the repo (zero `rg` hits), so
   forum-thread messages are never special-cased before the projection.
2. `v2SnapshotFor` is the ONLY snapshot builder and it is the one used on all
   three locked paths: NEW single-media textless
   (`service.go` `OnNewMessage` -> `SubmitV2Only`), captionless album member
   (`processMediaAlbum` member loop -> `SubmitV2Only`), and textless EDIT
   including album members (`editMessages` -> `SubmitV2Only`), plus the
   captioned paths' `SubmitV2` facts.
3. No gate on these paths drops or mutates a reply-bearing message: the
   ingress/album/edit gates only read `Content`, `MediaAlbumId` and
   `GetFormattedText(msg)`, and the message passes through by pointer
   untouched, so `ReplyTo`/`MessageThreadId` survive to submission.
4. The wire serializer already emits `reply_to` / `message_thread_id` /
   `saved_messages_topic_id` and validates them
   (`internal/service/push/messagev2/serializer.go`, pinned by
   `serializer_test.go` at the messagev2 level).
5. Existing M05 handler tests pin NOTHING about reply projection on the
   textless/album paths (zero `ReplyTo`/`IsTopicMessage` references in
   `textless_*_test.go`; the only handler `ReplyTo` tests are pre-M05
   copy-routing tests around `resolveReplyTo` in `service_test.go`).

Decision: the reply/thread projection already flows on all three locked
paths — the only gap is the missing test lock. This is therefore a
**pure-test, lock-only increment with NO behavioral RED** (the new tests are
expected GREEN immediately; CI proves no regression). No production change,
no doc gate (no product decision is open: replies are not v1-specific —
the v1 envelope does not carry reply facts at all, so there is nothing to
reconcile on the v1 side).

Test commit `31c8035` (`test: lock reply/thread projection on the
textless/album v2 paths`), new file
`internal/app/handler/textless_reply_lock_test.go`, four locks:

- `TestTextlessReply_NewSingleMediaCarriesReplyThreadFact`: a reply-bearing
  textless single-media NEW (photo, `CanBeSaved == false`,
  `ReplyTo = MessageReplyToMessage{ChatId: 100, MessageId: 9}`,
  `MessageThreadId: 7`) on a webhook-only rule, driven through the real
  `OnNewMessage` with the captured generation (outcomeSink). Locks exactly
  one v2 `new` fact carrying the reply projection verbatim
  (discriminator `messageReplyToMessage`, chat 100, message 9) and
  `message_thread_id` 7, the `messagePhoto` content fact, no fabricated v1
  envelope, accepted result, queue convergence, strict zero-expectation
  Telegram/State/RateLimiter/Transform mocks (no copy/forward/state/limiter
  work).
- `TestTextlessReply_NewCaptionlessAlbumMemberCarriesReplyThreadFact`:
  the same reply-bearing facts on a captionless ALBUM member
  (`MediaAlbumId: 42`), driven through the real NEW-path flow (ingress ->
  album collection -> quiet-period member loop, `q.ProcessAll()`), album
  mocks bound to the first-member end state. Locks exactly one v2 `new`
  fact with `media_album_id` 42 plus the verbatim reply/thread projection,
  no v1 envelope, queue convergence, zero Telegram/album-side work.
- `TestTextlessReply_EditCarriesReplyThreadFact` (subtests
  `single_media` and `album_member`): an edited reply-bearing textless
  member (both `MediaAlbumId` 0 and 42, `EditDate` set) driven through the
  real `OnEditedMessage` -> queue task. Locks exactly one v2 `edited` fact
  per case with `edited_at` present, the verbatim reply/thread projection
  (`media_album_id` 42 in the album case), no v1 envelope, strict album
  mock zero calls (album collection stays NEW-only), queue convergence.
- `TestTextlessReply_ContentlessNewMessageEmitsNoReplyFact` (negative): a
  content-LESS new message (`Content == nil`) that DOES carry
  `ReplyTo`/`MessageThreadId` on the source must produce NOTHING — no v1
  envelope, no v2 fact, no submission of any kind; in particular no v2
  snapshot is fabricated carrying a reply without a content fact (which the
  serializer would reject as a snapshot without content). The
  `recordingSubmitter` records zero submissions and the queue converges.

All four reuse existing M05 fixtures/helpers (`captureSubmitter`,
`outcomeSink`, `recordingSubmitter`, `NewGeneration`, strict mock
constructors, `domain.MediaAlbumKey`); one new shared helper
(`textlessReplyService`) plus the shared `replyPhotoMember`/
`requireReplyThreadFact` builders.

Verification (local = LIGHT ONLY): `gofmt -l` clean on the new file;
`git diff --check` clean; every referenced identifier resolves by `rg`
(`SubmitAccepted` in `internal/service/push/event`,
`NewGeneration` in `internal/service/push/dispatcher.go`,
`MessageThreadId`/`SavedMessagesTopicId`/
`MessageReplyToMessage{ChatId,MessageId}` in
go-tdlib v0.7.6 `client/type.go`, `TypeMessageReplyToMessage` in
`client/type.go`); `CGO_ENABLED=0 GOPROXY=off GOCACHE=$PWD/.tmp-go-cache
go test ./internal/service/push/...` green (ok push, ok push/messagev2,
event no test files). Handler (TDLib cgo) judged by CI only; no
full-repo build/typecheck/vet/race, no parallel heavy jobs, no dep
installs locally.

CI evidence (this increment; branch `feat/external-push-sink`):

- First dispatch on the test + docs head `19dee01` — run `38805879327`,
  conclusion **failure**. Not a product RED: a compile-only error in this
  increment's own helper — `client.Message.MediaAlbumId` is
  `client.JsonInt64` (an `int64`-based named type in go-tdlib v0.7.6), so the
  helper's typed `int64` parameter needed a cast; the untyped literals used
  by the existing M05 fixtures were unaffected. Fix commit `b7c60fd`
  (`test: fix reply lock helper JsonInt64 album id cast`) changes exactly
  that one line.
- Second dispatch on head `b7c60fd` — run `38806730626`, conclusion
  **success** across all packages: the four reply/thread locks pass in CI
  (the handler package is cgo and is only exercised there), and no existing
  handler or push test regressed. The final head of this increment is
  GREEN.

No production change: the increment is a lock-only increment; if CI is
green the three locked textless/album v2 paths are proven to carry the
reply/thread projection verbatim, closing the "replies" M05 item.
Remaining M05 items after this increment: the overall M05 closure
assessment only (all REDs in the M05 test list — textless single/album
new+edit+delete, protected eligibility, reload/retire, delivery/shutdown
invariance, albums, replies — now have production behavior plus green CI
locks; full-buffer/delivery-failure invariance for the edit path was
covered by increment 6's invariance series, and the album path's
delivery-invariance series is the one remaining candidate to confirm
against the M05 checklist during closure).

## M05 increment: edited captionless album member v2-only edited fact (B semantics, symmetric edit case)

Scope judgment (pre-agreed decision rule, applied without a doc gate): tracing
the real edit path (`OnEditedMessage` -> `editMessagesWithRetry` ->
`editMessages` in `internal/app/handler/service.go`) shows the gap is exactly
the obvious case. Today, when a captionless album member (Content != nil,
MediaAlbumId != 0, nil formatted text) is edited, `editMessages` reads
`GetCopiedMessageIDs` (nil on webhook-only routes) and `GetFormattedText`
(nil), enters the `text == nil` branch, but the textless guard
`msg.Content != nil && msg.MediaAlbumId == 0` is false for album members, so
the function falls through to `return false`: the edited album member is
**dropped on the v2 side, v1 silent, nothing emitted** — while the identical
single-media case submits a v2-only `edited` fact. No existing test pinned
that behavior (no `OnEditedMessage` test sets `MediaAlbumId`;
`TestPush_AlbumNewDoesNotSupersedeEditsOrDeletes` uses a captioned edited
message with `MediaAlbumId == 0`, `TestPush_EditWithoutCopies` a
non-album photo). No re-collection entanglement exists (the edit path never
touches `albumService`; album collection is driven only by `OnNewMessage`),
captioned album member edits are unaffected (their `text != nil`), and
`Content == nil` is already excluded by the guard's existing term. Gap == the
approved B case => implemented directly.

RED (commit `897bdf4`, test file
`internal/app/handler/textless_album_edit_test.go`):
- `TestTextlessAlbumMemberEdit_PushesV2EditedFactOnlyV1Silent`: an edited
  captionless protected album member (album 42, photo sizes carrying a real
  file identifier) on a webhook-only rule, driven through the real
  `OnEditedMessage` -> queue task; strict zero-expectation Telegram/Rate
  Limiter/Transform/Album mocks (zero Telegram copy/forward/caption/state/
  limiter work AND zero album-service calls — album collection stays
  NEW-only), `GetCopiedMessageIDs -> nil` and `GetFormattedText -> nil` bound
  once each; asserts exactly one v2 `edited` fact (chat 100, message 1,
  `media_album_id` 42, `edited_at` present, content type `messagePhoto`, no
  text fact), no v1 envelope, accepted result, queue convergence. RED
  expectation: fails today because the guard's `MediaAlbumId == 0` term
  excludes the album member (zero submissions).
- `TestTextlessAlbumMemberEdit_ContentlessProducesNothing` (negative pin): an
  edited content-less album member (Content == nil, MediaAlbumId 42) produces
  nothing — no v1 envelope, no v2 fact, no submission outcome, no album
  re-collection, queue converges. GREEN both before and after the change
  (locks the no-event rule).

GREEN (commit `9541175`): the smallest production change — the edit-path
textless guard in `editMessages` extends from
`msg.Content != nil && msg.MediaAlbumId == 0` to `msg.Content != nil`
(album members admitted, v2-only `edited` fact via the existing
`SubmitV2Only` + captured-route name collection), comment updated to state
the symmetric album rule and that the edit path submits directly without
re-entering album collection. Zero changes elsewhere; no file/network/
Telegram work added; the path still returns before the copies loop.

Verification (local = LIGHT ONLY): `gofmt -l` clean on both touched files;
`git diff --check` clean; every referenced identifier resolves
(`mocks.NewStateRepo/NewMessageService`, `captureSubmitter`/`outcomeSink`/
`requireNoSubmission` in `push_test.go`/shutdown-invariance test,
`messagev2.Notification`/`Content`/`SubmitResult` fields per
`internal/service/push/messagev2/model.go`);
`CGO_ENABLED=0 GOPROXY=off GOCACHE=$PWD/.tmp-go-cache go test
./internal/service/push/...` green (ok push, ok push/messagev2, event no
test files). Note: go requires an absolute GOCACHE, so the protected
`.tmp-go-cache` was used via an absolute path (same directory, not a new
cache). Handler (TDLib cgo) is judged by CI only; no full-repo
build/typecheck/vet/race, no parallel heavy jobs, no dep installs locally.

CI evidence (this increment; branch `feat/external-push-sink`):
- Dispatch `gh workflow run ci-test.yml --ref feat/external-push-sink`
  against the GREEN head: run `38803082371`, head `9541175`, conclusion
  **success** across all packages — the new album-edit tests pass in CI
  (the handler package is cgo and is only exercised there), and the
  extended guard regressed no existing handler or push test.

No further iterations were required. Remaining M05 items: replies, and the
overall M05 closure assessment.

## M05 closure assessment — 2026-10-15

Part A (read-only coverage audit) + one lock-only test increment for the
single gap found, then this closure record. Baseline verified at takeover:
branch `feat/external-push-sink`, HEAD `8ebcc99` in sync with origin,
tracked worktree clean, only the protected untracked entries present.

### Checklist table (every M05 RED/checklist item -> named test -> CI run)

| # | Checklist item | Covering test(s) file:func | CI run (head) |
|---|---|---|---|
| 1 | Webhook-only textless media (single media) NEW -> v2 content fact only, v1 silent, no Telegram copy/forward/download work | `textless_push_test.go:TestTextlessMedia_PushesV2FactOnlyV1Silent` (+ carrier: `internal/service/push/dispatcher_test.go`, `webhook_test.go`) | `38618093819` (7108e0c era, green) |
| 2 | Textless media EDIT -> v2-only `edited` full snapshot, v1 silent, no prior state invented, no Telegram work | `textless_edit_push_test.go:TestTextlessMediaEdit_PushesV2EditedFactOnlyV1Silent` | `38666202864` (30f4d81) |
| 3 | Content-LESS new message (Content == nil, non-system) -> no fabricated v2 `new` fact (no-event rule) | `textless_push_test.go:TestTextlessMedia_ContentlessNewMessageStaysSilent` | `38681131316` (5282917c) |
| 4 | PROTECTED textless media (CanBeSaved == false) stays eligible; eligibility does not inherit the copy/forward guard | `textless_protected_push_test.go:TestTextlessMediaProtected_PushesV2FactOnlyV1Silent` | `38738578362` (19736fd) |
| 5 | New-path delivery invariance: buffer-full isolation (v2 fact still built, submission rejected buffer_full, no v1 envelope, queue converges) | `textless_protected_push_test.go:TestTextlessMediaProtected_InvariantUnderFullBuffer` | `38738578362` (19736fd) |
| 6 | New-path reload (immutable captured route): reloaded ruleset/submitter never receives an already-processed textless fact; captured generation carries it | `textless_reload_push_test.go:TestTextlessMedia_ReloadKeepsCapturedRoute`, `TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured` | `38750426088` (585f63c) |
| 7 | Edit-path delivery-outcome invariance: full 5-outcome loop (buffer_full / timeout / http_5xx / serialization / retired) — v2 fact exactly once, no v1 envelope, zero Telegram work, queue converges | `textless_edit_invariance_test.go:TestTextlessMediaEdit_InvariantUnderDeliveryOutcomes` (all five subtests) | `38765952927` (673bf36) |
| 8 | Edit-path reload captured route (replacement submitter receives nothing) | `textless_edit_invariance_test.go:TestTextlessMediaEdit_ReloadRetiredOutcomeStaysCaptured` | `38765952927` (673bf36) |
| 9 | Album path scope decision (user-approved candidate B: captionless album member is v2-only) | doc gate `bcfe6b1`; behavior: `textless_album_push_test.go:TestTextlessAlbumMember_PushesV2FactOnlyV1Silent` | RED `38787762004` (d56aa52), GREEN `38790312809` (3df2970) |
| 10 | Album NEW delivery-failure outcome loop (buffer_full / timeout / http_5xx / serialization) | `textless_album_push_test.go:TestTextlessAlbumMember_InvariantUnderDeliveryOutcomes` (four subtests; retired covered by item 12) — **this closure's lock increment** | `38812463685` (d5e96b4) |
| 11 | Album EDIT path: edited captionless member -> v2-only `edited` fact; content-less edited member produces nothing | `textless_album_edit_test.go:TestTextlessAlbumMemberEdit_PushesV2EditedFactOnlyV1Silent`, `TestTextlessAlbumMemberEdit_ContentlessProducesNothing` | RED `897bdf4`/GREEN `9541175`, CI `38803082371` (9541175) |
| 12 | Shutdown invariance on the three new paths: NEW delivered/buffer_full/retired/contentless; EDIT buffer_full/retired/contentless; album buffer_full/retired/contentless — no v1 event ever, v2 fact at most once (re-drain adds nothing), zero Telegram work, no worker outlives the test | `textless_shutdown_invariance_test.go:TestTextlessMediaNew_ShutdownInvariant`, `TestTextlessMediaEdit_ShutdownInvariant`, `TestTextlessAlbumMember_ShutdownInvariant` (all subtests) | `38799241697` (fc71940; run `38799965060` head 9321016 re-verified green) |
| 13 | Replies: reply/thread projection verbatim on all three locked paths + content-less negative (no fabricated reply-bearing snapshot) | `textless_reply_lock_test.go:TestTextlessReply_NewSingleMediaCarriesReplyThreadFact`, `TestTextlessReply_NewCaptionlessAlbumMemberCarriesReplyThreadFact`, `TestTextlessReply_EditCarriesReplyThreadFact` (single_media + album_member), `TestTextlessReply_ContentlessNewMessageEmitsNoReplyFact` | `38806730626` (b7c60fd) |
| 14 | Deletes + missing predecessors: content-agnostic identity+delete facts; delete-without-copies emits one source fact; no predecessor state needed | `push_test.go:TestPush_DeleteWithoutCopiesHonorsSourceFact`, `push_test.go:TestPush_TelegramInvariance` (delete leg), `service_test.go:TestOnDeletedMessages_*` suite, `projection_v2_test.go:TestV2Projection_DeleteCarriesOnlyDeleteFacts`, `push_coverage_test.go:TestPush_AlbumNewDoesNotSupersedeEditsOrDeletes` | pre-M05 era (M01-M03); re-verified green on every M05 run, last `38812463685` (d5e96b4) |
| 15 | Unchanged system-message actions (delete-flag handling, v1/v2 behavior untouched by M05) | `service_test.go:TestOnNewMessage_SystemMessage_DeleteEnabled`, `TestOnNewMessage_SystemMessage_NoDeleteFlag`, `TestOnNewMessage_SystemMessage_DeleteFails` | pre-M05 era; re-verified green, last `38812463685` (d5e96b4) |
| 16 | General Telegram invariance under full buffers / delivery failure / reload (all pre-existing outcomes incl. http_4xx, redirect, tls_transport, serialization, disabled) | `push_test.go:TestPush_TelegramInvariance` (10-outcome loop), `push_test.go:TestPush_V2FailureIsolatesEndpoint`, `push_album_invariance_test.go:TestPush_AlbumPipelineInvariant` (forward/copy x 6 outcomes), `push_source_routing_test.go:TestPush_SourceEditRetainsSnapshotAcrossReload` | pre-M05 era; re-verified green, last `38812463685` (d5e96b4) |

No checklist item rests on "implicitly covered": every row names at least one
concrete test function (several name the exact subtests).

### Gap finding

One gap: the **captionless album member NEW** path (increment 9) had locks for
`success` (item 9), `buffer_full`/`retired`/`contentless` (item 12) and reply
projection (item 13), but NOT the `timeout` / `http_5xx` /
`serialization` delivery-failure outcomes that the EDIT path carries via the
full 5-outcome loop (item 7). Filled by one lock-only increment:
`textless_album_push_test.go:TestTextlessAlbumMember_InvariantUnderDeliveryOutcomes`
(subtests buffer_full / timeout / http_5xx / serialization), driving the real
NEW flow (OnNewMessage ingress -> album collection -> quiet-period member-loop
`SubmitV2Only`) through an `outcomeSink` generation per outcome class.
Assertions per outcome: the v2-only `new` photo content fact is submitted
exactly once through the captured route (re-drain adds nothing), no v1
envelope is fabricated, strict zero-expectation Telegram/State/RateLimiter/
Transform/Album mocks fail on any copy/forward/state/limiter/album work, and
the task queue converges; the buffer_full prefill mirrors the
`TestPush_TelegramInvariance` pattern (parked timeout prefill joined by
`synctest.Wait` before the bound is filled; `Retire()` joins the parked
worker). The retired-generation outcome is already locked on this exact path
by `TestTextlessAlbumMember_ShutdownInvariant/retired` (item 12), so the loop
deliberately holds the four un-covered classes. No production change.

### Final state of every M05 increment

1. textless single-media NEW v2-only — green, `38618093819`.
2. textless EDIT v2-only + `Content != nil` guard — green, `38666202864`.
3. new-path content-less scope fix — green, `38681131316` (5282917c).
4. protected eligibility + buffer-full lock — green, `38738578362` (19736fd).
5. new-path reload/retired captured route — green, `38750426088` (585f63c).
6. edit-path full 5-outcome invariance loop — green, `38765952927` (673bf36).
7. album doc gate -> user approved candidate B — `bcfe6b1`.
8. captionless album member NEW v2-only — green, `38790312809` (3df2970).
9. shutdown invariance (NEW/EDIT/album outcome sets) — green,
   `38799241697` (fc71940) and `38799965060` (9321016).
10. album EDIT path (edited captionless member v2-only; guard extended to
    `Content != nil`) — green, `38803082371` (9541175).
11. replies projection lock-only (all three paths + content-less negative) —
    green, `38806730626` (b7c60fd).
12. **closure gap-fill (this assessment)**: album NEW
    timeout/http_5xx/serialization outcome loop — lock-only test commit
    `d5e96b4`, CI `38812463685` **success** (job `test`), head `d5e96b4`.

Local verification (light only): `gofmt -l` clean on the touched test file;
`git diff --check` clean; new symbols (`albumOutcomeService`,
`requireAlbumOutcomeSubmission`, `TestTextlessAlbumMember_InvariantUnderDeliveryOutcomes`)
resolve with no package-level duplicates; `CGO_ENABLED=0 GOPROXY=off
GOCACHE=$PWD/.tmp-go-cache go test ./internal/service/push/...` green. The
handler package is TDLib cgo and is compiled/tested by CI only (no local
full-repo build/typecheck/vet/race, no parallel heavy jobs, no dep installs).

### What M05 does NOT include

- M04 remains a **product decision gate with no code**: nothing in M05 opens
  or bypasses the no-download decision; final M05 does not include any file
  access, download, cache, hosting, or resource lifecycle.
- **Nothing is auto-downloaded**: no media byte, file_id surface, or network
  fetch of message content is added; the v2 facts are content-identity
  projections (the source file identifier is asserted absent from every wire
  body by the wire test).
- **No `resource_ref`** (or any `resource_ref -> TDLib file_id` mapping) is
  introduced, persisted, or projected; Attachment/Relation authority is
  untouched.
- **`main` untouched**: all work is on `feat/external-push-sink`; no merge,
  no PR, no release/deploy/tag.
- No Telegram queue/limiter file/network work is added under full buffers,
  delivery failure, reload or shutdown (AC2/AC4 invariance locked by items
  5, 7, 8, 10, 12, 16).

### CI record (this assessment)

- Test increment commit `d5e96b4` pushed to `origin/feat/external-push-sink`;
  `gh workflow run ci-test.yml --ref feat/external-push-sink` -> run
  `38812463685` (https://github.com/killbus/budva/actions/runs/38812463685),
  head `d5e96b4`, conclusion **success** (job `test`: `go vet ./...` +
  `go test ./internal/...`, green across all packages; new handler tests
  exercised in CI only).
- Final verified head: `d5e96b4` (run `38812463685` green).

## M05 closure statement

M05 is **closed**. Every item in the M05 checklist (webhook-only textless
media; albums — including the user-approved candidate-B captionless-member
semantics on both NEW and EDIT; replies; edits/deletes with honest
missing-predecessor behavior; protected eligibility; unchanged
system-message actions; and Telegram invariance under full buffers, delivery
failure (all classes), reload, and shutdown for every new path) is covered by
a named test referenced in the checklist table above, and the branch is
green at `d5e96b4` with all evidence recorded. The only open remainder in
the plan is M06 (real-TDLib focused CI verification and final integration),
which is a separate milestone and is NOT part of M05 scope; no new M05
remainder was found or invented by this assessment.

## M06 progress (2026-10-15)

### Baseline verification (before any change)

- HEAD `fbb45e2` (task-stated baseline), branch
  `feat/external-push-sink` in sync with origin (`0 0` ahead/behind),
  tracked worktree clean; only the protected untracked entries present
  (`.tmp-go-cache/`, `.trellis/.developer`, `.trellis/.runtime/`,
  `.trellis/scripts/`, `.trellis/tasks/10-11-external-push-sink/*`,
  `.zvec-grep/`, `probe.js`). No unexplained modifications; baseline matched.
- PR #3 state at start: OPEN, draft, base `main`, head `fbb45e2`, ci-test
  run `38824497529` **success** at head `fbb45e2` (verified via the run API).

### Step 1 — dispatch-only M06 verification workflow

- Commit `caed41e` — `ci: add dispatch-only M06 verification workflow`
  (only file changed: `.github/workflows/ci-verify-m06.yml`), pushed to
  `origin/feat/external-push-sink`.
- The workflow is `workflow_dispatch` ONLY (no push/PR trigger) — it can
  never auto-fire on `main`, even after a future merge (defense in depth).
  It reuses ci-test.yml's exact TDLib sourcing + split
  restore/save pattern under the SAME cache key `tdlib-22d49d5-amd64`, so a
  hit restores the tree ci-test already built (no 20-25 min cold build);
  the cold-cache path keeps the Dockerfile's `--parallel 2` OOM lesson.
  Jobs (one job, sequential, one 16GB runner): `go build ./...`,
  `go vet ./...` (standalone evidence), focused race
  `go test -race ./internal/service/push/... ./internal/app/handler/...`,
  plain `go test ./internal/...`.
- Push side effect (expected): the PR's own `ci-test` re-fired on the
  `pull_request` trigger.

### Step 2 — dispatch + watch (BLOCKED at this record; see open question)

- `gh workflow run ci-verify-m06.yml --ref feat/external-push-sink` ->
  HTTP 404 (`/actions/workflows/ci-verify-m06.yml` "Not Found").
- Root-cause check: `GET /actions/workflows` (repo workflow index) lists
  only `ci-test.yml` (354474075) and `docker-publish.yml` (353176217)
  `state=active`; the new file is NOT indexed. The file IS on the remote
  branch (verified: branch tip `caed41e`, file present on
  `feat/external-push-sink` at 6657 bytes). GitHub's dispatch API resolves
  workflow_id-by-name against that index, and a dispatch-only workflow that
  exists only on a non-default branch is not in it (it has never fired, so
  it is not known to the index either; the default branch `main` carries
  only `ci-test.yml` + `docker-publish.yml`). In total ~20 dispatch
  attempts over ~45 min (immediate, +20s, +80s, then steady +2.5min
  intervals up to +~45min, CLI and raw endpoint) all returned 404; the
  index never picked the file up during this session. This is a
  deterministic platform constraint on the dispatch-only design, not a
  slow index: the only index population paths are (a) default branch,
  (b) a prior run on the ref — neither exists for a dispatch-only file
  on a feature branch.
- Consequence: the dedicated M06 evidence run (whole-repo build + vet +
  focused race) has NOT been executed. **Not skipped-as-passed: the M06
  evidence is recorded as not-executed**, and a focused-race finding, had
  one existed, is unknown.
- Open question for the owner (pick one):
  1. Merge the workflow file onto `main` separately (or approve a
     temporary one-line push-trigger on the branch) so the index knows the
     workflow, then re-run M06 step 2 with
     `gh workflow run ci-verify-m06.yml --ref feat/external-push-sink`; or
  2. accept PR #3 on the strength of the full ci-test suite at
     `caed41e` (vet + all `./internal/...` tests, incl. cgo handler
     packages, green) and execute the build/focused-race evidence on
     `main` post-merge.
- What IS covered without the dispatch: `go vet ./...` and
  `go test ./internal/...` (cgo TDLib packages included) are green in the
  PR ci-test run below at the final head. NOT covered anywhere: whole-repo
  `go build ./...` and the race detector (repo kill list — this is the
  gap M06 step 2 closes).

### Step 2' — PR ci-test re-fire on the push (executed, green)

- Pushing `caed41e` automatically re-fired `ci-test` (pull_request
  trigger): run `38825373952`
  (https://github.com/killbus/budva/actions/runs/38825373952), head
  `caed41e58046b89d31e27a0378595e507505b6e1`, conclusion **success** (job
  `test`: `go vet ./...` + `go test ./internal/...`, green across all
  packages). This is the final PR-triggered run and the "final state"
  evidence available before the dispatch channel.
- Restored-source state: no cold TDLib build was paid on this run (the
  `tdlib-22d49d5-amd64` cache hit; the run completed in normal ci-test
  duration, not the 20-25 min cold-build duration).
- The prior baseline run `38824497529` (head `fbb45e2`) also **success**.

### Step 3 — final structure / spec review (local, read-only)

- `resource_ref` / `file_id` mapping: rg over the tracked tree shows
  `resource_ref` ONLY in the decision/progress documentation
  (`.trellis/tasks/10-12-...`), where it appears exclusively in
  "do not / must not / retracted" negations. No Go code, wire schema,
  API, or cmd surface references it. `file_id` in Go appears only as
  NEGATIVE assertions (`require.NotContains(body, "file_id")` in the
  textless wire tests; the serializer_test banned-field list), pinning
  that no TDLib file_id leaks onto the wire. No mapping store, no
  Attachment/Relation authority: unchanged.
- No auto-download / cache / hosting code paths: rg over `*.go`
  (non-test) for download/host/cache surfaces returns nothing; the only
  `cache` token on the v2 wire is TDLib's own delete fact
  `delete.from_cache`, which is a preserved source fact, not a Budva
  cache. `api/` and `cmd/` expose no file/download endpoints.
- v1 wire unchanged: the v1 `Envelope`
  (`internal/service/push/event/envelope.go`) is untouched since the
  pre-M03 baseline; all M05 work is additive v2-only paths
  (`messagev2`, captured-route submission, v1 silence locked by tests).
  Wire tests still pin the v1 envelope shape and v1 silence on every
  textless/album path.
- M04 boundary text consistency across
  `.trellis/tasks/10-12-.../{prd.md, design.md}`,
  `research/message-attachment-boundaries.md`,
  `research/m02-protocol-decision.md`,
  `research/m03-message-v2-wire-schema.md`: all state the same amended
  boundary (no `resource_ref`/mapping; no Budva Attachment/Relation
  authority; webhook publication must not trigger downloading; a future
  approved schema may project reviewed native file facts in
  account/session scope as a direct source fact; media byte access /
  hosting / caching / expiry out of scope). Status lines agree
  (M03 done 2026-10-14, M04 a no-code gate, M05 the delivery
  milestone). No stale doc found; no text fix needed.

### Step 4 — live-account tests

- `test/bdd` requires a live Telegram account (LiveStack). NOT executed
  in M06 (no live-account claim); recorded as not-executed, not
  skipped-as-passed, per AC4/AC6.

### Local verification (light only, on `caed41e`)

- `gofmt -l` clean; `git diff --check` clean;
  `CGO_ENABLED=0 GOPROXY=off GOCACHE=$PWD/.tmp-go-cache go test
  ./internal/service/push/...` green (push + messagev2 `ok`). The
  cgo handler package is compiled/tested by CI only; no local full-repo
  build/vet/race, no parallel heavy jobs, no dep installs.

### M06 status at this record

- DONE: workflow authored + pushed (`caed41e`), dispatch-only, exact
  TDLib sourcing/cache reuse; final PR ci-test green at the same head
  (`38825373952` @ `caed41e`); structure/spec review clean; live-account
  tests recorded not-executed; gates honored (no main touch, no merge,
  PR #3 still draft, no release/deploy/tag, no task.py start, worktree
  never cleaned, protected entries untouched).
- NOT DONE (blocked, see open question above): the dispatched M06 evidence
  run itself (whole-repo build, standalone vet evidence, focused race) —
  GitHub's workflow index does not expose the branch-only dispatch-only
  workflow to the dispatch API (persistent 404 over ~30 min of retries).
