# Draft Execution Plan

Status: active; M01 is complete and the user approved the M02 Message-only
design on 2026-10-13. M03 source verification and RED/GREEN implementation are
authorized. Read `research/message-attachment-boundaries.md` first.

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
- [ ] M04: Deferred. Define or implement resource access only after a separate
  product requirement and design approval establish independent Attachment or
  byte-access needs.
- [ ] M05: Integrate source-handler lifecycle and immutable captured routes.
  Test webhook-only textless media, albums/replies, edits/deletes, missing
  predecessors, protected eligibility and unchanged system-message actions.
  Prove Telegram invariance under slow media/full buffers/failure/reload/shutdown
  without network/file work on the serial queue/limiter (AC2, AC4).
- [ ] M06: Execute real TDLib Linux internal tests, focused races, whole-repo
  vet/build, final full-scope/structure review and spec sync through GitHub CI when
  these jobs are unsafe or too heavy locally. Local verification stays narrowly
  scoped and memory-bounded. Record the iteration branch, commit SHA, workflow/run
  URL, executed red controls and restored-source hashes. Final integration requires
  explicit owner agreement and a pull request; no live-account claim, direct main
  update, merge or deployment by implication (AC4, AC6).

M03 is authorized by the user's explicit 2026-10-13 approval.
M04 is not an implied dependency of Message-only v2 and must not begin without a
separate resource product decision. Do not turn this into a second v1
album/edit repair.

## Resume Instructions

Read the boundary decision, context manifests, PRD, design, the content matrix,
M01 fixtures, this plan and `progress.md`. Continue M03 by freezing the exact
Message-only v2 wire schema and source mapping, then execute RED/GREEN runtime
implementation. Do not enter M04 resource access or broaden Message into media
delivery. Verify assembly against current code; the correction CI may finish
asynchronously but is not an M03 implementation blocker.
Check its exact run/SHA when accepting correction or choosing publication; do
not poll/watch it during the user's requested no-wait task-production turn.
