package handler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/service/push/event"
	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

// threadSafeSubmitter records submissions from any goroutine. The album
// regression test drives the album task through the REAL production worker
// (q.StartContext), whose goroutine appends here while the test goroutine
// reads; a plain slice would be a data race. The recorded submission mirrors
// recordingSubmitter so the assertions read the same way.
type threadSafeSubmitter struct {
	mu          sync.Mutex
	submissions []recordedSubmission
}

func (s *threadSafeSubmitter) Submit(names []string, env event.Envelope) event.SubmitResults {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submissions = append(s.submissions, recordedSubmission{names: append([]string(nil), names...), env: env})
	return nil
}

func (s *threadSafeSubmitter) SubmitV2(names []string, env event.Envelope, v2 *messagev2.Notification) event.SubmitResults {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submissions = append(s.submissions, recordedSubmission{names: append([]string(nil), names...), env: env, v2: v2})
	return nil
}

func (s *threadSafeSubmitter) SubmitV2Only(names []string, v2 *messagev2.Notification) event.SubmitResults {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submissions = append(s.submissions, recordedSubmission{names: append([]string(nil), names...), v2: v2})
	return nil
}

// snapshot returns a consistent copy of the recorded submissions.
func (s *threadSafeSubmitter) snapshot() []recordedSubmission {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedSubmission(nil), s.submissions...)
}

// TestAlbumWebhookOnly_workerDrainsAfterAlbumTask —
// Regression, 10-29 push stall (incident shape). A textless media album from
// the configured source reaches processMediaAlbum through the matching rule
// (internal/app/handler/service.go:270) and enqueues the album closure on the
// REAL production worker (q.StartContext). Under a webhook-only rule (PushTo
// only, no To) the closure still reaches the unconditional statistics Add
// (service.go:644), re-entering the queue. The old processQueue held the
// write lock across task execution, so the worker deadlocked on its own
// RWMutex and every later Add blocked forever — the update loop stopped
// (goroutines 39/44 in the dump). The worker must drain the album task and
// the statistics task it appends, and the next NEW text's Add (service.go:398)
// must not block.
//
// The production path is required: the bug is in processQueue (the
// StartContext worker), and ProcessAll already releases the lock before
// executing, so driving the test through ProcessAll would not reproduce it.
func TestAlbumWebhookOnly_workerDrainsAfterAlbumTask(t *testing.T) {
	t.Parallel()

	state, message, filter, album, q := mocks.NewStateRepo(t), mocks.NewMessageService(t),
		mocks.NewFilterService(t), mocks.NewAlbumService(t), queue.New()
	capture := &threadSafeSubmitter{}
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), album, q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
	h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, capture)

	albumKey := domain.MediaAlbumKey("r:42")
	albumMsg := &client.Message{ChatId: 100, Id: 1, CanBeSaved: false, MediaAlbumId: 42,
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "album-wedge-file-id-42"}},
			}}},
		}}
	message.EXPECT().IsSystemMessage(albumMsg).Return(false).Once()
	// Ingress read (nil → not a single-media fact) and the member submit-loop
	// read (nil → projected as a v2-only content fact).
	message.EXPECT().GetFormattedText(albumMsg).Return(nil).Times(2)
	album.EXPECT().AddMessage(albumKey, albumMsg).Return(true).Once()
	album.EXPECT().LastReceivedAge(albumKey).Return(3 * time.Second).Once()
	album.EXPECT().PopMessages(albumKey).Return([]*client.Message{albumMsg}).Once()

	// Bounded context: Repo.Close is a no-op, so ctx is the worker's only
	// stop signal — cancelling on test cleanup ends the run loop and its
	// 1s ticker instead of leaking them until the process exits.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, q.StartContext(ctx))

	// Ingress enqueues the album task; the real worker must run it (submitting
	// the member's v2-only fact) and drain the statistics task it appends.
	// Before the fix the worker deadlocked inside the album task and the queue
	// never drained. The combined condition (fact submitted AND queue empty) is
	// required: the worker removes the album task from the queue before running
	// it, so the queue is empty for a moment before the submission happens.
	h.OnNewMessage(ctx, albumMsg)
	require.Eventually(t,
		func() bool { subs := capture.snapshot(); return len(subs) >= 1 && q.Len() == 0 },
		5*time.Second, 20*time.Millisecond,
		"the worker must run the album task (submitting its v2 fact) and drain the nested statistics task")

	subs := capture.snapshot()
	require.Len(t, subs, 1, "the captionless member must submit its v2 fact exactly once")
	require.NotNil(t, subs[0].v2)
	require.Empty(t, subs[0].env.Text, "a captionless member submits a v2-only fact, no v1 envelope text")
	require.Zero(t, subs[0].env.ChatID, "a captionless member submits a v2-only fact, no v1 envelope")
	require.Equal(t, messagev2.EventNew, subs[0].v2.Event)
	require.Equal(t, int64(42), subs[0].v2.Message.MediaAlbumID, "the album fact identifies its album")

	// The next NEW text from the same source: its ingress must not block on the
	// queue lock, and its tasks must drain (the update loop is alive). This is
	// goroutine 44 in the dump — the sole update consumer blocked on the same
	// lock — now proven unblocked.
	text := &client.FormattedText{Text: "after the album"}
	textMsg := &client.Message{ChatId: 100, Id: 2, CanBeSaved: true,
		Content: &client.MessageText{Text: text}}
	message.EXPECT().IsSystemMessage(textMsg).Return(false).Once()
	message.EXPECT().GetFormattedText(textMsg).Return(text).Once()
	filter.EXPECT().Evaluate("after the album", rule).Return(domain.FiltersOK).Once()

	added := make(chan struct{})
	go func() {
		h.OnNewMessage(ctx, textMsg)
		close(added)
	}()
	select {
	case <-added:
	case <-time.After(3 * time.Second):
		t.Fatal("OnNewMessage blocked on the queue lock: the update loop is wedged")
	}
	require.Eventually(t,
		func() bool { return len(capture.snapshot()) >= 2 && q.Len() == 0 },
		5*time.Second, 20*time.Millisecond,
		"the worker must drain the post-album text's tasks")

	subs = capture.snapshot()
	require.GreaterOrEqual(t, len(subs), 2)
	require.Equal(t, event.EventNew, subs[1].env.Event)
	require.Equal(t, "after the album", subs[1].env.Text)
}
