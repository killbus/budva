// Package event defines the transport-independent external-delivery contract.
package event

import (
	"context"
	"time"
)

const SchemaVersion = 1

type Event string

const (
	EventNew     Event = "new"
	EventEdited  Event = "edited"
	EventDeleted Event = "deleted"
)

// Envelope describes a source fact, never a destination-specific rendering.
type Envelope struct {
	SchemaVersion int       `json:"schema_version"`
	Event         Event     `json:"event"`
	ChatID        int64     `json:"chat_id"`
	MessageID     int64     `json:"message_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	Text          string    `json:"text"`
}

type Sink interface {
	Push(context.Context, Envelope) error
}

type SubmitResult string

const (
	SubmitAccepted           SubmitResult = "accepted"
	SubmitRejectedBufferFull SubmitResult = "buffer_full"
	SubmitRejectedRetired    SubmitResult = "retired"
	SubmitRejectedUnknown    SubmitResult = "unknown_endpoint"
)

type SubmitResults map[string]SubmitResult

// Submitter performs only non-blocking enqueue attempts against fixed routes.
type Submitter interface {
	Submit([]string, Envelope) SubmitResults
}
