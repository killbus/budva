package grpc

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	"github.com/pure-golang/budva-claude/internal/infra/telegram"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func inlineMessage(chatID, id int64, markup client.ReplyMarkup) *client.Message {
	return &client.Message{ChatId: chatID, Id: id, ReplyMarkup: markup}
}

func TestResolveBot_Success(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().ResolveBot(mock.Anything, "example_bot").
		Return(&client.Chat{Id: 100}, &client.User{Id: 200}, nil)
	tr := New(facadeMock)

	resp, err := tr.ResolveBot(context.Background(), &pb.ResolveBotRequest{Username: "@example_bot"})

	require.NoError(t, err)
	assert.Equal(t, int64(100), resp.GetChatId())
	assert.Equal(t, int64(200), resp.GetBotUserId())
}

func TestResolveBot_EmptyUsername(t *testing.T) {
	t.Parallel()

	tr := New(mocks.NewFacadeService(t))
	_, err := tr.ResolveBot(context.Background(), &pb.ResolveBotRequest{Username: " @ "})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetBotInfo_Success(t *testing.T) {
	t.Parallel()

	fullInfo := &client.UserFullInfo{BotInfo: &client.BotInfo{
		ShortDescription: "short",
		Description:      "long",
		MenuButton:       &client.BotMenuButton{Text: "Open", Url: "https://example.test"},
		Commands: []*client.BotCommand{
			{Command: "/start", Description: "Start"},
			{Command: "/menu", Description: "Menu"},
		},
	}}
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetBotInfo(mock.Anything, int64(200)).Return(fullInfo, nil)
	tr := New(facadeMock)

	resp, err := tr.GetBotInfo(context.Background(), &pb.GetBotInfoRequest{BotUserId: 200})

	require.NoError(t, err)
	assert.Equal(t, int64(200), resp.GetBotUserId())
	assert.Equal(t, "short", resp.GetShortDescription())
	assert.Equal(t, "long", resp.GetDescription())
	assert.Equal(t, "Open", resp.GetMenuButton().GetText())
	assert.Equal(t, "https://example.test", resp.GetMenuButton().GetUrl())
	require.Len(t, resp.GetCommands(), 2)
	assert.Equal(t, "/menu", resp.GetCommands()[1].GetCommand())
}

func TestGetBotInfo_NonBot(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetBotInfo(mock.Anything, int64(200)).
		Return(&client.UserFullInfo{}, &facade.TargetNotBotError{Username: "200"})
	tr := New(facadeMock)

	_, err := tr.GetBotInfo(context.Background(), &pb.GetBotInfoRequest{BotUserId: 200})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestInlineMenuToProto(t *testing.T) {
	t.Parallel()

	msg := inlineMessage(100, 42, &client.ReplyMarkupInlineKeyboard{Rows: [][]*client.InlineKeyboardButton{{
		{Text: "Callback", Type: &client.InlineKeyboardButtonTypeCallback{Data: []byte("do-it")}},
		{Text: "URL", Type: &client.InlineKeyboardButtonTypeUrl{Url: "https://example.test"}},
		{Text: "Web App", Type: &client.InlineKeyboardButtonTypeWebApp{Url: "https://app.example.test"}},
		{Text: "Other", Type: &client.InlineKeyboardButtonTypeSwitchInline{Query: "switch"}},
	}}})

	resp := inlineMenuToProto(msg)
	require.Len(t, resp.GetRows(), 1)
	require.Len(t, resp.GetRows()[0].GetButtons(), 4)
	buttons := resp.GetRows()[0].GetButtons()
	assert.Equal(t, int64(42), resp.GetMessageId())
	assert.Equal(t, pb.MenuKind_MENU_KIND_INLINE, resp.GetMenuKind())
	assert.Equal(t, []byte("do-it"), buttons[0].GetCallbackData())
	assert.Equal(t, "https://example.test", buttons[1].GetUrl())
	assert.Equal(t, "https://app.example.test", buttons[2].GetWebApp().GetUrl())
	assert.Equal(t, "inlineKeyboardButtonTypeSwitchInline", buttons[3].GetOtherType())
}

func TestGetInlineMenu_NoKeyboardIsEmpty(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetInlineMenu(mock.Anything, int64(100), int64(42)).
		Return(inlineMessage(100, 42, nil), nil)
	tr := New(facadeMock)

	resp, err := tr.GetInlineMenu(context.Background(), &pb.GetInlineMenuRequest{ChatId: 100, MessageId: 42})

	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.GetMessageId())
	assert.Equal(t, pb.MenuKind_MENU_KIND_NONE, resp.GetMenuKind())
	assert.Empty(t, resp.GetRows())
	assert.Empty(t, resp.GetReplyRows())
}

func TestGetInlineMenu_LatestRequestPassesZero(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetInlineMenu(mock.Anything, int64(100), int64(0)).
		Return(inlineMessage(100, 77, &client.ReplyMarkupInlineKeyboard{}), nil)
	tr := New(facadeMock)

	resp, err := tr.GetInlineMenu(context.Background(), &pb.GetInlineMenuRequest{ChatId: 100})

	require.NoError(t, err)
	assert.Equal(t, int64(77), resp.GetMessageId())
	assert.Equal(t, pb.MenuKind_MENU_KIND_INLINE, resp.GetMenuKind())
}

func TestGetInlineMenu_ReplyKeyboard(t *testing.T) {
	t.Parallel()

	// Deleting this mapping must fail the kind, row, button, URL and attribute assertions below.
	msg := inlineMessage(100, 42, &client.ReplyMarkupShowKeyboard{
		Rows: [][]*client.KeyboardButton{
			{
				{Text: "Check-in", Type: &client.KeyboardButtonTypeText{}},
				nil,
				{Text: "Open", Type: &client.KeyboardButtonTypeWebApp{Url: "https://app.example.test"}},
			},
			{{Text: "Phone", Type: &client.KeyboardButtonTypeRequestPhoneNumber{}}},
		},
		IsPersistent:          true,
		ResizeKeyboard:        true,
		OneTime:               true,
		InputFieldPlaceholder: "Choose a menu...",
	})
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetInlineMenu(mock.Anything, int64(100), int64(0)).Return(msg, nil)
	tr := New(facadeMock)

	resp, err := tr.GetInlineMenu(context.Background(), &pb.GetInlineMenuRequest{ChatId: 100})

	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.GetMessageId())
	assert.Equal(t, pb.MenuKind_MENU_KIND_REPLY, resp.GetMenuKind())
	require.Len(t, resp.GetReplyRows(), 2)
	require.Len(t, resp.GetReplyRows()[0].GetButtons(), 2)
	require.Len(t, resp.GetReplyRows()[1].GetButtons(), 1)
	first := resp.GetReplyRows()[0].GetButtons()
	assert.Equal(t, "Check-in", first[0].GetText())
	assert.Equal(t, "keyboardButtonTypeText", first[0].GetType())
	assert.Empty(t, first[0].GetWebAppUrl())
	assert.Equal(t, "Open", first[1].GetText())
	assert.Equal(t, "keyboardButtonTypeWebApp", first[1].GetType())
	assert.Equal(t, "https://app.example.test", first[1].GetWebAppUrl())
	second := resp.GetReplyRows()[1].GetButtons()
	assert.Equal(t, "Phone", second[0].GetText())
	assert.Equal(t, "keyboardButtonTypeRequestPhoneNumber", second[0].GetType())
	assert.Empty(t, second[0].GetWebAppUrl())
	assert.True(t, resp.GetIsPersistent())
	assert.True(t, resp.GetResizeKeyboard())
	assert.True(t, resp.GetOneTime())
	assert.Equal(t, "Choose a menu...", resp.GetInputFieldPlaceholder())
	assert.Empty(t, resp.GetRows())
}

