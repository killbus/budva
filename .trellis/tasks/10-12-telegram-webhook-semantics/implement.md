# Draft Execution Plan

Status: active; M01-M03 and M05 are complete (M05 closed 2026-10-15 at
`d5e96b4`, CI `38812463685` green; closure checklist in `progress.md`).
The 2026-10-14 decision correction makes M04 a product
gate rather than an implementation milestone. Read
`research/message-attachment-boundaries.md` first.

## Ordered Work

- [x] M01: Retain the pinned 74-type inventory; revalidate the field mapping as
  separate message facts, direct media, attachment representations and Album
  semantics. Verify sizes/thumbnails/covers/main files and edit update shape.
  Write reasoned exclusions, ingress policy and separate fact fixtures (AC1).
- [x] M02 design candidate: Define Message v2 facts, new/edit/delete
  self-description, identity, album/reply semantics, v1/v2 migration,
  unknown-content and serialization-failure behavior. Defer Attachment,
  Relation, byte access and all resource infrastructure because current evidence
  does not justify those product contracts. The user approved this design on
  2026-10-13, authorizing M03 (AC1, AC3, AC5).
- [x] M03: Implement the approved canonical serializer and opt-in/version
  migration with known-red type/entity/unknown/nil/limit fixtures. Keep v1
  compatibility and avoid raw TDLib exposure (AC1, AC5).
- [x] M04 decision gate: No implementation in this task. `/app/engine` remains
  the passive Message webhook path and facade remains the external active-
  interaction boundary. Do not invent `resource_ref`, persist
  `resource_ref -> TDLib file_id`, or create Attachment/Relation authority.
  If native Telegram file access is approved later, open a narrow versioned
  follow-up that projects reviewed native message/file facts and extends the
  existing facade in account/session scope. No automatic download, cache,
  hosting or generic resource lifecycle is implied.
