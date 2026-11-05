package facade

import (
	"context"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/dto"
)

// telegramRepo — частично применяемый интерфейс к infra/telegram.
// Обёртки clientAdapter с raw-TDLib сигнатурами.
type telegramRepo interface {
	GetMessage(*client.GetMessageRequest) (*client.Message, error)
	GetChat(*client.GetChatRequest) (*client.Chat, error)
	GetChatHistory(*client.GetChatHistoryRequest) (*client.Messages, error)
	SendMessage(*client.SendMessageRequest) (*client.Message, error)
	SendMessageAlbum(*client.SendMessageAlbumRequest) (*client.Messages, error)
	ForwardMessages(*client.ForwardMessagesRequest) (*client.Messages, error)
	EditMessageText(*client.EditMessageTextRequest) (*client.Message, error)
	DeleteMessages(*client.DeleteMessagesRequest) (*client.Ok, error)
	GetMessageLink(*client.GetMessageLinkRequest) (*client.MessageLink, error)
	GetMessages(*client.GetMessagesRequest) (*client.Messages, error)
	GetMessageLinkInfo(*client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error)
	GetMe() (*client.User, error)
	GetOption(*client.GetOptionRequest) (client.OptionValue, error)
	SearchPublicChat(*client.SearchPublicChatRequest) (*client.Chat, error)
	GetUser(*client.GetUserRequest) (*client.User, error)
	GetUserFullInfo(*client.GetUserFullInfoRequest) (*client.UserFullInfo, error)
	GetCallbackQueryAnswer(*client.GetCallbackQueryAnswerRequest) (*client.CallbackQueryAnswer, error)
	SendBotStartMessage(*client.SendBotStartMessageRequest) (*client.Message, error)
}

// Service реализует фасад для внешнего доступа к Telegram.
type Service struct {
	telegramRepo telegramRepo
}

// New создаёт новый экземпляр фасада.
func New(telegramRepo telegramRepo) *Service {
	return &Service{telegramRepo: telegramRepo}
}

// GetMessage возвращает сообщение по ID.
func (s *Service) GetMessage(_ context.Context, chatID int64, messageID int64) (*client.Message, error) {
	return s.telegramRepo.GetMessage(&client.GetMessageRequest{ChatId: chatID, MessageId: messageID})
}

// SendMessage отправляет текстовое сообщение.
func (s *Service) SendMessage(_ context.Context, chatID int64, text string) error {
	_, err := s.telegramRepo.SendMessage(&client.SendMessageRequest{
		ChatId: chatID,
		InputMessageContent: &client.InputMessageText{
			Text: &client.FormattedText{Text: text},
		},
	})
	return err
}

// SendMessageAlbum отправляет несколько сообщений как альбом.
// ContentType определяется по расширению файла; без FilePath — отправляется как текст.
func (s *Service) SendMessageAlbum(_ context.Context, chatID int64, items []domain.AlbumItem) error {
	contents := make([]client.InputMessageContent, 0, len(items))
	for _, item := range items {
		caption := &client.FormattedText{Text: item.Text}
		if item.FilePath == "" {
			contents = append(contents, &client.InputMessageText{Text: caption})
			continue
		}
		contents = append(contents, inputMessageByFileExt(item.FilePath, caption))
	}
	_, err := s.telegramRepo.SendMessageAlbum(&client.SendMessageAlbumRequest{
		ChatId:               chatID,
		InputMessageContents: contents,
	})
	return err
}

// ForwardMessage пересылает сообщение внутри одного чата (копия).
func (s *Service) ForwardMessage(_ context.Context, chatID int64, messageID int64) error {
	_, err := s.telegramRepo.ForwardMessages(&client.ForwardMessagesRequest{
		ChatId:     chatID,
		FromChatId: chatID,
		MessageIds: []int64{messageID},
	})
	return err
}

// UpdateMessage обновляет текст сообщения.
func (s *Service) UpdateMessage(_ context.Context, chatID int64, messageID int64, text string) error {
	_, err := s.telegramRepo.EditMessageText(&client.EditMessageTextRequest{
		ChatId:              chatID,
		MessageId:           messageID,
		InputMessageContent: &client.InputMessageText{Text: &client.FormattedText{Text: text}},
	})
	return err
}

// DeleteMessages удаляет сообщения.
func (s *Service) DeleteMessages(_ context.Context, chatID int64, messageIDs []int64) error {
	_, err := s.telegramRepo.DeleteMessages(&client.DeleteMessagesRequest{
		ChatId:     chatID,
		MessageIds: messageIDs,
		Revoke:     true,
	})
	return err
}

// GetChatHistory возвращает сообщения чата с пагинацией.
func (s *Service) GetChatHistory(_ context.Context, chatID int64, fromMessageID int64, offset, limit int32) ([]*client.Message, error) {
	msgs, err := s.telegramRepo.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId:        chatID,
		FromMessageId: fromMessageID,
		Offset:        offset,
		Limit:         limit,
	})
	if err != nil {
		return nil, err
	}
	return msgs.Messages, nil
}

