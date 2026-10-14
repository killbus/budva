package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

type pushFixture struct {
	h     *Service
	queue *queue.Repo
	rs    *domain.RuleSet
	msg   *client.Message
	trace []string
}

// Strict mocks bind the same send/edit/delete, mapping, statistics and limiter
// calls for every push outcome; trace equality also binds serial scheduling.
func arrangePushFixture(t *testing.T, submitter event.Submitter) *pushFixture {
	t.Helper()
	f := &pushFixture{queue: queue.New()}
	tg, state := mocks.NewTelegramRepo(t), mocks.NewStateRepo(t)
	message, filter := mocks.NewMessageService(t), mocks.NewFilterService(t)
	transform, limiter := mocks.NewTransformService(t), mocks.NewRateLimiter(t)
	tracker := mocks.NewDedupTracker(t)
	f.h = New(tg, state, message, filter, transform, mocks.NewAlbumService(t), f.queue, limiter,
		func([]int64) DedupTracker { return tracker })
	rule := &domain.ForwardRule{ID: "r", From: 100, To: []int64{200}, SendCopy: true, PushTo: []string{"endpoint"}}
	f.rs = &domain.RuleSet{Sources: map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations: map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"},
		UniqueSources: map[int64]struct{}{100: {}}}
	f.h.SetRuleSetWithPush(f.rs, submitter)
	text := &client.FormattedText{Text: "full source text"}
	f.msg = &client.Message{ChatId: 100, Id: 1, CanBeSaved: true, Content: &client.MessageText{Text: text}}
	message.EXPECT().IsSystemMessage(f.msg).Return(false).Once()
	message.EXPECT().GetFormattedText(f.msg).Return(text).Times(2)
	message.EXPECT().GetReplyMarkupData(f.msg).Return(nil).Times(4)
	filter.EXPECT().Evaluate(text.Text, rule).Return(domain.FiltersOK).Once()
	tracker.EXPECT().TryMark(int64(200)).Return(true).Once()
	limiter.EXPECT().WaitForForward(mock.Anything, int64(200)).Run(func(context.Context, int64) { f.trace = append(f.trace, "limit") }).Once()
	transform.EXPECT().Transform(mock.Anything, mock.Anything).Return(text, nil).Times(2)
	message.EXPECT().BuildInputContent(f.msg, text).Return(&client.InputMessageText{Text: text}).Once()
	tg.EXPECT().SendMessage(mock.MatchedBy(func(r *client.SendMessageRequest) bool { return r.ChatId == 200 })).Run(func(*client.SendMessageRequest) { f.trace = append(f.trace, "send") }).Return(&client.Message{Id: 500}, nil).Once()
	state.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r:200:500").Run(func(int64, int64, string) { f.trace = append(f.trace, "map") }).Return(nil).Once()
	state.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Run(func(int64, string) { f.trace = append(f.trace, "viewed") }).Return(uint64(1), nil).Once()
	state.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Run(func(int64, string) { f.trace = append(f.trace, "forwarded") }).Return(uint64(1), nil).Once()
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r:200:500"}).Times(2)
	state.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600)).Times(2)
	tg.EXPECT().EditMessageText(mock.MatchedBy(func(r *client.EditMessageTextRequest) bool { return r.ChatId == 200 && r.MessageId == 600 })).Run(func(*client.EditMessageTextRequest) { f.trace = append(f.trace, "edit") }).Return(f.msg, nil).Once()
	state.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil).Times(2)
	tg.EXPECT().DeleteMessages(mock.MatchedBy(func(r *client.DeleteMessagesRequest) bool {
		return r.ChatId == 200 && len(r.MessageIds) == 1 && r.MessageIds[0] == 600 && r.Revoke
	})).Run(func(*client.DeleteMessagesRequest) { f.trace = append(f.trace, "delete") }).Return(&client.Ok{}, nil).Once()
	state.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil).Once()
	state.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil).Once()
	state.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
	return f
}

func (f *pushFixture) run(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	f.h.OnNewMessage(ctx, f.msg)
	require.Equal(t, 2, f.queue.Len())
	f.queue.ProcessAll()
	f.h.OnEditedMessage(ctx, f.msg)
	f.h.OnDeletedMessages(ctx, 100, []int64{1}, true, false)
	require.Equal(t, 2, f.queue.Len())
	f.queue.ProcessAll()
	require.Zero(t, f.queue.Len())
	require.Equal(t, []string{"limit", "send", "map", "viewed", "forwarded", "edit", "delete"}, f.trace)
}

type captureSubmitter struct {
	next    event.Submitter
	events  []event.Envelope
	v2s     []*messagev2.Notification
	results []event.SubmitResult
}

type recordedSubmission struct {
	names []string
	env   event.Envelope
	v2    *messagev2.Notification
}

