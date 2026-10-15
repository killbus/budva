# Draft Execution Plan

Status: active; M01-M03 are complete and M05 is in progress (first increment
done on 2026-10-14). The 2026-10-14 decision correction makes M04 a product
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
- [ ] M05: Integrate source-handler lifecycle and immutable captured routes.
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
  increments.
- [ ] M06: Execute real TDLib Linux internal tests, focused races, whole-repo
  vet/build, final full-scope/structure review and spec sync through GitHub CI when
  these jobs are unsafe or too heavy locally. Local verification stays narrowly
  scoped and memory-bounded. Record the iteration branch, commit SHA, workflow/run
  URL, executed red controls and restored-source hashes. Final integration requires
  explicit owner agreement and a pull request; no live-account claim, direct main
  update, merge or deployment by implication (AC4, AC6).

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
