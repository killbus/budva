# Approval — design lock (11-05 bot-reply-keyboard-menu)

- task: `.trellis/tasks/11-05-bot-reply-keyboard-menu`
- base_commit: `ab172b7`
- design: `design.md`
- design_sha256: `06ae508531c0af182ecca4c8e9b1da9c9a706f60147294c06a75a6eaa3fb4b2c`
- approved_round: 1
- verdict: PASS (D1–D8 met, no blocking finding)
- reviewer: Design Reviewer (paseo agent `30706945-2611-4157-8b38-036fd87de6c2`)
- approved_at: 2026-11-05

## Non-blocking assumption notes (from the gate)

- `MenuKind` enum naming/values and `InlineMenuResponse` field numbers 3–8 are
  `[ASSUMPTION]` (proto3 additive convention; A1/A4/A5 name the semantic values).
- `ReplyButton` shape/field numbers 1–3 are `[ASSUMPTION]`.
- flood-wait on `GetChat` maps to `Internal`; `Repo.GetChat` is intentionally unchanged
  (preserves transform/source_opener semantics). Known limitation.
- missing markup on the pointed message → `MENU_KIND_NONE`, no fallback.
- Implementer should explicitly assert `message_id` backfill in the reply-keyboard test
  (A1), not only the inline baseline.
