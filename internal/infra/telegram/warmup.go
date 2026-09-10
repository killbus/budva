package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"
	"go.opentelemetry.io/otel/metric"
)

// warmup.go — wait-for-ready при «400 Chat not found» на холодной БД TDLib.
//
// После authorizationStateReady TDLib не загружает полный список чатов —
// pull-вызовы по chat_id на холодной БД падают с «400 Chat not found».
// Вместо startup-прогрева (структурно слеп к hot-reload ruleset и к гонке
// «серверы уже подняты, TDLib ещё не готов») обёртки client_adapter.go при
// этой ошибке входят в withReady: в ограниченном окне они ждут, пока TDLib
// действительно материализует dialog, и только затем повторяют вызов.
//
// Источники материализации (проверено по исходникам TDLib pin 22d49d5):
// LoadChats-пагинация сервера и входящее сообщение — оба проходят через
// add_new_dialog, который ровно в момент «dialog вошёл в карту + negative
// cache стёрт» отправляет updateNewChat. Поэтому updateNewChat — точный
// сигнал готовности, а повтор самого вызова — единственный оракул.
//
// Один механизм покрывает четыре формы сбоя: холодный старт facade,
// холодный старт engine, новый destination через hot-reload ruleset и
// дедлок чисто выходного канала (его единственный «автор» — сам engine,
// поэтому никакая активность не приведёт chat в БД).
//
// Обсуждение и отклонённые альтернативы: см. convergence-запись задачи
// warmup-lazy-loadchats (startup warmup, app-layer loader) и
// design.md задачи warmup-wait-ready (одноразовый retry, отдельный
// GetChat-оракул, per-entry generation).

const (
	// warmLoadChatsLimit — размер одного LoadChats-чанка. Цикл крутится
	// до ошибки TDLib (официальная tdjson-семантика: «chat list is empty»).
	warmLoadChatsLimit = 100

	// warmLoadChatsMaxLoops — верхняя граница цикла LoadChats: защита от
	// экзотических ошибок, которые не останавливают итерации.
	warmLoadChatsMaxLoops = 10

	// warmMinInterval — минимальный интервал между LoadChats-бутстрапами.
	// Гарантирует, что несуществующий chat (не член аккаунта) не устроит
	// шторм прогревов: каждый miss в окне всё равно получает один retry,
	// но без повторного LoadChats.
	warmMinInterval = 30 * time.Second

	// Значения по умолчанию для конфигурируемых параметров ожидания;
	// конфиг (config.TelegramConfig) перекрывает их, здесь — только фолбэк.
	defaultWarmupDeadline = 5 * time.Second
	defaultWarmupTicker   = 500 * time.Millisecond

	// warmRetryInfoDelay — подсказка клиенту в gRPC RetryInfo: к этому
	// моменту типичный холодный старт уже завершился (наблюдение ~2.5s
	// бутстрап + ~2s пагинации), повторный запрос почти наверняка попадёт
	// на прогретую БД или готовый сертификат.
	warmRetryInfoDelay = 2 * time.Second
)

// ChatNotReadyError — typed-ошибка wait-окна: chat остался непрогретым до
// истечения deadline (не существует, вне списка диалогов этого аккаунта или
// сервер пагинирует дольше окна). Маппится транспортом в
// codes.Unavailable + RetryInfo; прочие ошибки остаются Internal.
//
// Unwrap отдаёт последнюю TDLib-ошибку (всегда «Chat not found»), чтобы
// errors.Is на общем экземпляре продолжал работать в тестах и вызовах.
type ChatNotReadyError struct {
	ChatID  int64
	Drives  int   // сколько раз в окне был запущен LoadChats-бутстрап
	LastErr error // последняя TDLib-ошибка
}

func (e *ChatNotReadyError) Error() string {
	return fmt.Sprintf(
		"chat %d not ready after warmup window (%d LoadChats drives): %v",
		e.ChatID, e.Drives, e.LastErr,
	)
}

func (e *ChatNotReadyError) Unwrap() error { return e.LastErr }