func TestGetInlineMenu_NoneKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		markup client.ReplyMarkup
	}{
		{name: "remove keyboard", markup: &client.ReplyMarkupRemoveKeyboard{}},
		{name: "force reply", markup: &client.ReplyMarkupForceReply{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := inlineMenuToProto(inlineMessage(100, 42, tt.markup))
			assert.Equal(t, int64(42), resp.GetMessageId())
			assert.Equal(t, pb.MenuKind_MENU_KIND_NONE, resp.GetMenuKind())
			assert.Empty(t, resp.GetRows())
			assert.Empty(t, resp.GetReplyRows())
		})
	}
}

func TestClickCallbackButton_Success(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().ClickCallbackButton(mock.Anything, int64(100), int64(42), []byte("do-it")).
		Return(&client.CallbackQueryAnswer{Text: "Done", ShowAlert: true, Url: "https://example.test"}, nil)
	tr := New(facadeMock)

	resp, err := tr.ClickCallbackButton(context.Background(), &pb.ClickCallbackButtonRequest{
		ChatId: 100, MessageId: 42, CallbackData: []byte("do-it"),
	})

	require.NoError(t, err)
	assert.True(t, resp.GetShowAlert())
	assert.Equal(t, "Done", resp.GetText())
	assert.Equal(t, "https://example.test", resp.GetUrl())
}

