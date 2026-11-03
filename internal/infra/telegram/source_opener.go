package telegram

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// source_opener.go — Budva владеет границей аквизиции своих источников
// (задача 10-30, Problem-1).
//
// Корень: для классов, гейтингованных open-state (каналы/супергруппы, любого
// размера) TDlib-движок доставляет апдейты только для ОТКРЫТЫХ чатов (open-
// dialog-гейт) и ведёт open_count-gated getChannelDifference-поллинг (pinned
// TDlib 22d49d5: gap-recovery :7206, re-arm `open_count>0` :37423). Пассивная
// сессия, которая никогда не вызывает openChat, не является активным
// аквирайером → тишина, пока какая-то сессия не откроет чат (наблюдаемый
// open→burst / close→silence).
//
// Искра (converged): открыть каждый канал/супергруппу-источник в СОБСТВЕННОЙ
// сессии Budva и ДЕРЖАТЬ открытым (open-once-keep; re-open только на state-
// reset-краях). Сам re-poll владет привязанный C++-движок — opener НЕ имеет
// собственного таймера (deliberate отличие от mtcute/teleproto, которые
// hand-roll'ят re-poll, потому что не линкуют TDlib).
//
// Два края (единственные драйверы ApplySet):
//   - startup adapter-swap (полная открыта): repo зажигает зарегистрированный
//     колбэк в runAuthLoop-ом swap-крит-секции (реальная граница сессии; НЕ
//     AuthorizationStateReady — тот отстаёт и может потеряться, repo.go:355-361);
//   - hot-reload diff (добавил→открыть / удалил→закрыть): из Replace-колбэка
//     (cmd/engine main.go), тот же колбэк, что переключает push-поколение.
//
// Приватные/базовые группы НЕ открываются (глобальный поток, не гейтен по
// open-state; официальный клиент их не «открывает»). Классификатор — по модулю
// id (domain.IsChannelOrSupergroup): |id|>1e12 → канал/супергруппа.

// openRetryPasses — сколько раз в пределах ОДНОГО края повторяется проход по
// ещё не открытым источникам после неудачных open (R4). «Retry on the next
// edge» НЕ сходится, когда «next edge» не существует (единственный источник,
// без reload и без re-auth) — in-edge retry и есть то, что заставляет
// неудавшийся open восстановиться без внешнего события. Ограничено, чтобы не
// держать worker вечно.
const openRetryPasses = 4

// openRetryBackoff — пауза между повторными проходами (bounded backoff, R4).
const openRetryBackoff = 2 * time.Second

// sourceOpenerClient — узкий RPC-контракт opener'a. Удовлетворяется *Repo
// (прод-путь; GetChat — withReady-обёртка, OpenChat/CloseChat — тонкие) и
// test-fake. OpenChat/CloseChat — 1-в-1 с go-tdlib v0.7.6.
type sourceOpenerClient interface {
	// GetChat — резолвит/греет input-peer перед OpenChat (open_dialog early-
	// return на !have_input_peer, mm.cpp:19527). На холодной БД сработает
	// wait-for-ready (LoadChats-драйв) через *Repo-обёртку.
	GetChat(*client.GetChatRequest) (*client.Chat, error)
	OpenChat(*client.OpenChatRequest) (*client.Ok, error)
	CloseChat(*client.CloseChatRequest) (*client.Ok, error)
}

