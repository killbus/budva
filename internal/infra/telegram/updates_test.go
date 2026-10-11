package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/infra/telegram/mocks"
)

const (
	updateLoopDeadline = time.Second
	updateBurstSize    = 4096
	updateChatID       = int64(77)
	updateTailChatID   = int64(9001)
)

// updateLoopFixture drives the production Repo loop, not SDK fan-out: this
// inactive listener is never registered with a native client. Only the loop
// closes its input. Install defer f.close() before starting it or any producer.
type updateLoopFixture struct {
	repo   *Repo
	client *mocks.ClientAdapter
	input  chan client.Type

	loopCtx    context.Context
	cancelLoop context.CancelFunc
	loopDone   chan struct{}
	workCtx    context.Context
	cancelWork context.CancelFunc
	producers  sync.WaitGroup
	operations sync.WaitGroup
}

func newUpdateLoopFixture(t *testing.T, mode UpdateMode) *updateLoopFixture {
	t.Helper()
	f := &updateLoopFixture{
		repo: New(config.TelegramConfig{
			WarmupDeadline: 30 * time.Second,
			WarmupTicker:   time.Minute,
		}, mode),
		client:   mocks.NewClientAdapter(t),
		input:    make(chan client.Type), // A completed send is an actual loop handoff.
		loopDone: make(chan struct{}),
	}
	f.loopCtx, f.cancelLoop = context.WithCancel(context.Background())
	f.workCtx, f.cancelWork = context.WithCancel(context.Background())
	f.repo.clientAdapter = f.client
	f.repo.logger = slog.New(slog.DiscardHandler)
	f.client.EXPECT().GetListener().Return(&client.Listener{Updates: f.input}).Once()
	return f
}

func (f *updateLoopFixture) start() {
	go func() {
		defer close(f.loopDone)
		f.repo.listenUpdates(f.loopCtx)
	}()
}

func (f *updateLoopFixture) inject(updates ...client.Type) <-chan int {
	done := make(chan int, 1)
	f.producers.Add(1)
	go func() {
		defer f.producers.Done()
		sent := 0
		defer func() { done <- sent }()
		for _, update := range updates {
			select {
			case <-f.workCtx.Done():
				return
			case f.input <- update:
				sent++
			}
		}
	}()
	return done
}

func (f *updateLoopFixture) close() {
	// Verdicts are recorded before this rescue. In the old-allocation/publication
	// control even a NoBusinessUpdates fixture has an actual, drainable outlet.
	// Never use Updates() here or close either channel to force progress.
	f.cancelWork()
	f.producers.Wait()
	f.operations.Wait()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for {
			select {
			case <-f.loopDone:
				return
			case <-f.repo.updates:
			}
		}
	}()
	f.cancelLoop()
	<-f.loopDone
	<-drainDone
}

func awaitLoopValue[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	timer := time.NewTimer(updateLoopDeadline)
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatalf("no progress within virtual deadline: %s", what)
		var zero T
		return zero
	}
}

func messageUpdates(n int) []client.Type {
	updates := make([]client.Type, n)
	for i := range updates {
		updates[i] = &client.UpdateNewMessage{Message: &client.Message{
			Id:     int64(i + 1),
			ChatId: updateChatID,
			Content: &client.MessageText{
				Text: &client.FormattedText{Text: fmt.Sprintf("message-%d", i+1)},
			},
		}}
	}
	return updates
}

func (f *updateLoopFixture) requireBurstProgress(t *testing.T) {
	t.Helper()
	marker := f.repo.currentTable().getOrInsert(updateTailChatID)
	updates := append(messageUpdates(updateBurstSize),
		&client.UpdateNewChat{Chat: &client.Chat{Id: updateTailChatID}})
	// Do not assert nil allocation here: the old behavior must reach its
	// stalled-progress verdict, independently of the constructor tests.
	require.Equal(t, len(updates), awaitLoopValue(t, f.inject(updates...), "4096 updates and tail input"))
	awaitLoopValue(t, marker.ready, "tail NewChat readiness effect")
}

