package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRepo_Add_and_Len(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act
	r.Add(func() {})
	r.Add(func() {})

	// Assert
	require.Equal(t, 2, r.Len())
}

func TestRepo_ProcessQueue_executes_task(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var called atomic.Bool
	r.Add(func() { called.Store(true) })

	// Act
	r.processQueue()

	// Assert
	require.True(t, called.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessQueue_recovers_from_panic(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	r.Add(func() { panic("test panic") })

	// Act / Assert — не паникует
	r.processQueue()
	require.Equal(t, 0, r.Len())
}

func TestRepo_StartContext_processes_tasks(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var called atomic.Bool
	r.Add(func() { called.Store(true) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Act
	err := r.StartContext(ctx)
	require.NoError(t, err)

	// Assert — дождаться выполнения
	require.Eventually(t, called.Load, 3*time.Second, 100*time.Millisecond)
}

func TestRepo_StartContext_stops_on_cancel(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	ctx, cancel := context.WithCancel(context.Background())

	// Act
	err := r.StartContext(ctx)
	require.NoError(t, err)
	cancel()

	// Assert — отмена контекста не вызывает паники; очередь пуста
	require.Equal(t, 0, r.Len())
}

func TestRepo_Close(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — всегда возвращает nil
	require.NoError(t, r.Close())
}

func TestRepo_ProcessAll(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })

	// Act
	r.ProcessAll()

	// Assert
	require.Equal(t, int32(3), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessAll_executes_tasks_added_during_processing(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		if counter.Load() == 1 {
			r.Add(func() { counter.Add(10) })
		}
	})

	// Act
	r.ProcessAll()

	// Assert — задача добавленная во время выполнения тоже выполнена
	require.Equal(t, int32(11), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })

	// Act
	r.ProcessBatch()

	// Assert
	require.Equal(t, int32(2), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_ignores_tasks_added_during_processing(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		r.Add(func() { counter.Add(10) })
	})

	// Act
	r.ProcessBatch()

	// Assert — задача добавленная во время выполнения НЕ выполнена
	require.Equal(t, int32(1), counter.Load())
	require.Equal(t, 1, r.Len())
}

func TestRepo_ProcessQueue_empty(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.processQueue()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessAll_empty_queue(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.ProcessAll()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_empty_queue(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.ProcessBatch()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_NilFront(t *testing.T) {
	t.Parallel()

	// Arrange — первая задача крадёт вторую; итерация 2 получит nil и выходит
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		r.mu.Lock()
		if front := r.queue.Front(); front != nil {
			r.queue.Remove(front)
		}
		r.mu.Unlock()
	})
	r.Add(func() { counter.Add(10) })

	// Act
	r.ProcessBatch()

	// Assert — вторая задача украдена изнутри первой, guard сработал
	require.Equal(t, int32(1), counter.Load())
	require.Equal(t, 0, r.Len())
}

// TestRepo_ProcessQueue_nested_Add_does_not_deadlock —
// Regression, 10-29 push stall. The production worker executes a task while
// the queued work can re-enter Add from inside it (the media-album closure
// adds its stats/forwarding tasks — internal/app/handler/service.go). The old
// processQueue held the write lock across executeTask, so the nested Add
// parked the worker on its own non-reentrant RWMutex forever and wedged the
// whole update loop. The edit-retry and delete-retry paths re-enter the same
// way, so this one test covers all three sites through the shared queue.
func TestRepo_ProcessQueue_nested_Add_does_not_deadlock(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		// Re-enter the queue from inside a running task.
		r.Add(func() { counter.Add(1) })
	})

	// Act — must return; before the fix it deadlocked on its own lock.
	done := make(chan struct{})
	go func() {
		r.processQueue()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("processQueue deadlocked: a task re-entered Add while the worker held the lock")
	}

	// Assert — the nested task is queued behind the first and runs next.
	require.Equal(t, 1, r.Len())
	r.processQueue()
	require.Equal(t, int32(2), counter.Load())
	require.Zero(t, r.Len())
}

// TestRepo_StartContext_nested_and_external_tasks_complete —
// Regression, 10-29 push stall, production worker path. A running task
// appends a follow-up task (the album closure) and an external Add arrives
// from the update loop (the next message's stats task, goroutine 44 in the
// dump). Both must complete: before the fix the worker deadlocked on the
// nested Add and every later Add blocked forever.
func TestRepo_StartContext_nested_and_external_tasks_complete(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var nested, external atomic.Bool
	r.Add(func() {
		// The running task appends its follow-up work.
		r.Add(func() { nested.Store(true) })
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, r.StartContext(ctx))

	// Act — the next message's Add must not block.
	added := make(chan struct{})
	go func() {
		r.Add(func() { external.Store(true) })
		close(added)
	}()
	select {
	case <-added:
	case <-time.After(3 * time.Second):
		t.Fatal("Add blocked on the queue lock: the worker is wedged")
	}

	// Assert — the worker drained both tasks on later ticks.
	require.Eventually(t,
		func() bool { return nested.Load() && external.Load() },
		5*time.Second, 50*time.Millisecond,
		"the worker must run the task added by a running task and the external task")
	require.Zero(t, r.Len())
}
