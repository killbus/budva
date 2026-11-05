# Research — reply keyboard / bottom chat menu in TDLib

Date: 2026-11-05. Baseline: `ab172b7` (v0.5.0). TDLib pinned by `Dockerfile:15`
(`git checkout 22d49d5`, full SHA `22d49d5b87a4d5fc60a194dab02dd1d71529687f`).
go-tdlib `v0.7.6`.

## 1. Problem evidence (live, owner-provided)

Bot `8862641954`, private chat. `GetChatHistory` returns a normal text message:

```
id: 181538914304
text: "🏠 Main Menu … 👤 User ID: 498139926 … Select an option below:"
```

Screenshot shows a bottom button panel (below the input): "Gemini Plus (12 Months)",
"Check-in", "Top Up", "Redeem Code", "Exchange Points", "Referral", "Profile",
and the input placeholder **"Choose a menu…"**.

`GetInlineMenu(chat_id=8862641954, message_id=0)` and with the explicit id both return
`rows: []`. `GetBotInfo` returns `commands: []`, `menu_button: null`.

Conclusion: this is **not** an inline keyboard and **not** bot profile info. It is a
TDLib **reply keyboard** (`replyMarkupShowKeyboard`); `input_field_placeholder` is a
field only present on that type.

## 2. TDLib data model (pinned source)

- `td/generate/scheme/td_api.tl:2197` — `chat.reply_markup_message_id`:
  "Identifier of the message from which reply markup needs to be used; 0 if there is
  no default custom reply markup in the chat". This is the authoritative pointer for
  the currently rendered bottom keyboard.
- `td/generate/scheme/td_api.tl:1813` — `message.reply_markup`: "Reply markup for the
  message; may be null if none". The reply keyboard is stored on the message that set
  it (the "Main Menu" message), even though the UI renders it below the input.
- `td/telegram/MessagesManager.cpp` — the dialog keeps `reply_markup_message_id` and
  restores the markup after restart:
  - `set_dialog_reply_markup(...)` (`:14372`)
  - `try_restore_dialog_reply_markup(...)` (`:14394`), used at `:9127`, `:9153`, `:14139`
  - non-inline markup of an expired/TTL message is stripped and flagged
    `had_reply_markup` (`:12446-12450`), so a message may legitimately have no markup
    even if it once had one.
- `td_api.tl:1208` — `botInfo` owns `commands` / `menu_button` / descriptions; these are
  profile-level data from `users.getFullUser`, unrelated to `message.reply_markup`.

## 3. go-tdlib v0.7.6 surface

- `client/type.go:14956` — `Chat.ReplyMarkupMessageId int64` (`json:"reply_markup_message_id"`).
- `client/type.go:15622` — `KeyboardButton{Text string; Type KeyboardButtonType}`.
- `client/type.go:16057-16071` — `ReplyMarkupShowKeyboard`:
  `Rows [][]*KeyboardButton`, `IsPersistent`, `ResizeKeyboard`, `OneTime`,
  `IsPersonal`, `InputFieldPlaceholder`.
- `client/type.go:950-956` — class names returned by `KeyboardButtonTypeType()`:
  `keyboardButtonTypeText`, `...RequestPhoneNumber`, `...RequestLocation`,
  `...RequestPoll`, `...RequestUsers`, `...RequestChat`, `...WebApp`.
- `client/type.go:15595` — `KeyboardButtonTypeWebApp{Url string}`.
- `client/type.go:2478` — `KeyboardButtonType interface { KeyboardButtonTypeType() string }`.
- `ReplyMarkup` also includes `ReplyMarkupRemoveKeyboard` / `ReplyMarkupForceReply`
  (`client/type.go:969-972`), which are not menus.

## 4. Current Budva code (baseline facts)

- `internal/transport/grpc/pb/facade.proto:156-159` — `InlineMenuResponse{message_id=1,
  rows=2}`; `GetInlineMenuRequest{chat_id=1, message_id=2}` at `:132`.
- `internal/app/facade/service.go:191-206` — `GetInlineMenu`: `message_id==0` →
  `GetChatHistory(limit=1)` and take `messages[0]`; no `GetChat` usage.
- `internal/transport/grpc/transport.go:413-458` — `inlineMenuToProto` maps **only**
  `*client.ReplyMarkupInlineKeyboard`; any other/nil markup returns an empty response.
- `internal/infra/telegram/client_adapter.go:290-301` — `Repo.GetChat` exists, wrapped by
  `withChatReady`, but does **not** call `FloodWaitFromError` (unlike
  `GetChatHistory`/`GetMessage` at `:261`/`:134`).
- `internal/app/facade/service.go:18-36` — app-layer partial `telegramRepo` interface does
  not yet include `GetChat`; mock at `internal/app/facade/mocks/telegram_repo.go`.
- `internal/transport/grpc/transport.go:39-85` — `mapFacadeError`; existing typed errors:
  `FloodWaitError`, `MessageNotFoundError`, `BotNotFoundError`, `TargetNotBotError`,
  `CallbackAnswerError`, `ChatNotReadyError`.

## 5. Wheel check / no-new-dependency

No library is needed: go-tdlib already exposes every field. The change is a mapping +
one `GetChat` call. No Bot API, no second session, no Web App runtime.

## 6. Verified interpretation

- The bottom menu = `Message.reply_markup` of `chat.reply_markup_message_id`; usually
  `ReplyMarkupShowKeyboard`.
- `botInfo.commands` / `menu_button` are a different feature (input-row "Menu" button /
  command list) and are legitimately empty for this bot.
- Clicking a reply-keyboard button sends its text as a regular message; it is not a
  callback query.
