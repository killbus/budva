package facade

import "fmt"

// TargetNotBotError marks a resolved non-bot target.
type TargetNotBotError struct {
	Username string
}

func (e *TargetNotBotError) Error() string {
	return fmt.Sprintf("target %q is not a bot", e.Username)
}

func NewTargetNotBotError(username string) *TargetNotBotError {
	return &TargetNotBotError{Username: username}
}

// MessageNotFoundError marks a missing message or an empty latest chat.
type MessageNotFoundError struct {
	ChatID    int64
	MessageID int64
}

func (e *MessageNotFoundError) Error() string {
	return fmt.Sprintf("message %d not found in chat %d", e.MessageID, e.ChatID)
}

func NewMessageNotFoundError(chatID, messageID int64) *MessageNotFoundError {
	return &MessageNotFoundError{ChatID: chatID, MessageID: messageID}
}
