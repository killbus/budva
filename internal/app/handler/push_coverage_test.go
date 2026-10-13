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
	"github.com/pure-golang/budva-claude/internal/service/push/event"
)

func TestPush_EditWithoutCopies(t *testing.T) {
	// RED: the original copy guard returns before submitting the source edit.
	for _, text := range []string{"current caption", ""} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			state, message, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), queue.New()
			h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t), mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
			capture := &recordingSubmitter{}
			h.SetRuleSetWithPush(&domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"endpoint"}}}, UniqueSources: map[int64]struct{}{100: {}}}, capture)
			msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: text}}}
			message.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: text}).Once()
			state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
			h.OnEditedMessage(context.Background(), msg)
			q.ProcessAll()
			require.Len(t, capture.submissions, 1)
			require.Equal(t, []string{"endpoint"}, capture.submissions[0].names)
			require.Equal(t, event.EventEdited, capture.submissions[0].env.Event)
			require.Equal(t, text, capture.submissions[0].env.Text)
		})
	}
}

func TestPush_WebhookOnlySourceLifecycle(t *testing.T) {
	// RED: the original copy guard suppresses the edit in this source-only flow.
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	capture := &recordingSubmitter{}
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
	h.SetRuleSetWithPush(&domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"}, UniqueSources: map[int64]struct{}{100: {}}}, capture)
	msg := &client.Message{ChatId: 100, Id: 1, CanBeSaved: true, Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: "caption"}}}
	text := &client.FormattedText{Text: "caption"}
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	message.EXPECT().GetFormattedText(msg).Return(text).Times(2)
	filter.EXPECT().Evaluate(text.Text, rule).Return(domain.FiltersOK).Once()
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Twice()
	h.OnNewMessage(context.Background(), msg)
	h.OnEditedMessage(context.Background(), msg)
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true, false)
	q.ProcessAll()
	require.Len(t, capture.submissions, 3)
	for i, kind := range []event.Event{event.EventNew, event.EventEdited, event.EventDeleted} {
		require.Equal(t, kind, capture.submissions[i].env.Event)
		require.Equal(t, []string{"endpoint"}, capture.submissions[i].names)
	}
	require.Equal(t, "caption", capture.submissions[1].env.Text)
	require.Empty(t, capture.submissions[2].env.Text)
}

func TestPush_AlbumMembersEmitOnce(t *testing.T) {
	// RED: the original album path never submits a source event.
	synctest.Test(t, func(t *testing.T) {
		message, album, q := mocks.NewMessageService(t), mocks.NewAlbumService(t), queue.New()
		filter := mocks.NewFilterService(t)
		h := New(mocks.NewTelegramRepo(t), mocks.NewStateRepo(t), message, filter, mocks.NewTransformService(t), album, q, mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
		capture := &recordingSubmitter{}
		rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"endpoint"}}}}
		first, second, unsupported := &client.Message{ChatId: 100, Id: 1}, &client.Message{ChatId: 100, Id: 2}, &client.Message{ChatId: 100, Id: 3}
		album.EXPECT().LastReceivedAge("album").Return(3 * time.Second).Once()
		album.EXPECT().PopMessages("album").Return([]*client.Message{first, second, second, unsupported}).Once()
		message.EXPECT().GetFormattedText(first).Return(&client.FormattedText{Text: "caption"}).Once()
		message.EXPECT().GetFormattedText(second).Return(&client.FormattedText{}).Once()
		message.EXPECT().GetFormattedText(unsupported).Return(nil).Once()
		filter.EXPECT().Evaluate("caption", rs.ForwardRules["r"]).Return(domain.FiltersOK).Once()
		filter.EXPECT().Evaluate("", rs.ForwardRules["r"]).Return(domain.FiltersOK).Once()
		h.processMediaAlbum(context.Background(), "album", rs, "r", first, nil, capture)
		q.ProcessAll()
		require.Len(t, capture.submissions, 2)
		require.Equal(t, "caption", capture.submissions[0].env.Text)
		require.Empty(t, capture.submissions[1].env.Text)
		require.Equal(t, int64(2), capture.submissions[1].env.MessageID)
	})
}

