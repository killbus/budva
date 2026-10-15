// Shutdown invariance for the M05 textless/album v2-only paths. Every fact
// these paths emit is a v2-only content fact on the generation captured at
// ingress; service shutdown is the generation retiring, and retirement
// rejects new leases and drains every accepted event. Under that shutdown the
// paths must stay invariant: no v1 event is ever fabricated (the v1 endpoint
// stays silent or is absent), the v2 fact is submitted at most once (no
// duplicate on a shutdown re-drain), no Telegram copy/forward/caption/status
// or network work runs (strict zero-expectation mocks), and the task queue
// converges without a hang or a worker outliving the test. Content-less
// messages (Content == nil) are not textless media, so they produce nothing.
//
// The helpers mirror the surrounding M05 fixtures: the new path reuses
// protectedTextlessService (webhook-only rule, strict mocks, pre-bound
// reads), the edit path reuses editTextlessService, and the album path binds
// the same album end state as textlessAlbumFixture.
package handler

import (
	"context"
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

// albumShutdownService wires one captionless album member through the NEW
// path against the supplied submitter on a webhook-only rule (no SendCopy/To
// destinations). It mirrors textlessAlbumFixture: the strict zero-expectation
// Telegram/State/RateLimiter/Transform mocks fail on any copy/forward/state
// or limiter work, and the album mocks are bound to the same end state (the
// captionless member is the first to arrive, the quiet period is already
// satisfied, and the member is popped for the v2-only projection).
func albumShutdownService(t *testing.T, submitter event.Submitter) (*Service, *queue.Repo, *client.Message) {
	t.Helper()
	state, message, filter, album, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), mocks.NewAlbumService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), album, q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, submitter)

	msg := protectedTextlessMessage()
	msg.MediaAlbumId = 42
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	// GetFormattedText is read at ingress (nil, so the member is not a
	// single-media textless fact) and again in the member submit loop (nil, so
	// it is projected as a v2-only content fact rather than dropped).
	message.EXPECT().GetFormattedText(msg).Return(nil).Times(2)
	album.EXPECT().AddMessage(domain.MediaAlbumKey("r:42"), msg).Return(true).Once()
	album.EXPECT().LastReceivedAge(domain.MediaAlbumKey("r:42")).Return(3 * time.Second).Once()
	album.EXPECT().PopMessages(domain.MediaAlbumKey("r:42")).Return([]*client.Message{msg}).Once()
	return h, q, msg
}

// requireV2FactOnly asserts the captured route recorded exactly one v2-only
// content fact and no v1 envelope, pins its identity and content fields, and
// (when withAlbum is set) its album id. mediaType is the expected content
// type and wantEdited selects the edit-time assertion.
func requireV2FactOnly(t *testing.T, capture *captureSubmitter, ev messagev2.Event, mediaType string, wantEdited, withAlbum bool) {
	t.Helper()
	require.Len(t, capture.v2s, 1, "the shutdown-invariant path must submit the v2 fact exactly once")
	require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
	n := capture.v2s[0]
	require.Equal(t, messagev2.SchemaVersion, n.SchemaVersion)
	require.Equal(t, ev, n.Event)
	require.Equal(t, int64(100), n.Message.ChatID)
	require.Equal(t, int64(1), n.Message.MessageID)
	if withAlbum {
		require.Equal(t, int64(42), n.Message.MediaAlbumID)
	}
	if wantEdited {
		require.NotNil(t, n.Message.EditedAt, "an edit fact carries the source edit time")
	}
	require.NotNil(t, n.Message.Content)
	require.Equal(t, mediaType, n.Message.Content.Type)
	require.Nil(t, n.Message.Content.FormattedText, "a captionless photo has no text fact")
}

