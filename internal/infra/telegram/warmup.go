package telegram

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

// warmup.go — ленивый прогрев кеша чатов TDLib (lazy LoadChats-on-miss).
//
// После authorizationStateReady TDLib не загружает полный список чатов —
// pull-вызовы по chat_id на холодной БД падают с «400 Chat not found».
// Вместо startup-прогрева (структурно слеп к hot-reload ruleset и к гонке
// «серверы уже подняты, TDLib ещё не готов») обёртки client_adapter.go
// при этой ошибке запускают LoadChats-бутстрап и повторяют вызов один раз.
//
// Один механизм покрывает четыре формы сбоя: холодный старт facade,
// холодный старт engine, новый destination через hot-reload ruleset и
// дедлок чисто выходного канала (его единственный «автор» — сам engine,
// поэтому никакая активность не приведёт chat в БД).
//
// Обсуждение и отклонённые альтернативы: см. convergence-запись задачи
// warmup-lazy-loadchats (startup warmup, app-layer loader).

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
)

// isChatNotFound сообщает, является ли ошибка TDLib-ответом
// «400 Chat not found».
//
// go-tdlib возвращает ResponseError ЗНАЧЕНИЕМ (buildResponseError →
// ResponseError{Err: ...}), поэтому errors.As таргетит именно value-тип,
// а код и сообщение лежат во вложенном поле Err. Тексты ошибок TDLib могут
// меняться между версиями, поэтому матчинг — код + регистронезависимое
// сообщение. При дрейфе текста матчинг вернёт false: прогрев молча не
// сработает, поведение выродится в текущий passthrough — без паники и
// ложных срабатываний.
func isChatNotFound(err error) bool {
	var respErr client.ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr.Err != nil &&
		respErr.Err.Code == 400 &&
		strings.EqualFold(respErr.Err.Message, "Chat not found")
}

// warmOnChatNotFound — точка входа для обёрток client_adapter.go.
//
// Возвращает true, если err — «Chat not found» и вызывающий должен
// повторить вызов один раз (после бутстрапа или, при сработавшем
// rate-limit, сразу). Возвращает false для любых других ошибок.
//
// method/chatID используются только для лога наблюдаемости.
func (r *Repo) warmOnChatNotFound(err error, method string, chatID int64) bool {
	if !isChatNotFound(err) {
		return false
	}
	// Логируем до бутстрапа: сам факт miss — наблюдаемое событие. Реальный
	// запуск LoadChats логируется отдельно («bootstrap started») — этот
	// лог не должен утверждать бутстрап, если его срезал rate-limit.
	r.logger.Info("TDLib chat warmup: chat not found, warming chats",
		slog.String("method", method),
		slog.Int64("chat_id", chatID),
	)
	r.warmChatsOnce()
	return true
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
			// Цикл завершается по ошибке — обычно это ожидаемое окончание
			// («chat list is empty»), но отличить его от реального сбоя LoadChats
			// по тексту нельзя (риск дрейфа), поэтому логируем Warn.
			r.logger.Warn("TDLib chat warmup: LoadChats loop finished",
				slog.Any("err", err),
				slog.Int("iterations", i+1),
			)
			return
		}
	}
	r.logger.Warn("TDLib chat warmup: LoadChats loop hit iteration bound",
		slog.Int("max_loops", warmLoadChatsMaxLoops),
	)
}

// initWarmState вызывается из New для инициализации cond и полей прогрева.
func (r *Repo) initWarmState() {
	r.warmCond = sync.NewCond(&r.warmMu)
}
