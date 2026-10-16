# Webhook-only ruleset loading

## Problem

`check()` (`internal/infra/ruleset/repo.go`) rejected a ruleset whenever
`UniqueDestinations` was empty. Only `enrich()` populates that set, and
only from `rule.to`. A forward rule that delivers exclusively to external
webhooks (`pushTo`, no `to`) therefore failed `Load()` with
`ErrEmptyConfig`, and the engine's load-error branch
(`cmd/engine/main.go`) skipped push generation initialization — a
webhook-only deployment could not deliver a single event.

## Scope

Loader only. No handler, delivery, schema, or Telegram behavior changes.
Push eligibility was already independent of the copy/forward path at the
handler; this makes the loader agree.

## Safety verification (by reading, before coding)

- Every `rule.To` consumer iterates a slice; empty is a no-op:
  `processMessage` destination loop, `processMediaAlbum` destination
  loop + `newTracker(rule.To)`, stats increment, edit/delete copy loops.
- Push submissions (`SubmitV2`/`SubmitV2Only`) happen before or
  independent of any `rule.To` loop, gated on `len(rule.PushTo) != 0`.
- Filter default without include/exclude is `FiltersOK`
  (`internal/service/filters/service.go`), so webhook-only rules receive
  all eligible messages.
- The dedup tracker is an injected factory (`DedupFactory`); with no
  destinations it is created per call but never marked.

## Change

`check()` passes when the ruleset has rules and every rule delivers
somewhere: `len(rule.To) > 0 || len(rule.PushTo) > 0`. An empty file
still fails, and a rule with neither `to` nor `pushTo` still fails with
`ErrEmptyConfig` (it would load as dead configuration).

## Tests (RED -> GREEN, CI adjudicated; package is cgo-gated locally)

- `TestRepo_Load_WebhookOnlyRule`: webhook-only file loads; unique
  source present (negated: `transform()` inverts chat IDs, the loaded
  set holds `-1001000`), `UniqueDestinations` empty, ordered rule and
  `pushTo` preserved.
- `TestRepo_Load_WebhookOnlyRuleHotReload`: fsnotify callback `Load()`
  succeeds, so the engine reload branch goes through `loadSnapshot`
  instead of `reload_failed`.

## CI evidence

- RED: run 38865775461 (`6b196cc`) — failure, behavioral (both tests at
  the `Load()` assertion: `ErrEmptyConfig`); package compiled, all other
  packages ok.
- First GREEN attempt: run 38866006408 (`9cba9a0`) — failure, test-side
  only: asserted the YAML-literal chat ID while `transform()` negates
  it; fixed in `3142b0d`.
- GREEN: run 38866247732 (`3142b0d`, branch tip) — success: `go vet`
  clean, `go test ./internal/...` all ok including
  `internal/infra/ruleset 0.025s` (fresh, not cached).