// requireAtMostOnceReDrain re-drains the queue after shutdown and proves the
// path submits nothing more: the fact stays exactly one, no v1 envelope is
// fabricated, and no Telegram work was re-enqueued.
func requireAtMostOnceReDrain(t *testing.T, capture *captureSubmitter, q *queue.Repo) {
	t.Helper()
	q.ProcessAll()
	require.Zero(t, q.Len(), "a shutdown re-drain must converge and enqueue no Telegram work")
	require.Len(t, capture.v2s, 1, "a shutdown re-drain must not resubmit the v2 fact")
	require.Empty(t, capture.events, "a shutdown re-drain must not fabricate a v1 envelope")
}

// requireNoSubmission asserts the content-less cases: the path produced no
// fact of any kind and no submission outcome.
func requireNoSubmission(t *testing.T, capture *captureSubmitter) {
	t.Helper()
	require.Empty(t, capture.events, "a content-less message must not fabricate a v1 envelope")
	require.Empty(t, capture.v2s, "a content-less message must not be pushed as a v2 fact")
	require.Empty(t, capture.results, "a content-less message must produce no submission")
}

// RED: the single-media textless NEW fact is shutdown-invariant.
// (a) delivered: through a real retirement drain, the v1 endpoint receives
// zero HTTP requests at the wire while the v2 endpoint receives exactly one
// canonical content fact -- v1 silence and at-most-once delivery at the
// endpoint boundary. (b) buffer_full: the prefill-bound buffer rejects the
// handler's submission before any worker consumes it. (c) retired: shutdown
// retirement rejects the captured submission. (d) contentless: a content-
// less new message produces nothing. In every case the v2 fact is submitted
// at most once (a queue re-drain after shutdown adds nothing), no Telegram
// copy/forward/caption/status work runs (strict zero-expectation mocks),
// and the task queue converges without a hang or a worker outliving the
// test.
func TestTextlessMediaNew_ShutdownInvariant(t *testing.T) {
	t.Run("delivered", func(t *testing.T) {
		// Real servers run outside synctest (repo convention: pure
		// transports inside synctest, real servers outside). The v1 endpoint
		// is a counter so "silent" means zero HTTP requests at the wire, not
		// just a missing capture. Retire() drains every endpoint and joins
		// the sink workers before the body channels are read, so no
		// goroutine outlives the test and no send can block.
		v1Hits := make(chan struct{}, 4)
		v1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			v1Hits <- struct{}{}
			w.WriteHeader(http.StatusAccepted)
		}))
		defer v1Server.Close()

		v2Bodies := make(chan []byte, 4)
		v2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			v2Bodies <- body
			w.WriteHeader(http.StatusAccepted)
		}))
		defer v2Server.Close()

		g := push.NewGeneration(map[string]push.EndpointConfig{
			"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1Server.URL}), BufferCapacity: 8, Timeout: time.Second},
			"v2": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Schema: push.DeliverySchemaV2}), BufferCapacity: 8, Timeout: time.Second},
		})
		capture := &captureSubmitter{next: g}
		h, q, msg := protectedTextlessService(t, []string{"v1", "v2"}, capture)

		h.OnNewMessage(context.Background(), msg)
		require.Zero(t, q.Len(), "the textless new path must not enqueue Telegram work")
		// Shutdown: retirement drains every endpoint and joins the workers
		// before the test ends, so no goroutine outlives it.
		g.Retire()

		// Exactly one v2 content fact is delivered on the wire and the v1
		// endpoint never observed a request.
		require.Len(t, v2Bodies, 1)
		require.Empty(t, v1Hits, "the v1 endpoint must stay silent for a textless fact")
		requireV2FactOnly(t, capture, messagev2.EventNew, "messagePhoto", false, false)
		// The wire is the authoritative delivery proof here (the capture
		// submitter records per-`endpoint` results only, so the v1/v2 names
		// are asserted through the servers instead).
		// The wire is a content fact, not a file: the projection never
		// surfaces the photo's file identifier.
		body := <-v2Bodies
		require.NotContains(t, string(body), "protected-file-id-9", "the projection must not surface the file identifier")
		require.NotContains(t, string(body), "file_id")
		requireAtMostOnceReDrain(t, capture, q)
	})
	t.Run("buffer_full", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// The first submission is taken in flight by the worker, which
			// blocks in the timeout class; synctest.Wait joins it there. The
			// second submission fills the bound to capacity, so the handler's
			// submission is rejected before any worker consumes it (mirrors
			// TestTextlessMediaProtected_InvariantUnderFullBuffer).
			sink := &outcomeSink{class: push.FailureTimeout}
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: time.Second}})
			g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())
			synctest.Wait()
			g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())
			defer g.Retire()

			capture := &captureSubmitter{next: g}
			h, q, msg := protectedTextlessService(t, []string{"endpoint"}, capture)
			h.OnNewMessage(context.Background(), msg)
			require.Zero(t, q.Len(), "the textless new path must not enqueue Telegram work")
			// Shutdown: retirement joins the parked worker before the test
			// ends, so no goroutine outlives it.
			g.Retire()
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedBufferFull}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventNew, "messagePhoto", false, false)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("retired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second}})
			capture := &captureSubmitter{next: g}
			h, q, msg := protectedTextlessService(t, []string{"endpoint"}, capture)
			g.Retire()
			defer g.Retire()

			h.OnNewMessage(context.Background(), msg)
			require.Zero(t, q.Len(), "the textless new path must not enqueue Telegram work")
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedRetired}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventNew, "messagePhoto", false, false)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("contentless", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			state, message, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), queue.New()
			g := push.NewGeneration(map[string]push.EndpointConfig{
				"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
			})
			capture := &captureSubmitter{next: g}
			h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t),
				mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
			rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
			h.SetRuleSetWithPush(&domain.RuleSet{
				ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
				OrderedForwardRules: []string{"r"},
				UniqueSources:       map[int64]struct{}{100: {}},
			}, capture)
			g.Retire()
			defer g.Retire()

			// Content == nil: not a system message and no formatted text, so
			// the source is neither forwardable nor textless media, and the
			// forward guard (SendCopy false, CanBeSaved false) skips it.
			msg := &client.Message{ChatId: 100, Id: 1}
			message.EXPECT().IsSystemMessage(msg).Return(false).Once()
			message.EXPECT().GetFormattedText(msg).Return(nil).Once()
			h.OnNewMessage(context.Background(), msg)
			require.Zero(t, q.Len(), "a content-less message must enqueue no Telegram work")
			requireNoSubmission(t, capture)
		})
	})
}

