// Package messagev2 defines the canonical Message-only v2 event contract:
// transport-independent Message facts, the frozen v2 wire JSON, and bounded
// unknown-content handling.
//
// The package deliberately has no TDLib dependency. Source projection happens
// at the handler boundary and endpoint version selection happens in the push
// delivery layer; this package only validates and serializes already-projected
// facts. Raw TDLib objects are never accepted or emitted.
package messagev2

import (
	"time"
)

// SchemaVersion is the only schema this package can serialize.
const SchemaVersion = 2

// UnknownContentType is the canonical discriminator for content that is not
// projected as a pinned TDLib content type.
const UnknownContentType = "unknown"

// MaxDiscriminatorBytes is the frozen maximum length in UTF-8 bytes for
// bounded source discriminators: unknown-content source_type and entity types.
const MaxDiscriminatorBytes = 128

// Event identifies the message operation a notification represents.
type Event string

const (
	// EventNew carries the complete current snapshot from the new-message input.
	EventNew Event = "new"
	// EventEdited carries the complete current snapshot from edit processing.
	EventEdited Event = "edited"
	// EventDeleted carries only identities and the delete-update facts.
	EventDeleted Event = "deleted"
)

// Notification is one observed message operation.
type Notification struct {
	SchemaVersion int
	Event         Event
	// OccurredAt is the local observation time captured at source emission.
	// It is not Telegram sent time, edit time or delivery time.
	OccurredAt time.Time
	// Message is required. For new/edited it is a current snapshot; for
	// deleted it holds identities and delete facts only.
	Message *Message
}

// Message is the canonical Message fact snapshot. Facts the source does not
// supply stay absent: zero values are not serialized.
type Message struct {
	ChatID    int64
	MessageID int64

	// Sender is nil when the source supplies no sender.
	Sender *Sender

	// SentAt is required for a snapshot.
	SentAt *time.Time
	// EditedAt is set only when the source supplies an edit time.
	EditedAt *time.Time

	// Content is required for a snapshot. Nil source content is expressed as
	// Content{Type: UnknownContentType}.
	Content *Content

	ReplyTo                 *ReplyTo
	MessageThreadID         int64
	SavedMessagesTopicID    int64
	MediaAlbumID            int64
	ViaBotUserID            int64
	SenderBusinessBotUserID int64
	AuthorSignature         string
	// HasSensitiveContent is a source boolean always emitted for a snapshot.
	HasSensitiveContent bool
	RestrictionReason   string

	Forward *Forward

	// Delete is set only for the deleted event.
	Delete *Delete
}

// Delete carries only the facts supplied by the delete update.
type Delete struct {
	IsPermanent bool
	FromCache   bool
}

// Sender is the bounded source sender representation.
type Sender struct {
	// Type is "messageSenderUser" or "messageSenderChat".
	Type   string
	UserID *int64
	ChatID *int64
}

// Content is the canonical message content projection. No nested TDLib media,
// file, thumbnail, cover, download-state or resource object is represented.
type Content struct {
	// Type is a pinned TDLib content discriminator or UnknownContentType.
	Type string
	// SourceType is set only when Type is UnknownContentType and a bounded
	// source discriminator is available.
	SourceType string

	// FormattedText is set only for text-bearing content.
	FormattedText *FormattedText

	// Flags carries the safe message-level media flags, set only for content
	// types that supply them.
	Flags *ContentFlags
}

// ContentFlags are message-level media flags.
type ContentFlags struct {
	ShowCaptionAboveMedia *bool
	HasSpoiler            *bool
}

// FormattedText carries message text or caption plus entities.
type FormattedText struct {
	Text string
	// Entities is non-nil whenever FormattedText is present; empty is allowed.
	Entities []Entity
}

// Entity is one text entity. Offsets and lengths are TDLib UTF-16 code units
// and are not converted.
type Entity struct {
	Offset int32
	Length int32
	// Type is the bounded TextEntityType discriminator.
	Type string
}

// ReplyTo is the shallow reply context.
type ReplyTo struct {
	// Type is the bounded MessageReplyTo discriminator.
	Type string
	// ChatID and MessageID are meaningful only for messageReplyToMessage.
	ChatID    int64
	MessageID int64
}

// Origin is the sanitized forward origin projection.
type Origin struct {
	Type string
	// messageOriginUser
	SenderUserID *int64
	// messageOriginHiddenUser
	SenderName string
	// messageOriginChat
	SenderChatID *int64
	// messageOriginChat and messageOriginChannel
	AuthorSignature string
	// messageOriginChannel
	ChatID    *int64
	MessageID *int64
}

// ForwardSource is the sanitized forwarded source message facts.
type ForwardSource struct {
	ChatID     int64
	MessageID  int64
	Sender     *Sender
	SenderName string
	SentAt     *time.Time
}

// Forward is the shallow sanitized forward provenance.
type Forward struct {
	Origin                        *Origin
	SentAt                        *time.Time
	Source                        *ForwardSource
	PublicServiceAnnouncementType string
}