func TestClickCallbackButton_ContextDeadlineMapsToDeadlineExceeded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		facadeMock := mocks.NewFacadeService(t)
		facadeMock.EXPECT().ClickCallbackButton(mock.Anything, int64(100), int64(42), []byte("do-it")).
			RunAndReturn(func(ctx context.Context, _ int64, _ int64, _ []byte) (*client.CallbackQueryAnswer, error) {
				// Simulate go-tdlib completing after the caller has timed out.
				select {
				case <-ctx.Done():
				case <-time.After(2 * callbackAnswerTimeout):
				}
				return nil, errors.New("late TDLib result")
			})
		tr := New(facadeMock)

		start := time.Now()
		_, err := tr.ClickCallbackButton(context.Background(), &pb.ClickCallbackButtonRequest{
			ChatId: 100, MessageId: 42, CallbackData: []byte("do-it"),
		})
		synctest.Wait()

		require.Error(t, err)
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
		assert.Equal(t, callbackAnswerTimeout, time.Since(start))
	})
}

func TestClickCallbackButton_EmptyData(t *testing.T) {
	t.Parallel()

	tr := New(mocks.NewFacadeService(t))
	_, err := tr.ClickCallbackButton(context.Background(), &pb.ClickCallbackButtonRequest{ChatId: 100, MessageId: 42})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendBotCommand_Success(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().SendBotCommand(mock.Anything, int64(100), "/menu", int64(200), "").Return(nil)
	tr := New(facadeMock)

	resp, err := tr.SendBotCommand(context.Background(), &pb.SendBotCommandRequest{
		ChatId: 100, Command: " /menu ", BotUserId: 200,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSendBotCommand_EmptyCommand(t *testing.T) {
	t.Parallel()

	tr := New(mocks.NewFacadeService(t))
	_, err := tr.SendBotCommand(context.Background(), &pb.SendBotCommandRequest{Command: "   "})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestBotMenuErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "message_not_found", err: &facade.MessageNotFoundError{ChatID: 100, MessageID: 1}, code: codes.NotFound},
		{name: "bot_not_found", err: &telegram.BotNotFoundError{Username: "missing"}, code: codes.NotFound},
		{name: "target_not_bot", err: &facade.TargetNotBotError{Username: "example"}, code: codes.FailedPrecondition},
		{name: "callback_timeout", err: &telegram.CallbackAnswerError{Code: 502, Message: "query timeout"}, code: codes.DeadlineExceeded},
		{name: "callback_invalid", err: &telegram.CallbackAnswerError{Code: 400, Message: "BUTTON_DATA_INVALID"}, code: codes.FailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.code, status.Code(mapFacadeError(tt.err)))
		})
	}
}

func TestFloodWait_MapsToResourceExhaustedWithRetryInfo(t *testing.T) {
	t.Parallel()

	// Act
	err := mapFacadeError(&telegram.FloodWaitError{Delay: 7*time.Second + 500*time.Millisecond})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	st := status.Convert(err)
	require.Len(t, st.Details(), 1)
	retryInfo, ok := st.Details()[0].(*errdetails.RetryInfo)
	require.True(t, ok)
	assert.Equal(t, durationpb.New(7*time.Second+500*time.Millisecond), retryInfo.RetryDelay)
}

func TestTransportErrorsStayMapped(t *testing.T) {
	t.Parallel()

	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().ResolveBot(mock.Anything, "example_bot").Return(nil, nil, errors.New("boom"))
	tr := New(facadeMock)

	_, err := tr.ResolveBot(context.Background(), &pb.ResolveBotRequest{Username: "example_bot"})
	assert.Equal(t, codes.Internal, status.Code(err))
}