// RED: the textless EDIT fact is shutdown-invariant. (a) buffer_full: the
// prefill-bound buffer rejects the in-task submission before any worker
// consumes it and the parked worker is joined by Retire() without a hang.
// (b) retired: shutdown retirement rejects the captured submission and the
// queue re-drain submits nothing more (at-most-once). (c) contentless: a
// content-less edited message is not textless media, so it produces nothing.
// No Telegram copy/caption/state work runs in any case (strict
// zero-expectation mocks), and the queue converges in every case.
func TestTextlessMediaEdit_ShutdownInvariant(t *testing.T) {
	t.Run("buffer_full", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// The first submission is taken in flight by the worker, which
			// blocks in the timeout class; synctest.Wait joins it there. The
			// second submission fills the bound to capacity, so the edit
			// submission is rejected before any worker consumes it (mirrors
			// TestPush_TelegramInvariance).
			sink := &outcomeSink{class: push.FailureTimeout}
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: time.Second}})
			g.Submit([]string{"endpoint"}, event.Envelope{})
			synctest.Wait()
			g.Submit([]string{"endpoint"}, event.Envelope{})
			defer g.Retire()

			capture := &captureSubmitter{next: g}
			h, q, msg := editTextlessService(t, capture)
			h.OnEditedMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")
			// Shutdown: retirement joins the parked worker before the test
			// ends, so no goroutine outlives it.
			g.Retire()
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedBufferFull}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventEdited, "messagePhoto", true, false)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("retired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second}})
			capture := &captureSubmitter{next: g}
			h, q, msg := editTextlessService(t, capture)
			g.Retire()
			defer g.Retire()

			h.OnEditedMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedRetired}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventEdited, "messagePhoto", true, false)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("contentless", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			state, message, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), queue.New()
			g := push.NewGeneration(map[string]push.EndpointConfig{
				"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
			})
			capture := &captureSubmitter{next: g}
			h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t),
				mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
			rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
			h.SetRuleSetWithPush(&domain.RuleSet{
				ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
				OrderedForwardRules: []string{"r"},
				UniqueSources:       map[int64]struct{}{100: {}},
			}, capture)
			g.Retire()
			defer g.Retire()

			// Content == nil: the edit task reads the copy state and the
			// formatted text, finds neither a copy nor a textless fact, and
			// produces nothing (no `unknown` fact).
			msg := &client.Message{ChatId: 100, Id: 1}
			state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
			message.EXPECT().GetFormattedText(msg).Return(nil).Once()
			h.OnEditedMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "a content-less edited message must enqueue no Telegram work")
			requireNoSubmission(t, capture)
		})
	})
}

