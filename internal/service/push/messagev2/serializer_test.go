package messagev2

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ts(t *testing.T, layout string, value string) time.Time {
	t.Helper()
	got, err := time.Parse(layout, value)
	require.NoError(t, err)
	return got
}

func wantJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var want map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &want), "frozen fixture must be valid JSON: %s", raw)
	return want
}

func TestMarshalNotification_FrozenFixtures(t *testing.T) {
	t.Parallel()

	newText := wantJSON(t, `{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sender":{"type":"messageSenderUser","user_id":42},"sent_at":"2026-10-13T07:59:00Z","content":{"type":"messageText","formatted_text":{"text":"hello","entities":[{"offset":0,"length":5,"type":"textEntityTypeBold"}]}},"has_sensitive_content":false}}`)

	editedPhoto := wantJSON(t, `{"schema_version":2,"event":"edited","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","edited_at":"2026-10-13T07:59:30Z","content":{"type":"messagePhoto","formatted_text":{"text":"caption","entities":[]},"show_caption_above_media":true,"has_spoiler":true},"media_album_id":1234,"has_sensitive_content":false}}`)

	deleted := wantJSON(t, `{"schema_version":2,"event":"deleted","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"delete":{"is_permanent":true,"from_cache":false}}}`)

	unknown := wantJSON(t, `{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","content":{"type":"unknown","source_type":"futureMessageType"},"has_sensitive_content":false}}`)

	int64Ptr := func(v int64) *int64 { return &v }

	tests := []struct {
		name string
		in   *Notification
		want map[string]any
	}{
		{
			name: "new text",
			in: &Notification{
				Event:      EventNew,
				OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
				Message: &Message{
					ChatID: -100123, MessageID: 456,
					Sender: &Sender{Type: "messageSenderUser", UserID: int64Ptr(42)},
					SentAt: &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
					Content: &Content{
						Type: "messageText",
						FormattedText: &FormattedText{
							Text: "hello",
							Entities: []Entity{
								{Offset: 0, Length: 5, Type: "textEntityTypeBold"},
							},
						},
					},
				},
			},
			want: newText,
		},
		{
			name: "edited photo caption",
			in: &Notification{
				Event:      EventEdited,
				OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
				Message: &Message{
					ChatID: -100123, MessageID: 456,
					SentAt:       &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
					EditedAt:     &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:30Z")}[0],
					MediaAlbumID: 1234,
					Content: &Content{
						Type:          "messagePhoto",
						FormattedText: &FormattedText{Text: "caption", Entities: []Entity{}},
						Flags: &ContentFlags{
							ShowCaptionAboveMedia: boolPtr(true),
							HasSpoiler:            boolPtr(true),
						},
					},
				},
			},
			want: editedPhoto,
		},
		{
			name: "delete",
			in: &Notification{
				Event:      EventDeleted,
				OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
				Message: &Message{
					ChatID: -100123, MessageID: 456,
					Delete: &Delete{IsPermanent: true, FromCache: false},
				},
			},
			want: deleted,
		},
		{
			name: "unknown content",
			in: &Notification{
				Event:      EventNew,
				OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
				Message: &Message{
					ChatID: -100123, MessageID: 456,
					SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
					Content: &Content{Type: UnknownContentType, SourceType: "futureMessageType"},
				},
			},
			want: unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := MarshalNotification(tt.in)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(data, &got))
			require.Equal(t, tt.want, got)
		})
	}
}

func boolPtr(v bool) *bool { return &v }

func int64p(v int64) *int64 { return &v }

