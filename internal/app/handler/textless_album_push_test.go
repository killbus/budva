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

// textlessAlbumFixture wires one captionless, protected album member through
// the real NEW-path flow (OnNewMessage ingress -> album collection -> the
// quiet-period member-loop submission) against two real webhook endpoints (v1
// default wire, v2 selected). The rule is webhook-only (PushTo only, no To,
// no SendCopy) so no Telegram copy/forward work is possible, and the strict
// zero-expectation Telegram/State/RateLimiter/Transform mocks fail on any such
// work, enforcing Telegram invariance. The album mocks are bound to the
// desired end state: the captionless member is the FIRST member to arrive
// (AddMessage -> true), the quiet period is satisfied without a real sleep,
// and the member is popped for the v2-only projection.
func textlessAlbumFixture(t *testing.T, v1URL string, v1Client *http.Client, v2Sink *push.WebhookSink) (*Service, *queue.Repo, *push.Generation, *client.Message) {
	t.Helper()
	state, message, filter, album, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), mocks.NewAlbumService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), album, q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"v1", "v2"}}
	rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"},
		UniqueSources: map[int64]struct{}{100: {}}}
	g := push.NewGeneration(map[string]push.EndpointConfig{
		"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1URL, Client: v1Client}), BufferCapacity: 8, Timeout: time.Second},
		"v2": {Sink: v2Sink, BufferCapacity: 8, Timeout: time.Second},
	})
	h.SetRuleSetWithPush(rs, g)

	// A captionless photo whose sizes carry a real file identifier: the
	// projection must never surface the file reference. The member is protected
	// (CanBeSaved == false) to prove eligibility is independent of the forward
	// guard, and carries MediaAlbumId so it takes the album collection path.
	msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800, CanBeSaved: false, MediaAlbumId: 42,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "photo-file-id-42"}},
			}}},
		}}
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	// GetFormattedText is read at ingress (nil, so the member is not a
	// single-media textless fact) and again in the member submit loop (nil, so
	// it is projected as a v2-only content fact rather than dropped).
	message.EXPECT().GetFormattedText(msg).Return(nil).Times(2)
	// The member is the first to arrive, so it triggers the album task.
	album.EXPECT().AddMessage(domain.MediaAlbumKey("r:42"), msg).Return(true).Once()
	// The quiet period is already satisfied: LastReceivedAge >= 3s so the loop
	// breaks on its first check without sleeping.
	album.EXPECT().LastReceivedAge(domain.MediaAlbumKey("r:42")).Return(3 * time.Second).Once()
	album.EXPECT().PopMessages(domain.MediaAlbumKey("r:42")).Return([]*client.Message{msg}).Once()
	return h, q, g, msg
}

// RED: a captionless album member (Content != nil, MediaAlbumId != 0, nil
// formatted text) is pushed only as the canonical v2 `new` content fact after
// the quiet period completes. The v2 endpoint receives one content fact with
// no text and no file reference; the v1 endpoint stays silent (a captionless
// member has no v1 representation, so the v2-only carrier keeps the v1
// endpoint silent at the delivery boundary). The member enters album
// collection as the FIRST member and is delivered through the same quiet-
// period member loop as captioned members, so the album's v2 projection is
// complete. No Telegram copy/forward, state or limiter work runs, and the
// task queue converges.
func TestTextlessAlbumMember_PushesV2FactOnlyV1Silent(t *testing.T) {
	// Real TLS servers run outside synctest (repo convention: pure transports
	// inside synctest, real servers outside). The single album member is
	// submitted from a queue task; q.ProcessAll() runs it synchronously, and
	// Retire() joins the sink workers before the body channels are drained.
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

	h, q, g, msg := textlessAlbumFixture(t, v1Server.URL, v1Server.Client(),
		push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Client: v2Server.Client(), Schema: push.DeliverySchemaV2}))

	h.OnNewMessage(context.Background(), msg)
	// Ingress adds the member to album collection and, as the first member,
	// enqueues the album task. The v2 fact is submitted only after collection,
	// inside that task, so it is not emitted at ingress.
	require.Equal(t, 1, q.Len(), "a captionless album member must enqueue the album collection task, not emit at ingress")
	q.ProcessAll()
	require.Zero(t, q.Len(), "the album task must converge and enqueue no Telegram work")
	g.Retire()

	// Exactly one v2 content fact is delivered; the v1 endpoint stays silent.
	require.Len(t, v2Bodies, 1)
	require.Empty(t, v1Bodies)

	body := <-v2Bodies
	var v2 v2Probe
	require.NoError(t, json.Unmarshal(body, &v2))
	require.Equal(t, 2, v2.SchemaVersion)
	require.Equal(t, "new", v2.Event)
	require.Equal(t, float64(100), v2.Message["chat_id"])
	require.Equal(t, float64(1), v2.Message["message_id"])
	// The fact identifies its album so the v2 projection is the full album.
	require.Equal(t, float64(42), v2.Message["media_album_id"])
	content, ok := v2.Message["content"].(map[string]any)
	require.True(t, ok, "a captionless album member must still carry a content fact")
	require.Equal(t, "messagePhoto", content["type"])
	require.NotContains(t, content, "formatted_text", "a captionless photo has no text fact")

	// The wire is a content fact, not a file: the projection never surfaces the
	// photo's file identifier, so no file or network reference leaks out.
	require.NotContains(t, string(body), "photo-file-id-42")
	require.NotContains(t, string(body), "file_id")
}
