package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/infra/telegram/mocks"
)

// warmup_test.go — тесты wait-for-ready прогрева чатов (см. warmup.go).
//
// Гонки и тайминги покрыты через testing/synctest: окно wait-for-ready
// (секунды) и rate-limit warmMinInterval (30s) проходят виртуально, без
// реальных sleep.
//
// Метрики (T4) — smoke через исполнение путей: otel-meter без exporter-а —
// no-op, значения счётчиков ненаблюдаемы; все тесты below проходят через
// вызовы warmMisses/warmReadiness/warmTimeouts — отсутствие паники и есть
// smoke-контракт до подключения exporter-а.

// --- isChatNotFound ---

func TestIsChatNotFound(t *testing.T) {
	t.Parallel()

	chatNotFound := func() error {
		return client.ResponseError{Err: &client.Error{Code: 400, Message: "Chat not found"}}
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "exact_match",
			err:  chatNotFound(),
			want: true,
		},
		{
			name: "case_insensitive_match",
			err: client.ResponseError{
				Err: &client.Error{Code: 400, Message: "CHAT NOT FOUND"},
			},
			want: true,
		},
		{
			name: "wrong_code_400_other_message",
			err: client.ResponseError{
				Err: &client.Error{Code: 400, Message: "MESSAGE_ID_INVALID"},
			},
			want: false,
		},
		{
			name: "wrong_code_429",
			err: client.ResponseError{
				Err: &client.Error{Code: 429, Message: "Chat not found"},
			},
			want: false,
		},
		{
			name: "plain_error",
			err:  errors.New("Chat not found"),
			want: false,
		},
		{
			name: "wrapped_response_error",
			err:  fmt.Errorf("get message: %w", chatNotFound()),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := isChatNotFound(tt.err)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- sessionReadiness ---

func TestSessionReadiness_GetOrInsertReturnsSameEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	table := newSessionReadiness()

	// Act
	e1 := table.getOrInsert(10)
	e2 := table.getOrInsert(10)

	// Assert: повторный getOrInsert возвращает ТОТ ЖЕ entry (подписка одна).
	assert.Same(t, e1, e2)
}

func TestSessionReadiness_SignalIdempotent(t *testing.T) {
	t.Parallel()

	// Arrange
	table := newSessionReadiness()

	// Assert: неизвестный chat — signal false, entry не создаётся.
	assert.False(t, table.signal(10))
	_, ok := table.ready[10]
	assert.False(t, ok, "signal must not create entries")

	// Act: первый сигнал закрывает, повторный — no-op.
	table.getOrInsert(10)
	assert.True(t, table.signal(10))
	assert.False(t, table.signal(10), "re-signal must be idempotent")

	// Закрытый канал остаётся level-сигналом.
	select {
	case <-table.getOrInsert(10).ready:
	default:
		t.Fatal("entry must be closed after signal")
	}
}

func TestSessionReadiness_CloseFailedClosesAndDeletes(t *testing.T) {
	t.Parallel()

	// Arrange: waiter ждёт на entry.
	table := newSessionReadiness()
	e := table.getOrInsert(10)
	done := make(chan struct{})
	go func() {
		<-e.ready
		close(done)
	}()

	// Act
	table.closeFailed(10)

	// Assert: waiter разблокирован, entry удалена.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waiter was not released by closeFailed")
	}
	_, ok := table.ready[10]
	assert.False(t, ok, "entry must be deleted after closeFailed")

	// Повторный closeFailed — no-op; signal после удаления — false.
	assert.NotPanics(t, func() { table.closeFailed(10) })
	assert.False(t, table.signal(10))
}

