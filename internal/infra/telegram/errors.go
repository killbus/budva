package telegram

import (
	"errors"
	"fmt"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

// FloodWaitError carries the wait duration parsed from a TDLib FLOOD_WAIT.
// The delay includes the same 500 ms margin as parseFloodWait.
type FloodWaitError struct {
	Delay time.Duration
}

func (e *FloodWaitError) Error() string {
	return fmt.Sprintf("flood wait for %s", e.Delay)
}

// FloodWaitFromError converts a TDLib FLOOD_WAIT into FloodWaitError.
// A non-flood error is passed through unchanged.
func FloodWaitFromError(err error) error {
	if err == nil {
		return nil
	}
	if responseError, ok := err.(client.ResponseError); ok && responseError.Err != nil &&
		responseError.Err.Code == 429 {
		if delay := parseFloodWait(responseError.Err.Message); delay > 0 {
			return &FloodWaitError{Delay: delay}
		}
	}
	if delay := parseFloodWait(err.Error()); delay > 0 {
		return &FloodWaitError{Delay: delay}
	}
	return err
}

// BotNotFoundError marks a username that TDLib could not resolve.
type BotNotFoundError struct {
	Username string
}

func (e *BotNotFoundError) Error() string {
	return fmt.Sprintf("username %q not found", e.Username)
}

// CallbackAnswerError carries the TDLib status of a failed callback query.
type CallbackAnswerError struct {
	Code    int32
	Message string
}

func (e *CallbackAnswerError) Error() string {
	return fmt.Sprintf("callback answer failed: %d %s", e.Code, e.Message)
}

// NewCallbackAnswerError creates a semantic callback failure from a TDLib error.
func NewCallbackAnswerError(err error) *CallbackAnswerError {
	responseError, ok := err.(client.ResponseError)
	if !ok || responseError.Err == nil {
		return &CallbackAnswerError{Message: err.Error()}
	}
	return &CallbackAnswerError{
		Code:    responseError.Err.Code,
		Message: responseError.Err.Message,
	}
}

func isFloodWait(err error) bool {
	var floodWait *FloodWaitError
	return errors.As(err, &floodWait)
}
