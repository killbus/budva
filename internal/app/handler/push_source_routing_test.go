package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/service/push/event"
)

type sourceEditFixture struct {
	h         *Service
	q         *queue.Repo
	rs        *domain.RuleSet
	state     *mocks.StateRepo
	message   *mocks.MessageService
	telegram  *mocks.TelegramRepo
	transform *mocks.TransformService
	capture   *recordingSubmitter
}

func arrangeSourceEditFixture(t *testing.T) *sourceEditFixture {
	t.Helper()
	f := &sourceEditFixture{
		q: queue.New(), state: mocks.NewStateRepo(t), message: mocks.NewMessageService(t),
		telegram: mocks.NewTelegramRepo(t), transform: mocks.NewTransformService(t),
		capture: &recordingSubmitter{},
	}
	f.h = New(f.telegram, f.state, f.message, mocks.NewFilterService(t), f.transform,
		mocks.NewAlbumService(t), f.q, mocks.NewRateLimiter(t), nil)
	f.rs = &domain.RuleSet{
		Sources:      map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations: map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules: map[string]*domain.ForwardRule{
			"mapped": {ID: "mapped", From: 100, To: []int64{200}, PushTo: []string{"primary", "audit"}, Include: "^keep", Exclude: "^blocked"},
			"webhook-only": {ID: "webhook-only", From: 100, To: []int64{}, CopyOnce: true,
				PushTo: []string{"unmapped"}, Include: "^keep", Exclude: "^blocked"},
			"foreign": {ID: "foreign", From: 999, PushTo: []string{"wrong-source"}},
		},
		OrderedForwardRules: []string{"mapped", "webhook-only", "foreign"},
		UniqueSources:       map[int64]struct{}{100: {}, 999: {}},
	}
	f.h.SetRuleSetWithPush(f.rs, f.capture)
	return f
}

func requireSourceEditSubmission(t *testing.T, capture *recordingSubmitter, text string) {
	t.Helper()
	require.Len(t, capture.submissions, 1)
	submission := capture.submissions[0]
	// Rule map iteration does not promise recipient order.
	require.ElementsMatch(t, []string{"primary", "audit", "unmapped"}, submission.names)
	require.Equal(t, event.SchemaVersion, submission.env.SchemaVersion)
	require.Equal(t, event.EventEdited, submission.env.Event)
	require.Equal(t, int64(100), submission.env.ChatID)
	require.Equal(t, int64(1), submission.env.MessageID)
	require.Equal(t, text, submission.env.Text)
	require.False(t, submission.env.OccurredAt.IsZero())
}

func TestPush_SourceEditRoutingIndependentOfCopies(t *testing.T) {
	// Expected RED on baseline: no copies skip the text read; mapped copies omit
	// the webhook-only rule. Strict mocks also bind unchanged Telegram editing.
	for _, tc := range []struct {
		name    string
		copies  []string
		mapped  bool
		missing bool
	}{
		{name: "no_copies"},
		{name: "missing_and_malformed_references", copies: []string{"removed:200:501", "broken", "mapped:200"}, missing: true},
		{name: "mapped_and_unmapped_rules", copies: []string{"mapped:200:500", "removed:200:501", "broken"}, mapped: true, missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := arrangeSourceEditFixture(t)
			text := &client.FormattedText{Text: "keep full source text"}
			msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageText{Text: text}}
			f.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(tc.copies).Once()
			f.message.EXPECT().GetFormattedText(msg).Return(text).Once()
			if tc.missing {
				// Legacy copy bookkeeping resolves IDs before checking rule existence.
				f.state.EXPECT().GetNewMessageID(int64(200), int64(501)).Return(int64(601)).Once()
			}
			if tc.mapped {
				transformed := &client.FormattedText{Text: "destination-specific text"}
				f.state.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600)).Once()
				f.message.EXPECT().GetReplyMarkupData(msg).Return(nil).Twice()
				f.transform.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil).Once()
				f.telegram.EXPECT().EditMessageText(mock.MatchedBy(func(req *client.EditMessageTextRequest) bool {
					content, ok := req.InputMessageContent.(*client.InputMessageText)
					return req.ChatId == 200 && req.MessageId == 600 && ok && content.Text == transformed
				})).Return(&client.Message{Id: 600}, nil).Once()
				f.state.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil).Once()
			}

			f.h.OnEditedMessage(context.Background(), msg)
			f.q.ProcessAll()
			requireSourceEditSubmission(t, f.capture, text.Text)
			require.Zero(t, f.q.Len())
		})
	}
}

