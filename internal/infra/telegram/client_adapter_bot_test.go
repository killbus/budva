package telegram

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/infra/telegram/mocks"
)

func floodRepo(t *testing.T, call func(adapter *mocks.ClientAdapter)) *Repo {
	t.Helper()
	m := mocks.NewClientAdapter(t)
	call(m)
	r := New(config.TelegramConfig{}, NoBusinessUpdates)
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	return r
}

func TestSearchPublicChat_UsernameNotFound(t *testing.T) {
	t.Parallel()

	m := mocks.NewClientAdapter(t)
	m.EXPECT().SearchPublicChat(mock.Anything).RunAndReturn(func(*client.SearchPublicChatRequest) (*client.Chat, error) {
		return nil, client.ResponseError{Err: &client.Error{Code: 400, Message: "Username not found"}}
	})
	r := New(config.TelegramConfig{}, NoBusinessUpdates)
	r.clientAdapter = m

	_, err := r.SearchPublicChat(&client.SearchPublicChatRequest{Username: "missing_bot"})

	var botNotFound *BotNotFoundError
	require.ErrorAs(t, err, &botNotFound)
	assert.Equal(t, "missing_bot", botNotFound.Username)
}

func TestFloodWaitThroughBotMenuWrappers(t *testing.T) {
	t.Parallel()

	floodErr := client.ResponseError{Err: &client.Error{Code: 429, Message: "Too Many Requests: retry after 7"}}
	want := 7*time.Second + 500*time.Millisecond

	t.Run("get message", func(t *testing.T) {
		t.Parallel()
		r := floodRepo(t, func(m *mocks.ClientAdapter) {
			m.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(*client.GetMessageRequest) (*client.Message, error) {
				return nil, floodErr
			})
		})
		_, err := r.GetMessage(&client.GetMessageRequest{ChatId: 1, MessageId: 2})
		var flood *FloodWaitError
		require.ErrorAs(t, err, &flood)
		assert.Equal(t, want, flood.Delay)
	})

	t.Run("get chat history", func(t *testing.T) {
		t.Parallel()
		r := floodRepo(t, func(m *mocks.ClientAdapter) {
			m.EXPECT().GetChatHistory(mock.Anything).RunAndReturn(func(*client.GetChatHistoryRequest) (*client.Messages, error) {
				return nil, floodErr
			})
		})
		_, err := r.GetChatHistory(&client.GetChatHistoryRequest{ChatId: 1})
		var flood *FloodWaitError
		require.ErrorAs(t, err, &flood)
		assert.Equal(t, want, flood.Delay)
	})

	t.Run("send message", func(t *testing.T) {
		t.Parallel()
		r := floodRepo(t, func(m *mocks.ClientAdapter) {
			m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(*client.SendMessageRequest) (*client.Message, error) {
				return nil, floodErr
			})
		})
		_, err := r.SendMessage(&client.SendMessageRequest{ChatId: 1})
		var flood *FloodWaitError
		require.ErrorAs(t, err, &flood)
		assert.Equal(t, want, flood.Delay)
	})

	t.Run("send bot start message", func(t *testing.T) {
		t.Parallel()
		r := floodRepo(t, func(m *mocks.ClientAdapter) {
			m.EXPECT().GetUser(&client.GetUserRequest{UserId: 2}).Return(&client.User{Id: 2}, nil).Once()
			m.EXPECT().SendBotStartMessage(mock.Anything).RunAndReturn(func(*client.SendBotStartMessageRequest) (*client.Message, error) {
				return nil, floodErr
			})
		})
		_, err := r.SendBotStartMessage(&client.SendBotStartMessageRequest{ChatId: 1, BotUserId: 2})
		var flood *FloodWaitError
		require.ErrorAs(t, err, &flood)
		assert.Equal(t, want, flood.Delay)
	})
}

// RED: omitting/reordering getUser makes the single start call fail. No
// private chat or chat-list bootstrap is required for a bot used in a group.
func TestSendBotStartMessage_LoadsUserBeforeSending(t *testing.T) {
	t.Parallel()
	const botID int64 = 7131987336
	for _, chatID := range []int64{botID, -1001078465602} {
		t.Run(fmt.Sprint(chatID), func(t *testing.T) {
			t.Parallel()
			r, m := newWarmupRepo(t)
			req := &client.SendBotStartMessageRequest{ChatId: chatID, BotUserId: botID, Parameter: "res_212"}
			want := &client.Message{ChatId: chatID, Id: 42}
			loaded := false
			m.EXPECT().GetUser(&client.GetUserRequest{UserId: botID}).RunAndReturn(func(*client.GetUserRequest) (*client.User, error) {
				loaded = true
				return &client.User{Id: botID, Type: &client.UserTypeBot{}}, nil
			}).Once()
			m.EXPECT().SendBotStartMessage(req).RunAndReturn(func(*client.SendBotStartMessageRequest) (*client.Message, error) {
				if !loaded {
					return nil, errors.New("bot user was not loaded before sending")
				}
				return want, nil
			}).Once()
			got, err := r.SendBotStartMessage(req)
			require.NoError(t, err)
			require.Same(t, want, got)
		})
	}
}

// Strict mocks forbid additional sends or chat loading based on bot errors.
func TestSendBotStartMessage_DoesNotRetryBotErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "not a bot", err: client.ResponseError{Err: &client.Error{Code: 400, Message: "User is not a bot"}}},
		{name: "deleted user", err: client.ResponseError{Err: &client.Error{Code: 400, Message: "User is deleted"}}},
		{name: "bot not found", err: client.ResponseError{Err: &client.Error{Code: 400, Message: "Bot not found"}}},
		{name: "transport error", err: errors.New("transport failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, m := newWarmupRepo(t)
			req := &client.SendBotStartMessageRequest{ChatId: 7131987336, BotUserId: 7131987336, Parameter: "res_212"}
			m.EXPECT().GetUser(&client.GetUserRequest{UserId: req.BotUserId}).Return(&client.User{Id: req.BotUserId}, nil).Once()
			m.EXPECT().SendBotStartMessage(req).Return(nil, tt.err).Once()
			_, err := r.SendBotStartMessage(req)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

// No SendBotStartMessage expectation: loading failure must prevent sending.
func TestSendBotStartMessage_UserLoadFailurePreventsSend(t *testing.T) {
	t.Parallel()
	for _, loadErr := range []error{
		client.ResponseError{Err: &client.Error{Code: 400, Message: "User not found"}},
		client.ResponseError{Err: &client.Error{Code: 429, Message: "Too Many Requests: retry after 7"}},
	} {
		t.Run(loadErr.Error(), func(t *testing.T) {
			t.Parallel()
			r, m := newWarmupRepo(t)
			req := &client.SendBotStartMessageRequest{ChatId: 7131987336, BotUserId: 7131987336, Parameter: "res_212"}
			m.EXPECT().GetUser(&client.GetUserRequest{UserId: req.BotUserId}).Return(nil, loadErr).Once()
			_, err := r.SendBotStartMessage(req)
			if isFloodWait(FloodWaitFromError(loadErr)) {
				var flood *FloodWaitError
				require.ErrorAs(t, err, &flood)
				require.Equal(t, 7500*time.Millisecond, flood.Delay)
			} else {
				require.ErrorIs(t, err, loadErr)
			}
		})
	}
}
