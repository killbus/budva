package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/service/push"
	"github.com/pure-golang/budva-claude/internal/service/push/event"
)

// reloadTextlessService builds the handler around a webhook-only rule (no
// SendCopy/To destinations) pushing to the supplied endpoint names on the
// active generation, mirroring protectedTextlessService. Strict mocks bind the
// only two message-service reads the handler performs before the textless
// branch (IsSystemMessage -> false, GetFormattedText -> nil) and a strict
// TelegramRepo, StateRepo, RateLimiter, TransformService and AlbumService with
// zero expectations, so any copy/forward, state or limiter work fails.
func reloadTextlessService(t *testing.T, pushTo []string, submitter event.Submitter) (*Service, *queue.Repo, *client.Message) {
	t.Helper()
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: pushTo}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, submitter)

	msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800, CanBeSaved: false,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "reload-file-id-9"}},
			}}},
		}}
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	message.EXPECT().GetFormattedText(msg).Return(nil).Once()
	return h, q, msg
}

// RED: the reload outcome for the new textless path (immutable captured route).
// The handler reads the ruleset snapshot once at the OnNewMessage entry and
// submits through the submitter captured with that snapshot. A ruleset reload
// (SetRuleSetWithPush) that swaps in a new ruleset and a new (replacement)
// submitter must not reroute a textless message already being processed: its
// v2 fact still goes to the captured (pre-reload) generation, the v1 endpoint
// stays silent, the replacement submitter receives nothing, and the task queue
// stays converged.
func TestTextlessMedia_ReloadKeepsCapturedRoute(t *testing.T) {
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

	// The active (pre-reload) generation. The worker is NOT retired before the
	// message is processed: a live ruleset reload keeps the previous generation
	// draining its captured route, exactly as pushManager.Replace does in
	// production.
	g := push.NewGeneration(map[string]push.EndpointConfig{
		"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1Server.URL, Client: v1Server.Client()}), BufferCapacity: 8, Timeout: time.Second},
		"v2": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Client: v2Server.Client(), Schema: push.DeliverySchemaV2}), BufferCapacity: 8, Timeout: time.Second},
	})
	h, q, msg := reloadTextlessService(t, []string{"v1", "v2"}, g)

	// Process the textless new message. On the webhook-only path this submits
	// synchronously to the captured (pre-reload) generation.
	h.OnNewMessage(context.Background(), msg)
	require.Zero(t, q.Len(), "a textless webhook-only delivery must not enqueue Telegram work")

	// Reload: swap in a new (empty) ruleset and a replacement submitter, then
	// join the captured generation's workers and read the wire.
	replacement := &recordingSubmitter{}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"replacement"}}},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, replacement)
	g.Retire()

	// The already-processed textless message was routed to the captured
	// (pre-reload) generation, not the replacement submitter.
	require.Empty(t, replacement.submissions,
		"the reload replacement submitter must not receive an already-processed textless message")

	// Exactly one v2 fact was delivered on the captured route; the v1 endpoint
	// stays silent.
	require.Len(t, v2Bodies, 1)
	require.Empty(t, v1Bodies)

	body := <-v2Bodies
	var v2 v2Probe
	require.NoError(t, json.Unmarshal(body, &v2))
	require.Equal(t, 2, v2.SchemaVersion)
	require.Equal(t, "new", v2.Event)
	require.Equal(t, float64(100), v2.Message["chat_id"])
	require.Equal(t, float64(1), v2.Message["message_id"])
	content, ok := v2.Message["content"].(map[string]any)
	require.True(t, ok, "the reloaded textless fact must still carry a content fact")
	require.Equal(t, "messagePhoto", content["type"])
	require.NotContains(t, content, "formatted_text", "a captionless photo has no text fact")

	// The wire is a content fact, not a file: the projection never surfaces the
	// photo's file identifier.
	require.NotContains(t, string(body), "reload-file-id-9")
	require.NotContains(t, string(body), "file_id")
}

// RED: a textless new message processed under a captured generation stays
// routed to that generation when the generation is retired (retired outcome),
// not the live ruleset. The captured submitter records the v2-only submission
// and no v1 envelope; the replacement submitter receives nothing; the task
// queue stays converged.
func TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		capture := &captureSubmitter{next: g}
		h, q, msg := reloadTextlessService(t, []string{"endpoint"}, capture)

		h.OnNewMessage(context.Background(), msg)
		require.Zero(t, q.Len(), "a textless webhook-only delivery must not enqueue Telegram work")

		// The captured submission is recorded and, because the generation has
		// not been retired yet, it is accepted.
		require.Len(t, capture.v2s, 1)
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)

		// Retire the captured generation, then reload to a replacement
		// submitter. A second textless message would now be routed to the
		// replacement, but the already-processed fact is the captured one.
		g.Retire()
		replacement := &recordingSubmitter{}
		h.SetRuleSetWithPush(&domain.RuleSet{
			ForwardRules:        map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"replacement"}}},
			OrderedForwardRules: []string{"r"},
			UniqueSources:       map[int64]struct{}{100: {}},
		}, replacement)

		require.Empty(t, replacement.submissions,
			"the replacement submitter must not receive an already-processed textless message")
		// The captured fact is still the only one recorded.
		require.Len(t, capture.v2s, 1)
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
	})
}
