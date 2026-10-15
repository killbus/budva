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

// textlessReplyService wires one webhook-only source rule (PushTo only, no To,
// no SendCopy) onto the handler and returns it with the queue and state and
// message mocks so the caller binds the exact reads the path performs. The
// strict zero-expectation TelegramRepo, StateRepo (state returns it),
// RateLimiter, TransformService and FilterService mocks fail on any
// copy/forward, state or limiter work, so Telegram invariance under the
// webhook-only reply paths is enforced. The submitter is the caller's
// captured generation, so the emitted v2 notifications are inspectable as
// canonical facts.
func textlessReplyService(t *testing.T, submitter event.Submitter) (*Service, *queue.Repo, *mocks.StateRepo, *mocks.MessageService, *mocks.AlbumService) {
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
	return h, q, state, message, album
}

// replyPhotoMember builds a protected captionless photo carrying the reply and
// forum-thread facts the v2 projection must carry through verbatim; the
// caller sets MediaAlbumId to select the single-media or album-member path.
func replyPhotoMember(id int64, albumID int64) *client.Message {
	return &client.Message{ChatId: 100, Id: id, Date: 1791868800, CanBeSaved: false, MediaAlbumId: client.JsonInt64(albumID),
		SenderId:        &client.MessageSenderUser{UserId: 42},
		ReplyTo:         &client.MessageReplyToMessage{ChatId: 100, MessageId: 9},
		MessageThreadId: 7,
		Content: &client.MessagePhoto{
			Photo: &client.Photo{Sizes: []*client.PhotoSize{{
				Type:   "i",
				Width:  900,
				Height: 600,
				Photo:  &client.File{Id: 7, Remote: &client.RemoteFile{Id: "reply-lock-file-id-42"}},
			}}},
		}}
}

// requireReplyThreadFact pins the reply and thread projection on one v2
// notification: the bounded message-reply discriminator with its chat and
// message identifiers, and the forum thread id carried verbatim.
func requireReplyThreadFact(t *testing.T, n *messagev2.Notification) {
	t.Helper()
	require.NotNil(t, n.Message.ReplyTo, "the v2 fact must carry the reply projection")
	require.Equal(t, client.TypeMessageReplyToMessage, n.Message.ReplyTo.Type)
	require.Equal(t, int64(100), n.Message.ReplyTo.ChatID)
	require.Equal(t, int64(9), n.Message.ReplyTo.MessageID)
	require.Equal(t, int64(7), n.Message.MessageThreadID, "the forum thread id must carry through verbatim")
}

// LOCK (no behavioral RED): a reply-bearing textless single-media NEW message
// is pushed exactly once as the canonical v2 `new` content fact, and the fact
// carries the reply and thread projection verbatim. The v1 endpoint stays
// silent (no v1 envelope is fabricated), and no Telegram copy/forward, state
// or limiter work runs (strict zero-expectation mocks). Pure transport, so it
// runs inside synctest.Test.
func TestTextlessReply_NewSingleMediaCarriesReplyThreadFact(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		defer g.Retire()
		capture := &captureSubmitter{next: g}
		msg := replyPhotoMember(1, 0)
		h, q, _, message, _ := textlessReplyService(t, capture)
		message.EXPECT().IsSystemMessage(msg).Return(false).Once()
		message.EXPECT().GetFormattedText(msg).Return(nil).Once()

		h.OnNewMessage(context.Background(), msg)
		require.Zero(t, q.Len(), "textless webhook-only delivery must not enqueue Telegram work")

		require.Len(t, capture.v2s, 1, "the reply-bearing textless media must submit the v2 fact exactly once")
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
		n := capture.v2s[0]
		require.Equal(t, messagev2.EventNew, n.Event)
		require.Equal(t, int64(100), n.Message.ChatID)
		require.Equal(t, int64(1), n.Message.MessageID)
		require.NotNil(t, n.Message.Content)
		require.Equal(t, "messagePhoto", n.Message.Content.Type)
		requireReplyThreadFact(t, n)
	})
}

