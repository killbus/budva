package push

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/pure-golang/budva-claude/internal/service/push/event"
	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const defaultBufferCapacity = 256

// SubmitResult is the terminal enqueue outcome for one endpoint.
type SubmitResult = event.SubmitResult

const (
	SubmitAccepted           = event.SubmitAccepted
	SubmitRejectedBufferFull = event.SubmitRejectedBufferFull
	SubmitRejectedRetired    = event.SubmitRejectedRetired
	SubmitRejectedUnknown    = event.SubmitRejectedUnknown
)

// SubmitResults contains the enqueue outcome for every requested endpoint.
type SubmitResults = event.SubmitResults

// EndpointConfig defines one independently buffered delivery worker.
type EndpointConfig struct {
	Sink           Sink
	BufferCapacity int
	Timeout        time.Duration
}

type endpointWorker struct {
	name    string
	sink    Sink
	queue   chan event.RenderedEnvelope
	timeout time.Duration
}

// Generation owns an immutable set of endpoint workers and their buffers.
type Generation struct {
	endpoints map[string]*endpointWorker
	metrics   *telemetry

	leaseMu      sync.Mutex
	leaseCond    *sync.Cond
	retiring     bool
	activeLeases int

	workers sync.WaitGroup

	retireOnce sync.Once
	retireDone chan struct{}

	// Tests use this hook to exercise the lease/retirement interleaving.
	afterLeaseAcquired func()
}

// NewGeneration starts one worker for each configured endpoint.
func NewGeneration(configs map[string]EndpointConfig) *Generation {
	return newGeneration(configs, defaultTelemetry())
}

func newGeneration(configs map[string]EndpointConfig, metrics *telemetry) *Generation {
	generation := &Generation{
		endpoints:  make(map[string]*endpointWorker, len(configs)),
		retireDone: make(chan struct{}),
		metrics:    metrics,
	}
	generation.leaseCond = sync.NewCond(&generation.leaseMu)

	for name, config := range configs {
		capacity := config.BufferCapacity
		if capacity <= 0 {
			capacity = defaultBufferCapacity
		}

		worker := &endpointWorker{
			name:    name,
			sink:    config.Sink,
			queue:   make(chan event.RenderedEnvelope, capacity),
			timeout: config.Timeout,
		}
		if worker.timeout <= 0 {
			worker.timeout = defaultWebhookTimeout
		}
		generation.endpoints[name] = worker
		metrics.mu.Lock()
		metrics.workers[worker] = name
		if _, exists := metrics.lastSuccess[name]; !exists {
			metrics.lastSuccess[name] = 0
		}
		metrics.mu.Unlock()
		generation.workers.Add(1)
		go generation.runWorker(worker)
	}

	return generation
}

// Submit obtains a short-lived generation lease and attempts only non-blocking
// sends. Routing names are fixed by the caller's ruleset snapshot.
func (g *Generation) Submit(endpointNames []string, env Envelope) SubmitResults {
	return g.submit(endpointNames, event.RenderedEnvelope{Envelope: env})
}

// SubmitV2 enqueues both representations of the source facts. Each endpoint
// worker renders the schema selected at generation time; a v2 projection that
// is missing or invalid fails only the v2-selected endpoint.
func (g *Generation) SubmitV2(endpointNames []string, env Envelope, v2 *messagev2.Notification) SubmitResults {
	return g.submit(endpointNames, event.RenderedEnvelope{Envelope: env, V2: v2})
}

func (g *Generation) submit(endpointNames []string, rend event.RenderedEnvelope) SubmitResults {
	if !g.acquireLease() {
		results := make(SubmitResults, len(endpointNames))
		for _, name := range endpointNames {
			if _, seen := results[name]; seen {
				continue
			}
			results[name] = SubmitRejectedRetired
			g.metrics.retired.Add(context.Background(), 1, endpointOption(name))
			slog.Warn("Push submission rejected", "endpoint", name, "reason", "retired")
		}
		return results
	}
	defer g.releaseLease()

	if g.afterLeaseAcquired != nil {
		g.afterLeaseAcquired()
	}

	results := make(SubmitResults, len(endpointNames))
	for _, name := range endpointNames {
		if _, seen := results[name]; seen {
			continue
		}
		worker, ok := g.endpoints[name]
		if !ok {
			results[name] = SubmitRejectedUnknown
			continue
		}

		select {
		case worker.queue <- rend:
			results[name] = SubmitAccepted
			g.metrics.enqueued.Add(context.Background(), 1, endpointOption(name))
		default:
			results[name] = SubmitRejectedBufferFull
			g.metrics.full.Add(context.Background(), 1, endpointOption(name))
			slog.Warn("Push submission rejected", "endpoint", name, "reason", "buffer_full")
		}
	}
	return results
}

// Retire rejects new leases, waits for active submitters, then closes and
// drains every endpoint. Concurrent and repeated calls wait for the same run.
func (g *Generation) Retire() {
	g.retireOnce.Do(func() {
		g.leaseMu.Lock()
		g.retiring = true
		for g.activeLeases != 0 {
			g.leaseCond.Wait()
		}
		for _, worker := range g.endpoints {
			close(worker.queue)
		}
		g.leaseMu.Unlock()

		g.workers.Wait()
		g.metrics.mu.Lock()
		for _, worker := range g.endpoints {
			delete(g.metrics.workers, worker)
		}
		g.metrics.mu.Unlock()
		close(g.retireDone)
	})
	<-g.retireDone
}

func (g *Generation) acquireLease() bool {
	g.leaseMu.Lock()
	defer g.leaseMu.Unlock()
	if g.retiring {
		return false
	}
	g.activeLeases++
	return true
}

func (g *Generation) releaseLease() {
	g.leaseMu.Lock()
	g.activeLeases--
	if g.activeLeases == 0 {
		g.leaseCond.Broadcast()
	}
	g.leaseMu.Unlock()
}

func (g *Generation) runWorker(worker *endpointWorker) {
	defer g.workers.Done()
	for rend := range worker.queue {
		ctx, cancel := context.WithTimeout(context.Background(), worker.timeout)
		start := time.Now()
		err := worker.sink.Push(ctx, rend)
		g.metrics.duration.Record(context.Background(), time.Since(start).Milliseconds(), endpointOption(worker.name))
		if err != nil {
			class := classifyWorkerError(err, ctx)
			var deliveryErr *DeliveryError
			if errors.As(err, &deliveryErr) {
				class = deliveryErr.Class
			}
			g.metrics.failed.Add(context.Background(), 1, metric.WithAttributes(attribute.String("endpoint", worker.name), attribute.String("class", string(class))))
			// Never log URLs, headers, message content, or wrapped transport errors.
			slog.Warn("Push delivery failed", "endpoint", worker.name, "class", class)
		} else {
			g.metrics.mu.Lock()
			g.metrics.lastSuccess[worker.name] = time.Now().Unix()
			g.metrics.mu.Unlock()
		}
		cancel()
	}
}

func classifyWorkerError(err error, ctx context.Context) FailureClass {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureTimeout
	}
	return FailureTLSTransport
}