// readyEntry — сертификат готовности одного chat: закрытый канал означает
// «dialog материализован в этой TDLib-сессии». Канал закрывается ровно
// один раз и остаётся закрытым навсегда: dialog, вошедший в карту TDLib,
// в пределах сессии из неё не уходит, поэтому закрытый канал — вечный
// level-сигнал (пропущенный edge ничего не стоит).
type readyEntry struct {
	ready chan struct{}
}

// sessionReadiness — мутабельная карта сертификатов, обёрнутая в
// НЕИЗМЕНЯЕМЫЙ контейнер, живущий за атомарным указателем.
//
// Контракт сессии (обязателен к соблюдению, не деталь реализации):
//   - Сертификаты валидны только в пределах одной TDLib-сессии. Карта
//     целиком заменяется в той же критической секции runAuthLoop, где
//     перезаписывается clientAdapter (это и есть фактическая граница
//     сессии: с этого момента fn-вызовы бьют в нового клиента).
//   - НЕОБХОДИМОЕ УСЛОВИЕ корректности: entry.ready закрывается ТОЛЬКО
//     успешным результатом fn самого держателя, а updateNewChat-dispatch
//     работает ТОЛЬКО с текущей таблицей. Если разрешить краю закрывать
//     entries в чужой (старой) таблице, гарантия «нет ложной готовности»
//     перестаёт выполняться молча.
//   - Следствие: waiter, держащий старую таблицу после смены поколения,
//     не получает ложного положительного ответа — самое худшее для него это
//     лишний неудачный fn и повторный lookup в новой таблице на следующем
//     тике.
type sessionReadiness struct {
	mu    sync.Mutex
	ready map[int64]*readyEntry
}

func newSessionReadiness() *sessionReadiness {
	return &sessionReadiness{ready: make(map[int64]*readyEntry)}
}

// getOrInsert возвращает entry chat-а, создавая при отсутствии. Вставка
// предшествует любому запуску LoadChats-драйва — иначе edge от его
// пагинации может уйти в никуда, до того как подписчик появился.
func (s *sessionReadiness) getOrInsert(chatID int64) *readyEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.ready[chatID]; ok {
		return e
	}
	e := &readyEntry{ready: make(chan struct{})}
	s.ready[chatID] = e
	return e
}

// signal закрывает сертификат chat-а, если он есть в ЭТОЙ таблице
// (идемпотентно, никогда не блокируется). Вызывается из listenUpdates при
// updateNewChat — горячий путь приёмника, поэтому только lock + close.
func (s *sessionReadiness) signal(chatID int64) (signaled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.ready[chatID]
	if !ok {
		return false
	}
	select {
	case <-e.ready:
		return false // уже закрыт ранее
	default:
		close(e.ready)
		return true
	}
}

// closeFailed закрывает сертификат «неудачей» и удаляет его: истёкшее окно
// не должно оставлять waiter-у навсегда закрытый канал. close и delete — в
// одной критической секции, чтобы параллельный signal не увидел полуфабрикат.
func (s *sessionReadiness) closeFailed(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.ready[chatID]
	if !ok {
		return
	}
	select {
	case <-e.ready:
	default:
		close(e.ready)
	}
	delete(s.ready, chatID)
}

// currentTable возвращает текущую таблицу сессии.
func (r *Repo) currentTable() *sessionReadiness {
	return r.readiness.Load()
}

// swapSessionTable подменяет таблицу сертификатов новой (при смене
// TDLib-сессии) и фиксирует наблюдаемость: поколение + число списанных
// сертификатов. Вызывается из runAuthLoop в той же критической секции, где
// перезаписывается clientAdapter (фактическая граница сессии — R-risk-2).
//
// atomic.Pointer.Swap возвращает только СТАРУЮ таблицу (один указатель, не
// generic-пара), поэтому число списанных сертификатов считается по её
// содержимому: длина карты до замены.
func (r *Repo) swapSessionTable() {
	old := r.readiness.Swap(newSessionReadiness())
	invalidated := 0
	if old != nil {
		old.mu.Lock()
		invalidated = len(old.ready)
		old.mu.Unlock()
	}
	if invalidated > 0 {
		r.logger.Info("TDLib session changed, warmup certificates invalidated",
			slog.Int("invalidated_entries", invalidated),
		)
	}
	warmSessionSwaps.Add(context.Background(), 1)
	warmInvalidated.Add(context.Background(), int64(invalidated))
}

