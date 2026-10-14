package push

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGeneration_SubmitRejectsFullBufferWithoutBlocking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		sink := newBlockingSink()
		generation := NewGeneration(map[string]EndpointConfig{
			"slow": {Sink: sink, BufferCapacity: 1},
		})
		defer func() {
			sink.Release()
			generation.Retire()
		}()

		require.Equal(t, SubmitAccepted, generation.Submit([]string{"slow"}, Envelope{MessageID: 1})["slow"])
		sink.WaitStarted(t)
		require.Equal(t, SubmitAccepted, generation.Submit([]string{"slow"}, Envelope{MessageID: 2})["slow"])

		started := time.Now()
		result := generation.Submit([]string{"slow"}, Envelope{MessageID: 3})

		require.Equal(t, SubmitRejectedBufferFull, result["slow"])
		require.Less(t, time.Since(started), 100*time.Millisecond)

	})
}

func TestGeneration_SubmitRejectsRetiredGeneration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		sink := &recordingSink{}
		generation := NewGeneration(map[string]EndpointConfig{
			"endpoint": {Sink: sink, BufferCapacity: 1},
		})
		generation.Retire()

		started := time.Now()
		result := generation.Submit([]string{"endpoint"}, Envelope{MessageID: 1})

		require.Equal(t, SubmitRejectedRetired, result["endpoint"])
		require.Less(t, time.Since(started), 100*time.Millisecond)
		require.Empty(t, sink.Messages())

	})
}

func TestGeneration_RetireWaitsForAcquiredSubmitLease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		sink := &recordingSink{}
		generation := NewGeneration(map[string]EndpointConfig{
			"endpoint": {Sink: sink, BufferCapacity: 1},
		})

		leaseAcquired := make(chan struct{})
		continueSubmit := make(chan struct{})
		generation.afterLeaseAcquired = func() {
			close(leaseAcquired)
			<-continueSubmit
		}

		submitDone := make(chan SubmitResults, 1)
		go func() {
			submitDone <- generation.Submit([]string{"endpoint"}, Envelope{MessageID: 1})
		}()
		requireChannelClosed(t, leaseAcquired, "submit lease")

		retireDone := make(chan struct{})
		go func() {
			generation.Retire()
			close(retireDone)
		}()

		synctest.Wait()
		select {
		case <-retireDone:
			t.Fatal("Retire returned while an acquired submit lease was active")
		default:
		}

		close(continueSubmit)
		require.Equal(t, SubmitAccepted, (<-submitDone)["endpoint"])
		requireChannelClosed(t, retireDone, "retirement")
		require.Equal(t, []int64{1}, messageIDs(sink.Messages()))

	})
}

func TestGeneration_RetireIsIdempotentAndDrainsAcceptedEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		sink := &recordingSink{}
		generation := NewGeneration(map[string]EndpointConfig{
			"endpoint": {Sink: sink, BufferCapacity: 4},
		})
		for messageID := int64(1); messageID <= 3; messageID++ {
			require.Equal(t, SubmitAccepted, generation.Submit(
				[]string{"endpoint"},
				Envelope{MessageID: messageID},
			)["endpoint"])
		}

		var callers sync.WaitGroup
		callers.Add(2)
		for range 2 {
			go func() {
				defer callers.Done()
				generation.Retire()
			}()
		}
		callers.Wait()

		require.Equal(t, []int64{1, 2, 3}, messageIDs(sink.Messages()))
		require.Equal(t, SubmitRejectedRetired, generation.Submit(
			[]string{"endpoint"},
			Envelope{MessageID: 4},
		)["endpoint"])

	})
}

func TestGeneration_EndpointsUseIndependentWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {

		slow := newBlockingSink()
		fast := &recordingSink{received: make(chan struct{}, 1)}
		generation := NewGeneration(map[string]EndpointConfig{
			"slow": {Sink: slow, BufferCapacity: 1},
			"fast": {Sink: fast, BufferCapacity: 1},
		})
		defer func() {
			slow.Release()
			generation.Retire()
		}()

		result := generation.Submit([]string{"slow", "fast"}, Envelope{MessageID: 1})
		require.Equal(t, SubmitAccepted, result["slow"])
		require.Equal(t, SubmitAccepted, result["fast"])
		slow.WaitStarted(t)
		requireChannelClosed(t, fast.received, "fast endpoint delivery")

	})
}

type blockingSink struct {
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingSink() *blockingSink {
	return &blockingSink{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingSink) Push(context.Context, RenderedEnvelope) error {
	s.startedOnce.Do(func() { close(s.started) })
	<-s.release
	return nil
}

func (s *blockingSink) WaitStarted(t *testing.T) {
	t.Helper()
	requireChannelClosed(t, s.started, "blocking sink start")
}

func (s *blockingSink) Release() {
	s.releaseOnce.Do(func() { close(s.release) })
}

type recordingSink struct {
	mu       sync.Mutex
	messages []RenderedEnvelope
	received chan struct{}
	attempts atomic.Int64
}

func (s *recordingSink) Push(_ context.Context, rend RenderedEnvelope) error {
	s.attempts.Add(1)
	s.mu.Lock()
	s.messages = append(s.messages, rend)
	s.mu.Unlock()
	if s.received != nil {
		select {
		case s.received <- struct{}{}:
		default:
		}
	}
	return nil
}

func (s *recordingSink) Messages() []Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Envelope, 0, len(s.messages))
	for _, rend := range s.messages {
		out = append(out, rend.Envelope)
	}
	return out
}

func messageIDs(messages []Envelope) []int64 {
	ids := make([]int64, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.MessageID)
	}
	return ids
}

func requireChannelClosed(t *testing.T, channel <-chan struct{}, description string) {
	t.Helper()
	<-channel
}

// RED: duplicate route names delivered twice before Submit deduplicated them.
func TestGeneration_DuplicateNamesReceiveOneAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &recordingSink{}
		generation := NewGeneration(map[string]EndpointConfig{"a": {Sink: sink}})
		generation.Submit([]string{"a", "a"}, Envelope{MessageID: 1})
		generation.Retire()
		require.Len(t, sink.Messages(), 1)
	})
}

// RED: without worker deadlines Retire never drains a context-bound sink.
func TestGeneration_WorkerTimeoutBoundsRetirement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &deadlineSink{}
		generation := NewGeneration(map[string]EndpointConfig{
			"a": {Sink: sink, Timeout: time.Second, BufferCapacity: 1},
		})
		generation.Submit([]string{"a"}, Envelope{})
		synctest.Wait()
		generation.Submit([]string{"a"}, Envelope{})
		start := time.Now()
		generation.Retire()
		require.Equal(t, 2*time.Second, time.Since(start))
		require.Equal(t, int64(2), sink.attempts.Load())
	})
}

type deadlineSink struct{ attempts atomic.Int64 }

func (s *deadlineSink) Push(ctx context.Context, _ RenderedEnvelope) error {
	s.attempts.Add(1)
	<-ctx.Done()
	return ctx.Err()
}