- [x] M05: Integrate source-handler lifecycle and immutable captured routes.
  Test webhook-only textless media, albums/replies, edits/deletes, missing
  predecessors, protected eligibility and unchanged system-message actions.
  Here "textless media" means lifecycle eligibility and canonical content facts
  within the already approved M03 schema; it does not mean file download or
  byte delivery. Prove Telegram invariance under full buffers, delivery failure,
  reload and shutdown without file/network work added to the Telegram serial
  queue/limiter (AC2, AC4). Progress 2026-10-14: first increment landed —
  webhook-only textless media pushes as a canonical v2 content fact with the
  v1 endpoint silent (user decision "v2 fact only, v1 silent"); second
  increment landed — a textless media EDIT now pushes a v2-only `edited`
  content fact (full current snapshot, no prior state invented), the v1
  endpoint stays silent (no v1 empty event), and no Telegram copy/caption
  work runs; the delete path was audited and already needed no change (it
  emits content-agnostic identity+delete facts, so "missing predecessor" is
  handled honestly with no v1 empty event). The guard is scoped to
  content-bearing textless media (`msg.Content != nil && MediaAlbumId == 0`),
  so a content-less edit stays silent per the earlier no-event rule; final CI
  `38666202864` (head `30f4d81`) is green across all packages. Third increment
  landed 2026-10-15 — the NEW-path textless block in `OnNewMessage` is now
  scoped the same way the edit path already was (`msg.Content != nil &&
  MediaAlbumId == 0`), so a content-LESS new message (Content == nil, non-
  system, nil formatted text) is left to the earlier no-event rule instead of
  fabricating a v2 `new` fact with `content.type = "unknown"`; the already-green
  textless-media NEW delivery (captionless photo, Content != nil) is unchanged
  and keeps the v2-only content fact with v1 silent. Fourth increment landed
  2026-10-15 (pure-test lock, no production change) — a PROTECTED textless media
  message (Telegram cannot save/copy it: `CanBeSaved == false`, carrying
  `HasSensitiveContent`/`RestrictionReason`) is still eligible for the
  webhook-only v2 content fact; eligibility deliberately does NOT inherit the
  copy/forward guard. The new handler test
  `internal/app/handler/textless_protected_push_test.go` pins the `success`
  outcome (one v2 body carrying the protected facts verbatim, v1 silent, queue
  converged to 0) and the `buffer_full` outcome (the protected v2 fact is still
  built from the source, the submission is rejected `buffer_full` and isolated
  to the endpoint, no v1 envelope fabricated, queue converged). Fifth
  increment landed 2026-10-15 (pure-test lock, no production change) — the
  RELOAD outcome for the new textless path: the handler reads the ruleset
  snapshot once at the `OnNewMessage` entry and submits through the submitter
  captured with that snapshot (immutable captured route); after a
  `SetRuleSetWithPush` reload swaps in a new ruleset/submitter, an already-
  processed textless message still routes to the captured (old) generation,
  the post-reload submitter receives nothing, v1 stays silent, and the task
  queue converges to zero. The new handler test
  `internal/app/handler/textless_reload_push_test.go` pins both the live-reload
  outcome (real TLS v1/v2 endpoints, active generation not pre-retired, mirroring
  production `pushManager.Replace`) and the retired outcome (synctest +
  outcomeSink, captured submission accepted, replacement receives nothing).
  Remaining REDs (albums/replies, and full-buffer/delivery-failure/reload/
  shutdown invariance for the edit path) continue as separate minimal
  increments. Sixth increment landed 2026-10-15 (pure-test lock, no
  production change) — the EDIT textless path is delivery-invariant: under
  `buffer_full`, `timeout`, `http_5xx`, `serialization` and retired
  generation the webhook-only textless media EDIT fact stays submitted
  exactly once as the canonical v2 `edited` content fact through the
  submitter captured at the `OnEditedMessage` entry (v2-only submission
  happens inside the queue task, driven by `q.ProcessAll()`), with strict
  zero-expectation mocks pinning no Telegram copy/caption/state/limiter
  work, no fabricated v1 envelope, and the task queue converged to zero;
  the reload/retired outcome stays on the captured generation (the
  replacement submitter receives nothing). The new handler test
  `internal/app/handler/textless_edit_invariance_test.go` pins both the
  outcome loop and the captured-route reload/retired outcome. First CI on
  the test commit exposed a test-only sequencing flaw in the `buffer_full`
  prefill (both prefills enqueued before the worker ran, so the bound was
  not actually full at submission time); it is fixed by pre-filling one,
  joining it in flight, then filling the bound (the
  `TestPush_TelegramInvariance` pattern). Final CI `38765952927` (head
  `673bf36`) is green across all packages. Remaining REDs (albums/replies,
  and shutdown invariance) continue as separate minimal increments. Seventh
  increment 2026-10-15 (documentation gate, no code or test) — the ALBUM path
  for textless media: a scope judgment found that a captionless album member
  (`Content != nil`, `MediaAlbumId != 0`) is dropped at `OnNewMessage` ingress
  (the textless guard is scoped to `MediaAlbumId == 0`) and again in
  `processMediaAlbum` (the `text == nil` gate), so it produces no v1 AND no v2
  fact — the album's v2 projection is partial, unlike the single-media textless
  rule that pushes a v2 content fact with v1 silent. No existing test locks
  that captionless-album-member v2 behavior, so this is a genuine behavior gap,
  not a lockable invariant. Two candidate semantics are recorded in
  `progress.md` (A: keep the drop, pure-test lock — the no-change option;
  B: push a v2 content fact for captionless album members — a behavior change
  touching album membership/ingress and v1 guard reconciliation). This
  increment marked the album item **pending a user decision**; no album
  membership or v1 behavior was changed. Eighth increment landed 2026-10-15
  under the user's decision (candidate B — a production behavior change): a
  captionless album member (`Content != nil`, `MediaAlbumId != 0`, nil
  formatted text) is now v2-only delivered, consistent with the single-media
  textless rule. Two minimal `service.go` edits — `OnNewMessage` lets a
  content-bearing captionless album member enter the quiet-period album
  collection (before the forward guard, so a protected `CanBeSaved == false`
  member is collected too) and, as the first member, triggers
  `processMediaAlbum`; `processMediaAlbum` routes a `text == nil`
  content-bearing member through the v2-only carrier (`SubmitV2Only`) so the
  v1 endpoint stays silent at the delivery boundary, while content-less
  members stay dropped. Captioned members are unchanged. Forced-RED
  `38787762004` (head `d56aa52`) failed behaviorally (no compile error), and
  the GREEN run `38790312809` (head `3df2970`) is fully green. Ninth
  increment landed 2026-10-15 (pure-test lock, no production change) —
  SHUTDOWN invariance for the three new M05 paths (single-media textless
  NEW, textless EDIT, captionless album member). Service shutdown is
  modeled as generation retirement; per path the new handler test
  `internal/app/handler/textless_shutdown_invariance_test.go` locks: no v1
  event ever (the delivered case proves v1 silence at the wire against a
  real counting endpoint through the real retirement drain, the other cases
  assert no fabricated v1 envelope), the v2 fact submitted at most once
  (a post-shutdown queue re-drain adds nothing and re-enqueues no Telegram
  work), no Telegram copy/forward/caption/status/network work (strict
  zero-expectation mocks), queue convergence with no hang and no worker
  outliving the test (the parked `FailureTimeout` prefill worker is joined
  by the idempotent `Retire()`), and content-less messages (including the
  album member, which never enters collection) produce nothing. Subtests:
  NEW delivered/buffer_full/retired/contentless; EDIT
  buffer_full/retired/contentless (driven with `q.ProcessAll()`); album
  buffer_full/retired/contentless. No production change was needed — the
  first CI run RED (`38798652816`) was a test-harness defect in this
  increment's own delivered subtest (plain `httptest.NewServer` with a
  non-trusting client), fixed test-only (`fc71940`, per-server TLS clients
  as in the other M05 wire tests); final CI `38799241697` (head `fc71940`)
  is green across all packages. Tenth increment landed 2026-10-15 (small
  production change, the symmetric edit case of the album B decision) — the
  ALBUM EDIT path for textless media: tracing the real edit path
  (`OnEditedMessage` -> `editMessagesWithRetry` -> `editMessages`) showed
  today's behavior of an edited captionless album member (`Content != nil`,
  `MediaAlbumId != 0`, nil formatted text) is exactly the approved gap — it is
  dropped on the v2 side (the textless guard `msg.Content != nil &&
  msg.MediaAlbumId == 0` is false for album members, so `editMessages`
  returns before any emission), v1 silent, nothing emitted — while the
  identical single-media case submits a v2-only `edited` fact; no existing
  test pinned that, and no re-collection entanglement exists (the edit path
  never touches `albumService`; album collection stays NEW-only). Per the
  pre-agreed decision rule the gap matched the obvious case, so it was
  implemented directly without a doc gate. GREEN: the guard extends to
  `msg.Content != nil` (commit `9541175`) so an edited captionless album
  member submits the canonical v2 `edited` content fact v2-only (captured
  route, v1 endpoint silent, no v1 event fabricated, no file/network/
  Telegram copy/forward/caption/state/limiter work, no album re-collection);
  a content-less edited album member (`Content == nil`) still produces
  nothing. New handler test file
  `internal/app/handler/textless_album_edit_test.go` (commit `897bdf4`):
  positive (exactly one v2 `edited` fact, `media_album_id` carried, v1
  envelope-free, strict zero-expectation Telegram AND album mocks, queue
  convergence) plus the content-less negative pin. CI run
  `38803082371` (head `9541175`) is green across all packages — the cgo
  handler tests pass in CI and no existing test regressed. Remaining M05
  items: replies, and the overall M05 closure assessment.
  Eleventh increment landed 2026-10-15 (pure-test lock, no production
  change) — REPLIES on the three locked textless/album v2 paths. The
  decision-rule check found no behavioral gap: `v2SnapshotFor` projects
  `ReplyTo` (bounded discriminator + chat/message id for
  `messageReplyToMessage`), `MessageThreadID` and `SavedMessagesTopicID`
  unconditionally from `*client.Message`, is the only snapshot builder, and
  is what all three locked paths submit (NEW single-media, captionless
  album member, textless EDIT incl. album member); no `IsTopicMessage`/
  topic gate exists anywhere, and the message reaches submission by
  pointer untouched, so the only gap was the missing test lock. The new
  handler test
  `internal/app/handler/textless_reply_lock_test.go` (commit `31c8035`)
  locks: a reply-bearing textless single-media NEW with the verbatim
  reply/thread projection (v2-only, v1 silent, no Telegram work), a
  reply-bearing captionless album member (same projection through the
  quiet-period member loop, `media_album_id` carried), a reply-bearing
  textless EDIT for both single-media and album member (`edited` fact with
  the projection, no album re-collection), and the content-less negative
  (a `Content == nil` source that carries reply/thread facts emits
  nothing — no fabricated reply-bearing v2 snapshot). No behavioral RED:
  the projection already flowed; CI proves no regression.
  M05 closed 2026-10-15: closure assessment confirmed every checklist item
  has a named test; the one coverage gap (album NEW delivery-failure outcome
  loop) was filled by a lock-only test (`d5e96b4`, CI `38812463685` green);
  see the "M05 closure assessment" entry in `progress.md`.
