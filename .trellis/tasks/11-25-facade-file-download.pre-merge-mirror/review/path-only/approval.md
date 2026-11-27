# Approval — design lock (11-25 facade-file-download)

- task: `.trellis/tasks/11-25-facade-file-download`
- base_commit: `fbc22cc`
- design: `design.md`
- design_sha256: `e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75`
- approved_round: 4
- verdict: PASS (D1-D8 met, no blocking findings)
- reviewer: Design Reviewer (paseo agent `36594bbf-03bb-4428-b10e-c5920fb60433`)
- approved_at: 2026-11-25
- supersedes: round-2 lock (design_sha256 `653f4a18122e3bd6acc95ddcad5119290cdb89de3d9ba2975942f14fc45c4d89`),
  invalidated by the G2/G3 design revisions.

## Round history

- Round 1: FAIL — G1 (D6): prd R1's "optional variant selection" had no design
  landing point. Orchestrator resolution: PRD is the intent source; the user
  requirement was download capability only (variant selection = YAGNI).
  `prd.md` revised (R1 narrowed to chat_id+message_id; variant selection moved
  to Non-goals). Design Reviser (agent
  `270e40fa-6155-48f2-92b7-32fb9e7ca129`) aligned `design.md` via option (b):
  new non-goal N8 + §1/§3/§5/§6/§9/§11 wording alignment
  (`review/revision-notes.md`).
- Round 2: PASS — G1 verified resolved against the revised prd.md/design.md;
  no new D1-D8 violations. (Lock superseded by the G2/G3 revisions.)
- Round 3: FAIL — G2 (D4): design §6.5 used the nonexistent
  `GetFileMimeTypeRequest{FileId}` (real struct carries only `FileName`,
  function.go:6366-6369) and the `GetFile` passthrough has no live consumer;
  plus G3: the §2 mapping table carried 6/8 wrong go-tdlib field names
  (`MessageVoice` does not exist; it is `MessageVoiceNote`). Resolution
  (Revisers `96d3ade5` — Revision 2, `c5aefa1b` — Revision 3): `mime_type`
  derived from the resolved message content (§6.5 table); adapter growth
  narrowed to `DownloadFile` only (§6/§8); §2 table corrected to the real
  v0.7.6 fields with line anchors; prd.md R3 defers the wrapper set to
  design §8. See `review/revision-notes.md`.
- Round 4: PASS — G1/G2/G3 all verified resolved (table cells checked
  cell-by-cell against the pinned go-tdlib v0.7.6 module cache); D1-D8 met;
  no new violations; no regressions. **This is the lock.**

## Non-blocking assumption notes (from the gate)

- [ASSUMPTION] §1 photo variant = max `PhotoSize.Photo.Size`, ties -> last in
  slice order (mechanical definition; prd fixes only "largest variant").
- [ASSUMPTION] §6.3 `DownloadFile` `Priority: 16` (mid of TDLib's 1-32).
- [ASSUMPTION] §6.5 `mime_type` is best-effort, derived from the resolved
  message content: photo -> `"image/jpeg"` (PhotoSize has no MIME field);
  video/document/audio/animation/voiceNote -> their `MimeType` field;
  sticker/videoNote -> `""`. `GetFileMimeType` is NOT part of the contract
  (see §1 Revision 2 fact note).
- [ASSUMPTION] §7 `FileDownloadFailedError` -> `codes.Internal` (no dedicated
  gRPC code; PRD R4 defers the mapping to §7).
- [ASSUMPTION] §8 `FloodWaitFromError` wrap on `Repo.DownloadFile` is
  defensive only.
- N8: no client-selectable photo variant selection in this ticket (largest
  variant only); an additive proto field can be introduced later
  (forward-compatible).

## Note for P2 (implementer)

The uncommitted worktree code predates this lock (it was written against the
superseded round-2 design). It must be realigned to this lock: mime from
message content fields, `GetFile`/`GetFileMimeType` passthroughs removed from
`client_adapter.go` + mocks + tests, adapter growth = `DownloadFile` only.
See `review/design-findings.json` (round 4, process note),
`review/revision-notes.md` (Revisions 2-3), and `implement.md` steps 3/5.