// TestSessionReadiness_SignalFloodNonBlocking — поток из 300 signal-ов
// (в т.ч. повторы и неизвестные chat-ы) не блокируется и не паникует:
// signal — только lock + close, никогда send.
func TestSessionReadiness_SignalFloodNonBlocking(t *testing.T) {
	t.Parallel()

	// Arrange
	table := newSessionReadiness()
	const n = 300
	for i := 0; i < n; i++ {
		table.getOrInsert(int64(i))
	}

	// Act + Assert: цикл завершается — значит, ни один signal не заблокировался.
	signaled := 0
	for i := 0; i < n; i++ {
		if table.signal(int64(i)) {
			signaled++
		}
	}
	assert.Equal(t, n, signaled)

	for i := 0; i < n; i++ {
		table.signal(int64(i))     // повтор — no-op
		table.signal(int64(n + i)) // неизвестный — no-op
	}
	_, ok := table.ready[int64(n)]
	assert.False(t, ok, "signal must not create entries for unknown chats")
}

// --- withReady ---

// newWarmupRepoCfg создаёт Repo с моком clientAdapter и заданным конфигом.
func newWarmupRepoCfg(t *testing.T, cfg config.TelegramConfig) (*Repo, *mocks.ClientAdapter) {
	t.Helper()

	m := mocks.NewClientAdapter(t)
	r := New(cfg, NoBusinessUpdates)
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	// Тихий логгер: тесты прогрева активно пишут Info-логи наблюдаемости.
	r.logger = slog.New(slog.DiscardHandler)
	return r, m
}

// newWarmupRepo создаёт Repo с моком clientAdapter и дефолтным конфигом
// (окно 5s/tick 500ms — под synctest виртуальны).
func newWarmupRepo(t *testing.T) (*Repo, *mocks.ClientAdapter) {
	t.Helper()
	return newWarmupRepoCfg(t, config.TelegramConfig{})
}

// chatNotFoundErr — каноничная TDLib-ошибка «400 Chat not found».
// go-tdlib возвращает ResponseError значением (buildResponseError),
// а код/сообщение лежат во вложенном поле Err.
//
// Дисциплина shared-инстанса: errors.Is сравнивает ResponseError по
// указателю Err — для assert.ErrorIs все вызовы мока должны возвращать
// ОДИН экземпляр, созданный вне замыкания.
func chatNotFoundErr() error {
	return client.ResponseError{Err: &client.Error{Code: 400, Message: "Chat not found"}}
}

// expectLoadChatsOnce настраивает LoadChats: ровно один вызов, завершается ошибкой
// «chat list is empty» (естественное окончание цикла LoadChats).
// Порядок цепочки обязателен: RunAndReturn есть только на типизированном
// *ClientAdapter_LoadChats_Call, а Times/Maybe промоутятся из *mock.Call.
func expectLoadChatsOnce(m *mocks.ClientAdapter) *mock.Call {
	return m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
		return nil, errors.New("Too Much Requests: chat list is empty")
	}).Times(1)
}

// assertEntryClosed проверяет, что сертификат chat в текущей таблице закрыт.
func assertEntryClosed(t *testing.T, r *Repo, chatID int64, msg string) {
	t.Helper()
	e, ok := r.currentTable().ready[chatID]
	require.True(t, ok, "entry must exist in current table")
	select {
	case <-e.ready:
	default:
		t.Fatal(msg)
	}
}

// TestWithReady_HotPathSuccess — успешный fn закрывает сертификат; LoadChats
// не вызывается (горячий путь — ни окна, ни драйва).
func TestWithReady_HotPathSuccess(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: LoadChats не ожидается вовсе — строгий мок уронит тест при вызове.
		r, m := newWarmupRepo(t)
		var calls int

		// Act
		err := r.withReady(context.Background(), 10, func() error {
			calls++
			return nil
		})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		assertEntryClosed(t, r, 10, "successful fn must close the certificate")
		m.AssertNotCalled(t, "LoadChats", mock.Anything)
	})
}

// TestWithReady_FastPathClosedEntry — уже закрытый сертификат (второй waiter):
// fn выполняется ровно один раз (оракул), без окна и без LoadChats-драйва.
func TestWithReady_FastPathClosedEntry(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: сертификат закрыт заранее (эмуляция чужого fn-успеха).
		r, _ := newWarmupRepo(t)
		table := r.currentTable()
		table.getOrInsert(10)
		table.signal(10)
		var calls int

		// Act
		err := r.withReady(context.Background(), 10, func() error {
			calls++
			return nil
		})

		// Assert: O(1) hit — один fn, ни одного LoadChats.
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})
}

