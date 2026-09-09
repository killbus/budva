package telegram

import (
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

// warmup_test.go — тесты ленивого прогрева чатов (см. warmup.go).
//
// Гонки и тайминги покрыты через testing/synctest: rate-limit-интервал
// warmMinInterval проходит виртуально, без реальных sleep.

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

// --- warmup helpers ---

// newWarmupRepo создаёт Repo с подключённым моком clientAdapter.
func newWarmupRepo(t *testing.T) (*Repo, *mocks.ClientAdapter) {
	t.Helper()

	m := mocks.NewClientAdapter(t)
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	// Тихий логгер: тесты прогрева активно пишут Info-логи наблюдаемости.
	r.logger = slog.New(slog.DiscardHandler)
	return r, m
}

// chatNotFoundErr — каноничная TDLib-ошибка «400 Chat not found».
// go-tdlib возвращает ResponseError значением (buildResponseError),
// а код/сообщение лежат во вложенном поле Err.
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

// TestWarmOnChatNotFound_MissTriggersLoadChatsAndRetry — базовый сценарий:
// miss → LoadChats ×1 → retry успешен.
func TestWarmOnChatNotFound_MissTriggersLoadChatsAndRetry(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		var calls int
		m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
			calls++
			if calls == 1 {
				return nil, chatNotFoundErr()
			}
			return &client.Message{Id: 1}, nil
		})
		expectLoadChatsOnce(m)

		// Act
		msg, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 5})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(1), msg.Id)
		assert.Equal(t, 2, calls)
	})
}

// TestWarmOnChatNotFound_MissRetryFails — miss → LoadChats ×1 → retry снова
// падает → ошибка заворачивается с контекстом обёртки.
func TestWarmOnChatNotFound_MissRetryFails(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		// Один экземпляр ошибки на оба вызова мока: errors.Is сравнивает
		// ResponseError по указателю Err, свежая аллокация не совпала бы.
		notFound := chatNotFoundErr()
		m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(_ *client.GetMessageRequest) (*client.Message, error) {
			return nil, notFound
		}).Times(2)
		expectLoadChatsOnce(m)

		// Act
		msg, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 5})

		// Assert
		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "get message:")
		assert.ErrorIs(t, err, notFound)
	})
}

// TestWarmOnChatNotFound_NonMatchingErrorsPassthrough — ошибки, не являющиеся
// «Chat not found» (иной текст при 400 / код 429 / plain error), не запускают
// прогрев: ни LoadChats, ни повторного вызова.
func TestWarmOnChatNotFound_NonMatchingErrorsPassthrough(t *testing.T) {
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
func TestWarmChatsOnce_SecondMissWithinIntervalNoBootstrap(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		expectLoadChatsOnce(m)

		mu := sync.Mutex{}
		chatCalls := 0
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			chatCalls++
			// Вызовы 1-2: первый miss + retry. Вызовы 3-4: второй miss + retry
			// (retry успешен, хотя rate-limit не дал повторить бутстрап).
			if chatCalls == 1 || chatCalls == 3 {
				return nil, chatNotFoundErr()
			}
			return &client.Chat{Id: 10}, nil
		})

		// Act: первый miss запускает бутстрап.
		_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)
		mu.Lock()
		assert.Equal(t, 2, chatCalls)
		mu.Unlock()

		// Сдвигаем виртуальное время внутрь окна rate-limit-а (например, +1s).
		time.Sleep(time.Second)

		// Act: второй miss в окне — retry есть, второго LoadChats нет.
		_, err = r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)
		mu.Lock()
		assert.Equal(t, 4, chatCalls)
		mu.Unlock()
	})
}