type recordingSubmitter struct{ submissions []recordedSubmission }

func (s *recordingSubmitter) Submit(names []string, env event.Envelope) event.SubmitResults {
	s.submissions = append(s.submissions, recordedSubmission{names: append([]string(nil), names...), env: env})
	return nil
}

func (s *recordingSubmitter) SubmitV2(names []string, env event.Envelope, v2 *messagev2.Notification) event.SubmitResults {
	s.submissions = append(s.submissions, recordedSubmission{names: append([]string(nil), names...), env: env, v2: v2})
	return nil
}

func TestPush_RetryUsesCapturedSnapshotAndEmitsOnce(t *testing.T) {
	// Strict mocks: unresolved permanent IDs cause four original Telegram
	// attempts; retaining emission on retry would produce duplicate source facts.
	for _, kind := range []event.Event{event.EventEdited, event.EventDeleted} {
		t.Run(string(kind), func(t *testing.T) {
			state, message := mocks.NewStateRepo(t), mocks.NewMessageService(t)
			q := queue.New()
			h := New(mocks.NewTelegramRepo(t), state, message, mocks.NewFilterService(t), mocks.NewTransformService(t), mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
			old, next := &recordingSubmitter{}, &recordingSubmitter{}
			rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": {From: 100, PushTo: []string{"old"}}}, UniqueSources: map[int64]struct{}{100: {}}}
			h.SetRuleSetWithPush(rs, old)
			msg := &client.Message{ChatId: 100, Id: 1}
			state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r:200:500"}).Times(4)
			state.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(0)).Times(4)
			if kind == event.EventEdited {
				message.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "edited"}).Times(4)
				h.OnEditedMessage(context.Background(), msg)
			} else {
				h.OnDeletedMessages(context.Background(), 100, []int64{1}, true, false)
			}
			h.SetRuleSetWithPush(&domain.RuleSet{}, next)
			q.ProcessAll()
			require.Len(t, old.submissions, 1)
			require.Equal(t, []string{"old"}, old.submissions[0].names)
			require.Equal(t, kind, old.submissions[0].env.Event)
			require.Empty(t, next.submissions)
		})
	}
}

func TestPush_DeleteWithoutCopiesHonorsSourceFact(t *testing.T) {
	// Strict mocks disallow Telegram delete/ID mutations when no copies exist;
	// duplicate update IDs must still produce only one source event per ID.
	state, q, capture := mocks.NewStateRepo(t), queue.New(), &recordingSubmitter{}
	h := New(mocks.NewTelegramRepo(t), state, mocks.NewMessageService(t), mocks.NewFilterService(t), mocks.NewTransformService(t), mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t), nil)
	h.SetRuleSetWithPush(&domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": {From: 100, Indelible: true, PushTo: []string{"endpoint"}}}, UniqueSources: map[int64]struct{}{100: {}}}, capture)
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Times(2)
	h.OnDeletedMessages(context.Background(), 100, []int64{1, 1}, false, false)
	require.Zero(t, q.Len())
	h.OnDeletedMessages(context.Background(), 100, []int64{1, 1}, true, false)
	q.ProcessAll()
	require.Len(t, capture.submissions, 1)
	require.Empty(t, capture.submissions[0].env.Text)
}

func (s *captureSubmitter) Submit(names []string, env event.Envelope) event.SubmitResults {
	s.events = append(s.events, env)
	results := s.next.Submit(names, env)
	s.results = append(s.results, results["endpoint"])
	return results
}

func (s *captureSubmitter) SubmitV2(names []string, env event.Envelope, v2 *messagev2.Notification) event.SubmitResults {
	s.events = append(s.events, env)
	s.v2s = append(s.v2s, v2)
	results := s.next.SubmitV2(names, env, v2)
	s.results = append(s.results, results["endpoint"])
	return results
}

type outcomeSink struct {
	class    push.FailureClass
	attempts int
}

func (s *outcomeSink) Push(ctx context.Context, _ event.RenderedEnvelope) error {
	s.attempts++
	if s.class == push.FailureTimeout {
		<-ctx.Done()
	}
	if s.class != "" {
		return &push.DeliveryError{Class: s.class}
	}
	return nil
}

