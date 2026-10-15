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
	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

// protectedTextlessMessage is one protected captionless source photo: Telegram
// cannot save or copy it (CanBeSaved is false) and it carries the source
// protected facts (HasSensitiveContent, RestrictionReason). It is still a
// content-bearing textless media message, so the new path must project it as a
// canonical v2 content fact. The projection carries the protected facts
// verbatim and never the photo's file identifier.
func protectedTextlessMessage() *client.Message {
	return &client.Message{ChatId: 100, Id: 1, Date: 1791868800,
		CanBeSaved: false, HasSensitiveContent: true, RestrictionReason: "restricted",
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "protected-file-id-9"}},
			}}},
		}}
}

// protectedTextlessService builds the handler around a webhook-only rule (no
// SendCopy/To destinations) with the supplied submitter. Strict mocks bind the
// only two message-service reads the handler performs before the textless
// branch (IsSystemMessage -> false, GetFormattedText -> nil), and a strict
// TelegramRepo, StateRepo, RateLimiter, TransformService and AlbumService with
// zero expectations fail on any copy/forward, state or limiter work, so
// Telegram invariance on the webhook-only path is enforced.
func protectedTextlessService(t *testing.T, submitter *push.Generation) (*Service, *queue.Repo, *client.Message) {
	t.Helper()
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"v1", "v2"}}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, submitter)

	msg := protectedTextlessMessage()
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	message.EXPECT().GetFormattedText(msg).Return(nil).Once()
	return h, q, msg
}

// RED: a PROTECTED textless media message (Telegram cannot save/copy it, and
// it carries the source protected facts) is still eligible for the webhook-only
// v2 content fact -- eligibility must not inherit the Telegram copy/forward
// guard (CanBeSaved). The v2 endpoint receives the photo content fact with the
// protected facts carried verbatim, no text and no file reference; the v1
// endpoint stays silent (an empty envelope is not an event); and no Telegram
// copy/forward, state or limiter work runs, so the task queue converges to
// zero.
func TestTextlessMediaProtected_PushesV2FactOnlyV1Silent(t *testing.T) {
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

	g := push.NewGeneration(map[string]push.EndpointConfig{
		"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1Server.URL, Client: v1Server.Client()}), BufferCapacity: 8, Timeout: time.Second},
		"v2": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Client: v2Server.Client(), Schema: push.DeliverySchemaV2}), BufferCapacity: 8, Timeout: time.Second},
	})
	h, q, msg := protectedTextlessService(t, g)

	h.OnNewMessage(context.Background(), msg)
	require.Zero(t, q.Len(), "a protected textless webhook-only delivery must not enqueue Telegram work")
	g.Retire()

	// Exactly one v2 fact is delivered; the v1 endpoint stays silent.
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
	require.True(t, ok, "a protected textless media message must still carry a content fact")
	require.Equal(t, "messagePhoto", content["type"])
	require.NotContains(t, content, "formatted_text", "a captionless photo has no text fact")

	// The source protected facts are carried verbatim by the projection, not
	// dropped because the message cannot be saved or copied.
	require.Equal(t, true, v2.Message["has_sensitive_content"], "the source sensitive flag must be carried")
	require.Equal(t, "restricted", v2.Message["restriction_reason"], "the source restriction reason must be carried")

	// The wire is a content fact, not a file: the projection never surfaces the
	// photo's file identifier, so no file or network reference leaks out.
	require.NotContains(t, string(body), "protected-file-id-9")
	require.NotContains(t, string(body), "file_id")
}

// protectedSnapshot is the handler's v2 projection of one protected textless
// photo, used to prefill a push buffer the way the handler itself would build
// the fact.
func protectedSnapshot() *messagev2.Notification {
	return v2SnapshotFor(protectedTextlessMessage(), messagev2.EventNew, time.Now().UTC())
}

// RED: a protected textless fact whose v2 endpoint buffer is full is still
// built from the source facts and stays v2-only. The endpoint submission is
// rejected as buffer-full (isolated to that endpoint), no v1 fallback body is
// emitted, no Telegram work runs, and the task queue converges to zero.
func TestTextlessMediaProtected_InvariantUnderFullBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// A single v2-selected endpoint backed by a sink that blocks on its
		// (single) buffered item, so the buffer is full by the time the handler
		// submits.
		sink := &outcomeSink{class: push.FailureTimeout}
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: 10 * time.Second},
		})
		defer g.Retire()
		// Prefill twice (album-test convention): the worker takes the first
		// item and blocks in the sink while the buffer holds the second, so the
		// handler's submission finds a full buffer.
		g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())
		synctest.Wait()
		g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())

		capture := &captureSubmitter{next: g}
		h, q, msg := protectedTextlessService(t, capture)

		h.OnNewMessage(context.Background(), msg)
		require.Zero(t, q.Len(), "a full push buffer must not enqueue Telegram work")

		// The handler built the v2-only fact from the source; the endpoint
		// rejected it as buffer-full, and no v1 envelope was fabricated.
		require.Len(t, capture.v2s, 1)
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitRejectedBufferFull}, capture.results)

		// The protected facts survived the build even though delivery failed.
		require.Equal(t, true, capture.v2s[0].Message.HasSensitiveContent)
		require.Equal(t, "restricted", capture.v2s[0].Message.RestrictionReason)
		require.Equal(t, "messagePhoto", capture.v2s[0].Message.Content.Type)
	})
}
