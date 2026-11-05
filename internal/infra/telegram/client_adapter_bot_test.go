package telegram

import (
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
