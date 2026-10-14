// MarshalNotification validates projected Message facts and renders the
// frozen v2 wire JSON. Rendering is deterministic: map keys are sorted by
// encoding/json, so equal facts always produce equal bytes.
package messagev2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidNotification wraps a message explaining why projected facts
// cannot form a valid v2 notification. The serializer never falls back to
// v1 or to raw source data on invalid input.
var ErrInvalidNotification = errors.New("messagev2: invalid notification")

// MaxBoundedStringBytes caps sanitized source strings such as sender names
// and announcement types that are not strict TDLib discriminators.
const MaxBoundedStringBytes = 256

// MarshalNotification validates and renders the frozen v2 wire JSON.
// The result contains no nested TDLib media, file, thumbnail, cover,
// download-state or resource object and never invents absent facts.
func MarshalNotification(n *Notification) ([]byte, error) {
	if err := validateNotification(n); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"schema_version": SchemaVersion,
		"event":          string(n.Event),
		"occurred_at":    rfc3339UTC(n.OccurredAt),
		"message":        n.Message.marshal(),
	})
}

func rfc3339UTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func (m *Message) marshal() map[string]any {
	out := map[string]any{
		"chat_id":    m.ChatID,
		"message_id": m.MessageID,
	}
	if m.Sender != nil {
		out["sender"] = m.Sender.marshal()
	}
	if m.SentAt != nil {
		out["sent_at"] = rfc3339UTC(*m.SentAt)
	}
	if m.EditedAt != nil {
		out["edited_at"] = rfc3339UTC(*m.EditedAt)
	}
	if m.Content != nil {
		out["content"] = m.Content.marshal()
	}
	if m.ReplyTo != nil {
		out["reply_to"] = m.ReplyTo.marshal()
	}
	if m.MessageThreadID != 0 {
		out["message_thread_id"] = m.MessageThreadID
	}
	if m.SavedMessagesTopicID != 0 {
		out["saved_messages_topic_id"] = m.SavedMessagesTopicID
	}
	if m.MediaAlbumID != 0 {
		out["media_album_id"] = m.MediaAlbumID
	}
	if m.ViaBotUserID != 0 {
		out["via_bot_user_id"] = m.ViaBotUserID
	}
	if m.SenderBusinessBotUserID != 0 {
		out["sender_business_bot_user_id"] = m.SenderBusinessBotUserID
	}
	if m.AuthorSignature != "" {
		out["author_signature"] = m.AuthorSignature
	}
	// has_sensitive_content is a snapshot fact: always emitted for
	// new/edited, never synthesized for deleted notifications.
	if m.Delete == nil {
		out["has_sensitive_content"] = m.HasSensitiveContent
	}
	if m.RestrictionReason != "" {
		out["restriction_reason"] = m.RestrictionReason
	}
	if m.Forward != nil {
		out["forward"] = m.Forward.marshal()
	}
	if m.Delete != nil {
		out["delete"] = map[string]any{
			"is_permanent": m.Delete.IsPermanent,
			"from_cache":   m.Delete.FromCache,
		}
	}
	return out
}

func (s *Sender) marshal() map[string]any {
	out := map[string]any{"type": s.Type}
	if s.UserID != nil {
		out["user_id"] = *s.UserID
	}
	if s.ChatID != nil {
		out["chat_id"] = *s.ChatID
	}
	return out
}

func (c *Content) marshal() map[string]any {
	out := map[string]any{"type": c.Type}
	if c.Type == UnknownContentType && c.SourceType != "" {
		out["source_type"] = c.SourceType
	}
	if c.FormattedText != nil {
		out["formatted_text"] = c.FormattedText.marshal()
	}
	if c.Flags != nil {
		out["show_caption_above_media"] = c.Flags.ShowCaptionAboveMedia
		out["has_spoiler"] = c.Flags.HasSpoiler
	}
	return out
}

func (f *FormattedText) marshal() map[string]any {
	entities := make([]any, 0, len(f.Entities))
	for i := range f.Entities {
		entities = append(entities, f.Entities[i].marshal())
	}
	return map[string]any{"text": f.Text, "entities": entities}
}

func (e *Entity) marshal() map[string]any {
	return map[string]any{
		"offset": e.Offset,
		"length": e.Length,
		"type":   e.Type,
	}
}

func (r *ReplyTo) marshal() map[string]any {
	out := map[string]any{"type": r.Type}
	if r.ChatID != 0 {
		out["chat_id"] = r.ChatID
	}
	if r.MessageID != 0 {
		out["message_id"] = r.MessageID
	}
	return out
}