// TestWithReady_ColdMissEdgeReleases — холодный miss, затем edge
// (updateNewChat-сигнал той же таблицы) будит waiter: fn повторён, успех.
// Edge-путь не требует LoadChats-драйва вовсе.
func TestWithReady_ColdMissEdgeReleases(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: LoadChats не ожидается — edge приходит раньше первого тика.
		r, m := newWarmupRepo(t)
		notFound := chatNotFoundErr()
		var calls int
		fn := func() error {
			calls++
			if calls == 1 {
				return notFound
			}
			return nil
		}

		// Фоновый «listener»: после ухода waiter-а в select шлёт edge.
		table := r.currentTable()
		var signaled bool
		go func() {
			synctest.Wait()
			signaled = table.signal(10)
		}()

		// Act
		err := r.withReady(context.Background(), 10, fn)

		// Assert
		require.NoError(t, err)
		assert.True(t, signaled, "edge must close the waiter's entry")
		assert.Equal(t, 2, calls, "fn: first probe + retry after edge")
		m.AssertNotCalled(t, "LoadChats", mock.Anything)
	})
}

// TestWithReady_TickDrivesLoadChatsAndRetries — холодный miss без edge:
// тик запускает LoadChats-бутстрап (ровно один, rate-limit), retry успешен.
func TestWithReady_TickDrivesLoadChatsAndRetries(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		expectLoadChatsOnce(m)
		notFound := chatNotFoundErr()
		var calls int

		// Act
		err := r.withReady(context.Background(), 10, func() error {
			calls++
			if calls == 1 {
				return notFound
			}
			return nil
		})

		// Assert: fn 1 (miss) + fn 2 (после тика с бутстрапом).
		require.NoError(t, err)
		assert.Equal(t, 2, calls)
	})
}

// TestWithReady_DeadlineChatNotReadyError — chat так и не материализовался:
// по deadline — ChatNotReadyError с числом драйвов и последней TDLib-ошибкой;
// сертификат закрыт «неудачей» и удалён из таблицы.
//
// Окно 1.2s / тик 500ms — детерминированно: тики на 0.5s и 1.0s, deadline
// на 1.2s (следующий тик 1.5s не наступает) → ровно 2 драйва, 3 fn.
func TestWithReady_DeadlineChatNotReadyError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepoCfg(t, config.TelegramConfig{
			WarmupDeadline: 1200 * time.Millisecond,
			WarmupTicker:   500 * time.Millisecond,
		})
		expectLoadChatsOnce(m)
		notFound := chatNotFoundErr()
		var calls int

		// Act
		err := r.withReady(context.Background(), 10, func() error {
			calls++
			return notFound
		})

		// Assert
		require.Error(t, err)
		var notReady *ChatNotReadyError
		require.ErrorAs(t, err, &notReady)
		assert.Equal(t, int64(10), notReady.ChatID)
		assert.Equal(t, 2, notReady.Drives, "ticks at 0.5s and 1.0s within the 1.2s window")
		// Unwrap отдаёт последнюю TDLib-ошибку (общий экземпляр).
		assert.ErrorIs(t, err, notFound)
		assert.Equal(t, 3, calls, "initial probe + one fn per tick")
		// Сертификат удалён: следующие waiter-ы не унаследуют его.
		_, ok := r.currentTable().ready[10]
		assert.False(t, ok, "failed certificate must be deleted from the table")
	})
}

// TestWithReady_MidWaitNonMatchingError — ошибка, НЕ являющаяся «Chat not
// found», в середине окна: немедленный passthrough без маскировки
// not-ready-семантикой; сертификат удалён (второй вызов не унаследует).
func TestWithReady_MidWaitNonMatchingError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepoCfg(t, config.TelegramConfig{
			WarmupDeadline: 1200 * time.Millisecond,
			WarmupTicker:   500 * time.Millisecond,
		})
		expectLoadChatsOnce(m)
		notFound := chatNotFoundErr()
		broken := errors.New("transport broken")
		var calls int

		// Act
		err := r.withReady(context.Background(), 10, func() error {
			calls++
			if calls == 1 {
				return notFound
			}
			return broken
		})

		// Assert: ошибка — именно broken, на первом же тике (0.5s);
		// LoadChats ровно один, дальнейших тиков нет.
		require.ErrorIs(t, err, broken)
		assert.NotErrorIs(t, err, notFound)
		assert.Equal(t, 2, calls)
		_, ok := r.currentTable().ready[10]
		assert.False(t, ok, "entry must be deleted on non-matching error")
	})
}