// SourceOpener держит in-process состояние open-set и failure/pending и
// выпускает ровно один idempotent вход: ApplySet(ids). Реальные RPC-вызовы
// (GetChat→OpenChat / CloseChat) выполняет ЕДИНЫЙ фоновый worker (lifecycle
// ctx) — serialize-уя их (натуральный stagger) и с явным per-open pace-floor,
// что исключает reentrancy-класс 10-29 и даёт SB4 (retry живёт в lifecycle
// ctx, а не в per-reload ctx — hot-reload не отменяет retry раннего батча).
type SourceOpener struct {
	client sourceOpenerClient
	logger *slog.Logger
	pace   time.Duration                              // per-open pace-floor (0 = только serialize)
	sleep  func(ctx context.Context, d time.Duration) // injectable (тесты); default = time.After

	mu           sync.Mutex
	sessionReady bool                       // true после первого adapter-swap
	desired      map[domain.ChatID]struct{} // канал/супергруппа-подмножество последнего ApplySet
	opened       map[domain.ChatID]struct{} // успешно открыты в текущей сессии

	reconcileReq chan struct{} // buffered(1): флаг «реконсиляция ждёт»
}

// NewSourceOpener создаёт opener. pace — per-open pace-floor (инжектируется в
// тестах; в проде — из конфига). c — *Repo (прод) или fake (тесты).
func NewSourceOpener(c sourceOpenerClient, logger *slog.Logger, pace time.Duration) *SourceOpener {
	if logger == nil {
		logger = slog.Default()
	}
	return &SourceOpener{
		client:       c,
		logger:       logger.With("module", "infra.telegram.sourceopener"),
		pace:         pace,
		sleep:        defaultOpenerSleep,
		desired:      make(map[domain.ChatID]struct{}),
		opened:       make(map[domain.ChatID]struct{}),
		reconcileReq: make(chan struct{}, 1),
	}
}

// Start запускает единый worker под lifecycle ctx (живёт дольше любого
// ApplySet — SB4). Worker делает начальную реконсиляцию (подхватывает
// состояние, выставленное до Start) и далее спит на сигнале.
func (o *SourceOpener) Start(ctx context.Context) {
	go o.worker(ctx)
}

// ApplySet — единственный вход для open-set. Вызывается на каждом крае с
// ПОЛНЫМ текущим набором идентификаторов источников (ключи rs.Sources).
// Определяет desired как канал/супергруппа-подмножество и будит worker.
// Идемпотентна: повторный вызов тем же набором — no-op (diff по opened).
func (o *SourceOpener) ApplySet(ids []domain.ChatID) {
	o.mu.Lock()
	desired := make(map[domain.ChatID]struct{}, len(ids))
	for _, id := range ids {
		if domain.IsChannelOrSupergroup(id) {
			desired[id] = struct{}{}
		}
	}
	o.desired = desired
	o.mu.Unlock()
	o.signal()
}

// OnSessionSwap зажигается repo в swap-крит-секции (единый триггер полного
// re-open на свежей сессии). Свежий C++-клиент имеет open_count==0 для всех
// чатов по построению → сбрасываем opened (ничего НЕ открыто реально на новом
// клиенте) и сигналим: worker сам сводит opened к актуальному desired (текущий
// канал/супергруппа-набор правил) — полная открытая. (teleproto
// _resumeAuthorization→_resume прецедент: re-arm набора после session reset.)
//
// Важно: swap НЕ перезаписывает desired (только readiness + чистка opened),
// поэтому lost-update при пересечении с hot-reload ApplySet невозможен;
// worker сводит opened к свежайшему desired. Оба порядка (ApplySet до/после
// swap) сходятся: до — worker ждёт readiness, после — swap видит fresh desired.
func (o *SourceOpener) OnSessionSwap() {
	o.mu.Lock()
	o.sessionReady = true
	o.opened = make(map[domain.ChatID]struct{})
	o.mu.Unlock()
	o.signal()
}

func (o *SourceOpener) signal() {
	select {
	case o.reconcileReq <- struct{}{}:
	default: // уже есть ожидающая реконсиляция — состояние под mu актуально
	}
}

// worker — единый драйвер. Начальная реконсиляция подхватывает состояние,
// выставленное до Start. Далее спит на сигнале/ctx.
func (o *SourceOpener) worker(ctx context.Context) {
	o.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.reconcileReq:
			o.reconcile(ctx)
		}
	}
}

