// Package event defines the transport-independent external-delivery contract.
package event

import (
	"context"
	"time"

	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
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

// Sink performs one bounded delivery attempt for one rendered envelope; the
// concrete representation is chosen by the endpoint at render time.
type Sink interface {
	Push(context.Context, RenderedEnvelope) error
}

// RenderedEnvelope pairs the v1 source-fact envelope with the optional
// Message-only v2 projection. It is the delivery-boundary carrier: endpoint
// schema selection happens only where the payload is rendered, never in the
// source handler.
type RenderedEnvelope struct {
	// Envelope is the v1 representation; it stays the default rendering.
	Envelope Envelope
	// V2 is nil when the source projection could not form a valid v2
	// notification. A v2-only endpoint then fails its submission explicitly
	// instead of falling back to v1 or raw source data.
	V2 *messagev2.Notification
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
	// SubmitV2 enqueues both representations; each endpoint renders the one
	// its selected schemaVersion requests.
	SubmitV2([]string, Envelope, *messagev2.Notification) SubmitResults
	// SubmitV2Only enqueues a carrier with no v1 representation. It is the
	// webhook-only path: a v2-selected endpoint renders the fact, a
	// v1-selected endpoint reports the fact unavailable and sends nothing
	// instead of emitting an empty or text-only envelope.
	SubmitV2Only([]string, *messagev2.Notification) SubmitResults
}