// TestWithReady_CtxCancelled — отмена вызывающего: не «не готов» — ctx.Err()
// наружу, сертификат удалён, LoadChats не запускается.
func TestWithReady_CtxCancelled(t *testing.T) {
	t.Parallel()

	// Arrange: LoadChats не ожидается. ctx уже отменён.
	r, _ := newWarmupRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	notFound := chatNotFoundErr()
	var calls int

	// Act
	err := r.withReady(ctx, 10, func() error {
		calls++
		return notFound
	})

	// Assert: первый fn проходит до select, отмена ловится в окне.
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
	_, ok := r.currentTable().ready[10]
	assert.False(t, ok, "cancelled wait must not leave an open certificate")
}

// TestWithReady_SubscriptionBeforeDrive — подписка обязана предшествовать
// первому LoadChats-драйву: иначе edge от его пагинации ушёл бы в никуда.
// Мок LoadChats сам проверяет наличие entry в текущей таблице.
func TestWithReady_SubscriptionBeforeDrive(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepoCfg(t, config.TelegramConfig{
			WarmupDeadline: 1200 * time.Millisecond,
			WarmupTicker:   500 * time.Millisecond,
		})
		m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
			_, ok := r.currentTable().ready[10]
			require.True(t, ok, "waiter entry must be registered before the first LoadChats drive")
			return nil, errors.New("Too Much Requests: chat list is empty")
		}).Times(1)
		notFound := chatNotFoundErr()

		// Act: runs to deadline; подписка проверена моком на каждом драйве.
		err := r.withReady(context.Background(), 10, func() error {
			return notFound
		})

		// Assert
		var notReady *ChatNotReadyError
		require.ErrorAs(t, err, &notReady)
	})
}

// --- Session swap (R-risk-2, седьмой раунд) ---

// TestSessionSwap_StaleCertificateNotHonored — сертификат, выданный под
// «клиентом A» (сессия 1), не спасает сессию 2: после swapSessionTable
// (та же критическая секция, что перезапись clientAdapter) waiter обязан
// снова вызвать fn и получить его вердикт. Регрессия «карта переиспользована»
// дала бы ложный успех без fn — тест падает, т.к. fn всегда не готов.
func TestSessionSwap_StaleCertificateNotHonored(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: сессия 1 — fn успешен, сертификат закрыт.
		r, m := newWarmupRepoCfg(t, config.TelegramConfig{
			WarmupDeadline: 1200 * time.Millisecond,
			WarmupTicker:   500 * time.Millisecond,
		})
		err := r.withReady(context.Background(), 10, func() error { return nil })
		require.NoError(t, err)
		assertEntryClosed(t, r, 10, "session 1 certificate must be closed")

		// Act: смена сессии (swapSessionTable — точка :324 runAuthLoop).
		r.swapSessionTable()

		// Сессия 2: chat не готов; старый сертификат учитывать нельзя.
		// Окно сессии 2 содержит один LoadChats-драйв (первый тик; второй
		// тик внутри warmMinInterval — rate-limit).
		expectLoadChatsOnce(m)
		notFound := chatNotFoundErr()
		var calls int
		err = r.withReady(context.Background(), 10, func() error {
			calls++
			return notFound
		})

		// Assert: НЕ успех — fn-оракул отработал в новом поколении.
		var notReady *ChatNotReadyError
		require.ErrorAs(t, err, &notReady)
		assert.GreaterOrEqual(t, calls, 1, "fn must run under the new session")
		assert.Equal(t, 2, notReady.Drives, "timing deterministic in the fresh table")
	})
}