func TestPush_TelegramInvariance(t *testing.T) {
	// RED: strict expectations fail on any changed Telegram call; removing an
	// emit hook fails the three-event assertion (recorded mutation validation).
	for _, outcome := range []string{"disabled", "buffer_full", "retired", "success", "http_4xx", "http_5xx", "redirect", "timeout", "tls_transport", "serialization"} {
		t.Run(outcome, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				if outcome == "disabled" {
					arrangePushFixture(t, nil).run(t)
					return
				}
				sink := &outcomeSink{}
				if outcome != "success" && outcome != "retired" && outcome != "serialization" {
					sink.class = push.FailureClass(outcome)
				}
				if outcome == "serialization" {
					// A serialization-class delivery failure must leave the
					// bound Telegram copy/forward/delete work untouched.
					sink.class = push.FailureSerialization
				}
				g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 4, Timeout: time.Second}})
				defer g.Retire()
				want := event.SubmitAccepted
				if outcome == "retired" {
					g.Retire()
					want = event.SubmitRejectedRetired
				}
				if outcome == "buffer_full" {
					sink.class = push.FailureTimeout
					g.Submit([]string{"endpoint"}, event.Envelope{})
					synctest.Wait()
					for range 4 {
						g.Submit([]string{"endpoint"}, event.Envelope{})
					}
					want = event.SubmitRejectedBufferFull
				}
				capture := &captureSubmitter{next: g}
				arrangePushFixture(t, capture).run(t)
				require.Equal(t, []event.SubmitResult{want, want, want}, capture.results)
				require.Len(t, capture.events, 3)
				for i, kind := range []event.Event{event.EventNew, event.EventEdited, event.EventDeleted} {
					require.Equal(t, kind, capture.events[i].Event)
					require.Equal(t, event.SchemaVersion, capture.events[i].SchemaVersion)
					require.Equal(t, int64(100), capture.events[i].ChatID)
					require.Equal(t, int64(1), capture.events[i].MessageID)
					require.False(t, capture.events[i].OccurredAt.IsZero())
				}
				require.Equal(t, "full source text", capture.events[1].Text)
				require.Empty(t, capture.events[2].Text)
				g.Retire()
				if want == event.SubmitAccepted {
					require.Equal(t, 3, sink.attempts)
				}
			})
		})
	}
}

func TestPush_ProductionHooksDeliverCanonicalWebhook(t *testing.T) {
	// RED: removing any production emit hook leaves fewer than three bodies.
	bodies := make(chan event.Envelope, 3)
	errors := make(chan error, 3)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env event.Envelope
		err := json.NewDecoder(r.Body).Decode(&env)
		errors <- err
		bodies <- env
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	g := push.NewGeneration(map[string]push.EndpointConfig{"endpoint": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: server.URL, Client: server.Client()})}})
	defer g.Retire()
	arrangePushFixture(t, g).run(t)
	g.Retire()
	require.Len(t, bodies, 3)
	for _, kind := range []event.Event{event.EventNew, event.EventEdited, event.EventDeleted} {
		require.NoError(t, <-errors)
		env := <-bodies
		require.Equal(t, kind, env.Event)
		require.Equal(t, event.SchemaVersion, env.SchemaVersion)
		require.Equal(t, int64(100), env.ChatID)
		require.Equal(t, int64(1), env.MessageID)
		if kind == event.EventDeleted {
			require.Empty(t, env.Text)
		} else {
			require.Equal(t, "full source text", env.Text)
		}
	}
}

// v2Probe decodes the frozen v2 top-level shape without pulling the full
// contract types into the assertion.
type v2Probe struct {
	SchemaVersion int            `json:"schema_version"`
	Event         string         `json:"event"`
	Message       map[string]any `json:"message"`
}

// mixedFixture drives the webhook-only lifecycle (new, edit, delete) through
// two real webhook endpoints: v1 keeps the default wire, v2 selects
// Message-only v2. Strict mocks bind unchanged Telegram copy/forward/delete
// behavior (RED 10).
func mixedFixture(t *testing.T, v1URL string, v1Client *http.Client, v2Sink *push.WebhookSink) (*queue.Repo, *push.Generation) {
	t.Helper()
	state, message, filter, q := mocks.NewStateRepo(t), mocks.NewMessageService(t), mocks.NewFilterService(t), queue.New()
	h := New(mocks.NewTelegramRepo(t), state, message, filter, mocks.NewTransformService(t), mocks.NewAlbumService(t), q,
		mocks.NewRateLimiter(t), func([]int64) DedupTracker { return mocks.NewDedupTracker(t) })
	rule := &domain.ForwardRule{From: 100, PushTo: []string{"v1", "v2"}}
	rs := &domain.RuleSet{ForwardRules: map[string]*domain.ForwardRule{"r": rule}, OrderedForwardRules: []string{"r"},
		UniqueSources: map[int64]struct{}{100: {}}}
	g := push.NewGeneration(map[string]push.EndpointConfig{
		"v1": {Sink: push.NewWebhookSink(push.WebhookConfig{URL: v1URL, Client: v1Client}), BufferCapacity: 8, Timeout: time.Second},
		"v2": {Sink: v2Sink, BufferCapacity: 8, Timeout: time.Second},
	})
	h.SetRuleSetWithPush(rs, g)

	msg := &client.Message{ChatId: 100, Id: 1, Date: 1791868800, CanBeSaved: true,
		SenderId: &client.MessageSenderUser{UserId: 42},
		Content:  &client.MessageText{Text: &client.FormattedText{Text: "full source text"}}}
	text := &client.FormattedText{Text: "full source text"}
	message.EXPECT().IsSystemMessage(msg).Return(false).Once()
	message.EXPECT().GetFormattedText(msg).Return(text).Times(2)
	filter.EXPECT().Evaluate(text.Text, rule).Return(domain.FiltersOK).Once()
	state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil).Twice()
	ctx := context.Background()
	h.OnNewMessage(ctx, msg)
	h.OnEditedMessage(ctx, msg)
	h.OnDeletedMessages(ctx, 100, []int64{1}, true, false)
	q.ProcessAll()
	return q, g
}