func TestPush_SourceEditDoesNotRerunNewMessageEligibility(t *testing.T) {
	// Strict mocks: IsSystemMessage/Evaluate are forbidden on edits. Adding
	// saveability/filter guards suppresses the event, including a cleared caption.
	for _, tc := range []struct {
		name    string
		text    string
		caption bool
	}{
		{name: "text_leaves_include_filter", text: "no longer matches"},
		{name: "caption_enters_exclude_filter", text: "blocked caption", caption: true},
		{name: "caption_cleared", caption: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := arrangeSourceEditFixture(t)
			text := &client.FormattedText{Text: tc.text}
			msg := &client.Message{ChatId: 100, Id: 1, CanBeSaved: false, Content: &client.MessageText{Text: text}}
			if tc.caption {
				msg.Content = &client.MessagePhoto{Caption: text}
			}
			f.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
			f.message.EXPECT().GetFormattedText(msg).Return(text).Once()

			f.h.OnEditedMessage(context.Background(), msg)
			f.q.ProcessAll()
			requireSourceEditSubmission(t, f.capture, tc.text)
			require.Zero(t, f.q.Len())
		})
	}
}

func TestPush_SourceEditNilFormattedTextSkipsEvent(t *testing.T) {
	// Strict mocks require text inspection but forbid copy resolution and edits;
	// treating nil as a supported empty caption would incorrectly submit an event.
	t.Parallel()
	f := arrangeSourceEditFixture(t)
	msg := &client.Message{ChatId: 100, Id: 1}
	f.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"mapped:200:500"}).Once()
	f.message.EXPECT().GetFormattedText(msg).Return(nil).Once()

	f.h.OnEditedMessage(context.Background(), msg)
	f.q.ProcessAll()
	require.Empty(t, f.capture.submissions)
	require.Zero(t, f.q.Len())
}

func TestPush_SourceEditRetainsSnapshotAcrossReload(t *testing.T) {
	// Expected RED on baseline: no-copy edit skips text retrieval. Reading live
	// rules or the replacement submitter during queue execution also fails.
	t.Parallel()
	f := arrangeSourceEditFixture(t)
	text := &client.FormattedText{Text: "full edited text"}
	msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageText{Text: text}}
	f.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
	f.message.EXPECT().GetFormattedText(msg).Return(text).Once()
	f.h.OnEditedMessage(context.Background(), msg)
	require.Equal(t, 1, f.q.Len())
	require.Empty(t, f.capture.submissions)

	next := &recordingSubmitter{}
	f.h.SetRuleSetWithPush(&domain.RuleSet{
		ForwardRules:  map[string]*domain.ForwardRule{"replacement": {From: 100, PushTo: []string{"replacement"}}},
		UniqueSources: map[int64]struct{}{100: {}},
	}, next)
	f.q.ProcessAll()
	requireSourceEditSubmission(t, f.capture, text.Text)
	require.Empty(t, next.submissions)
	require.Zero(t, f.q.Len())
}

func TestPush_SourceEditDisabledWithoutCopiesPreservesFastPath(t *testing.T) {
	// Strict mocks have no message-service/filter/Telegram expectations: any
	// newly introduced read or edit on the disabled no-copy path fails the test.
	t.Parallel()
	f := arrangeSourceEditFixture(t)
	f.h.SetRuleSet(f.rs)
	msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageText{Text: &client.FormattedText{Text: "edited"}}}
	f.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()

	f.h.OnEditedMessage(context.Background(), msg)
	require.Equal(t, 1, f.q.Len())
	f.q.ProcessAll()
	require.Empty(t, f.capture.submissions)
	require.Zero(t, f.q.Len())
}