// TestSessionSwap_NewSessionCertifiesFresh — после смены сессии успешный fn
// выдаёт НОВЫЙ сертификат в новой таблице (горячий путь нового поколения).
func TestSessionSwap_NewSessionCertifiesFresh(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, _ := newWarmupRepo(t)
		require.NoError(t, r.withReady(context.Background(), 10, func() error { return nil }))

		// Act
		r.swapSessionTable()
		err := r.withReady(context.Background(), 10, func() error { return nil })

		// Assert
		require.NoError(t, err)
		assertEntryClosed(t, r, 10, "session 2 must certify in its own table")
	})
}

// TestSessionSwap_MidWaitReanchorsToCurrentTable — waiter, вошедший в окно
// до смены сессии, после пробуждения перечитывает ТЕКУЩУЮ таблицу: edge
// нового поколения его выпускает. Регрессия «waiter остался на старом
// entry» дождался бы deadline, несмотря на закрытый сертификат новой таблицы.
func TestSessionSwap_MidWaitReanchorsToCurrentTable(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: fn готов только с третьего вызова.
		r, m := newWarmupRepo(t)
		expectLoadChatsOnce(m)
		notFound := chatNotFoundErr()
		var mu sync.Mutex
		var calls int
		fn := func() error {
			mu.Lock()
			calls++
			n := calls
			mu.Unlock()
			if n <= 2 {
				return notFound
			}
			return nil
		}

		// Фон: пока waiter стоит в select — меняем сессию и закрываем
		// сертификат chat 10 в НОВОЙ таблице.
		go func() {
			synctest.Wait()
			r.swapSessionTable()
			table := r.currentTable()
			table.getOrInsert(10)
			table.signal(10)
		}()

		// Act
		err := r.withReady(context.Background(), 10, fn)

		// Assert: успех — waiter переанкорился и увидел edge новой таблицы.
		require.NoError(t, err)
		mu.Lock()
		total := calls
		mu.Unlock()
		assert.Equal(t, 3, total, "probe, post-tick retry, retry after new-table edge")
	})
}

// --- warmChatsOnce (через обёртку GetChat) ---

// TestWarmChatsOnce_ConcurrentMissesSingleBootstrap — конкурентные miss-ы (10
// goroutine) дают ровно один LoadChats-бутстрап; все miss-ы получают свой retry
// на прогретой БД. GetChat отвечает miss, пока бутстрап не завершился, —
// так все goroutine гарантированно попадают в warmChatsOnce и либо становятся
// лидером, либо ждут cond до Broadcast.
func TestWarmChatsOnce_ConcurrentMissesSingleBootstrap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		// bootstrapDone закрывается внутри единственного LoadChats: после него
		// GetChat отвечает успехом, т.е. «БД прогрета».
		bootstrapDone := make(chan struct{})
		m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
			close(bootstrapDone)
			return nil, errors.New("Too Much Requests: chat list is empty")
		}).Times(1)

		var mu sync.Mutex
		var calls int
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			select {
			case <-bootstrapDone:
				return &client.Chat{Id: 10}, nil
			default:
				return nil, chatNotFoundErr()
			}
		})

		const workers = 10
		results := make(chan error, workers)
		var start sync.WaitGroup
		start.Add(1)
		var done sync.WaitGroup
		done.Add(workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer done.Done()
				// Стартуем одновременно: максимизируем конкуренцию за warmMu.
				start.Wait()
				if _, err := r.GetChat(&client.GetChatRequest{ChatId: 10}); err != nil {
					results <- err
				}
			}()
		}
		start.Done()
		done.Wait()
		close(results)

		// Assert: все вызовы завершились успешно, LoadChats ровно один
		// (Times(1) + AssertExpectations), и каждый goroutine сделал хотя бы
		// один вызов (10..20: все initial-ы + retry-и тех, кто словил miss).
		for err := range results {
			t.Errorf("unexpected error: %v", err)
		}
		mu.Lock()
		total := calls
		mu.Unlock()
		assert.GreaterOrEqual(t, total, workers)
		assert.LessOrEqual(t, total, 2*workers)
	})
}