// failingBodyTransport records each rendered body and then fails the attempt,
// proving a failed v2 endpoint never produces a v1 fallback body.
type failingBodyTransport struct{ bodies chan []byte }

func (t *failingBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	t.bodies <- body
	return nil, errors.New("v2 endpoint down")
}

// RED 7: mixed v1/v2 endpoints each receive the representation they
// selected, from the same source facts.
func TestPush_MixedEndpointsDeliverSelectedSchemas(t *testing.T) {
	// Real TLS servers run outside synctest (repo convention: pure transports
	// inside synctest, real servers outside). Retire() joins the sink workers
	// before the bodies are drained; both sinks record the body before the
	// response returns, so the buffered channels hold all three events.
	v1Bodies := make(chan []byte, 3)
	v1Server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		v1Bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer v1Server.Close()

	v2Bodies := make(chan []byte, 3)
	v2Server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		v2Bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer v2Server.Close()

	_, g := mixedFixture(t, v1Server.URL, v1Server.Client(),
		push.NewWebhookSink(push.WebhookConfig{URL: v2Server.URL, Client: v2Server.Client(), Schema: push.DeliverySchemaV2}))
	g.Retire()

	for _, kind := range []string{"new", "edited", "deleted"} {
		var v1 event.Envelope
		require.NoError(t, json.Unmarshal(<-v1Bodies, &v1))
		require.Equal(t, 1, v1.SchemaVersion)
		require.Equal(t, kind, string(v1.Event))
		require.Equal(t, int64(100), v1.ChatID)
		require.Equal(t, int64(1), v1.MessageID)
		if kind == "deleted" {
			require.Empty(t, v1.Text)
		} else {
			require.Equal(t, "full source text", v1.Text)
		}

		var v2 v2Probe
		require.NoError(t, json.Unmarshal(<-v2Bodies, &v2))
		require.Equal(t, 2, v2.SchemaVersion)
		require.Equal(t, kind, v2.Event)
		require.Equal(t, float64(100), v2.Message["chat_id"])
		require.Equal(t, float64(1), v2.Message["message_id"])
	}
}

// RED 8: a v2-selected endpoint that cannot deliver fails only itself; the
// rendered body is v2 (never v1, raw TDLib JSON or text-only) and the v1
// sibling keeps receiving the full default wire.
func TestPush_V2FailureIsolatesEndpoint(t *testing.T) {
	// Real TLS server outside synctest (same convention as the mixed test);
	// Retire() joins the workers before the body channels are drained.
	v1Bodies := make(chan []byte, 3)
	v1Server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		v1Bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer v1Server.Close()

	v2Bodies := make(chan []byte, 3)
	v2Sink := push.NewWebhookSink(push.WebhookConfig{
		URL:    "https://push.example.test/events",
		Schema: push.DeliverySchemaV2,
		Client: &http.Client{Transport: &failingBodyTransport{bodies: v2Bodies}},
	})

	_, g := mixedFixture(t, v1Server.URL, v1Server.Client(), v2Sink)
	g.Retire()

	for _, kind := range []string{"new", "edited", "deleted"} {
		var v1 event.Envelope
		require.NoError(t, json.Unmarshal(<-v1Bodies, &v1))
		require.Equal(t, kind, string(v1.Event))

		body := <-v2Bodies
		// The failed endpoint's body must still be the v2
		// representation: never a v1 envelope, raw TDLib data or a
		// text-only fallback.
		var v2 v2Probe
		require.NoError(t, json.Unmarshal(body, &v2))
		require.Equal(t, 2, v2.SchemaVersion)
		require.Equal(t, kind, v2.Event)
		var asV1 event.Envelope
		require.NoError(t, json.Unmarshal(body, &asV1))
		require.NotEqual(t, 1, asV1.SchemaVersion)
		require.NotContains(t, string(body), "\"content\":\"")
	}
}
