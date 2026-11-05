# Bot menu operations over gRPC

## Goal

Decide whether/how Budva's gRPC can support interacting with a bot's menu.

**Semantics confirmed by owner 2026-11-04** (was ambiguous; see `research/bot-menu-grpc.md` §8):

> A **user account** interacts with the menu of a bot inside that bot's private chat.

This is option **A2** in the research — **not** managing a bot's own commands via Bot API.
No bot token, no second session, no new dependency; reuse the existing TDLib user session.

Research phase (done): survey existing implementations first (go-tdlib, gotd/td, go-telegram
libs) so Budva does not reinvent; deliver a report with feasibility verdict and the wheel
survey. Implementation still requires owner go-ahead.

## Agreed v1 scope (owner, 2026-11-04)

"完整 v1", with the explicit carve-out **不取信息 = 不轮询 chat 中的回复**.

In: `ResolveBot`; `GetBotInfo`; `GetInlineMenu`; `ClickCallbackButton`; `SendBotCommand`.

Out: **any fetching/polling of bot replies** (no such RPC, no stream); Web App menus;
bot-account menu CRUD; a generic `Execute(opcode, payload)` RPC.

Design is in `design.md` (proto surface, layering, error mapping, tests, prerequisites).

## Requirements

- Research is the dominant deliverable; no production code in this task.
- Establish which account/session each menu operation requires (user vs bot) and prove it against the pinned libraries/docs.
- Survey reusable implementations at the client layer and at the gRPC layer; explicitly record the absence of a usable gRPC-layer wheel.
- Produce a decision tree mapping the three plausible meanings of "bot menu" to an implementable option (A/A2/B/C/D/E) with cost and risk.
- Identify the blocking ambiguity (what the requester means by "bot menu 操作码") and stop before design/implementation.

## Acceptance Criteria

- [x] `research/bot-menu-grpc.md` exists with evidence (file:line / official docs / module-cache source).
- [x] Current gRPC surface and its gaps are documented (10 RPCs; `Message` DTO drops `ReplyMarkup`).
- [x] The "for bots only" constraint on TDLib menu functions is documented against `go-tdlib@v0.7.6`.
- [x] Candidate wheels are compared with maintenance/star data; no mature gRPC-layer wheel is found.
- [x] Options A/A2/B/C/D/E include cost, risk, fit, and a "do not reinvent" rule.
- [x] Owner answered the interpretation question (2026-11-04): user-side menu interaction in a bot's private chat → option A2.
- [ ] Owner confirms the v1 scope (command menu + inline callback click; WebApp excluded) and gives go-ahead.
- [ ] Only then create `design.md` / `implement.md` and start implementation.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
