// Package ruleset загружает и отслеживает YAML-файл правил пересылки.
//
// Использование:
//
//	r := ruleset.New(cfg)
//	rs, err := r.Load()
//	r.WatchContext(ctx, func() { /* reload callback */ })
//	defer r.Close()
//
// Конфигурация:
//
//	RULESET_PATH — путь к YAML-файлу с правилами пересылки (default: ruleset.yml)
//
// Ограничения:
//
//   - Load() читает и валидирует файл, возвращает domain.RuleSet.
//   - WatchContext() запускает fsnotify-наблюдатель за файлом.
//   - Close() останавливает наблюдатель.
//   - chatIdFormat: tdlib uses native signed IDs in every chat reference:
//     positive for private/bot chats, negative for groups and channels.
//   - Missing chatIdFormat or explicit legacy keeps the historical convention:
//     positive group/channel IDs in YAML are negated at load time.
package ruleset
