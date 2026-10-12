package push

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

// RED: missing Manager; replacement must publish before draining old workers.
func TestManager_ReplacementAndInvalidRetention(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := NewManager()
		defer manager.Close()
		var current atomic.Pointer[Generation]
		publish := current.Store
		slow := newBlockingSink()
		require.NoError(t, manager.Replace(map[string]EndpointConfig{"old": {Sink: slow}}, publish))
		old := current.Load()
		old.Submit([]string{"old"}, Envelope{MessageID: 1})
		synctest.Wait()
		done := make(chan struct{})
		go func() {
			_ = manager.Replace(map[string]EndpointConfig{"new": {Sink: &recordingSink{}}}, publish)
			close(done)
		}()
		synctest.Wait()
		require.NotSame(t, old, current.Load())
		require.Equal(t, SubmitRejectedRetired, old.Submit([]string{"old"}, Envelope{})["old"])
		slow.Release()
		<-done
		replacement := current.Load()
		require.Error(t, manager.Replace(map[string]EndpointConfig{"invalid": {}}, publish))
		require.Same(t, replacement, current.Load())
		require.Equal(t, SubmitAccepted, current.Load().Submit([]string{"new"}, Envelope{})["new"])
	})
}

func TestManager_ConcurrentSubmitReloadAndClose(t *testing.T) {
	// RED mutation: skipping previous.Retire accepts a late old-snapshot event.
	manager := NewManager()
	defer manager.Close()
	var current atomic.Pointer[Generation]
	sink := &recordingSink{}
	configs := map[string]EndpointConfig{"endpoint": {Sink: sink, BufferCapacity: 256}}
	require.NoError(t, manager.Replace(configs, current.Store))
	old := current.Load()
	var accepted atomic.Int64
	var senders sync.WaitGroup
	for range 8 {
		senders.Go(func() {
			for range 100 {
				if current.Load().Submit([]string{"endpoint"}, Envelope{})["endpoint"] == SubmitAccepted {
					accepted.Add(1)
				}
			}
		})
	}
	for range 5 {
		require.NoError(t, manager.Replace(configs, current.Store))
	}
	senders.Wait()
	manager.Close()
	require.Equal(t, SubmitRejectedRetired, old.Submit([]string{"endpoint"}, Envelope{})["endpoint"])
	require.Equal(t, int(accepted.Load()), len(sink.Messages()))
}

// RED: replacing after shutdown must not publish workers that outlive Close.
func TestManager_CloseSerializesReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := NewManager()
		var current *Generation
		require.NoError(t, manager.Replace(map[string]EndpointConfig{"a": {Sink: &recordingSink{}}}, func(g *Generation) { current = g }))
		manager.Close()
		manager.Close()
		err := manager.Replace(nil, func(*Generation) { t.Fatal("published after shutdown") })
		require.True(t, errors.Is(err, ErrManagerClosed))
		require.Equal(t, SubmitRejectedRetired, current.Submit([]string{"a"}, Envelope{})["a"])
	})
}
