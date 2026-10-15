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
  handled honestly with no v1 empty event). Remaining REDs (albums/replies,
  protected eligibility, full-buffer/delivery-failure/reload/shutdown
  invariance for the new path) continue as separate minimal increments.
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