func (f *updateLoopFixture) startPendingSend(t *testing.T, tmpID int64) <-chan sendResult {
	t.Helper()
	req := &client.SendMessageRequest{ChatId: updateChatID}
	f.client.EXPECT().SendMessage(req).Return(&client.Message{Id: tmpID, ChatId: updateChatID}, nil).Once()
	done := make(chan sendResult, 1)
	f.operations.Add(1)
	go func() {
		defer f.operations.Done()
		msg, err := f.repo.SendMessageAndWait(f.workCtx, req)
		done <- sendResult{msg: msg, err: err}
	}()
	// SendMessage returns before pendingSends.Store; the mock callback is not
	// a registration barrier. Wait for the actual operation's result wait.
	synctest.Wait()
	_, registered := f.repo.pendingSends.Load(tmpID)
	require.True(t, registered, "real pending send must exist before confirmation")
	return done
}

func TestListenUpdates_NoBusinessConsumerBurst(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		f := newUpdateLoopFixture(t, NoBusinessUpdates)
		defer f.close()
		f.start()

		// Red: unconditional allocation/publication stalls the burst; bypassing
		// listening or guarding before internal dispatch loses the tail effect.
		f.requireBurstProgress(t)
	})
}

func TestListenUpdates_SendResultsAfterBurst(t *testing.T) {
	permanent := &client.Message{Id: 8001, ChatId: updateChatID}
	for _, tc := range []struct {
		name    string
		update  client.Type
		wantMsg *client.Message
		wantErr string
	}{
		{
			name:    "success",
			update:  &client.UpdateMessageSendSucceeded{OldMessageId: -1, Message: permanent},
			wantMsg: permanent,
		},
		{
			name: "failure",
			update: &client.UpdateMessageSendFailed{
				OldMessageId: -1, Error: &client.Error{Code: 400, Message: "BAD_REQUEST"},
			},
			wantErr: "send failed: code=400 BAD_REQUEST",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				f := newUpdateLoopFixture(t, NoBusinessUpdates)
				defer f.close()
				f.start()
				f.requireBurstProgress(t)
				done := f.startPendingSend(t, -1)

				// Red: moving the outlet guard before dispatch loses the real result;
				// failed sends must dispatch even though they are not business events.
				require.Equal(t, 1, awaitLoopValue(t, f.inject(tc.update), "confirmation input"))
				result := awaitLoopValue(t, done, "pending send result")
				if tc.wantErr != "" {
					require.EqualError(t, result.err, tc.wantErr)
					assert.Nil(t, result.msg)
				} else {
					require.NoError(t, result.err)
					assert.Same(t, tc.wantMsg, result.msg)
				}
				_, pending := f.repo.pendingSends.Load(int64(-1))
				assert.False(t, pending, "completed send must remove its pending entry")
			})
		})
	}
}

func TestListenUpdates_ReadinessAfterBurstBeforeTicker(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		f := newUpdateLoopFixture(t, NoBusinessUpdates)
		defer f.close()
		f.start()
		f.requireBurstProgress(t)

		const coldChatID = int64(9002)
		notFound := chatNotFoundErr()
		var materialized atomic.Bool
		var probes atomic.Int32
		done := make(chan error, 1)
		f.operations.Add(1)
		start := time.Now()
		go func() {
			defer f.operations.Done()
			done <- f.repo.withReady(f.workCtx, coldChatID, func() error {
				probes.Add(1)
				if !materialized.Load() {
					return notFound
				}
				return nil
			})
		}()
		synctest.Wait()
		require.EqualValues(t, 1, probes.Load(), "initial cold probe must have run")
		table := f.repo.currentTable()
		table.mu.Lock()
		entry, registered := table.ready[coldChatID]
		table.mu.Unlock()
		require.True(t, registered, "real same-chat waiter must precede the edge")
		select {
		case <-entry.ready:
			t.Fatal("cold chat must not already be certified")
		default:
		}

		// Model TDLib materialization explicitly, independently of call count.
		// Red: removing NewChat dispatch misses the 1s verdict, before the 1m
		// ticker can rescue the waiter. Unexpected LoadChats is also a mock failure.
		materialized.Store(true)
		require.Equal(t, 1, awaitLoopValue(t, f.inject(
			&client.UpdateNewChat{Chat: &client.Chat{Id: coldChatID}},
		), "cold-chat NewChat input"))
		require.NoError(t, awaitLoopValue(t, done, "readiness retry after NewChat"))
		assert.Less(t, time.Since(start), f.repo.warmupTicker())
		assert.EqualValues(t, 2, probes.Load(), "initial miss and edge-triggered retry")
	})
}

