# Revision notes — round 2 (G1: variant-selection coverage vs prd.md R1)

Verdict addressed: `FAIL` / blocking finding **G1** (round 1).
Chosen resolution: **option (b)** — explicit non-goal entry documenting that
client-selectable photo variant selection is omitted from this ticket, aligned
with the updated `prd.md` (R1 + Non-goals).

Scope: `design.md` only. No `prd.md`, `.trellis/spec/`, or source changes.

## Changes to design.md

- **§1 Context** — after the existing [ASSUMPTION] photo-variant paragraph,
  added an explicit statement that client-selectable variant selection is NOT
  part of this ticket: `GetMessageFile` always resolves to the largest variant
  (no request field, no app-layer parameter, no test for optional selection);
  the omission is deliberate and declared in non-goal N8; an additive proto
  field can be introduced later without breaking the contract.
- **§2 Non-goals** — added **N8**: no client-selectable photo variant
  selection in this ticket; largest variant only; the request carries no
  variant field; future additive proto field is forward-compatible.
- **§3 Module boundaries** — reworded the `internal/app/facade` responsibility
  cell to "largest-photo-variant selection (default-only, N8)" so the module
  row matches the chosen scope.
- **§5 gRPC contract** — after the field-numbers note, added a sentence that
  `GetMessageFileRequest` intentionally carries ONLY `chat_id` and `message_id`
  (no variant-selection field), matching N8.
- **§6 App-layer contract** — reworded the `*client.MessagePhoto` resolution
  step to the "fixed largest variant by max `Photo.Size` (ties → last in slice
  order), no client-selectable variant parameter (N8)" — the design already
  implemented only the default; wording now says so explicitly.
- **§9 Tests** — added one sentence to the A1 photo test bullet stating that no
  test exercises client-selectable variant selection (out of scope, N8); only
  the fixed largest-variant default is asserted. Test plan is now explicitly
  consistent with the non-goal.
- **§11 File set** — extended the "Out of scope" line to include
  "no client-selectable photo variant selection (N8)", so the file set is
  explicitly consistent with the chosen option.

## Consistency check

- §5 request shape (chat_id + message_id only) ↔ N8 ↔ §6 (no variant
  parameter) ↔ §9 (no variant-selection test) ↔ §11 (out of scope): all
  aligned.
- §1 assumption text retained (max-size, ties → last) and now explicitly
  scoped as the only selection behavior in this ticket.
- No other sections rewritten or expanded; all other round-1 findings were
  already met and were left untouched.

## Outcome

G1 resolved under option (b): the design now records an explicit non-goal (N8)
plus §1/§5/§6/§9/§11 wording consistent with it, giving the Implementer a
ruler and the P3 reviewer a verifiable in-scope/out-of-scope boundary.

# Revision 2 — G2

Scope: `design.md` only. No `prd.md`, `.trellis/spec/`, or source changes.

## Resolution

- Added the Revision 2 fact note in §1: `GetFileMimeTypeRequest`
  (`client/function.go:6366-6369`) has only `FileName`; the prior
  `{FileId: file.Id}` call shape does not exist, and the API guesses MIME from
  a filename and returns empty on failure. Consequently, the design no longer
  adds or calls `GetFileMimeType`; it also omits `GetFile` because
  `GetMessageFile` already has the message content. Only `DownloadFile`
  remains as a clientAdapter/Repo method with a live consumer.
- §5 keeps the additive `mime_type` proto field but updates its semantics:
  MIME comes from the resolved message content; missing/empty content fields
  map to `""` without turning `GetMessageFile` into an error.
- §6.5 now specifies direct content-derived MIME with the complete mapping:
  photo → `image/jpeg` [ASSUMPTION], video/document/audio/animation/voiceNote
  → their content MIME fields, sticker/videoNote → `""`; the app-layer
  `GetMessageFile` signature is unchanged.
- §6 interface growth and §8 clientAdapter/Repo scope are reduced to
  `DownloadFile` only, preserving the "only methods with live consumers" rule.
- §9 replaces the MIME-RPC error test with content-source assertions for
  document, video/audio/animation/voiceNote, sticker/videoNote, photo, and the
  nil/empty-field fallback. §11 shrinks the client_adapter.go increment to one
  adapter method and one Repo passthrough.

## Consistency check

- §2 non-goals require no clientAdapter changes; §6/§8 now agree by adding
  only the one live-consumer passthrough.
- Removed the contradictory blanket "no `FloodWaitFromError`" wording in §8;
  the existing method-specific `Repo.DownloadFile` wrapping sentence is now
  the sole statement.
- `GetFileMimeType` and `GetFile` remain only in the §1 revision fact note
  describing the rejected approach; neither remains in the contract.
- All A1/A2/A3 assertions remain concrete and mechanically falsifiable; D5
  section numbers are unchanged and no new decision was introduced.

# Revision 3 — G3

Scope: `design.md` only. No `prd.md`, `research/`, `.trellis/spec/`, or source
changes.

## Resolution

- Corrected the §1 `Message content → file mapping` table so all 8 rows match
  go-tdlib v0.7.6 and carry `type.go` line anchors:
  `MessagePhoto` → `Photo.Sizes []*PhotoSize`, `PhotoSize.Photo *File`
  (`type.go:4420-4425`); `MessageVideo` → `Video.Video *File`
  (`type.go:5470`); `MessageDocument` → `Document.Document *File`
  (`type.go:5328`); `MessageAudio` → `Audio.Audio *File` (`type.go:5297`);
  `MessageVoiceNote` → `VoiceNote.Voice *File` (`type.go:5565`);
  `MessageAnimation` → `Animation.Animation *File` (`type.go:5258`);
  `MessageVideoNote` → `VideoNote.Video *File` (`type.go:5505`);
  `MessageSticker` → `Sticker.Sticker *File` (`type.go:5394`). Renamed the
  nonexistent `MessageVoice` content-type row to `MessageVoiceNote` and its
  wrapper field to `VoiceNote *VoiceNote`.
- Added a §1 note directly below the table explaining TDLib's repeated wrapper
  and inner-file field names and clarifying that `MessageVoice` does not exist
  in v0.7.6. The note references the superseded `Video.File` spelling only to
  rule it out; no such symbol remains in the contract.
- Changed §6.2 from the concrete, incorrect “the table's `File` field” wording
  to a reference to the file object named in the §2 table's `file object`
  column.
- Updated §9 A1 to cover the 8 content types of the §2 table, renamed the
  listed `voice` shorthand to `voiceNote`, and added that each fixture uses the
  file object named in the §2 table's `file object` column. No wrong field
  names are listed.

## Changed lines in design.md

- §1 mapping table: lines 36-43.
- §1 TDLib schema/type-name note: lines 45-49 (new).
- §6.2 other-media resolution reference: lines 160-161.
- §9 A1 per-media-type test wording: lines 242-253.

## Consistency check

- Verified the corrected table against the pinned module cache at
  `C:\Users\ying\go\pkg\mod\github.com\zelenin\go-tdlib@v0.7.6\client\type.go`.
- Verified that `Video.File`, `Audio.File`, `Voice.File`, `Animation.File`,
  `VideoNote.File`, and `Sticker.File` no longer appear as contract fields;
  the sole `Video.File` occurrence is the explanatory superseded-spelling note.
- Verified that `MessageVoice` remains only in the explanatory note stating
  that the type does not exist.
- Verified §1-§13 remain unchanged in number and order, and the photo-variant
  [ASSUMPTION], §3 module responsibility wording, §5 proto contract, §7 errors,
  §8 adapter scope, §11 file set, and all other G1/G2 content were not touched.