- [x] M06: Execute real TDLib Linux internal tests, focused races, whole-repo
  vet/build, final full-scope/structure review and spec sync through GitHub CI when
  these jobs are unsafe or too heavy locally. Local verification stays narrowly
  scoped and memory-bounded. Record the iteration branch, commit SHA, workflow/run
  URL, executed red controls and restored-source hashes. Final integration requires
  explicit owner agreement and a pull request; no live-account claim, direct main
  update, merge or deployment by implication (AC4, AC6).
  Progress 2026-10-15: the dispatch-only evidence workflow
  `.github/workflows/ci-verify-m06.yml` (whole-repo build, standalone vet,
  FOCUSED race on push+handler, plain `./internal/...`) is committed
  `caed41e` and pushed; the PR's ci-test re-fired and is green at the same
  head (`38825373952`). The final structure/spec review is clean (no
  `resource_ref`/file_id mapping, no download/cache/hosting paths, v1 wire
  unchanged, boundary text consistent; live-account tests recorded
  not-executed). BLOCKED on the dispatched M06 run itself: GitHub's
  workflow registry does not expose a brand-new dispatch-only workflow that
  exists only on a non-default branch to the dispatch API (persistent 404
  over ~30+ min of retries; file verified on the branch tip). Open question
  for the owner is recorded in `progress.md` (M06 progress entry): either
  land the workflow file on `main` (or a temporary trigger) so the dispatch
  channel works, or accept the full ci-test suite at `caed41e` and run the
  build/race evidence on `main` post-merge. M06 is NOT marked complete: the
  evidence run did not execute and is not claimed.
Closed 2026-10-15: main evidence run 38845373348 (ci-verify-m06.yml @ c9368c9) succeeded with whole-repo build, standalone vet, focused race (push+handler) and plain test re-run. Main carries the 8-commit logical history per .trellis/spec/guides/commit-hygiene.md; the docs increment lands via PR #4 (squash-merge). See the M06-closed entry in progress.md.

M03 is complete. M04 is resolved as a no-code decision gate and is not a
dependency of M05. Do not turn M05 into a file API, resource platform or second
v1 album/edit repair.

## Resume Instructions

Read the amended boundary decision, PRD, design, schema decision and
`progress.md`. Continue with M05 only after checking that its RED controls do
not presume media bytes, file download, a resource mapping or new facade APIs.
Keep local work narrow and memory-bounded; move necessary TDLib/CGO or broad
verification to GitHub CI. Work stays on the iteration branch. Integration into
`main` requires explicit owner agreement and a pull request.
