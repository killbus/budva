// Package telegram реализует обёртку над TDLib для работы с Telegram API.
//
// Request-only role (facade and stand):
//
//	r := telegram.New(cfg, telegram.NoBusinessUpdates)
//	if err := r.Start(ctx); err != nil { ... }
//	defer r.Close()
//	events := r.AuthStates() // Authorization events still need their normal consumer.
//
// NoBusinessUpdates allocates no business outlet and requires no update drain.
// Calling Updates in this mode panics with a programming error. The SDK pump
// and Repo listener still run: private send results, NewChat readiness and
// convergence, and authorization handling are independent of business delivery.
//
// Business-consumer role (engine and LiveStack):
//
//	r := telegram.New(cfg, telegram.BusinessUpdates)
//	updates := r.Updates() // Allocated at construction, before Start.
//	if err := r.Start(ctx); err != nil { ... }
//	defer r.Close()
//	consumeUpdates(ctx, updates) // Alongside the normal authorization consumer.
//
// BusinessUpdates retains a capacity-100, filtered outlet with ordered blocking
// delivery. The mode is fixed at construction. Updates is an accessor, not
// registration: repeated calls return the same channel; multiple readers compete
// for events, with no broadcast. Consumers must keep draining it for publication
// to progress. No new channel-closure guarantee is introduced; consumers should
// use their context for shutdown. Engine stalls and SDK close races remain
// unresolved; no cancellation or full shutdown safety is established.
//
// Конфигурация:
//
//	TELEGRAM_API_ID            — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH          — хеш приложения Telegram (required)
//	TELEGRAM_PHONE             — номер телефона для авторизации (required)
//	TELEGRAM_DATABASE_DIR      — путь к директории TDLib (default: .data/tdlib)
//	TELEGRAM_FILES_DIR         — путь к файлам TDLib (default: .data/tdlib-files)
//	TELEGRAM_SYSTEM_LANG       — код языка системы (default: en)
//	TELEGRAM_DEVICE_MODEL      — модель устройства (default: Server)
//	TELEGRAM_LOG_VERBOSITY     — уровень логирования TDLib (default: 0)
//	TELEGRAM_USE_FILE_DB       — файловый кеш TDLib (default: true)
//	TELEGRAM_USE_CHAT_INFO_DB  — кеш информации о чатах (default: true)
//	TELEGRAM_USE_MESSAGE_DB    — кеш сообщений (default: true)
//	TELEGRAM_USE_SECRET_CHATS  — поддержка секретных чатов (default: false)
//	TELEGRAM_SYSTEM_VERSION    — версия системы (default: "")
//	TELEGRAM_APP_VERSION       — версия приложения (default: 1.0.0)
//	TELEGRAM_LOG_DIR           — директория логов TDLib (default: .data/tdlib-logs)
//	TELEGRAM_LOG_MAX_SIZE      — макс размер лог-файла в MB (default: 10)
//	TELEGRAM_WARMUP_DEADLINE   — окно wait-for-ready на холодной БД (default: 5s)
//	TELEGRAM_WARMUP_TICKER     — период тика ожидания прогрева (default: 500ms)
//
// Ограничения:
//
//   - Start() инициализирует TDLib-клиент, настраивает логирование и запускает цикл авторизации.
//   - SubmitPhone/SubmitCode/SubmitPassword делегируют ввод в TDLib authorizer.
//   - Close() retains the existing adapter reset, not full SDK shutdown or outlet closure.
//   - ParseTextEntities/GetMarkdownText — статические вызовы TDLib, работают до авторизации.
//   - GetOption — метод *Repo, обёртка над client.GetOption; доступен до авторизации.
//   - CreateNewSupergroupChat/CreateNewBasicGroupChat/SetSupergroupUsername/DeleteChat — методы для cmd/stand.
//   - SendMessageAndWait блокирует до получения permanent ID (таймаут 60 сек), подписывается через pendingSends; поддерживает retry при FLOOD_WAIT.
//   - Обёртки pull-вызовов (см. client_adapter.go) на холодной БД входят в
//     wait-for-ready: при «400 Chat not found» ждут материализации chat в
//     ограниченном окне (см. warmup.go); по истечении окна — ChatNotReadyError
//     (транспорт маппит её в gRPC Unavailable + RetryInfo).
//   - Updates() returns filtered client.Type values only in BusinessUpdates mode;
//     resolving UpdateMessageEdited through GetMessage remains the consumer's
//     responsibility (cmd/engine/main.go, internal/test/support/live_stack.go).
package telegram
