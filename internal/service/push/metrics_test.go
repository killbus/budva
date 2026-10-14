package push

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// RED: missing telemetry; failed attempts must never initialize last-success.
func TestMetrics_TerminalOutcomesAndActualSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		defer func() { _ = provider.Shutdown(context.Background()) }()
		metrics := newTelemetry(provider.Meter("push-test"))
		sink := &outcomeSink{err: &DeliveryError{Class: FailureHTTP4xx}}
		g := newGeneration(map[string]EndpointConfig{"a": {Sink: sink}}, metrics)
		defer g.Retire()
		require.Zero(t, collectMetric(t, reader, "budva.push.last_success_unix"))
		g.Submit([]string{"a"}, Envelope{})
		synctest.Wait()
		require.Equal(t, int64(1), collectMetric(t, reader, "budva.push.delivery_failed_total"))
		require.Zero(t, collectMetric(t, reader, "budva.push.last_success_unix"))
		sink.err = nil
		g.Submit([]string{"a"}, Envelope{})
		synctest.Wait()
		require.Equal(t, time.Now().Unix(), collectMetric(t, reader, "budva.push.last_success_unix"))
		require.Equal(t, int64(2), collectMetric(t, reader, "budva.push.enqueued_total"))
		require.Zero(t, collectMetric(t, reader, "budva.push.buffer_depth"))
		g.Retire()
		g.Submit([]string{"a"}, Envelope{})
		require.Equal(t, int64(1), collectMetric(t, reader, "budva.push.rejected_retired_total"))
	})
}

// RED: a full buffer must increment rejection, not delivery failure.
func TestMetrics_BufferRejectionAndDepth(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		defer func() { _ = provider.Shutdown(context.Background()) }()
		metrics := newTelemetry(provider.Meter("push-test"))
		sink := newBlockingSink()
		g := newGeneration(map[string]EndpointConfig{"a": {Sink: sink, BufferCapacity: 1}}, metrics)
		defer func() { sink.Release(); g.Retire() }()
		g.Submit([]string{"a"}, Envelope{})
		synctest.Wait()
		g.Submit([]string{"a"}, Envelope{})
		g.Submit([]string{"a"}, Envelope{})
		require.Equal(t, int64(1), collectMetric(t, reader, "budva.push.buffer_depth"))
		require.Equal(t, int64(1), collectMetric(t, reader, "budva.push.rejected_buffer_full_total"))
	})
}

type outcomeSink struct{ err error }

func (s *outcomeSink) Push(context.Context, RenderedEnvelope) error { return s.err }

func collectMetric(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	for _, scope := range data.ScopeMetrics {
		for _, item := range scope.Metrics {
			if item.Name != name {
				continue
			}
			switch value := item.Data.(type) {
			case metricdata.Gauge[int64]:
				var total int64
				for _, point := range value.DataPoints {
					total += point.Value
				}
				return total
			case metricdata.Sum[int64]:
				var total int64
				for _, point := range value.DataPoints {
					total += point.Value
				}
				return total
			}
		}
	}
	t.Fatalf("metric %s was not collected", name)
	return 0
}
