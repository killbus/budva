// Package push defines the canonical external-delivery contract.
package push

import "github.com/pure-golang/budva-claude/internal/service/push/event"

// SchemaVersion is the current canonical envelope schema.
const SchemaVersion = event.SchemaVersion

// Event identifies the source-side message fact represented by an envelope.
type Event = event.Event

const (
	EventNew     = event.EventNew
	EventEdited  = event.EventEdited
	EventDeleted = event.EventDeleted
)

// Envelope is the canonical, product-neutral payload delivered to every sink.
// Deleted events carry an empty Text value.
type Envelope = event.Envelope

// Sink performs one delivery attempt for an accepted envelope.
type Sink = event.Sink

// RenderedEnvelope carries the v1 envelope and the optional v2 projection
// from the source boundary to the endpoint-selected renderer.
type RenderedEnvelope = event.RenderedEnvelope