func (o *Origin) marshal() map[string]any {
	out := map[string]any{"type": o.Type}
	if o.SenderUserID != nil {
		out["sender_user_id"] = *o.SenderUserID
	}
	if o.SenderName != "" {
		out["sender_name"] = o.SenderName
	}
	if o.SenderChatID != nil {
		out["sender_chat_id"] = *o.SenderChatID
	}
	if o.AuthorSignature != "" {
		out["author_signature"] = o.AuthorSignature
	}
	if o.ChatID != nil {
		out["chat_id"] = *o.ChatID
	}
	if o.MessageID != nil {
		out["message_id"] = *o.MessageID
	}
	return out
}

func (s *ForwardSource) marshal() map[string]any {
	out := map[string]any{
		"chat_id":    s.ChatID,
		"message_id": s.MessageID,
	}
	if s.Sender != nil {
		out["sender"] = s.Sender.marshal()
	}
	if s.SenderName != "" {
		out["sender_name"] = s.SenderName
	}
	if s.SentAt != nil {
		out["sent_at"] = rfc3339UTC(*s.SentAt)
	}
	return out
}

func (f *Forward) marshal() map[string]any {
	out := map[string]any{}
	if f.Origin != nil {
		out["origin"] = f.Origin.marshal()
	}
	if f.SentAt != nil {
		out["sent_at"] = rfc3339UTC(*f.SentAt)
	}
	if f.Source != nil {
		out["source"] = f.Source.marshal()
	}
	if f.PublicServiceAnnouncementType != "" {
		out["public_service_announcement_type"] = f.PublicServiceAnnouncementType
	}
	return out
}

// validateNotification enforces the frozen wire contract over projected
// facts. Errors name the invalid field so the delivery boundary can log a
// bounded message; raw source objects are never part of the error.
func validateNotification(n *Notification) error {
	if n == nil {
		return fmt.Errorf("%w: nil notification", ErrInvalidNotification)
	}
	switch n.Event {
	case EventNew, EventEdited, EventDeleted:
	default:
		return fmt.Errorf("%w: unknown event %q", ErrInvalidNotification, n.Event)
	}
	if n.Message == nil {
		return fmt.Errorf("%w: nil message", ErrInvalidNotification)
	}
	if n.OccurredAt.IsZero() {
		return fmt.Errorf("%w: zero occurred_at", ErrInvalidNotification)
	}
	return n.Message.validate(n.Event)
}

func (m *Message) validate(ev Event) error {
	if m.ChatID == 0 {
		return fmt.Errorf("%w: zero chat_id", ErrInvalidNotification)
	}
	if m.MessageID == 0 {
		return fmt.Errorf("%w: zero message_id", ErrInvalidNotification)
	}
	if err := validateSender(m.Sender); err != nil {
		return err
	}
	switch ev {
	case EventDeleted:
		// Deleted notifications carry identity and delete facts only.
		if m.Delete == nil {
			return fmt.Errorf("%w: deleted event without delete facts", ErrInvalidNotification)
		}
		if m.Sender != nil || m.SentAt != nil || m.EditedAt != nil || m.Content != nil ||
			m.ReplyTo != nil || m.Forward != nil || m.HasSensitiveContent ||
			m.MessageThreadID != 0 || m.SavedMessagesTopicID != 0 || m.MediaAlbumID != 0 ||
			m.ViaBotUserID != 0 || m.SenderBusinessBotUserID != 0 ||
			m.AuthorSignature != "" || m.RestrictionReason != "" {
			return fmt.Errorf("%w: deleted event carries snapshot facts", ErrInvalidNotification)
		}
	default:
		// new and edited snapshots require identity, sent time and content.
		if m.SentAt == nil {
			return fmt.Errorf("%w: snapshot without sent_at", ErrInvalidNotification)
		}
		if m.Content == nil {
			return fmt.Errorf("%w: snapshot without content", ErrInvalidNotification)
		}
		if m.Delete != nil {
			return fmt.Errorf("%w: snapshot carries delete facts", ErrInvalidNotification)
		}
	}
	return m.validateContext()
}

func (m *Message) validateContext() error {
	if m.SavedMessagesTopicID < 0 || m.MediaAlbumID < 0 || m.ViaBotUserID < 0 ||
		m.SenderBusinessBotUserID < 0 {
		return fmt.Errorf("%w: negative identifier", ErrInvalidNotification)
	}
	if m.ReplyTo != nil {
		if err := ValidateDiscriminator(m.ReplyTo.Type); err != nil {
			return fmt.Errorf("%w: reply_to.type: %v", ErrInvalidNotification, err)
		}
		// Chat IDs are legitimately negative (supergroups, channels);
		// message IDs and thread/topic IDs are not.
		if m.ReplyTo.MessageID < 0 {
			return fmt.Errorf("%w: negative reply_to message_id", ErrInvalidNotification)
		}
	}
	if m.Forward != nil {
		if err := validateForward(m.Forward); err != nil {
			return err
		}
	}
	if m.Content != nil {
		return m.Content.validate()
	}
	return nil
}

