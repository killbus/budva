# Webhook-only ruleset — progress

## Current status: GREEN on branch, PR pending (2026-10-16)

User requested (option 1 of the ruleset discussion): make webhook-only
rulesets load, so a deployment whose forward rule has `pushTo` and no
Telegram `to` can deliver events.

## Commits (branch `feat/webhook-only-ruleset`, base `main` @ `df85b6b`)

- `6b196cc` test: pin webhook-only ruleset loading (RED) + task.json
- `9cba9a0` feat: load webhook-only rulesets (check() relaxation)
- `3142b0d` test: expect negated chat ID in loaded unique sources
- docs commit (this entry)

## CI

- RED run 38865775461 (`6b196cc`): failure at both `Load()` assertions —
  behavioral, as intended.
- GREEN run 38866247732 (`3142b0d`, tip): success (vet + full internal
  test scope).

## Behavior notes for consumers

- A loaded webhook-only rule has empty `rule.To`; all Telegram-side
  consumers no-op, push eligibility is unchanged.
- `UniqueDestinations` stays empty for such rules; no destination-level
  state, rate limiting, or dedup is engaged.
- Hot reload of a webhook-only ruleset succeeds end to end (watcher
  callback `Load()` green in the RED/GREEN pair).

## Gate

Branch green; merge to main via squash-merge PR per
`.trellis/spec/guides/commit-hygiene.md` (owner approval required).

## Merge hygiene (2026-10-16)

- Squash-merge PR #6 opened (base `main`).
- Branch cleaned into a single logical commit `714bd33`
  (`git reset --soft main` + commit) before merge; tree-identity
  verified against the pre-clean tip `a6f1549` (empty diff) and against
  `origin` tip `ed3f1ed` (same tree).
- Cleaned-tip CI run `38870037275` (`714bd33`): success.
- Merge step: `gh pr merge --squash` - main receives exactly one commit.
