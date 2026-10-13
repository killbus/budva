package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

// fakeFutureContent is a runtime content discriminator outside the pinned
// M03 inventory; it proves unknown content stays bounded without raw data.
type fakeFutureContent struct{}

func (fakeFutureContent) MessageContentType() string { return "futureMessageType" }

// RED: the source projection must emit bounded v2 facts only: pinned
// discriminators, bounded entity types, unknown content without raw TDLib
// data, and delete facts without synthesized snapshot facts.
func TestV2Projection_BoundedFacts(t *testing.T) {
	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)

	msg := &client.Message{
		ChatId:   100,
		Id:       1,
		Date:     1791868800,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessageText{
			Text: &client.FormattedText{
				Text: "hello",
				Entities: []*client.TextEntity{
					{Offset: 0, Length: 5, Type: &client.TextEntityTypeBold{}},
					{Offset: 5, Length: 5, Type: &client.TextEntityTypeUrl{}},
				},
			},
		},
		ReplyTo:         &client.MessageReplyToMessage{ChatId: 100, MessageId: 9},
		MessageThreadId: 7,
		MediaAlbumId:    1234,
		ForwardInfo: &client.MessageForwardInfo{
			Origin: &client.MessageOriginUser{SenderUserId: 42},
			Date:   1791868700,
			Source: &client.ForwardSource{
				ChatId:     100,
				MessageId:  8,
				SenderId:   &client.MessageSenderUser{UserId: 42},
				SenderName: "src",
				Date:       1791868700,
			},
		},
	}

	n := v2SnapshotFor(msg, messagev2.EventNew, occurredAt)
	require.NotNil(t, n)
	require.Equal(t, messagev2.SchemaVersion, n.SchemaVersion)
	require.Equal(t, messagev2.EventNew, n.Event)

	m := n.Message
	require.Equal(t, int64(100), m.ChatID)
	require.Equal(t, int64(1), m.MessageID)
	require.NotNil(t, m.SentAt)
	require.Nil(t, m.EditedAt)
	require.Nil(t, m.Delete)
	require.Equal(t, client.TypeMessageSenderUser, m.Sender.Type)
	require.NotNil(t, m.Sender.UserID)
	require.Equal(t, int64(42), *m.Sender.UserID)

	require.Equal(t, client.TypeMessageText, m.Content.Type)
	require.Empty(t, m.Content.SourceType)
	require.NotNil(t, m.Content.FormattedText)
	require.Equal(t, "hello", m.Content.FormattedText.Text)
	require.Len(t, m.Content.FormattedText.Entities, 2)
	require.Equal(t, client.TypeTextEntityTypeBold, m.Content.FormattedText.Entities[0].Type)
	require.Equal(t, client.TypeTextEntityTypeUrl, m.Content.FormattedText.Entities[1].Type)

	require.NotNil(t, m.ReplyTo)
	require.Equal(t, client.TypeMessageReplyToMessage, m.ReplyTo.Type)
	require.Equal(t, int64(9), m.ReplyTo.MessageID)
	require.Equal(t, int64(7), m.MessageThreadID)
	require.Equal(t, int64(1234), m.MediaAlbumID)
	require.NotNil(t, m.Forward)
	require.NotNil(t, m.Forward.Origin)
	require.Equal(t, client.TypeMessageOriginUser, m.Forward.Origin.Type)
	require.NotNil(t, m.Forward.Source)
	require.Equal(t, "src", m.Forward.Source.SenderName)

	// Serialized bytes are the frozen wire contract: no raw TDLib data.
	body, err := messagev2.MarshalNotification(n)
	require.NoError(t, err)
	require.Contains(t, string(body), `"schema_version":2`)
	require.Contains(t, string(body), `"event":"new"`)
	require.NotContains(t, string(body), "link_preview")
}

func TestV2Projection_UnknownContentStaysBounded(t *testing.T) {
	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)

	msg := &client.Message{
		ChatId:   100,
		Id:       1,
		Date:     1791868800,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content:  fakeFutureContent{},
	}

	n := v2SnapshotFor(msg, messagev2.EventNew, occurredAt)
	require.NotNil(t, n)
	require.Equal(t, messagev2.UnknownContentType, n.Message.Content.Type)
	require.Equal(t, "futureMessageType", n.Message.Content.SourceType)

	body, err := messagev2.MarshalNotification(n)
	require.NoError(t, err)
	require.Contains(t, string(body), `"source_type":"futureMessageType"`)
}

func TestV2Projection_DeleteCarriesOnlyDeleteFacts(t *testing.T) {
	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)

	n := v2DeleteFor(100, 5, true, false, occurredAt)
	require.NotNil(t, n)
	require.Equal(t, messagev2.EventDeleted, n.Event)
	require.Equal(t, int64(100), n.Message.ChatID)
	require.Equal(t, int64(5), n.Message.MessageID)
	require.NotNil(t, n.Message.Delete)
	require.True(t, n.Message.Delete.IsPermanent)
	require.False(t, n.Message.Delete.FromCache)

	body, err := messagev2.MarshalNotification(n)
	require.NoError(t, err)
	require.Contains(t, string(body), `"event":"deleted"`)
	// JSON object keys sort alphabetically on the wire, so assert each
	// delete fact semantically instead of the frozen document's key order.
	require.Contains(t, string(body), `"delete":{`)
	require.Contains(t, string(body), `"is_permanent":true`)
	require.Contains(t, string(body), `"from_cache":false`)
	require.NotContains(t, string(body), "sent_at")
	require.NotContains(t, string(body), "content")
}