func TestPush_AlbumNewDoesNotSupersedeEditsOrDeletes(t *testing.T) {
	// RED: timestamping after collection makes the old caption win downstream LWW.
	synctest.Test(t, func(t *testing.T) {
		state, message, filter := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t)
		album, q, capture := mocks.NewAlbumService(t), queue.New(), &recordingSubmitter{}
		h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), album, q, mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
		rule := &domain.ForwardRule{From: 100, PushTo: []string{"endpoint"}}
		h.SetRuleSetWithPush(&domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"}, UniqueSources: map[int64]struct{}{100: {}}}, capture)
		text, editedText := &client.FormattedText{Text: "old caption"}, &client.FormattedText{Text: "edited caption"}
		msg := &client.Message{ChatId: 100, Id: 1, CanBeSaved: true, MediaAlbumId: 42, Content: &client.MessagePhoto{Caption: text}}
		second := &client.Message{ChatId: 100, Id: 2, CanBeSaved: true, MediaAlbumId: 42, Content: &client.MessagePhoto{Caption: text}}
		edited := &client.Message{ChatId: 100, Id: 1, Content: &client.MessagePhoto{Caption: editedText}}
		message.EXPECT().IsSystemMessage(msg).Return(false).Once()
		message.EXPECT().IsSystemMessage(second).Return(false).Once()
		message.EXPECT().GetFormattedText(msg).Return(text).Twice()
		message.EXPECT().GetFormattedText(second).Return(text).Twice()
		message.EXPECT().GetFormattedText(edited).Return(editedText).Once()
		filter.EXPECT().Evaluate(text.Text, rule).Return(domain.FiltersOK).Twice()
		album.EXPECT().AddMessage("r:42", msg).Return(true).Once()
		album.EXPECT().AddMessage("r:42", second).Return(false).Once()
		var lastReceived time.Time
		album.EXPECT().LastReceivedAge("r:42").RunAndReturn(func(string) time.Duration {
			return time.Since(lastReceived)
		}).Twice()
		album.EXPECT().PopMessages("r:42").Run(func(string) {
			require.GreaterOrEqual(t, time.Since(lastReceived), 3*time.Second)
		}).Return([]*client.Message{msg, second}).Once()
		state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Twice()
		started := time.Now().UTC()
		h.OnNewMessage(context.Background(), msg)
		time.Sleep(500 * time.Millisecond)
		h.OnNewMessage(context.Background(), second)
		lastReceived = time.Now()
		time.Sleep(500 * time.Millisecond)
		h.OnEditedMessage(context.Background(), edited)
		time.Sleep(time.Second)
		h.OnDeletedMessages(context.Background(), 100, []int64{1}, true, false)
		q.ProcessAll()
		require.Len(t, capture.submissions, 4)
		for i, kind := range []event.Event{event.EventNew, event.EventNew, event.EventEdited, event.EventDeleted} {
			require.Equal(t, kind, capture.submissions[i].env.Event)
		}
		newEvent, editEvent, deleteEvent := capture.submissions[0].env, capture.submissions[2].env, capture.submissions[3].env
		require.Equal(t, started, newEvent.OccurredAt)
		require.Equal(t, newEvent.OccurredAt, capture.submissions[1].env.OccurredAt)
		require.Equal(t, int64(2), capture.submissions[1].env.MessageID)
		require.True(t, newEvent.OccurredAt.Before(editEvent.OccurredAt))
		require.True(t, editEvent.OccurredAt.Before(deleteEvent.OccurredAt))
		require.Equal(t, "old caption", newEvent.Text)
		require.Equal(t, "edited caption", editEvent.Text)
	})
}