// TestWarmChatsOnce_AfterIntervalBootstrapResumes — после истечения
// warmMinInterval miss снова запускает LoadChats.
func TestWarmChatsOnce_AfterIntervalBootstrapResumes(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r, m := newWarmupRepo(t)
		// Два бутстрапа: первый сразу, второй после виртуального warmMinInterval+.
		m.EXPECT().LoadChats(mock.Anything).RunAndReturn(func(_ *client.LoadChatsRequest) (*client.Ok, error) {
			return nil, errors.New("Too Much Requests: chat list is empty")
		}).Times(2)

		mu := sync.Mutex{}
		chatCalls := 0
		m.EXPECT().GetChat(mock.Anything).RunAndReturn(func(_ *client.GetChatRequest) (*client.Chat, error) {
			mu.Lock()
			defer mu.Unlock()
			chatCalls++
			if chatCalls == 1 || chatCalls == 3 {
				return nil, chatNotFoundErr()
			}
			return &client.Chat{Id: 10}, nil
		})

		// Act
		_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)

		// Виртуально ждём больше warmMinInterval.
		time.Sleep(warmMinInterval + time.Second)

		_, err = r.GetChat(&client.GetChatRequest{ChatId: 10})
		require.NoError(t, err)
		mu.Lock()
		assert.Equal(t, 4, chatCalls)
		mu.Unlock()
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

// TestAllWrappers_RetryOnChatNotFound — table-driven проверка всех 9 обёрток:
// miss → retry после LoadChats-бутстрапа.
func TestAllWrappers_RetryOnChatNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		act  func(r *Repo) error
	}{
		{
			name: "ForwardMessages",
			act: func(r *Repo) error {
				_, err := r.ForwardMessages(&client.ForwardMessagesRequest{ChatId: 10, FromChatId: 20})
				return err
			},
		},
		{
			name: "SendMessage",
			act: func(r *Repo) error {
				_, err := r.SendMessage(&client.SendMessageRequest{ChatId: 10})
				return err
			},
		},
		{
			name: "SendMessageAlbum",
			act: func(r *Repo) error {
				_, err := r.SendMessageAlbum(&client.SendMessageAlbumRequest{ChatId: 10})
				return err
			},
		},
		{
			name: "GetMessage",
			act: func(r *Repo) error {
				_, err := r.GetMessage(&client.GetMessageRequest{ChatId: 10, MessageId: 1})
				return err
			},
		},
		{
			name: "GetMessages",
			act: func(r *Repo) error {
				_, err := r.GetMessages(&client.GetMessagesRequest{ChatId: 10, MessageIds: []int64{1}})
				return err
			},
		},
		{
			name: "GetChatHistory",
			act: func(r *Repo) error {
				_, err := r.GetChatHistory(&client.GetChatHistoryRequest{ChatId: 10})
				return err
			},
		},
		{
			name: "GetMessageLink",
			act: func(r *Repo) error {
				_, err := r.GetMessageLink(&client.GetMessageLinkRequest{ChatId: 10, MessageId: 1})
				return err
			},
		},
		{
			name: "GetMessageLinkInfo",
			act: func(r *Repo) error {
				_, err := r.GetMessageLinkInfo(&client.GetMessageLinkInfoRequest{Url: "https://t.me/c/10/1"})
				return err
			},
		},
		{
			name: "GetChat",
			act: func(r *Repo) error {
				_, err := r.GetChat(&client.GetChatRequest{ChatId: 10})
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				r, m := newWarmupRepo(t)
				expectLoadChatsOnce(m)

				// Каждая обёртка получает miss на первом вызове и успех на retry.
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

				// Act
				err := tt.act(r)

				// Assert: retry после miss вернул успех.
				require.NoError(t, err)
			})
		})
	}
}

// TestAllWrappers_WrappedErrorContext — при повторном miss ошибка каждой
// обёртки сохраняет исходный контекст ( «...: Chat not found»).
func TestAllWrappers_WrappedErrorContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				r, m := newWarmupRepo(t)
				expectLoadChatsOnce(m)

				// Все вызовы — miss (retry тоже падает).
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

				// Act
				err := tt.act(r)

				// Assert
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errText)
				assert.Contains(t, err.Error(), "Chat not found")
			})
		})
	}
}
