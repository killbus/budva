package handler

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/service/push"
	"github.com/pure-golang/budva-claude/internal/service/push/event"
)

// Strict mocks bind the untouched album list, quiet period, serial scheduling,
// limiter, copy mappings and statistics. Missing hooks fail the event assertion;
// routing the non-OK member to a Telegram check chat is an unexpected call.
func TestPush_AlbumPipelineInvariant(t *testing.T) {
	for _, sendCopy := range []bool{false, true} {
		mode := "forward"
		if sendCopy {
			mode = "copy"
		}
		for _, outcome := range []string{"disabled", "buffer_full", "retired", "success", "http_5xx", "timeout"} {
			t.Run(mode+"/"+outcome, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					var submitter event.Submitter
					var capture *captureSubmitter
					var generation *push.Generation
					want := event.SubmitAccepted
					if outcome != "disabled" {
						sink := &outcomeSink{}
						if outcome == "http_5xx" || outcome == "timeout" {
							sink.class = push.FailureClass(outcome)
						}
						generation = push.NewGeneration(map[string]push.EndpointConfig{
							"endpoint": {Sink: sink, BufferCapacity: 1, Timeout: 10 * time.Second},
						})
						defer generation.Retire()
						if outcome == "retired" {
							generation.Retire()
							want = event.SubmitRejectedRetired
						} else if outcome == "buffer_full" {
							sink.class = push.FailureTimeout
							generation.Submit([]string{"endpoint"}, event.Envelope{})
							synctest.Wait()
							generation.Submit([]string{"endpoint"}, event.Envelope{})
							want = event.SubmitRejectedBufferFull
						}
						capture = &captureSubmitter{next: generation}
						submitter = capture
					}
					trace := runPushAlbumFixture(t, sendCopy, submitter)
					require.Equal(t, []string{"limit", "album", "viewed", "forwarded"}, trace)
					if capture != nil {
						require.Equal(t, []event.SubmitResult{want}, capture.results)
						require.Len(t, capture.events, 1)
						require.Equal(t, event.EventNew, capture.events[0].Event)
						require.Equal(t, event.SchemaVersion, capture.events[0].SchemaVersion)
						require.Equal(t, int64(100), capture.events[0].ChatID)
						require.Equal(t, "caption", capture.events[0].Text)
						require.Equal(t, int64(1), capture.events[0].MessageID)
					}
				})
			})
		}
	}
}

func runPushAlbumFixture(t *testing.T, sendCopy bool, submitter event.Submitter) []string {
	t.Helper()
	tg, state := mocks.NewTelegramRepo(t), mocks.NewStateRepo(t)
	message, filter, album := mocks.NewMessageService(t), mocks.NewFilterService(t), mocks.NewAlbumService(t)
	limiter, tracker, q := mocks.NewRateLimiter(t), mocks.NewDedupTracker(t), queue.New()
	h := New(tg, state, message, filter, mocks.NewTransformService(t), album, q, limiter,
		func([]int64) DedupTracker { return tracker })
	rule := &domain.ForwardRule{ID: "r", From: 100, To: []int64{200}, SendCopy: sendCopy, Check: 300, PushTo: []string{"endpoint"}}
	rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"}, UniqueSources: map[int64]struct{}{100: {}}}
	h.SetRuleSetWithPush(rs, submitter)
	first := &client.Message{ChatId: 100, Id: 1, CanBeSaved: true, MediaAlbumId: 42}
	second := &client.Message{ChatId: 100, Id: 2, CanBeSaved: true, MediaAlbumId: 42}
	text := &client.FormattedText{Text: "caption"}
	message.EXPECT().IsSystemMessage(first).Return(false).Once()
	message.EXPECT().IsSystemMessage(second).Return(false).Once()
	firstReads, secondReads := 1, 1
	if submitter != nil {
		firstReads++
		secondReads++
		filter.EXPECT().Evaluate("caption", rule).Return(domain.FiltersOK).Once()
		filter.EXPECT().Evaluate("", rule).Return(domain.FiltersCheck).Once()
	}
	if sendCopy {
		firstReads++
		secondReads += 2
		message.EXPECT().BuildInputContent(first, text).Return(&client.InputMessageText{Text: text}).Once()
		message.EXPECT().BuildInputContent(second, mock.Anything).Return(&client.InputMessageText{}).Twice()
	}
	message.EXPECT().GetFormattedText(first).Return(text).Times(firstReads)
	message.EXPECT().GetFormattedText(second).Return(&client.FormattedText{}).Times(secondReads)
	album.EXPECT().AddMessage("r:42", first).Return(true).Once()
	album.EXPECT().AddMessage("r:42", second).Return(false).Once()
	// The delay also proves push runs after collection, not on member ingress.
	collectedAt := time.Now()
	album.EXPECT().LastReceivedAge("r:42").RunAndReturn(func(string) time.Duration {
		return time.Since(collectedAt)
	}).Twice()
	album.EXPECT().PopMessages("r:42").Run(func(string) {
		require.GreaterOrEqual(t, time.Since(collectedAt), 3*time.Second)
	}).Return([]*client.Message{first, second, second}).Once()
	tracker.EXPECT().TryMark(int64(200)).Return(true).Once()
	trace := make([]string, 0, 4)
	limiter.EXPECT().WaitForForward(mock.Anything, int64(200)).Run(func(context.Context, int64) {
		require.GreaterOrEqual(t, time.Since(collectedAt), 3*time.Second)
		trace = append(trace, "limit")
	}).Once()
	if sendCopy {
		tg.EXPECT().SendMessageAlbum(mock.MatchedBy(func(req *client.SendMessageAlbumRequest) bool {
			return req.ChatId == 200 && len(req.InputMessageContents) == 3
		})).Run(func(*client.SendMessageAlbumRequest) { trace = append(trace, "album") }).Return(&client.Messages{Messages: []*client.Message{{Id: 501}, {Id: 502}, {Id: 503}}}, nil).Once()
		state.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r:200:501").Return(nil).Once()
		state.EXPECT().SetCopiedMessageID(int64(100), int64(2), "r:200:502").Return(nil).Once()
		state.EXPECT().SetCopiedMessageID(int64(100), int64(2), "r:200:503").Return(nil).Once()
	} else {
		tg.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
			return req.ChatId == 200 && req.FromChatId == 100 && len(req.MessageIds) == 3 &&
				req.MessageIds[0] == 1 && req.MessageIds[1] == 2 && req.MessageIds[2] == 2
		})).Run(func(*client.ForwardMessagesRequest) { trace = append(trace, "album") }).Return(&client.Messages{}, nil).Once()
	}
	state.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Run(func(int64, string) { trace = append(trace, "viewed") }).Return(uint64(1), nil).Once()
	state.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Run(func(int64, string) { trace = append(trace, "forwarded") }).Return(uint64(1), nil).Once()
	h.OnNewMessage(context.Background(), first)
	h.OnNewMessage(context.Background(), second)
	require.Equal(t, 1, q.Len())
	if capture, ok := submitter.(*captureSubmitter); ok {
		require.Empty(t, capture.events)
	}
	// Reload before collection must not change the captured rule or dispatcher.
	next := &recordingSubmitter{}
	h.SetRuleSetWithPush(&domain.RuleSet{}, next)
	q.ProcessAll()
	require.Empty(t, next.submissions)
	require.Zero(t, q.Len())
	return trace
}
