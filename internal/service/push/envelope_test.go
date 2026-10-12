package push

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnvelope_MarshalCanonicalEvents(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		event     Event
		text      string
		wantEvent string
	}{
		{name: "new", event: EventNew, text: "new text", wantEvent: "new"},
		{name: "edited", event: EventEdited, text: "full edited text", wantEvent: "edited"},
		{name: "deleted", event: EventDeleted, text: "", wantEvent: "deleted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := Envelope{
				SchemaVersion: SchemaVersion,
				Event:         tt.event,
				ChatID:        -1001234567890,
				MessageID:     12345,
				OccurredAt:    occurredAt,
				Text:          tt.text,
			}

			data, err := json.Marshal(env)
			require.NoError(t, err)

			var got map[string]any
			require.NoError(t, json.Unmarshal(data, &got))
			require.Equal(t, float64(1), got["schema_version"])
			require.Equal(t, tt.wantEvent, got["event"])
			require.Equal(t, float64(-1001234567890), got["chat_id"])
			require.Equal(t, float64(12345), got["message_id"])
			require.Equal(t, "2026-10-11T12:00:00Z", got["occurred_at"])
			require.Equal(t, tt.text, got["text"])
			require.Len(t, got, 6)
		})
	}
}

func TestEnvelope_DeletedTextIsEmpty(t *testing.T) {
	t.Parallel()

	env := Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventDeleted,
		ChatID:        -1001,
		MessageID:     42,
		OccurredAt:    time.Unix(1, 0).UTC(),
	}

	data, err := json.Marshal(env)
	require.NoError(t, err)

	var got struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(data, &got))
	require.Empty(t, got.Text)
}

func TestEnvelope_HasNoRawPayloadField(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Envelope{})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotContains(t, got, "raw")
	require.NotContains(t, got, "payload")
	require.NotContains(t, got, "tdlib")
}

func TestSinkContract(t *testing.T) {
	t.Parallel()

	var sink Sink = sinkFunc(func(context.Context, Envelope) error {
		return nil
	})
	require.NoError(t, sink.Push(context.Background(), Envelope{}))
}

type sinkFunc func(context.Context, Envelope) error

func (f sinkFunc) Push(ctx context.Context, env Envelope) error {
	return f(ctx, env)
}
