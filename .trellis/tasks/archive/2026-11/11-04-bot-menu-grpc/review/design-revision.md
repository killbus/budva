# Design revision — round 1

## G1 — flood-wait detection and RetryInfo source

- Revised `design.md` §6: the 5 new RPC TDLib call sites are wrapped by an exported
  infra helper; existing call paths are not required to change this round.
- Revised `design.md` §7: infra owns `FloodWaitError{ Delay time.Duration }` and
  `FloodWaitFromError(err error) error`; it reuses package-private `parseFloodWait`
  (`repo.go:299`) without duplicating the regex. Transport only maps via `errors.As`.
- Revised `design.md` §7: `RetryInfo.RetryDelay` uses the existing `parseFloodWait`
  semantics of `N seconds + 500ms` (`repo.go:311`), not exactly `N seconds`.
- Revised `design.md` §8: added the mechanical test for input `retry after 7`,
  asserting infra delay `7.5s`, gRPC `ResourceExhausted`, and matching `RetryInfo`.

## Non-blocking assumptions

- `design.md` §4③: tagged the `message_id=0` fallback as `[ASSUMPTION]`; it is one
  `GetChatHistory(limit=1)` call, not polling.
- `design.md` §7: replaced “such as 10s” with `callbackAnswerTimeout =
  10 * time.Second` and tagged it `[ASSUMPTION]`; identified as a server deadline,
  not polling, based on go-tdlib v0.7.6 502 timeout semantics (`function.go:7458`).
- `design.md` §5: tagged `WebAppButtonInfo` as `[ASSUMPTION]`; it faithfully exposes
  the button type while v1 does not execute Web App actions.
- `design.md` §5/§10: added `InlineButton.other_type` for non-executable button
  classes and corrected the user-reference class name to
  `InlineKeyboardButtonTypeUser` (`type.go:15903` in go-tdlib v0.7.6). The review
  prompt’s example said `UserShared`, but the vendored dependency uses `User`;
  this is a factual correction, not an added behavior. v1 executes only
  `callback_data`.

## Untreated items

- None. `task.json` was intentionally left unchanged per instructions; its stale
  scope wording remains for the main session to synchronize.
