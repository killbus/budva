package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	alogger "github.com/pure-golang/adapters/logger"
	"github.com/zelenin/go-tdlib/client"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/telegram"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

// chatNotReadyRetryDelay — подсказка клиенту в gRPC RetryInfo: к этому
// моменту типичный wait-for-ready уже завершился, повторный запрос почти
// наверняка попадёт на прогретую БД (значение зеркалит
// telegram.warmRetryInfoDelay — пакетная константа не экспортируется).
const chatNotReadyRetryDelay = 2 * time.Second

// callbackAnswerTimeout is the server-side wait for ClickCallbackButton.
// go-tdlib returns TDLib error 502 when the bot misses its query timeout;
// that error is mapped to DeadlineExceeded rather than Internal.
const callbackAnswerTimeout = 10 * time.Second

// mapFacadeError маппит ошибку facade-сервиса в gRPC-статус.
// ChatNotReadyError (chat остался непрогретым в окне wait-for-ready) —
// Unavailable + RetryInfo: сервис временно не готов, а не сломан, и клиенту
// известно, когда повторять. Новые semantic typed errors only apply to bot-menu
// RPC paths. Остальные ошибки — Internal (прежнее поведение).
func mapFacadeError(err error) error {
	var floodWait *telegram.FloodWaitError
	if errors.As(err, &floodWait) {
		st := status.New(codes.ResourceExhausted, err.Error())
		if withRetry, derr := st.WithDetails(&errdetails.RetryInfo{
			RetryDelay: durationpb.New(floodWait.Delay),
		}); derr == nil {
			st = withRetry
		}
		return st.Err()
	}
	var messageNotFound *facade.MessageNotFoundError
	if errors.As(err, &messageNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	var botNotFound *telegram.BotNotFoundError
	if errors.As(err, &botNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	var targetNotBot *facade.TargetNotBotError
	if errors.As(err, &targetNotBot) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	var callbackError *telegram.CallbackAnswerError
	if errors.As(err, &callbackError) {
		if callbackError.Code == 502 {
			return status.Error(codes.DeadlineExceeded, err.Error())
		}
		if callbackError.Code >= 400 && callbackError.Code < 500 {
			return status.Error(codes.FailedPrecondition, err.Error())
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	var notReady *telegram.ChatNotReadyError
	if errors.As(err, &notReady) {
		st := status.New(codes.Unavailable, err.Error())
		if withRetry, derr := st.WithDetails(&errdetails.RetryInfo{
			RetryDelay: durationpb.New(chatNotReadyRetryDelay),
		}); derr == nil {
			st = withRetry
		}
		return st.Err()
	}
	return status.Error(codes.Internal, err.Error())
}

// facadeService — частично применяемый интерфейс к service/facade.
// Все методы вернут raw-TDLib типы, которые gRPC-слой сворачивает в свой proto-DTO.
type facadeService interface {
	GetMessage(ctx context.Context, chatID int64, messageID int64) (*client.Message, error)
	GetMessages(ctx context.Context, chatID int64, messageIDs []int64) ([]*client.Message, error)
	GetChatHistory(ctx context.Context, chatID int64, fromMessageID int64, offset, limit int32) ([]*client.Message, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageAlbum(ctx context.Context, chatID int64, items []domain.AlbumItem) error
	ForwardMessage(ctx context.Context, chatID int64, messageID int64) error
	UpdateMessage(ctx context.Context, chatID int64, messageID int64, text string) error
	DeleteMessages(ctx context.Context, chatID int64, messageIDs []int64) error
	GetMessageLink(ctx context.Context, chatID int64, messageID int64) (string, error)
	GetMessageLinkInfo(ctx context.Context, link string) (*client.MessageLinkInfo, error)
	ResolveBot(ctx context.Context, username string) (*client.Chat, *client.User, error)
	GetBotInfo(ctx context.Context, botUserID int64) (*client.UserFullInfo, error)
	GetInlineMenu(ctx context.Context, chatID int64, messageID int64) (*client.Message, error)
	ClickCallbackButton(ctx context.Context, chatID int64, messageID int64, callbackData []byte) (*client.CallbackQueryAnswer, error)
	SendBotCommand(ctx context.Context, chatID int64, command string, botUserID int64, parameter string) error
}

// Transport реализует gRPC-сервер FacadeGRPC.
type Transport struct {
	pb.UnimplementedFacadeGRPCServer
	facadeService facadeService
}

// New создаёт новый экземпляр gRPC-транспорта.
func New(facadeService facadeService) *Transport {
	return &Transport{facadeService: facadeService}
}

// GetMessages возвращает сообщения по списку ID (batch).
func (t *Transport) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessagesResponse, error) {
	msgs, err := t.facadeService.GetMessages(ctx, req.GetChatId(), req.GetMessageIds())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get messages", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	var messages []*pb.Message
	for _, msg := range msgs {
		if msg != nil {
			messages = append(messages, clientMessageToProto(msg))
		}
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// GetChatHistory возвращает историю чата с пагинацией.
func (t *Transport) GetChatHistory(ctx context.Context, req *pb.GetChatHistoryRequest) (*pb.MessagesResponse, error) {
	msgs, err := t.facadeService.GetChatHistory(ctx, req.GetChatId(), req.GetFromMessageId(), req.GetOffset(), req.GetLimit())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get chat history", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	var messages []*pb.Message
	for _, msg := range msgs {
		messages = append(messages, clientMessageToProto(msg))
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// SendMessage отправляет сообщение.
func (t *Transport) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facadeService.SendMessage(ctx, msg.GetChatId(), msg.GetText()); err != nil {
		alogger.FromContext(ctx).Error("Failed to send message", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// SendMessageAlbum отправляет несколько сообщений как альбом.
func (t *Transport) SendMessageAlbum(ctx context.Context, req *pb.SendMessageAlbumRequest) (*pb.EmptyResponse, error) {
	msgs := req.GetMessages()
	if len(msgs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one message is required")
	}
	chatID := msgs[0].GetChatId()
	items := make([]domain.AlbumItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, domain.AlbumItem{
			Text:     m.GetText(),
			FilePath: m.GetFilePath(),
		})
	}
	if err := t.facadeService.SendMessageAlbum(ctx, chatID, items); err != nil {
		alogger.FromContext(ctx).Error("Failed to send message album", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// ForwardMessage пересылает сообщение.
func (t *Transport) ForwardMessage(ctx context.Context, req *pb.ForwardMessageRequest) (*pb.EmptyResponse, error) {
	if err := t.facadeService.ForwardMessage(ctx, req.GetChatId(), req.GetMessageId()); err != nil {
		alogger.FromContext(ctx).Error("Failed to forward message", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessage возвращает одно сообщение.
func (t *Transport) GetMessage(ctx context.Context, req *pb.GetMessageRequest) (*pb.MessageResponse, error) {
	msg, err := t.facadeService.GetMessage(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.MessageResponse{Message: clientMessageToProto(msg)}, nil
}

// UpdateMessage обновляет текст сообщения.
func (t *Transport) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facadeService.UpdateMessage(ctx, msg.GetChatId(), msg.GetId(), msg.GetText()); err != nil {
		alogger.FromContext(ctx).Error("Failed to update message", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// DeleteMessages удаляет сообщения.
func (t *Transport) DeleteMessages(ctx context.Context, req *pb.DeleteMessagesRequest) (*pb.EmptyResponse, error) {
	if err := t.facadeService.DeleteMessages(ctx, req.GetChatId(), req.GetMessageIds()); err != nil {
		alogger.FromContext(ctx).Error("Failed to delete messages", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (t *Transport) GetMessageLink(ctx context.Context, req *pb.GetMessageLinkRequest) (*pb.MessageLinkResponse, error) {
	link, err := t.facadeService.GetMessageLink(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message link", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.MessageLinkResponse{Link: link}, nil
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (t *Transport) GetMessageLinkInfo(ctx context.Context, req *pb.GetMessageLinkInfoRequest) (*pb.MessageResponse, error) {
	info, err := t.facadeService.GetMessageLinkInfo(ctx, req.GetLink())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message link info", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	var messageID int64
	if info != nil && info.Message != nil {
		messageID = info.Message.Id
	}
	var chatID int64
	if info != nil {
		chatID = info.ChatId
	}
	return &pb.MessageResponse{
		Message: &pb.Message{
			Id:     messageID,
			ChatId: chatID,
		},
	}, nil
}

// ResolveBot resolves a bot username to its private chat and user IDs.
func (t *Transport) ResolveBot(ctx context.Context, req *pb.ResolveBotRequest) (*pb.ResolveBotResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	chat, user, err := t.facadeService.ResolveBot(ctx, username)
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to resolve bot", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.ResolveBotResponse{
		ChatId:    chat.Id,
		BotUserId: user.Id,
	}, nil
}

// GetBotInfo returns bot commands, menu button and descriptions.
func (t *Transport) GetBotInfo(ctx context.Context, req *pb.GetBotInfoRequest) (*pb.BotInfoResponse, error) {
	fullInfo, err := t.facadeService.GetBotInfo(ctx, req.GetBotUserId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get bot info", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return botInfoToProto(req.GetBotUserId(), fullInfo), nil
}

// GetInlineMenu returns the inline or reply keyboard of a message.
func (t *Transport) GetInlineMenu(ctx context.Context, req *pb.GetInlineMenuRequest) (*pb.InlineMenuResponse, error) {
	msg, err := t.facadeService.GetInlineMenu(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get inline menu", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return inlineMenuToProto(msg), nil
}

// ClickCallbackButton clicks a callback button and returns the bot answer.
func (t *Transport) ClickCallbackButton(ctx context.Context, req *pb.ClickCallbackButtonRequest) (*pb.CallbackAnswerResponse, error) {
	if len(req.GetCallbackData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "callback_data is required")
	}

	// go-tdlib's callback call blocks independently of the request context.
	// The goroutine keeps it alive for one call; on timeout its result is
	// discarded and TDLib completes in the background.
	callCtx, cancel := context.WithTimeout(ctx, callbackAnswerTimeout)
	defer cancel()
	type callbackResult struct {
		answer *client.CallbackQueryAnswer
		err    error
	}
	resultCh := make(chan callbackResult, 1)
	go func() {
		answer, err := t.facadeService.ClickCallbackButton(callCtx, req.GetChatId(), req.GetMessageId(), req.GetCallbackData())
		resultCh <- callbackResult{answer: answer, err: err}
	}()

	select {
	case result := <-resultCh:
		switch callCtx.Err() {
		case context.DeadlineExceeded:
			return nil, status.Error(codes.DeadlineExceeded, "bot did not answer callback before deadline")
		case context.Canceled:
			return nil, status.Error(codes.Canceled, "callback request canceled")
		}
		if result.err != nil {
			alogger.FromContext(ctx).Error("Failed to click callback button", slog.Any("err", result.err))
			return nil, mapFacadeError(result.err)
		}
		return callbackAnswerToProto(result.answer), nil
	case <-callCtx.Done():
		if callCtx.Err() == context.Canceled {
			return nil, status.Error(codes.Canceled, "callback request canceled")
		}
		return nil, status.Error(codes.DeadlineExceeded, "bot did not answer callback before deadline")
	}
}

// SendBotCommand sends a slash command to a bot chat.
func (t *Transport) SendBotCommand(ctx context.Context, req *pb.SendBotCommandRequest) (*pb.EmptyResponse, error) {
	command := strings.TrimSpace(req.GetCommand())
	if command == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}
	if err := t.facadeService.SendBotCommand(ctx, req.GetChatId(), command, req.GetBotUserId(), req.GetParameter()); err != nil {
		alogger.FromContext(ctx).Error("Failed to send bot command", slog.Any("err", err))
		return nil, mapFacadeError(err)
	}
	return &pb.EmptyResponse{}, nil
}

// clientMessageToProto маппит *client.Message (raw TDLib) в pb.Message.
func clientMessageToProto(msg *client.Message) *pb.Message {
	if msg == nil {
		return nil
	}
	var text string
	switch c := msg.Content.(type) {
	case *client.MessageText:
		if c.Text != nil {
			text = c.Text.Text
		}
	case *client.MessagePhoto:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	case *client.MessageVideo:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	case *client.MessageDocument:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	}
	return &pb.Message{
		Id:      msg.Id,
		ChatId:  msg.ChatId,
		Text:    text,
		Forward: msg.ForwardInfo != nil,
	}
}

// botInfoToProto maps raw TDLib UserFullInfo to the bot menu DTO.
func botInfoToProto(botUserID int64, fullInfo *client.UserFullInfo) *pb.BotInfoResponse {
	if fullInfo == nil || fullInfo.BotInfo == nil {
		return &pb.BotInfoResponse{BotUserId: botUserID}
	}
	botInfo := fullInfo.BotInfo
	commands := make([]*pb.BotCommand, 0, len(botInfo.Commands))
	for _, command := range botInfo.Commands {
		if command == nil {
			continue
		}
		commands = append(commands, &pb.BotCommand{
			Command:     command.Command,
			Description: command.Description,
		})
	}
	response := &pb.BotInfoResponse{
		BotUserId:        botUserID,
		Commands:         commands,
		Description:      botInfo.Description,
		ShortDescription: botInfo.ShortDescription,
	}
	if botInfo.MenuButton != nil {
		response.MenuButton = &pb.BotMenuButtonInfo{
			Text: botInfo.MenuButton.Text,
			Url:  botInfo.MenuButton.Url,
		}
	}
	return response
}

// inlineMenuToProto maps a raw message's inline or reply keyboard, preserving row and button order.
func inlineMenuToProto(msg *client.Message) *pb.InlineMenuResponse {
	if msg == nil {
		return &pb.InlineMenuResponse{}
	}
	response := &pb.InlineMenuResponse{MessageId: msg.Id, MenuKind: pb.MenuKind_MENU_KIND_NONE}
	keyboard, ok := msg.ReplyMarkup.(*client.ReplyMarkupInlineKeyboard)
	if !ok {
		if showKeyboard, ok := msg.ReplyMarkup.(*client.ReplyMarkupShowKeyboard); ok {
			response.MenuKind = pb.MenuKind_MENU_KIND_REPLY
			response.ReplyRows = make([]*pb.ReplyButtonRow, 0, len(showKeyboard.Rows))
			for _, row := range showKeyboard.Rows {
				protoRow := &pb.ReplyButtonRow{Buttons: make([]*pb.ReplyButton, 0, len(row))}
				for _, button := range row {
					if button == nil {
						continue
					}
					protoButton := &pb.ReplyButton{Text: button.Text}
					if button.Type != nil {
						protoButton.Type = button.Type.KeyboardButtonTypeType()
						if webAppType, ok := button.Type.(*client.KeyboardButtonTypeWebApp); ok {
							protoButton.WebAppUrl = webAppType.Url
						}
					}
					protoRow.Buttons = append(protoRow.Buttons, protoButton)
				}
				response.ReplyRows = append(response.ReplyRows, protoRow)
			}
			response.IsPersistent = showKeyboard.IsPersistent
			response.ResizeKeyboard = showKeyboard.ResizeKeyboard
			response.OneTime = showKeyboard.OneTime
			response.InputFieldPlaceholder = showKeyboard.InputFieldPlaceholder
			return response
		}
		return response
	}
	response.MenuKind = pb.MenuKind_MENU_KIND_INLINE
	response.Rows = make([]*pb.InlineButtonRow, 0, len(keyboard.Rows))
	for _, row := range keyboard.Rows {
		protoRow := &pb.InlineButtonRow{Buttons: make([]*pb.InlineButton, 0, len(row))}
		for _, button := range row {
			if button == nil {
				continue
			}
			protoButton := &pb.InlineButton{Text: button.Text}
			switch buttonType := button.Type.(type) {
			case *client.InlineKeyboardButtonTypeCallback:
				protoButton.Action = &pb.InlineButton_CallbackData{CallbackData: buttonType.Data}
			case *client.InlineKeyboardButtonTypeUrl:
				protoButton.Action = &pb.InlineButton_Url{Url: buttonType.Url}
			case *client.InlineKeyboardButtonTypeWebApp:
				protoButton.Action = &pb.InlineButton_WebApp{
					WebApp: &pb.WebAppButtonInfo{Text: button.Text, Url: buttonType.Url},
				}
			default:
				var otherType string
				if button.Type != nil {
					otherType = button.Type.InlineKeyboardButtonTypeType()
				}
				protoButton.Action = &pb.InlineButton_OtherType{OtherType: otherType}
			}
			protoRow.Buttons = append(protoRow.Buttons, protoButton)
		}
		response.Rows = append(response.Rows, protoRow)
	}
	return response
}

// callbackAnswerToProto maps raw TDLib callback answer to the response DTO.
func callbackAnswerToProto(answer *client.CallbackQueryAnswer) *pb.CallbackAnswerResponse {
	if answer == nil {
		return &pb.CallbackAnswerResponse{}
	}
	return &pb.CallbackAnswerResponse{
		Text:      answer.Text,
		ShowAlert: answer.ShowAlert,
		Url:       answer.Url,
	}
}
