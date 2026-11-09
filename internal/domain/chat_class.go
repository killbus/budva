package domain

// chat_class.go — классификация чата по идентификатору для open-state-политики
// (задача 10-30, Problem-1: Budva должен владеть границей аквизиции своих
// источников, открывая каналы/супергруппы в собственной сессии).

// channelIDMagnitude — граница модуля идентификатора канала/супергруппы.
// ID чатов-каналов/супергрупп в TDLib по модулю БОЛЬШЕ этой величины (метка
// PeerChannel, префикс «-100...»); ID базовых групп по модулю — МЕНЬШЕ.
// Источник: Telethon v1.42.0 resolve_id (`if marked_id > 1000000000000:
// PeerChannel`) и фикстура 1002641439846 = 1e12 + 2641439846 в
// internal/infra/ruleset/repo_test.go. Граница включена в «не канал»:
// m == 1e12 → базовая группа (не открыть), m == 1e12+1 → канал (открыть).
const channelIDMagnitude = 1_000_000_000_000

// IsChannelOrSupergroup сообщает, требует ли чат с идентификатором id OpenChat,
// чтобы TDLib доставлял ему апдейты. Открытое состояние (open_count) гейтит
// getChannelDifference-поллинг и пассивные апдейты ТОЛЬКО для каналов/
// супергрупп (pinned TDlib: open-dialog-гейт, gap-recovery :7206, re-arm
// open_count>0 :37423). ID < -1e12 — канал/супергруппа (нужен OpenChat).
// Базовые группы и приватные чаты используют глобальный поток обновлений.
//
// Runtime IDs use TDLib's signed representation after ruleset loading:
// private chats (including bots) are positive, groups/channels negative.
// A positive ID must never be classified as a channel by magnitude alone.
func IsChannelOrSupergroup(id ChatID) bool {
	return id < -channelIDMagnitude
}
