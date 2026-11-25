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

// MessageFileNotFoundError marks media content whose downloadable file is absent.
type MessageFileNotFoundError struct {
	ChatID    int64
	MessageID int64
}

func (e *MessageFileNotFoundError) Error() string {
	return fmt.Sprintf("file for message %d not found in chat %d", e.MessageID, e.ChatID)
}

func NewMessageFileNotFoundError(chatID, messageID int64) *MessageFileNotFoundError {
	return &MessageFileNotFoundError{ChatID: chatID, MessageID: messageID}
}

// FileDownloadFailedError marks a download that did not reach the local-complete oracle.
type FileDownloadFailedError struct {
	FileID int32
	Err    error
}

func (e *FileDownloadFailedError) Error() string {
	return fmt.Sprintf("download file %d failed: %v", e.FileID, e.Err)
}

func (e *FileDownloadFailedError) Unwrap() error {
	return e.Err
}

func NewFileDownloadFailedError(fileID int32, err error) *FileDownloadFailedError {
	return &FileDownloadFailedError{FileID: fileID, Err: err}
}