// reconcile сводит opened к desired: закрытия (удаления) сначала, затем
// открытия (added/pending) с ограниченным in-edge retry (R4).
func (o *SourceOpener) reconcile(ctx context.Context) {
	// --- Закрытия: opened - desired (удаление из ruleset). Non-fatal. ---
	o.mu.Lock()
	var toClose []domain.ChatID
	for id := range o.opened {
		if _, want := o.desired[id]; !want {
			toClose = append(toClose, id)
		}
	}
	o.mu.Unlock()

	for _, id := range toClose {
		if _, err := o.client.CloseChat(&client.CloseChatRequest{ChatId: id}); err != nil {
			// Non-fatal: lingering open безобиден (чат просто продолжает
			// аквайряться; это не возвращает баг тишины).
			o.logger.Warn("source close failed (lingering open is harmless)",
				slog.Int64("chat_id", id), slog.Any("err", err))
		}
		o.mu.Lock()
		delete(o.opened, id)
		o.mu.Unlock()
	}

	// --- Открытия: bounded in-edge retry (R4) ---
	for pass := 1; ; pass++ {
		o.mu.Lock()
		if !o.sessionReady {
			// Свежая сессия ещё не готова (adapter не переезжает) — ждём
			// OnSessionSwap; тот пересигналит и повторит полную открытую.
			o.mu.Unlock()
			return
		}
		var toOpen []domain.ChatID
		for id := range o.desired {
			if _, ok := o.opened[id]; !ok {
				toOpen = append(toOpen, id)
			}
		}
		o.mu.Unlock()

		if len(toOpen) == 0 {
			return // desired ⊆ opened — сходимости достигнута
		}

		stillUnopened := 0
		for _, id := range toOpen {
			if ctx.Err() != nil {
				return // mid-batch interruption: pending (desired-opened)
				// восстанавливается на следующем крае (startup-swap ре-открывает все).
			}
			o.sleep(ctx, o.pace) // stagger: per-open pace-floor (0 = no-op)
			if !o.openOne(ctx, id) {
				stillUnopened++
			}
		}

		// Все открылись в этом проходе — сходимости достигнута (без лишнего
		// backoff-сна после чистого прохода).
		if stillUnopened == 0 {
			return
		}
		// Ограниченные повторные проходы исчерпаны: pending (desired-opened)
		// перепроверяется на следующем крае. Никогда не падаем load.
		if pass >= openRetryPasses {
			return
		}
		o.sleep(ctx, openRetryBackoff)
	}
}

// openOne выполняет последовательность GetChat (греет/резолвит peer) →
// OpenChat для одного id (порядок R1). Возвращает true при успехе.
func (o *SourceOpener) openOne(ctx context.Context, id domain.ChatID) bool {
	// GetChat: резолвит/греет input-peer (open_dialog early-return на
	// !have_input_peer, mm.cpp:19527); на холодной БД *Repo-обёртка драйвит
	// wait-for-ready (LoadChats). ПЕРЕД OpenChat.
	if _, err := o.client.GetChat(&client.GetChatRequest{ChatId: id}); err != nil {
		o.logger.Warn("source open: GetChat failed, retrying in-edge",
			slog.Int64("chat_id", id), slog.Any("err", err))
		return false
	}
	if _, err := o.client.OpenChat(&client.OpenChatRequest{ChatId: id}); err != nil {
		o.logger.Warn("source open: OpenChat failed, retrying in-edge",
			slog.Int64("chat_id", id), slog.Any("err", err))
		return false
	}
	o.mu.Lock()
	o.opened[id] = struct{}{}
	o.mu.Unlock()
	o.logger.Info("source opened (open-once-keep)", slog.Int64("chat_id", id))
	return true
}

// defaultOpenerSleep — реальный ctx-aware сон (time.After). Тесты подменяют
// o.sleep, чтобы stagger/retry были детерминированными (virtual no-op).
func defaultOpenerSleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