// GetMessages возвращает сообщения по списку ID (batch).
func (s *Service) GetMessages(_ context.Context, chatID int64, messageIDs []int64) ([]*client.Message, error) {
	msgs, err := s.telegramRepo.GetMessages(&client.GetMessagesRequest{
		ChatId:     chatID,
		MessageIds: messageIDs,
	})
	if err != nil {
		return nil, err
	}
	return msgs.Messages, nil
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (s *Service) GetMessageLink(_ context.Context, chatID int64, messageID int64) (string, error) {
	link, err := s.telegramRepo.GetMessageLink(&client.GetMessageLinkRequest{
		ChatId:    chatID,
		MessageId: messageID,
		ForAlbum:  false,
	})
	if err != nil {
		return "", err
	}
	return link.Link, nil
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (s *Service) GetMessageLinkInfo(_ context.Context, url string) (*client.MessageLinkInfo, error) {
	return s.telegramRepo.GetMessageLinkInfo(&client.GetMessageLinkInfoRequest{Url: url})
}

// GetStatus возвращает текущий статус сервиса.
func (s *Service) GetStatus(_ context.Context) (*dto.GraphQLStatusResponse, error) {
	versionOpt, err := s.telegramRepo.GetOption(&client.GetOptionRequest{Name: "version"})
	if err != nil {
		return nil, err
	}
	var version string
	if v, ok := versionOpt.(*client.OptionValueString); ok {
		version = v.Value
	}
	user, err := s.telegramRepo.GetMe()
	if err != nil {
		return nil, err
	}
	return &dto.GraphQLStatusResponse{
		ReleaseVersion: releaseVersion(),
		TDLibVersion:   version,
		UserID:         user.Id,
	}, nil
}

// GetBotInfo returns raw TDLib bot profile information.
func (s *Service) GetBotInfo(_ context.Context, botUserID int64) (*client.UserFullInfo, error) {
	fullInfo, err := s.telegramRepo.GetUserFullInfo(&client.GetUserFullInfoRequest{UserId: botUserID})
	if err != nil {
		return nil, err
	}
	if fullInfo == nil || fullInfo.BotInfo == nil {
		return nil, NewTargetNotBotError(strconv.FormatInt(botUserID, 10))
	}
	return fullInfo, nil
}

// GetInlineMenu returns a raw message; keyboard mapping stays in transport.
func (s *Service) GetInlineMenu(ctx context.Context, chatID, messageID int64) (*client.Message, error) {
	if messageID == 0 {
		chat, err := s.telegramRepo.GetChat(&client.GetChatRequest{ChatId: chatID})
		if err != nil {
			return nil, err
		}
		if chat.ReplyMarkupMessageId != 0 {
			messageID = chat.ReplyMarkupMessageId
		}
	}
	if messageID != 0 {
		return s.GetMessage(ctx, chatID, messageID)
	}

	messages, err := s.telegramRepo.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId: chatID,
		Limit:  1,
	})
	if err != nil {
		return nil, err
	}
	if len(messages.Messages) == 0 {
		return nil, NewMessageNotFoundError(chatID, 0)
	}
	return messages.Messages[0], nil
}

// ClickCallbackButton sends callback button data and returns the bot answer.
func (s *Service) ClickCallbackButton(_ context.Context, chatID, messageID int64, callbackData []byte) (*client.CallbackQueryAnswer, error) {
	return s.telegramRepo.GetCallbackQueryAnswer(&client.GetCallbackQueryAnswerRequest{
		ChatId:    chatID,
		MessageId: messageID,
		Payload:   &client.CallbackQueryPayloadData{Data: callbackData},
	})
}

// SendBotCommand sends a slash command; /start uses TDLib's deep-link method.
func (s *Service) SendBotCommand(_ context.Context, chatID int64, command string, botUserID int64, parameter string) error {
	if command == "/start" && parameter != "" {
		_, err := s.telegramRepo.SendBotStartMessage(&client.SendBotStartMessageRequest{
			BotUserId: botUserID,
			ChatId:    chatID,
			Parameter: parameter,
		})
		return err
	}
	_, err := s.telegramRepo.SendMessage(&client.SendMessageRequest{
		ChatId: chatID,
		InputMessageContent: &client.InputMessageText{
			Text: &client.FormattedText{Text: command},
		},
	})
	return err
}

// ResolveBot resolves a bot username to its private chat and user IDs.
func (s *Service) ResolveBot(_ context.Context, username string) (*client.Chat, *client.User, error) {
	chat, err := s.telegramRepo.SearchPublicChat(&client.SearchPublicChatRequest{Username: username})
	if err != nil {
		return nil, nil, err
	}
	privateChat, ok := chat.Type.(*client.ChatTypePrivate)
	if !ok {
		return nil, nil, NewTargetNotBotError(username)
	}
	user, err := s.telegramRepo.GetUser(&client.GetUserRequest{UserId: privateChat.UserId})
	if err != nil {
		return nil, nil, err
	}
	if _, ok := user.Type.(*client.UserTypeBot); !ok {
		return nil, nil, NewTargetNotBotError(username)
	}
	return chat, user, nil
}

func releaseVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

// inputMessageByFileExt выбирает корректный InputMessageContent по расширению файла.
func inputMessageByFileExt(filePath string, caption *client.FormattedText) client.InputMessageContent {
	ext := strings.ToLower(filepath.Ext(filePath))
	file := &client.InputFileLocal{Path: filePath}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return &client.InputMessagePhoto{Photo: file, Caption: caption}
	case ".mp4", ".mov", ".avi", ".mkv":
		return &client.InputMessageVideo{Video: file, Caption: caption}
	case ".mp3", ".ogg", ".m4a", ".flac", ".wav":
		return &client.InputMessageAudio{Audio: file, Caption: caption}
	case ".gif":
		return &client.InputMessageAnimation{Animation: file, Caption: caption}
	default:
		return &client.InputMessageDocument{Document: file, Caption: caption}
	}
}
