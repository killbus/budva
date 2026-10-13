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

Resume M03: verify exact canonical field names and source availability for
edit/delete, freeze the v2 wire-schema fixtures, then implement M03 with
RED/GREEN tests. Keep v1 as the default; do not enter M04 resource access.
Do not push, publish an image, merge or deploy. `task.py start` requires
recording intent and stopping for user approval.

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
`messagev2.MarshalNotification` at the delivery boundary. Push of this task
branch for CI evidence remains possible only when a relevant workflow check
is required; none is required for this pure-Go increment until the handler
wiring lands.