// TestWarmChatsOnce_SecondMissWithinIntervalNoBootstrap — второй miss в окне
// warmMinInterval не запускает второй LoadChats, но retry всё равно выполняется.
// Оба miss — на РАЗНЫХ chat-ах: сертификат первого закрыт успехом, и повторный
// miss того же chat-а в новом withReady идёт через refutation, а не через окно.
func TestWarmChatsOnce_SecondMissWithinIntervalNoBootstrap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		expectLoadChatsOnce(m)

		mu := sync.Mutex{}
		misses := 0
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(req *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			// Miss-ы только на первых вызовах каждого chat-а; успех —
			// сразу после второго miss (rate-limit не дал повторить
			// бутстрап, но retry всё равно попал на прогретую БД).
			if misses < 2 {
				misses++
				return nil, chatNotFoundErr()
			}
			return &client.Chat{Id: req.ChatId}, nil
		})

		// Act: первый miss (chat 10) запускает бутстрап.
		_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)

		// Сдвигаем виртуальное время внутрь окна rate-limit-а (например, +1s).
		time.Sleep(time.Second)

		// Act: второй miss (chat 11) в окне — retry есть, второго LoadChats нет.
		_, err = r.GetChat(&client.GetChatRequest{ChatId: 11})
		require.NoError(t, err)
	})
}

// TestWarmChatsOnce_AfterIntervalBootstrapResumes — после истечения
// warmMinInterval miss снова запускает LoadChats. Оба miss — на разных
// chat-ах (см. SecondMissWithinIntervalNoBootstrap); успех GetChat
// отпирается завершением соответствующего бутстрапа, а не счётчиком
// вызовов — иначе retry «успешен» мимо пропущенного LoadChats.
func TestWarmChatsOnce_AfterIntervalBootstrapResumes(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		// Два бутстрапа: первый сразу, второй после виртуального warmMinInterval+.
		var mu sync.Mutex
		bootstraps := 0
		m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
			mu.Lock()
			bootstraps++
			mu.Unlock()
			return nil, errors.New("Too Much Requests: chat list is empty")
		}).Times(2)

		misses := 0
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(req *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			if misses < 2 {
				misses++
				return nil, chatNotFoundErr()
			}
			// Второй miss (chat 11) успешен только если второй бутстрап
			// уже прошёл (прошёл — из-за истёкшего rate-limit-а).
			if req.ChatId == 11 && bootstraps < 2 {
				return nil, chatNotFoundErr()
			}
			return &client.Chat{Id: req.ChatId}, nil
		})

		// Act: первый miss (chat 10) запускает бутстрап #1.
		_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)

		// Виртуально ждём больше warmMinInterval.
		time.Sleep(warmMinInterval + time.Second)

		// Act: второй miss (chat 11) запускает бутстрап #2, retry успешен.
		_, err = r.GetChat(&client.GetChatRequest{ChatId: 11})
		require.NoError(t, err)
	})
}

// TestWarmChatsOnce_LoadChatsAlwaysFails — LoadChats падает на первой итерации:
// бутстрап завершается, retry вызывающего всё равно выполняется (частичный
// прогрев мог закрыть miss).
func TestWarmChatsOnce_LoadChatsAlwaysFails(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
			return nil, errors.New("database corrupted")
		}).Times(1)

		mu := sync.Mutex{}
		chatCalls := 0
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			chatCalls++
			if chatCalls == 1 {
				return nil, chatNotFoundErr()
			}
			return &client.Chat{Id: 10}, nil
		})

		// Act
		chat, err := r.GetChat(&client.GetChatRequest{ChatId: 10})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(10), chat.Id)
		mu.Lock()
		assert.Equal(t, 2, chatCalls)
		mu.Unlock()
	})
}

// --- Обёртки (table-driven) ---