func TestMarshalNotification_OmissionSemantics(t *testing.T) {
	t.Parallel()

	in := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: "messageText", FormattedText: &FormattedText{Text: "x", Entities: []Entity{}}},
		},
	}

	data, err := MarshalNotification(in)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	msg := got["message"].(map[string]any)

	// Always-present snapshot facts.
	for _, key := range []string{"chat_id", "message_id", "sent_at", "content", "has_sensitive_content"} {
		_, ok := msg[key]
		require.True(t, ok, "required key %s must be present", key)
	}

	// Zero-valued or nil source facts stay absent, never epoch/empty.
	for _, key := range []string{"sender", "edited_at", "reply_to", "message_thread_id",
		"saved_messages_topic_id", "media_album_id", "via_bot_user_id",
		"sender_business_bot_user_id", "author_signature", "restriction_reason",
		"forward", "delete"} {
		_, ok := msg[key]
		require.False(t, ok, "optional key %s must be omitted", key)
	}
	require.Equal(t, false, msg["has_sensitive_content"])

	// Unknown content without a source discriminator omits source_type.
	unknown, err := MarshalNotification(&Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: UnknownContentType},
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"unknown"}`, string(mustUnmarshalMessagePart(t, unknown, "content")))

	// Non-UTC source times are projected as UTC RFC 3339.
	local := time.Date(2026, 10, 13, 3, 59, 0, 0, time.FixedZone("EST", -5*3600))
	localData, err := MarshalNotification(&Notification{
		Event:      EventNew,
		OccurredAt: local,
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &local,
			Content: &Content{Type: "messageText", FormattedText: &FormattedText{Text: "x", Entities: []Entity{}}},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(localData), `"sent_at":"2026-10-13T08:59:00Z"`)
}

func TestMarshalNotification_ShallowContext(t *testing.T) {
	t.Parallel()

	in := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: "messagePhoto", FormattedText: &FormattedText{Text: "c", Entities: []Entity{}}},
			ReplyTo: &ReplyTo{Type: "messageReplyToMessage", ChatID: -100123, MessageID: 400},
			Forward: &Forward{
				Origin: &Origin{
					Type:            "messageOriginChannel",
					ChatID:          int64p(-100500),
					MessageID:       int64p(20),
					AuthorSignature: "author",
				},
				SentAt: &[]time.Time{ts(t, time.RFC3339, "2026-10-12T10:00:00Z")}[0],
				Source: &ForwardSource{
					ChatID:     -100500,
					MessageID:  20,
					Sender:     &Sender{Type: "messageSenderChat", ChatID: int64p(-100500)},
					SenderName: "hidden",
					SentAt:     &[]time.Time{ts(t, time.RFC3339, "2026-10-12T10:00:00Z")}[0],
				},
				PublicServiceAnnouncementType: "type",
			},
		},
	}

	data, err := MarshalNotification(in)
	require.NoError(t, err)

	var got struct {
		Message struct {
			ReplyTo struct {
				Type      string `json:"type"`
				ChatID    int64  `json:"chat_id"`
				MessageID int64  `json:"message_id"`
			} `json:"reply_to"`
			Forward struct {
				Origin struct {
					Type            string `json:"type"`
					ChatID          int64  `json:"chat_id"`
					MessageID       int64  `json:"message_id"`
					AuthorSignature string `json:"author_signature"`
				} `json:"origin"`
				SentAt string `json:"sent_at"`
				Source struct {
					ChatID    int64 `json:"chat_id"`
					MessageID int64 `json:"message_id"`
					Sender    struct {
						Type   string `json:"type"`
						ChatID int64  `json:"chat_id"`
					} `json:"sender"`
					SenderName string `json:"sender_name"`
					SentAt     string `json:"sent_at"`
				} `json:"source"`
				PublicServiceAnnouncementType string `json:"public_service_announcement_type"`
			} `json:"forward"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, "messageReplyToMessage", got.Message.ReplyTo.Type)
	require.Equal(t, int64(-100123), got.Message.ReplyTo.ChatID)
	require.Equal(t, int64(400), got.Message.ReplyTo.MessageID)
	require.Equal(t, "messageOriginChannel", got.Message.Forward.Origin.Type)
	require.Equal(t, int64(-100500), got.Message.Forward.Origin.ChatID)
	require.Equal(t, int64(20), got.Message.Forward.Origin.MessageID)
	require.Equal(t, "author", got.Message.Forward.Origin.AuthorSignature)
	require.Equal(t, "2026-10-12T10:00:00Z", got.Message.Forward.SentAt)
	require.Equal(t, int64(-100500), got.Message.Forward.Source.ChatID)
	require.Equal(t, "messageSenderChat", got.Message.Forward.Source.Sender.Type)
	require.Equal(t, "hidden", got.Message.Forward.Source.SenderName)
	require.Equal(t, "type", got.Message.Forward.PublicServiceAnnouncementType)
}