// isChatNotFound сообщает, является ли ошибка TDLib-ответом
// «400 Chat not found».
//
// go-tdlib возвращает ResponseError ЗНАЧЕНИЕМ (buildResponseError →
// ResponseError{Err: ...}), поэтому errors.As таргетит именно value-тип,
// а код и сообщение лежат во вложенном поле Err. Тексты ошибок TDLib могут
// меняться между версиями, поэтому матчинг — код + регистронезависимое
// сообщение. При дрейфе текста матчинг вернёт false: прогрев молча не
// сработает, поведение выродится в passthrough — без паники и ложных
// срабатываний.
func isChatNotFound(err error) bool {
	var respErr client.ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr.Err != nil &&
		respErr.Err.Code == 400 &&
		strings.EqualFold(respErr.Err.Message, "Chat not found")
}

// withReady — точка входа обёрток client_adapter.go: выполняет fn, а при
// «400 Chat not found» ждёт в ограниченном окне материализации chat и
// повторяет fn.
//
// Состояние машины (конвергенция 6-го раунда экспертной сессии):
//
//  1. Первый вызов fn — единственный оракул и первый зонд: и горячий путь
//     (chat уже готов), и холодный miss проходят через него. Не-«Chat not
//     found» ошибка — passthrough, окно не открывается.
//  2. Ожидание: select {ready-канал, тик (драйв warmChatsOnce + retry),
//     deadline}. Каждое пробуждение заново читает ТЕКУЩУЮ таблицу:
//     если сессия сменилась, старый сертификат не засчитывается (см.
//     sessionReadiness). fn остаётся единственным оракулом.
//     Edge-пробуждение с последующим miss опровергает сертификат — он
//     удаляется, иначе select каждой итерации немедленно просыпается на
//     его закрытом канале (hot-loop fn-вызовов до deadline: диалог мог
//     покинуть карту после закрытия — chat удалён/покинут).
//  3. Deadline → ChatNotReadyError (число драйвов + последняя ошибка);
//     сертификат закрывается «неудачей» и удаляется.
//
// Подписка (getOrInsert) обязана предшествовать первому драйву: иначе
// updateNewChat от пагинации первого бутстрапа уйдёт в никуда.
func (r *Repo) withReady(ctx context.Context, chatID int64, fn func() error) error {
	table := r.currentTable()
	entry := table.getOrInsert(chatID)

	err := fn()
	if err == nil {
		table.signal(chatID)
		return nil
	}
	if !isChatNotFound(err) {
		return err
	}

	// Логируем открытие окна (не каждый retry): сам факт miss — наблюдаемое
	// событие; запуск LoadChats логируется отдельно в warmChatsOnce.
	r.logger.Info("TDLib chat warmup: chat not found, waiting for readiness",
		slog.Int64("chat_id", chatID),
		slog.Duration("deadline", r.warmupDeadline()),
	)
	warmMisses.Add(context.Background(), 1)

	deadline := time.NewTimer(r.warmupDeadline())
	defer deadline.Stop()
	ticker := time.NewTicker(r.warmupTicker())
	defer ticker.Stop()

	drives := 0
	lastErr := err
	for {
		select {
		case <-ctx.Done():
			// Отмена вызывающего — не «не готов»: удаляем entry, чтобы
			// следующие waiter-ы не унаследовали открытый сертификат.
			// Текущая таблица, не захваченная: после смены сессии именно
			// в ней живёт entry, на которой ждёт этот waiter.
			r.currentTable().closeFailed(chatID)
			return ctx.Err()
		case <-deadline.C:
			r.currentTable().closeFailed(chatID)
			warmTimeouts.Add(context.Background(), 1)
			warmReadiness.Add(context.Background(), 1, metric.WithAttributes(outcomeAttr(outcomeTimeout)))
			return &ChatNotReadyError{ChatID: chatID, Drives: drives, LastErr: lastErr}
		case <-ticker.C:
			r.warmChatsOnce()
			drives++
		case <-entry.ready:
			// Edge от updateNewChat/чужого fn-успеха. Ничего не делаем:
			// fn ниже сам подтвердит готовность (электорат — только fn).
		}

		// Пробуждение — перечитываем текущую таблицу: смена сессии
		// инвалидирует старые сертификаты (R-risk-2).
		cur := r.currentTable()
		curEntry := cur.getOrInsert(chatID)
		entry = curEntry

		if ferr := fn(); ferr == nil {
			cur.signal(chatID)
			warmReadiness.Add(context.Background(), 1, metric.WithAttributes(outcomeAttr(outcomeSuccess)))
			return nil
		} else if !isChatNotFound(ferr) {
			cur.closeFailed(chatID)
			return ferr
		} else {
			lastErr = ferr
			// Сертификат, опровергнутый оракулом (закрыт, но fn всё ещё
			// «Chat not found»), удаляется: select следующей итерации иначе
			// немедленно просыпается на нём же — hot-loop fn-вызовов до
			// deadline. closeFailed также отпускает других waiter-ов; их
			// retry и дальнейшие тики продолжат по свежей entry.
			select {
			case <-curEntry.ready:
				cur.closeFailed(chatID)
			default:
			}
		}
	}
}