// wrapperCases — общая таблица всех 9 обёрток wait-for-ready: вызов с
// chat 10 и контекст ошибки обёртки. Используется тестами miss-ретрая
// и контекста ошибок.
var wrapperCases = []struct {
	name    string
	act     func(r *Repo) error
	errText string
}{
	{
		name: "ForwardMessages",
		act: func(r *Repo) error {
			_, err := r.ForwardMessages(&client.ForwardMessagesRequest{ChatId: 10, FromChatId: 20})
			return err
		},
		errText: "forward messages:",
	},
	{
		name: "SendMessage",
		act: func(r *Repo) error {
			_, err := r.SendMessage(&client.SendMessageRequest{ChatId: 10})
			return err
		},
		errText: "send message:",
	},
	{
		name: "SendMessageAlbum",
		act: func(r *Repo) error {
			_, err := r.SendMessageAlbum(&client.SendMessageAlbumRequest{ChatId: 10})
			return err
		},
		errText: "send message album:",
	},
	{
		name: "GetMessage",
		act: func(r *Repo) error {
			_, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 1})
			return err
		},
		errText: "get message:",
	},
	{
		name: "GetMessages",
		act: func(r *Repo) error {
			_, err := r.GetMessages(&client.GetMessagesRequest{ChatId: 10, MessageIds: []int64{1}})
			return err
		},
		errText: "get messages:",
	},
	{
		name: "GetChatHistory",
		act: func(r *Repo) error {
			_, err := r.GetChatHistory(&client.GetChatHistoryRequest{ChatId: 10})
			return err
		},
		errText: "get chat history:",
	},
	{
		name: "GetMessageLink",
		act: func(r *Repo) error {
			_, err := r.GetMessageLink(&client.GetMessageLinkRequest{ChatId: 10, MessageId: 1})
			return err
		},
		errText: "get message link:",
	},
	{
		name: "GetMessageLinkInfo",
		act: func(r *Repo) error {
			_, err := r.GetMessageLinkInfo(&client.GetMessageLinkInfoRequest{Url: "https://t.me/c/10/1"})
			return err
		},
		errText: "get message link info:",
	},
	{
		name: "GetChat",
		act: func(r *Repo) error {
			_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
			return err
		},
		errText: "get chat:",
	},
}