// subscribedTail lists expected business events explicitly, without reusing
// isRelevantUpdate as the oracle. Each call creates an independent snapshot.
func subscribedTail() (input, business []client.Type) {
	confirmation := &client.UpdateMessageSendSucceeded{
		OldMessageId: -2, Message: &client.Message{Id: 8002, ChatId: updateChatID},
	}
	edit := &client.UpdateMessageEdited{ChatId: updateChatID, MessageId: 8002, EditDate: 12345}
	deleted := &client.UpdateDeleteMessages{
		ChatId: updateChatID, MessageIds: []int64{2, 3}, IsPermanent: true,
	}
	return []client.Type{
		&client.UpdateOption{Name: "ignored"},
		confirmation,
		edit,
		&client.UpdateDeleteMessages{ChatId: updateChatID, MessageIds: []int64{4}, FromCache: true},
		deleted,
		&client.UpdateMessageSendFailed{OldMessageId: -3, Error: &client.Error{Code: 400, Message: "ignored"}},
	}, []client.Type{confirmation, edit, deleted}
}

func drainThroughMarker(t *testing.T, outlet <-chan client.Type, marker <-chan struct{}) []client.Type {
	t.Helper()
	var got []client.Type
	timer := time.NewTimer(updateLoopDeadline)
	defer timer.Stop()
	for {
		select {
		case update := <-outlet:
			got = append(got, update)
		case <-marker:
			// The producer's final event has had its internal effect. Quiesce the
			// loop, then include everything still buffered, not just an expected N.
			synctest.Wait()
			for {
				select {
				case update := <-outlet:
					got = append(got, update)
				default:
					return got
				}
			}
		case <-timer.C:
			t.Fatal("business consumer did not reach the internal tail marker")
			return nil
		}
	}
}

func TestListenUpdates_SubscribedBackpressurePreservesEvents(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		f := newUpdateLoopFixture(t, BusinessUpdates)
		defer f.close()
		f.start()
		outlet := f.repo.Updates()
		require.Equal(t, 100, cap(outlet))
		result := f.startPendingSend(t, -2)

		initial := messageUpdates(101)
		require.Equal(t, 101, awaitLoopValue(t, f.inject(initial...), "101st unbuffered handoff"))
		// No consumer has run. After the 101st handoff and quiescence, the
		// production loop has attempted publication into the full outlet.
		synctest.Wait()
		require.Len(t, outlet, 100)

		tail, business := subscribedTail()
		_, expectedTail := subscribedTail()
		expected := append(messageUpdates(101), expectedTail...)
		identities := append(initial, business...)
		marker := f.repo.currentTable().getOrInsert(updateTailChatID)
		tail = append(tail, &client.UpdateNewChat{Chat: &client.Chat{Id: updateTailChatID}})
		sent := f.inject(tail...)
		got := drainThroughMarker(t, outlet, marker.ready) // Resume the real consumer.
		require.Equal(t, len(tail), awaitLoopValue(t, sent, "subscribed tail input"))

		// Red: default/drop on a full queue loses event 101; weakened filtering
		// adds extras. Independent snapshots also detect in-place content changes.
		require.Len(t, got, len(expected), "exact count after processing the tail")
		assert.Equal(t, expected, got, "exact content and per-listener order")
		for i := range identities {
			assert.Same(t, identities[i], got[i], "event %d identity", i)
		}
		confirmation := awaitLoopValue(t, result, "internal result alongside business confirmation")
		require.NoError(t, confirmation.err)
		assert.Same(t, business[0].(*client.UpdateMessageSendSucceeded).Message, confirmation.msg)
		_, pending := f.repo.pendingSends.Load(int64(-2))
		assert.False(t, pending)
	})
}

func TestListenUpdates_CancelReceiveWait(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode UpdateMode
	}{
		{"without business outlet", NoBusinessUpdates},
		{"with business outlet", BusinessUpdates},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				f := newUpdateLoopFixture(t, tc.mode)
				defer f.close()
				f.start()
				synctest.Wait()

				// No producer exists during cancellation. Red: omitting deferred
				// listener.Close leaves the input open after the observed loop exit.
				f.cancelLoop()
				awaitLoopValue(t, f.loopDone, "normal receive cancellation")
				select {
				case _, open := <-f.input:
					assert.False(t, open, "production loop owns fixture closure")
				default:
					t.Fatal("listener input remains open after loop exit")
				}
			})
		})
	}
}
