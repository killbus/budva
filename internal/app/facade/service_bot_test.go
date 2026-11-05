package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/facade/mocks"
)

func TestService_ResolveBot(t *testing.T) {
	t.Parallel()

	t.Run("bot", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantChat := &client.Chat{Id: 100, Type: &client.ChatTypePrivate{UserId: 200}}
		wantUser := &client.User{Id: 200, Type: &client.UserTypeBot{}}
		gw.EXPECT().SearchPublicChat(mock.MatchedBy(func(req *client.SearchPublicChatRequest) bool {
			return req.Username == "example_bot"
		})).Return(wantChat, nil)
		gw.EXPECT().GetUser(mock.MatchedBy(func(req *client.GetUserRequest) bool {
			return req.UserId == 200
		})).Return(wantUser, nil)

		chat, user, err := svc.ResolveBot(context.Background(), "example_bot")

		require.NoError(t, err)
		assert.Same(t, wantChat, chat)
		assert.Same(t, wantUser, user)
	})

	t.Run("not bot", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().SearchPublicChat(mock.Anything).Return(&client.Chat{
			Id:   100,
			Type: &client.ChatTypePrivate{UserId: 200},
		}, nil)
		gw.EXPECT().GetUser(mock.Anything).Return(&client.User{Id: 200, Type: &client.UserTypeRegular{}}, nil)

		chat, user, err := svc.ResolveBot(context.Background(), "example_user")

		var notBot *TargetNotBotError
		require.ErrorAs(t, err, &notBot)
		assert.Nil(t, chat)
		assert.Nil(t, user)
	})

	t.Run("non private chat", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().SearchPublicChat(mock.Anything).Return(&client.Chat{Id: -100, Type: &client.ChatTypeSupergroup{}}, nil)

		_, _, err := svc.ResolveBot(context.Background(), "example_channel")
		require.ErrorAs(t, err, new(*TargetNotBotError))
	})
}

func TestService_GetBotInfo(t *testing.T) {
	t.Parallel()

	t.Run("bot", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.UserFullInfo{BotInfo: &client.BotInfo{Description: "bot"}}
		gw.EXPECT().GetUserFullInfo(mock.MatchedBy(func(req *client.GetUserFullInfoRequest) bool {
			return req.UserId == 200
		})).Return(want, nil)

		got, err := svc.GetBotInfo(context.Background(), 200)

		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("non bot", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetUserFullInfo(mock.Anything).Return(&client.UserFullInfo{}, nil)

		_, err := svc.GetBotInfo(context.Background(), 200)
		require.ErrorAs(t, err, new(*TargetNotBotError))
	})
}

func TestService_GetInlineMenu(t *testing.T) {
	t.Parallel()

	t.Run("explicit message skips GetChat", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.Message{Id: 42}
		gw.EXPECT().GetMessage(mock.MatchedBy(func(req *client.GetMessageRequest) bool {
			return req.ChatId == 100 && req.MessageId == 42
		})).Return(want, nil)

		got, err := svc.GetInlineMenu(context.Background(), 100, 42)

		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("reply markup pointer is authoritative", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.Message{Id: 777}
		gw.EXPECT().GetChat(&client.GetChatRequest{ChatId: 100}).
			Return(&client.Chat{ReplyMarkupMessageId: 777}, nil)
		gw.EXPECT().GetMessage(mock.MatchedBy(func(req *client.GetMessageRequest) bool {
			return req.ChatId == 100 && req.MessageId == 777
		})).Return(want, nil)

		got, err := svc.GetInlineMenu(context.Background(), 100, 0)

		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("empty markup pointer falls back to latest", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.Message{Id: 43}
		gw.EXPECT().GetChat(&client.GetChatRequest{ChatId: 100}).Return(&client.Chat{}, nil)
		gw.EXPECT().GetChatHistory(mock.MatchedBy(func(req *client.GetChatHistoryRequest) bool {
			return req.ChatId == 100 && req.FromMessageId == 0 && req.Limit == 1
		})).Return(&client.Messages{Messages: []*client.Message{want}}, nil)

		got, err := svc.GetInlineMenu(context.Background(), 100, 0)

		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("latest empty", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetChat(&client.GetChatRequest{ChatId: 100}).Return(&client.Chat{}, nil)
		gw.EXPECT().GetChatHistory(mock.Anything).Return(&client.Messages{}, nil)

		_, err := svc.GetInlineMenu(context.Background(), 100, 0)
		require.ErrorAs(t, err, new(*MessageNotFoundError))
	})

	t.Run("GetChat error", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("chat unavailable")
		gw.EXPECT().GetChat(mock.Anything).Return(nil, wantErr)

		got, err := svc.GetInlineMenu(context.Background(), 100, 0)

		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestService_ClickCallbackButton(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	want := &client.CallbackQueryAnswer{Text: "done"}
	gw.EXPECT().GetCallbackQueryAnswer(mock.MatchedBy(func(req *client.GetCallbackQueryAnswerRequest) bool {
		if req.ChatId != 100 || req.MessageId != 42 {
			return false
		}
		payload, ok := req.Payload.(*client.CallbackQueryPayloadData)
		return ok && string(payload.Data) == "do-it"
	})).Return(want, nil)

	got, err := svc.ClickCallbackButton(context.Background(), 100, 42, []byte("do-it"))

	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestService_SendBotCommand(t *testing.T) {
	t.Parallel()

	t.Run("start parameter uses bot start", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().SendBotStartMessage(mock.MatchedBy(func(req *client.SendBotStartMessageRequest) bool {
			return req.BotUserId == 200 && req.ChatId == 100 && req.Parameter == "ref"
		})).Return(&client.Message{Id: 1}, nil)

		err := svc.SendBotCommand(context.Background(), 100, "/start", 200, "ref")
		require.NoError(t, err)
	})

	t.Run("regular command uses message", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
			if req.ChatId != 100 {
				return false
			}
			text, ok := req.InputMessageContent.(*client.InputMessageText)
			return ok && text.Text != nil && text.Text.Text == "/menu"
		})).Return(&client.Message{Id: 2}, nil)

		err := svc.SendBotCommand(context.Background(), 100, "/menu", 0, "")
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("send failed")
		gw.EXPECT().SendMessage(mock.Anything).Return(nil, wantErr)

		err := svc.SendBotCommand(context.Background(), 100, "/menu", 0, "")
		require.ErrorIs(t, err, wantErr)
	})
}
