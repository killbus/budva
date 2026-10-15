package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/service/push"
)

// textlessEditFixture wires one captionless source photo through two real
// webhook endpoints (v1 default wire, v2 selected) and returns the handler,
// queue and generation so the caller drives the edit update. Strict mocks bind
// the only two reads the handler performs (GetCopiedMessageIDs,
// GetFormattedText) and a strict TelegramRepo, StateRepo, RateLimiter and
// TransformService with zero expectations fail on any copy/forward, state or
// limiter work, so Telegram invariance under the webhook-only edit path is
// enforced. The rule has no SendCopy/To destinations: delivery is webhook-only.
func textlessEditFixture(t *testing.T, v1URL string, v1Client *http.Client, v2Sink *push.WebhookSink) (*Service, *queue.Repo, *push.Generation, *client.Message) {
	t.Helper()
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"v1", "v2"}}
	rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"},
		UniqueSources: map[int64]struct{}{100: {}}}
	g := push.NewGeneration(map[string]push.EndpointConfig{
		"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1URL, Client: v1Client}), BufferCapacity: 8, Timeout: time.Second},
		"v2": {Sink: v2Sink, BufferCapacity: 8, Timeout: time.Second},
	})
	h.SetRuleSetWithPush(rs, g)

	// A captionless photo whose sizes carry a real file identifier, with both
	// a send and an edit time so the projected snapshot carries sent_at and
	// edited_at. The projection must never surface the file reference.
	msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800, EditDate: 1791868830, CanBeSaved: false,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "photo-file-id-42"}},
			}}},
		}}
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
	message.EXPECT().GetFormattedText(msg).Return(nil).Once()
	return h, q, g, msg
}

// RED: a webhook-only textless media EDIT is pushed only as the canonical v2
// `edited` content fact. The v2 endpoint receives the photo content type with
// an edit time and no text and no file reference; the v1 endpoint stays silent
// (an empty envelope is not an event, so no v1 "edited" empty event is
// fabricated); and no Telegram copy/forward, state or limiter work runs. The
// edit is the full current snapshot; no prior state is read or invented.
func TestTextlessMediaEdit_PushesV2EditedFactOnlyV1Silent(t *testing.T) {
	// Real TLS servers run outside synctest (repo convention: pure transports
	// inside synctest, real servers outside). Retire() joins the sink workers
	// before the body channels are drained.
	v1Bodies := make(chan []byte, 4)
	v1Server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		v1Bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer v1Server.Close()

	v2Bodies := make(chan []byte, 4)
	v2Server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		v2Bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer v2Server.Close()

	h, q, g, msg := textlessEditFixture(t, v1Server.URL, v1Server.Client(),
		push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Client: v2Server.Client(), Schema: push.DeliverySchemaV2}))

	// The edit path is async: OnEditedMessage enqueues editMessagesWithRetry, so
	// drain the task queue synchronously, then join the sink workers before
	// reading the body channels.
	h.OnEditedMessage(context.Background(), msg)
	q.ProcessAll()
	require.Zero(t, q.Len(), "textless webhook-only edit delivery must not re-enqueue Telegram work")
	g.Retire()

	// Exactly one v2 fact is delivered; the v1 endpoint stays silent.
	require.Len(t, v2Bodies, 1)
	require.Empty(t, v1Bodies)

	body := <-v2Bodies
	var v2 v2Probe
	require.NoError(t, json.Unmarshal(body, &v2))
	require.Equal(t, 2, v2.SchemaVersion)
	require.Equal(t, "edited", v2.Event)
	require.Equal(t, float64(100), v2.Message["chat_id"])
	require.Equal(t, float64(1), v2.Message["message_id"])
	require.NotEmpty(t, v2.Message["edited_at"], "an edit fact carries the source edit time")
	content, ok := v2.Message["content"].(map[string]any)
	require.True(t, ok, "textless media edit must still carry a content fact")
	require.Equal(t, "messagePhoto", content["type"])
	require.NotContains(t, content, "formatted_text", "a captionless photo has no text fact")

	// The wire is a content fact, not a file: the projection never surfaces the
	// photo's file identifier, so no file or network reference leaks out.
	require.NotContains(t, string(body), "photo-file-id-42")
	require.NotContains(t, string(body), "file_id")
}
