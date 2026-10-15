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

// textlessAlbumEditService wires one edited captionless album member through
// the real EDIT flow (OnEditedMessage ingress -> editMessages task) on a
// webhook-only rule (PushTo only, no To, no SendCopy). The strict
// zero-expectation TelegramRepo, RateLimiter, TransformService and
// AlbumService mocks fail on any copy/forward, state-limiter, or album
// collection work, so Telegram invariance and edit-path album invariance are
// both enforced. The album mocks carry the strict zero expectations: an
// edited album member must never re-enter album collection (album collection
// stays NEW-only), so any AddMessage/LastReceivedAge/PopMessages call fails.
// The state and message mocks are returned so the caller binds the two reads
// the edit task performs (GetCopiedMessageIDs -> nil, GetFormattedText -> nil).
func textlessAlbumEditService(t *testing.T, submitter event.Submitter) (*Service, *queue.Repo, *mocks.StateRepo, *mocks.MessageService) {
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
	return h, q, state, message
}

// textlessAlbumEditedMember builds the shared edited captionless album-member
// input: a protected photo member of album 42 with real send and edit times
// and no caption, whose source size carries a real file identifier the v2
// projection must never surface.
func textlessAlbumEditedMember() *client.Message {
	return &client.Message{ChatId: 100, Id: 1, Date: 1791868800, EditDate: 1791868830,
		CanBeSaved: false, MediaAlbumId: 42,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "album-edit-file-id-42"}},
			}}},
		}}
}

// RED: an edited captionless album member (Content != nil, MediaAlbumId != 0,
// nil formatted text) is pushed only as the canonical v2 `edited` content
// fact, symmetric to the single-media textless edit. No v1 envelope is
// fabricated (the v1 endpoint stays silent at the delivery boundary), the
// edit never re-enters album collection (album collection stays NEW-only, so
// the strict album mocks see zero calls), and no Telegram copy/forward, state
// or limiter work runs (strict zero-expectation mocks). The v2-only
// submission happens inside the queue task, so the caller drives
// q.ProcessAll() to observe it; the textless edit path never retries, so the
// queue converges in one pass. Pure transport, so it runs inside
// synctest.Test.
func TestTextlessAlbumMemberEdit_PushesV2EditedFactOnlyV1Silent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		defer g.Retire()
		capture := &captureSubmitter{next: g}
		msg := textlessAlbumEditedMember()
		h, q, state, message := textlessAlbumEditService(t, capture)
		state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
		message.EXPECT().GetFormattedText(msg).Return(nil).Once()

		h.OnEditedMessage(context.Background(), msg)
		q.ProcessAll()
		require.Zero(t, q.Len(), "the textless album member edit path must not re-enqueue Telegram work")

		require.Len(t, capture.v2s, 1, "the edited captionless album member must submit the v2 fact exactly once")
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
		n := capture.v2s[0]
		require.Equal(t, messagev2.SchemaVersion, n.SchemaVersion)
		require.Equal(t, messagev2.EventEdited, n.Event)
		require.Equal(t, int64(100), n.Message.ChatID)
		require.Equal(t, int64(1), n.Message.MessageID)
		require.Equal(t, int64(42), n.Message.MediaAlbumID, "the edited fact identifies its album")
		require.NotNil(t, n.Message.EditedAt, "an edit fact carries the source edit time")
		require.NotNil(t, n.Message.Content)
		require.Equal(t, "messagePhoto", n.Message.Content.Type)
		require.Nil(t, n.Message.Content.FormattedText, "a captionless photo has no text fact")
	})
}

// RED: an edited content-LESS album member (Content == nil, MediaAlbumId != 0)
// is not textless media and produces nothing: no v1 envelope, no v2 fact, no
// submission outcome, and no album re-collection. The path is the no-event
// rule, so the queue task converges without emitting. Pure transport, so it
// runs inside synctest.Test.
func TestTextlessAlbumMemberEdit_ContentlessProducesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		defer g.Retire()
		capture := &captureSubmitter{next: g}
		msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800, EditDate: 1791868830,
			CanBeSaved: false, MediaAlbumId: 42}
		h, q, state, message := textlessAlbumEditService(t, capture)
		state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
		message.EXPECT().GetFormattedText(msg).Return(nil).Once()

		h.OnEditedMessage(context.Background(), msg)
		q.ProcessAll()
		require.Zero(t, q.Len(), "a content-less album member edit must enqueue no Telegram work")
		requireNoSubmission(t, capture)
	})
}