// LOCK (no behavioral RED): a reply-bearing captionless album member (Content
// != nil, MediaAlbumId != 0, nil formatted text) is pushed exactly once as
// the canonical v2 `new` content fact after the quiet period, and the fact
// carries the reply and thread projection verbatim. The member enters album
// collection the same way a captioned member does, the v1 endpoint stays
// silent, and the task queue converges with no Telegram work. Pure transport,
// so it runs inside synctest.Test.
func TestTextlessReply_NewCaptionlessAlbumMemberCarriesReplyThreadFact(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := push.NewGeneration(map[string]push.EndpointConfig{
			"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
		})
		defer g.Retire()
		capture := &captureSubmitter{next: g}
		msg := replyPhotoMember(1, 42)
		h, q, _, message, album := textlessReplyService(t, capture)
		message.EXPECT().IsSystemMessage(msg).Return(false).Once()
		// GetFormattedText is read at ingress (nil, so the member takes the
		// album collection path) and again in the member submit loop (nil, so
		// it is projected as a v2-only content fact).
		message.EXPECT().GetFormattedText(msg).Return(nil).Times(2)
		// The member is the first to arrive, triggering the album task; the
		// quiet period is already satisfied, and the member is popped once.
		album.EXPECT().AddMessage(domain.MediaAlbumKey("r:42"), msg).Return(true).Once()
		album.EXPECT().LastReceivedAge(domain.MediaAlbumKey("r:42")).Return(3 * time.Second).Once()
		album.EXPECT().PopMessages(domain.MediaAlbumKey("r:42")).Return([]*client.Message{msg}).Once()

		h.OnNewMessage(context.Background(), msg)
		require.Equal(t, 1, q.Len(), "a captionless album member must enqueue the album collection task, not emit at ingress")
		q.ProcessAll()
		require.Zero(t, q.Len(), "the album task must converge and enqueue no Telegram work")

		require.Len(t, capture.v2s, 1, "the reply-bearing captionless album member must submit the v2 fact exactly once")
		require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
		require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
		n := capture.v2s[0]
		require.Equal(t, messagev2.EventNew, n.Event)
		require.Equal(t, int64(42), n.Message.MediaAlbumID, "the fact identifies its album")
		require.NotNil(t, n.Message.Content)
		require.Equal(t, "messagePhoto", n.Message.Content.Type)
		requireReplyThreadFact(t, n)
	})
}

// LOCK (no behavioral RED): an edited reply-bearing textless member is pushed
// exactly once as the canonical v2 `edited` content fact, and the fact
// carries the reply and thread projection verbatim, for both the single-media
// and the album-member case (album collection stays NEW-only, so an edited
// album member never re-enters it — the strict album mock sees zero calls).
// No v1 envelope is fabricated and no Telegram work runs. Pure transport, so
// it runs inside synctest.Test.
func TestTextlessReply_EditCarriesReplyThreadFact(t *testing.T) {
	for _, albumID := range []int64{0, 42} {
		name := "single_media"
		if albumID != 0 {
			name = "album_member"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				g := push.NewGeneration(map[string]push.EndpointConfig{
					"endpoint": {Sink: &outcomeSink{}, BufferCapacity: 4, Timeout: time.Second},
				})
				defer g.Retire()
				capture := &captureSubmitter{next: g}
				msg := replyPhotoMember(1, albumID)
				msg.EditDate = 1791868830
				h, q, state, message, _ := textlessReplyService(t, capture)
				state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
				message.EXPECT().GetFormattedText(msg).Return(nil).Once()

				h.OnEditedMessage(context.Background(), msg)
				q.ProcessAll()
				require.Zero(t, q.Len(), "the textless edit path must not re-enqueue Telegram work")

				require.Len(t, capture.v2s, 1, "the edited reply-bearing textless member must submit the v2 fact exactly once")
				require.Empty(t, capture.events, "a v2-only fact must not fabricate a v1 envelope")
				require.Equal(t, []event.SubmitResult{event.SubmitAccepted}, capture.results)
				n := capture.v2s[0]
				require.Equal(t, messagev2.EventEdited, n.Event)
				require.Equal(t, int64(albumID), n.Message.MediaAlbumID)
				require.NotNil(t, n.Message.EditedAt, "an edit fact carries the source edit time")
				require.NotNil(t, n.Message.Content)
				require.Equal(t, "messagePhoto", n.Message.Content.Type)
				requireReplyThreadFact(t, n)
			})
		})
	}
}

// LOCK (no behavioral RED): a reply-bearing but content-LESS new message
// (Content == nil) is not textless media, so it is left to the no-event rule:
// nothing is submitted at all, and in particular no v2 fact is fabricated
// carrying the reply (a reply projection without a content fact would be an
// invalid snapshot and must not be invented).
func TestTextlessReply_ContentlessNewMessageEmitsNoReplyFact(t *testing.T) {
	t.Parallel()
	recording := &recordingSubmitter{}
	state, message, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t),
		mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
	h.SetRuleSetWithPush(&domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		ForwardRules:        map[string]*domain.ForwardRule{"r": rule},
		OrderedForwardRules: []string{"r"},
		UniqueSources:       map[int64]struct{}{100: {}},
	}, recording)

	// Reply and thread facts are present on the source, but Content == nil:
	// not a system message and no formatted text, so the source is neither
	// forwardable nor textless media. Strict mocks bind only the two reads the
	// handler performs before the textless branch.
	msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800,
		ReplyTo:         &client.MessageReplyToMessage{ChatId: 100, MessageId: 9},
		MessageThreadId: 7}
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	message.EXPECT().GetFormattedText(msg).Return(nil).Once()

	h.OnNewMessage(context.Background(), msg)
	require.Zero(t, q.Len(), "a content-less new message must not enqueue Telegram work")
	require.Empty(t, recording.submissions, "a content-less message must produce no v1 or v2 submission, and no fabricated reply fact")
}
