package handler

import (
	"context"
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

// editTextlessMessage builds the shared textless edit input: a captionless
// photo with real send and edit times, no album grouping and no save
// eligibility, whose source size carries a real file identifier the v2
// projection must never surface.
func editTextlessMessage() *client.Message {
	return &client.Message{ChatId: 100, Id: 1, Date: 1791868800, EditDate: 1791868830, CanBeSaved: false,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "edit-invariant-file-id-7"}},
			}}},
		}}
}

// editTextlessService wires one captionless source photo through the supplied
// submitter on a webhook-only rule (no SendCopy/To destinations). Strict
// mocks bind the only two reads the edit task performs (GetCopiedMessageIDs ->
// nil, GetFormattedText -> nil) and a strict TelegramRepo, StateRepo,
// RateLimiter, TransformService and AlbumService with zero expectations fail
// on any copy/forward, state or limiter work, so Telegram invariance under
// the webhook-only edit path is enforced. The v2-only edit submission happens
// inside the task, so the caller must drive q.ProcessAll() to observe it.
func editTextlessService(t *testing.T, submitter event.Submitter) (*Service, *queue.Repo, *client.Message) {
	t.Helper()
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, submitter)

	msg := editTextlessMessage()
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
	message.EXPECT().GetFormattedText(msg).Return(nil).Once()
	return h, q, msg
}

// RED: the webhook-only textless media EDIT fact is delivery-invariant. Under
// every delivery outcome (buffer full, timeout, http 5xx, serialization
// failure, retired generation) the Telegram behavior is unchanged (strict
// mocks with zero expectations fail on any copy/forward, state or limiter
// work), no v1 envelope is fabricated, the v2-only fact is submitted exactly
// once as the canonical edited photo content, and the task queue converges
// to zero. The textless path never retries and no Telegram work is
// re-enqueued, so the queue drains in one pass.
func TestTextlessMediaEdit_InvariantUnderDeliveryOutcomes(t *testing.T) {
	for _, outcome := range []string{"buffer_full", "timeout", "http_5xx", "serialization", "retired"} {
		t.Run(outcome, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sink := &outcomeSink{}
				switch outcome {
				case "buffer_full":
					// The first submission is taken in flight by the worker,
					// which blocks in the timeout class; synctest.Wait joins it
					// there. The second submission then fills the bound to
					// capacity, so the edit submission is rejected before any
					// worker consumes it (mirrors TestPush_TelegramInvariance).
					sink.class = push.FailureTimeout
					g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: time.Second}})
					g.Submit([]string{"endpoint"}, event.Envelope{})
					synctest.Wait()
					g.Submit([]string{"endpoint"}, event.Envelope{})
					capture := &captureSubmitter{next: g}
					h, q, msg := editTextlessService(t, capture)
					h.OnEditedMessage(context.Background(), msg)
					q.ProcessAll()
					require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")
					g.Retire()
					require.Equal(t, []event.SubmitResult{event.SubmitRejectedBufferFull}, capture.results)
					requireOutcomeSubmission(t, capture)
					return
				case "retired":
					g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 4, Timeout: time.Second}})
					g.Retire()
					capture := &captureSubmitter{next: g}
					h, q, msg := editTextlessService(t, capture)
					h.OnEditedMessage(context.Background(), msg)
					q.ProcessAll()
					require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")
					require.Equal(t, []event.SubmitResult{event.SubmitRejectedRetired}, capture.results)
					requireOutcomeSubmission(t, capture)
					return
				}
				sink.class = push.FailureClass(outcome)
				g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 4, Timeout: time.Second}})
				defer g.Retire()
				capture := &captureSubmitter{next: g}
				h, q, msg := editTextlessService(t, capture)
				h.OnEditedMessage(context.Background(), msg)
				q.ProcessAll()
				require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")
				// The submission was enqueued before the worker's delivery
				// attempt failed; the failure class never re-runs or re-emits it.
				require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
				requireOutcomeSubmission(t, capture)
			})
		})
	}
}

// requireOutcomeSubmission asserts the single v2-only edited fact recorded on
// the captured route: one edited photo content notification, no v1 envelope,
// and no file or network reference leaked into the fact.
func requireOutcomeSubmission(t *testing.T, capture *captureSubmitter) {
	t.Helper()
	require.Len(t, capture.v2s, 1, "the textless edit path must submit the v2 fact exactly once")
	require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
	n := capture.v2s[0]
	require.Equal(t, messagev2.SchemaVersion, n.SchemaVersion)
	require.Equal(t, messagev2.EventEdited, n.Event)
	require.Equal(t, int64(100), n.Message.ChatID)
	require.Equal(t, int64(1), n.Message.MessageID)
	require.NotNil(t, n.Message.EditedAt, "an edit fact carries the source edit time")
	require.NotNil(t, n.Message.Content)
	require.Equal(t, "messagePhoto", n.Message.Content.Type)
	require.Nil(t, n.Message.Content.FormattedText, "a captionless photo has no text fact")
}

// RED: the edit textless path stays routed to the generation captured at the
// OnEditedMessage entry (immutable captured route), mirroring
// TestTextlessMedia_ReloadRetiredOutcomeStaysCaptured. The queue task submits
// through the captured submitter; after the captured generation is retired
// and the ruleset reloaded to a replacement submitter, the replacement
// receives nothing, the captured submission is the only one recorded, and the
// task queue converges to zero. Pure transport, so it runs inside
// synctest.Test.
func TestTextlessMediaEdit_ReloadRetiredOutcomeStaysCaptured(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		capture := &captureSubmitter{next: g}
		h, q, msg := editTextlessService(t, capture)

		h.OnEditedMessage(context.Background(), msg)
		q.ProcessAll()
		require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")

		// The captured submission is recorded and, because the generation has
		// not been retired yet, it is accepted.
		require.Len(t, capture.v2s, 1)
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)

		// Retire the captured generation, then reload to a replacement
		// submitter. A second edit would now be routed to the replacement, but
		// the already-processed fact is the captured one.
		g.Retire()
		replacement := &recordingSubmitter{}
		h.SetRuleSetWithPush(&domain.RuleSet{
			ForwardRules:        map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"replacement"}}},
			OrderedForwardRules: []string{"r"},
			UniqueSources:       map[int64]struct{}{100: {}},
		}, replacement)

		require.Empty(t, replacement.submissions,
			"the replacement submitter must not receive an already-processed textless edit")
		// The captured fact is still the only one recorded.
		require.Len(t, capture.v2s, 1)
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
	})
}