func validateSender(s *Sender) error {
	if s == nil {
		return nil
	}
	if err := ValidateDiscriminator(s.Type); err != nil {
		return fmt.Errorf("%w: sender.type: %v", ErrInvalidNotification, err)
	}
	switch s.Type {
	case "messageSenderUser":
		if s.UserID == nil || *s.UserID <= 0 {
			return fmt.Errorf("%w: messageSenderUser requires user_id", ErrInvalidNotification)
		}
		if s.ChatID != nil {
			return fmt.Errorf("%w: messageSenderUser carries chat_id", ErrInvalidNotification)
		}
	case "messageSenderChat":
		if s.ChatID == nil || *s.ChatID == 0 {
			return fmt.Errorf("%w: messageSenderChat requires chat_id", ErrInvalidNotification)
		}
		if s.UserID != nil {
			return fmt.Errorf("%w: messageSenderChat carries user_id", ErrInvalidNotification)
		}
	default:
		return fmt.Errorf("%w: unsupported sender type %q", ErrInvalidNotification, s.Type)
	}
	return nil
}

func (c *Content) validate() error {
	if err := ValidateDiscriminator(c.Type); err != nil {
		return fmt.Errorf("%w: content.type: %v", ErrInvalidNotification, err)
	}
	if c.Type == UnknownContentType {
		if c.SourceType != "" {
			if err := ValidateDiscriminator(c.SourceType); err != nil {
				return fmt.Errorf("%w: content.source_type: %v", ErrInvalidNotification, err)
			}
		}
	} else if c.SourceType != "" {
		return fmt.Errorf("%w: source_type with known content type", ErrInvalidNotification)
	}
	if c.FormattedText != nil {
		if c.FormattedText.Entities == nil {
			return fmt.Errorf("%w: formatted_text requires entities slice", ErrInvalidNotification)
		}
		for i := range c.FormattedText.Entities {
			e := &c.FormattedText.Entities[i]
			if e.Offset < 0 || e.Length < 0 {
				return fmt.Errorf("%w: negative entity offset or length", ErrInvalidNotification)
			}
			if err := ValidateDiscriminator(e.Type); err != nil {
				return fmt.Errorf("%w: entity.type: %v", ErrInvalidNotification, err)
			}
		}
	}
	return nil
}

func validateForward(f *Forward) error {
	if f.Origin != nil {
		if err := ValidateDiscriminator(f.Origin.Type); err != nil {
			return fmt.Errorf("%w: origin.type: %v", ErrInvalidNotification, err)
		}
	}
	if f.Source != nil {
		if f.Source.ChatID == 0 || f.Source.MessageID == 0 {
			return fmt.Errorf("%w: forward.source requires chat_id and message_id", ErrInvalidNotification)
		}
		if err := validateSender(f.Source.Sender); err != nil {
			return err
		}
		if err := ValidateBoundedString(f.Source.SenderName); err != nil {
			return fmt.Errorf("%w: forward.source.sender_name: %v", ErrInvalidNotification, err)
		}
	}
	if f.PublicServiceAnnouncementType != "" {
		if err := ValidateDiscriminator(f.PublicServiceAnnouncementType); err != nil {
			return fmt.Errorf("%w: public_service_announcement_type: %v", ErrInvalidNotification, err)
		}
	}
	return nil
}

// ValidateDiscriminator accepts only bounded TDLib discriminators: ASCII
// letters, digits, dot and hyphen, at most MaxDiscriminatorBytes bytes.
// Source projection uses it before emitting optional bounded strings so an
// out-of-bounds value is omitted instead of failing the whole endpoint.
func ValidateDiscriminator(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("empty discriminator")
	}
	if len(s) > MaxDiscriminatorBytes {
		return fmt.Errorf("discriminator exceeds %d bytes", MaxDiscriminatorBytes)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		allowed := (c >= '0' && c <= '9') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			c == '.' || c == '-'
		if !allowed {
			return fmt.Errorf("discriminator contains invalid byte %#x", c)
		}
	}
	return nil
}

// ValidateBoundedString accepts a sanitized non-empty source string: no
// control characters, at most MaxBoundedStringBytes bytes. Source projection
// uses it before emitting optional bounded strings so an out-of-bounds value
// is omitted instead of failing the whole endpoint.
func ValidateBoundedString(s string) error {
	if s == "" || len(s) > MaxBoundedStringBytes {
		return fmt.Errorf("string exceeds %d bytes or is empty", MaxBoundedStringBytes)
	}
	if strings.ContainsRune(s, '\n') || strings.ContainsRune(s, '\r') || strings.ContainsRune(s, '\t') {
		return fmt.Errorf("string contains control characters")
	}
	return nil
}