// RED: the captionless album member fact is shutdown-invariant. (a)
// buffer_full: the prefill-bound buffer rejects the in-task submission after
// collection and the parked worker is joined by Retire() without a hang.
// (b) retired: shutdown retirement rejects the captured submission after the
// album task has popped the member, and the re-drain submits nothing more
// (at-most-once). (c) contentless: a content-less album member is dropped by
// the member loop, so it produces nothing. No Telegram copy/forward work
// runs in any case (strict zero-expectation mocks), and the queue converges
// in every case.
func TestTextlessAlbumMember_ShutdownInvariant(t *testing.T) {
	t.Run("buffer_full", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			sink := &outcomeSink{class: push.FailureTimeout}
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: time.Second}})
			g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())
			synctest.Wait()
			g.SubmitV2Only([]string{"endpoint"}, protectedSnapshot())
			defer g.Retire()

			capture := &captureSubmitter{next: g}
			h, q, msg := albumShutdownService(t, capture)
			h.OnNewMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "the album task must converge and enqueue no Telegram work")
			// Shutdown: retirement joins the parked worker before the test
			// ends, so no goroutine outlives it.
			g.Retire()
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedBufferFull}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventNew, "messagePhoto", false, true)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("retired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second}})
			capture := &captureSubmitter{next: g}
			h, q, msg := albumShutdownService(t, capture)
			g.Retire()
			defer g.Retire()

			h.OnNewMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "the album task must converge and enqueue no Telegram work")
			require.Equal(t, []event.SubmitResult{event.SubmitRejectedRetired}, capture.results)
			requireV2FactOnly(t, capture, messagev2.EventNew, "messagePhoto", false, true)
			requireAtMostOnceReDrain(t, capture, q)
		})
	})
	t.Run("contentless", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			state, message, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), queue.New()
			g := push.NewGeneration(map[string]push.EndpointConfig{
				"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
			})
			capture := &captureSubmitter{next: g}
			h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t),
				mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
			rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
			h.SetRuleSetWithPush(&domain.RuleSet{
				ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
				OrderedForwardRules: []string{"r"},
				UniqueSources:       map[int64]struct{}{100: {}},
			}, capture)
			g.Retire()
			defer g.Retire()

			// A content-less album member (Content == nil): it is not a
			// system message and carries no formatted text, so it is neither
			// a textless album member nor forwardable (SendCopy false,
			// CanBeSaved false) and never enters collection.
			msg := &client.Message{ChatId: 100, Id: 1, MediaAlbumId: 42}
			message.EXPECT().IsSystemMessage(msg).Return(false).Once()
			message.EXPECT().GetFormattedText(msg).Return(nil).Once()
			h.OnNewMessage(context.Background(), msg)
			q.ProcessAll()
			require.Zero(t, q.Len(), "a content-less album member must enqueue no Telegram work")
			requireNoSubmission(t, capture)
		})
	})
}