// arrangeAllMiss настраивает все 9 методов мока как вечный miss («Chat not
// found»): окно обёртки доживает до deadline. Неиспользуемые методы — .Maybe().
func arrangeAllMiss(m *mocks.ClientAdapter) {
	m.EXPECT().ForwardMessages(mock.Anything).RunAndReturn(func(_ *client.ForwardMessagesRequest) (*client.Messages, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().SendMessageAlbum(mock.Anything).RunAndReturn(func(_ *client.SendMessageAlbumRequest) (*client.Messages, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessages(mock.Anything).RunAndReturn(func(_ *client.GetMessagesRequest) (*client.Messages, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetChatHistory(mock.Anything).RunAndReturn(func(_ *client.GetChatHistoryRequest) (*client.Messages, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessageLink(mock.Anything).RunAndReturn(func(_ *client.GetMessageLinkRequest) (*client.MessageLink, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessageLinkInfo(mock.Anything).RunAndReturn(func(_ *client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
		return nil, chatNotFoundErr()
	}).Maybe()
}

// arrangeRetrySucceeds настраивает все 9 методов мока: первый вызов — miss,
// последующие — успех. Неиспользуемые методы — .Maybe().
func arrangeRetrySucceeds(m *mocks.ClientAdapter) {
	var mu sync.Mutex
	calls := 0
	retrySucceeds := func() bool {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return calls > 1
	}

	m.EXPECT().ForwardMessages(mock.Anything).RunAndReturn(func(_ *client.ForwardMessagesRequest) (*client.Messages, error) {
		if retrySucceeds() {
			return &client.Messages{}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		if retrySucceeds() {
			return &client.Message{Id: 1}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().SendMessageAlbum(mock.Anything).RunAndReturn(func(_ *client.SendMessageAlbumRequest) (*client.Messages, error) {
		if retrySucceeds() {
			return &client.Messages{}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
		if retrySucceeds() {
			return &client.Message{Id: 1}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessages(mock.Anything).RunAndReturn(func(_ *client.GetMessagesRequest) (*client.Messages, error) {
		if retrySucceeds() {
			return &client.Messages{}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetChatHistory(mock.Anything).RunAndReturn(func(_ *client.GetChatHistoryRequest) (*client.Messages, error) {
		if retrySucceeds() {
			return &client.Messages{}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessageLink(mock.Anything).RunAndReturn(func(_ *client.GetMessageLinkRequest) (*client.MessageLink, error) {
		if retrySucceeds() {
			return &client.MessageLink{Link: "https://t.me/c/10/1"}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetMessageLinkInfo(mock.Anything).RunAndReturn(func(_ *client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error) {
		if retrySucceeds() {
			return &client.MessageLinkInfo{}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
	m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
		if retrySucceeds() {
			return &client.Chat{Id: 10}, nil
		}
		return nil, chatNotFoundErr()
	}).Maybe()
}

// TestAllWrappers_MissDrivesLoadChatsAndRetries — table-driven проверка всех
// 9 обёрток: miss → тик с LoadChats-бутстрапом → retry успешен.
func TestAllWrappers_MissDrivesLoadChatsAndRetries(t *testing.T) {
	t.Parallel()

	for _, tt := range wrapperCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange: каждая обёртка получает miss на первом вызове
				// и успех на retry.
				r, m := newWarmupRepo(t)
				expectLoadChatsOnce(m)
				arrangeRetrySucceeds(m)

				// Act
				err := tt.act(r)

				// Assert: retry после miss вернул успех.
				require.NoError(t, err)
			})
		})
	}
}

// TestAllWrappers_AllMissDeadlineChatNotReady — все вызовы miss: обёртка по
// deadline возвращает ChatNotReadyError, обёрнутую её контекстом, с последней
// TDLib-ошибкой в цепочке (errors.Is на общем экземпляре).
func TestAllWrappers_AllMissDeadlineChatNotReady(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		expectLoadChatsOnce(m)
		// Один экземпляр ошибки на все вызовы мока: errors.Is сравнивает
		// ResponseError по указателю Err.
		notFound := chatNotFoundErr()
		m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
			return nil, notFound
		}).Maybe()

		// Act
		msg, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 5})

		// Assert
		require.Error(t, err)
		assert.Nil(t, msg)
		var notReady *ChatNotReadyError
		require.ErrorAs(t, err, &notReady)
		assert.Equal(t, int64(10), notReady.ChatID)
		assert.Contains(t, err.Error(), "get message:")
		assert.ErrorIs(t, err, notFound)
	})
}

// TestAllWrappers_NonMatchingErrorsPassthrough — ошибки, не являющиеся
// «Chat not found» (иной текст при 400 / код 429 / plain error), не запускают
// прогрев: ни LoadChats, ни повторного вызова.
func TestAllWrappers_NonMatchingErrorsPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "code_400_other_message",
			err:  client.ResponseError{Err: &client.Error{Code: 400, Message: "MESSAGE_ID_INVALID"}},
		},
		{
			name: "code_429",
			err:  client.ResponseError{Err: &client.Error{Code: 429, Message: "Too Many Requests: retry after 5"}},
		},
		{
			name: "plain_error",
			err:  errors.New("transport broken"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				r, m := newWarmupRepo(t)
				var calls int
				m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
					calls++
					return nil, tt.err
				})
				// LoadChats не ожидается вовсе: mockery уронит тест при вызове.

				// Act
				msg, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 5})

				// Assert
				require.Error(t, err)
				assert.Nil(t, msg)
				assert.ErrorIs(t, err, tt.err)
				assert.Equal(t, 1, calls)
			})
		})
	}
}

// TestAllWrappers_WrappedErrorContext — при непрогретом chat ошибка каждой
// обёртки сохраняет исходный контекст («...: chat N not ready ...»).
func TestAllWrappers_WrappedErrorContext(t *testing.T) {
	t.Parallel()

	for _, tt := range wrapperCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange: все вызовы — miss (окно до deadline).
				r, m := newWarmupRepo(t)
				expectLoadChatsOnce(m)
				arrangeAllMiss(m)

				// Act
				err := tt.act(r)

				// Assert: контекст обёртки + ChatNotReadyError + текст TDLib.
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errText)
				assert.Contains(t, err.Error(), "not ready after warmup window")
				assert.Contains(t, err.Error(), "Chat not found")
			})
		})
	}
}