func TestMarshalNotification_UnsupportedReplyDiscriminatorOnly(t *testing.T) {
	t.Parallel()

	in := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: "messageText", FormattedText: &FormattedText{Text: "x", Entities: []Entity{}}},
			ReplyTo: &ReplyTo{Type: "messageReplyToStory"},
		},
	}
	data, err := MarshalNotification(in)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"messageReplyToStory"}`,
		string(mustUnmarshalMessagePart(t, data, "reply_to")))
}

func mustUnmarshalMessagePart(t *testing.T, data []byte, key string) []byte {
	t.Helper()
	var got struct {
		Message map[string]json.RawMessage `json:"message"`
	}
	require.NoError(t, json.Unmarshal(data, &got))
	raw, ok := got.Message[key]
	require.True(t, ok, "message.%s present", key)
	return raw
}

func TestMarshalNotification_UTF16OffsetsAndBoundedDiscriminators(t *testing.T) {
	t.Parallel()

	// Offsets and lengths are copied verbatim from TDLib UTF-16 code units:
	// no conversion, no clamping.
	in := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt: &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{
				Type: "messageText",
				FormattedText: &FormattedText{
					Text: "\U0001F600",
					Entities: []Entity{
						{Offset: 65535, Length: 2, Type: "textEntityTypeEmoji"},
					},
				},
			},
		},
	}
	data, err := MarshalNotification(in)
	require.NoError(t, err)
	require.Contains(t, string(data), `"offset":65535`)
	require.Contains(t, string(data), `"length":2`)

	// A discriminator at the frozen 128-byte bound is accepted.
	exact := strings.Repeat("a", 128)
	ok := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: UnknownContentType, SourceType: exact},
		},
	}
	_, err = MarshalNotification(ok)
	require.NoError(t, err)

	// One byte over the bound is rejected.
	tooLong := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: UnknownContentType, SourceType: exact + "x"},
		},
	}
	_, err = MarshalNotification(tooLong)
	require.ErrorIs(t, err, ErrInvalidNotification)
}

func TestMarshalNotification_RejectsRawOrUnsafeContent(t *testing.T) {
	t.Parallel()

	makeUnknown := func(sourceType string) *Notification {
		return &Notification{
			Event:      EventNew,
			OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
			Message: &Message{
				ChatID: -100123, MessageID: 456,
				SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
				Content: &Content{Type: UnknownContentType, SourceType: sourceType},
			},
		}
	}

	// Control characters and non-ASCII payloads are never retained.
	for _, bad := range []string{
		"message\nContent",
		"message/content",
		`{"raw":"json"}`,
		"\u043a\u043b\u044e\u0447",
	} {
		_, err := MarshalNotification(makeUnknown(bad))
		require.ErrorIs(t, err, ErrInvalidNotification, "source_type %q must be rejected", bad)
	}

	// A known content type must not carry source_type.
	known := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: "messagePhoto", SourceType: "leak"},
		},
	}
	_, err := MarshalNotification(known)
	require.ErrorIs(t, err, ErrInvalidNotification)

	// Negative entity offsets/lengths and malformed senders are rejected.
	badEntity := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt: &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{
				Type: "messageText",
				FormattedText: &FormattedText{
					Text:     "x",
					Entities: []Entity{{Offset: -1, Length: 1, Type: "textEntityTypeBold"}},
				},
			},
		},
	}
	_, err = MarshalNotification(badEntity)
	require.ErrorIs(t, err, ErrInvalidNotification)

	senderMismatch := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Sender:  &Sender{Type: "messageSenderUser", ChatID: int64p(-1)},
			Content: &Content{Type: "messageText", FormattedText: &FormattedText{Text: "x", Entities: []Entity{}}},
		},
	}
	_, err = MarshalNotification(senderMismatch)
	require.ErrorIs(t, err, ErrInvalidNotification)

	// Unsupported sender discriminators are rejected rather than emitted.
	unsupportedSender := *senderMismatch
	unsupportedSender.Message.Sender = &Sender{Type: "messageSenderOther"}
	_, err = MarshalNotification(&unsupportedSender)
	require.ErrorIs(t, err, ErrInvalidNotification)
}

func TestMarshalNotification_Validation(t *testing.T) {
	t.Parallel()

	base := func() *Notification {
		return &Notification{
			Event:      EventNew,
			OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
			Message: &Message{
				ChatID: -100123, MessageID: 456,
				SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
				Content: &Content{Type: "messageText", FormattedText: &FormattedText{Text: "x", Entities: []Entity{}}},
			},
		}
	}

	invalid := []struct {
		name string
		mut  func(*Notification)
	}{
		{"nil notification", func(n *Notification) { *n = Notification{}; *n = *base(); n.Message = nil }},
		{"unknown event", func(n *Notification) { n.Event = Event("bogus") }},
		{"zero occurred_at", func(n *Notification) { n.OccurredAt = time.Time{} }},
		{"zero chat_id", func(n *Notification) { n.Message.ChatID = 0 }},
		{"zero message_id", func(n *Notification) { n.Message.MessageID = 0 }},
		{"snapshot without sent_at", func(n *Notification) { n.Message.SentAt = nil }},
		{"snapshot without content", func(n *Notification) { n.Message.Content = nil }},
		{"snapshot carries delete facts", func(n *Notification) { n.Message.Delete = &Delete{} }},
		{"deleted without delete facts", func(n *Notification) {
			n.Event = EventDeleted
			n.Message.SentAt = nil
			n.Message.Content = nil
		}},
		{"deleted carries snapshot content", func(n *Notification) {
			n.Event = EventDeleted
			n.Message.Delete = &Delete{IsPermanent: true}
			n.Message.Content = &Content{Type: "messageText"}
		}},
		{"deleted carries sensitive flag", func(n *Notification) {
			n.Event = EventDeleted
			n.Message.SentAt = nil
			n.Message.Content = nil
			n.Message.Delete = &Delete{IsPermanent: true}
			n.Message.HasSensitiveContent = true
		}},
		{"nil entity type", func(n *Notification) {
			n.Message.Content.FormattedText.Entities = []Entity{{Type: ""}}
		}},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := base()
			tt.mut(n)
			_, err := MarshalNotification(n)
			require.ErrorIs(t, err, ErrInvalidNotification)
		})
	}

	// Every error is bounded: it wraps ErrInvalidNotification and never
	// echoes raw source data.
	_, err := MarshalNotification(nil)
	require.ErrorIs(t, err, ErrInvalidNotification)
	require.True(t, errors.Is(err, ErrInvalidNotification))
}

func TestMarshalNotification_NoForbiddentransportLeak(t *testing.T) {
	t.Parallel()

	// The rendered wire JSON must never contain raw TDLib, resource,
	// credential, URL or delivery fields for any projected facts.
	in := &Notification{
		Event:      EventNew,
		OccurredAt: ts(t, time.RFC3339, "2026-10-13T08:00:00Z"),
		Message: &Message{
			ChatID: -100123, MessageID: 456,
			Sender:  &Sender{Type: "messageSenderUser", UserID: int64p(42)},
			SentAt:  &[]time.Time{ts(t, time.RFC3339, "2026-10-13T07:59:00Z")}[0],
			Content: &Content{Type: "messagePhoto", FormattedText: &FormattedText{Text: "caption", Entities: []Entity{}}},
			Forward: &Forward{
				Origin: &Origin{Type: "messageOriginUser", SenderUserID: int64p(7)},
				Source: &ForwardSource{ChatID: -100500, MessageID: 20, SenderName: "hidden"},
			},
		},
	}
	data, err := MarshalNotification(in)
	require.NoError(t, err)

	// The absence set must hold on the frozen fixtures too.
	for _, raw := range []string{
		`{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sender":{"type":"messageSenderUser","user_id":42},"sent_at":"2026-10-13T07:59:00Z","content":{"type":"messageText","formatted_text":{"text":"hello","entities":[{"offset":0,"length":5,"type":"textEntityTypeBold"}]}},"has_sensitive_content":false}}`,
		`{"schema_version":2,"event":"edited","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","edited_at":"2026-10-13T07:59:30Z","content":{"type":"messagePhoto","formatted_text":{"text":"caption","entities":[]},"show_caption_above_media":true,"has_spoiler":true},"media_album_id":1234,"has_sensitive_content":false}}`,
		`{"schema_version":2,"event":"deleted","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"delete":{"is_permanent":true,"from_cache":false}}}`,
		`{"schema_version":2,"event":"new","occurred_at":"2026-10-13T08:00:00Z","message":{"chat_id":-100123,"message_id":456,"sent_at":"2026-10-13T07:59:00Z","content":{"type":"unknown","source_type":"futureMessageType"},"has_sensitive_content":false}}`,
	} {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
		rendered, err := json.Marshal(decoded)
		require.NoError(t, err)
		assertWireClean(t, rendered)
	}
	assertWireClean(t, data)
}

// assertWireClean fails when the rendered v2 wire JSON contains raw TDLib,
// resource, credential, URL, path or delivery-state tokens.
func assertWireClean(t *testing.T, data []byte) {
	t.Helper()
	for _, forbidden := range []string{
		`"@type"`, `"file_id"`, `"path"`, `"http://"`, `"https://"`,
		"Authorization", "Bearer", "token", "secret",
		"relation", "delivery", "queue", "thumbnail", "cover",
		"cache_state", "download_url", "file_token",
	} {
		require.NotContains(t, string(data), forbidden, "forbidden token %s in v2 wire JSON", forbidden)
	}
}