// warmupDeadline / warmupTicker — параметры окна из конфига с фолбэком на
// константы (конфиг может быть пуст в тестах).
func (r *Repo) warmupDeadline() time.Duration {
	if r.cfg.WarmupDeadline > 0 {
		return r.cfg.WarmupDeadline
	}
	return defaultWarmupDeadline
}

func (r *Repo) warmupTicker() time.Duration {
	if r.cfg.WarmupTicker > 0 {
		return r.cfg.WarmupTicker
	}
	return defaultWarmupTicker
}

// warmChatsOnce выполняет LoadChats-бутстрап с дедупликацией:
//
//   - одновременно в полёте максимум один бутстрап; конкурентные miss-ы
//     ждут его завершения (их retry увидит прогретую БД);
//   - повторный запуск раньше warmMinInterval не выполняется — поздний
//     miss получает только свой retry.
//
// Ошибки LoadChats логируются Warn и не прерывают retry вызывающего:
// частичный прогрев мог закрыть и этот miss.
func (r *Repo) warmChatsOnce() {
	r.warmMu.Lock()

	// Уже идёт бутстрап — ждём завершения.
	if r.warmInFlight {
		for r.warmInFlight {
			r.warmCond.Wait()
		}
		r.warmMu.Unlock()
		return
	}

	// Rate-limit: недавний бутстрап уже загрузил всё, что мог LoadChats.
	if time.Since(r.lastWarm) < warmMinInterval {
		r.warmMu.Unlock()
		return
	}

	r.warmInFlight = true
	r.warmMu.Unlock()

	defer func() {
		r.warmMu.Lock()
		r.warmInFlight = false
		r.lastWarm = time.Now()
		r.warmCond.Broadcast()
		r.warmMu.Unlock()
	}()

	r.logger.Info("TDLib chat warmup: LoadChats bootstrap started")
	for i := 0; i < warmLoadChatsMaxLoops; i++ {
		_, err := r.clientAdapter.LoadChats(&client.LoadChatsRequest{Limit: warmLoadChatsLimit})
		if err != nil {
			// Ошибка завершает цикл. Штатное окончание («chat list is empty» /
			// 404) — официальная семантика «список исчерпан», это Info; отличить
			// её от реального сбоя по тексту нельзя (риск дрейфа), поэтому
			// счётчик warmLoadChatsErrors ведёт все ошибки, а уровень — Info.
			r.logger.Info("TDLib chat warmup: LoadChats loop finished",
				slog.Any("err", err),
				slog.Int("iterations", i+1),
			)
			warmLoadChatsErrors.Add(context.Background(), 1)
			return
		}
	}
	r.logger.Warn("TDLib chat warmup: LoadChats loop hit iteration bound",
		slog.Int("max_loops", warmLoadChatsMaxLoops),
	)
}

// initWarmState вызывается из New для инициализации cond, полей прогрева
// и пустой таблицы сертификатов текущей сессии.
func (r *Repo) initWarmState() {
	r.warmCond = sync.NewCond(&r.warmMu)
	initMetrics()
	r.readiness.Store(newSessionReadiness())
	r.convergence = newConvergenceTracker()
}
