package push

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type telemetry struct {
	enqueued, full, retired, failed, reload metric.Int64Counter
	duration                                metric.Int64Histogram
	depth, success                          metric.Int64ObservableGauge
	mu                                      sync.Mutex
	workers                                 map[*endpointWorker]string
	lastSuccess                             map[string]int64
}

var metricsOnce sync.Once
var metrics *telemetry

func defaultTelemetry() *telemetry {
	metricsOnce.Do(func() {
		metrics = newTelemetry(otel.Meter("github.com/pure-golang/budva-claude/internal/service/push"))
	})
	return metrics
}

func newTelemetry(meter metric.Meter) *telemetry {
	m := &telemetry{workers: make(map[*endpointWorker]string), lastSuccess: make(map[string]int64)}
	var err error
	for name, instrument := range map[string]*metric.Int64Counter{
		"enqueued_total":             &m.enqueued,
		"rejected_buffer_full_total": &m.full,
		"rejected_retired_total":     &m.retired,
		"delivery_failed_total":      &m.failed,
		"reload_total":               &m.reload,
	} {
		*instrument, err = meter.Int64Counter("budva.push." + name)
		if err != nil {
			panic(err)
		}
	}
	m.duration, err = meter.Int64Histogram("budva.push.send_duration_ms", metric.WithUnit("ms"))
	if err != nil {
		panic(err)
	}
	m.depth, err = meter.Int64ObservableGauge("budva.push.buffer_depth")
	if err != nil {
		panic(err)
	}
	m.success, err = meter.Int64ObservableGauge("budva.push.last_success_unix")
	if err != nil {
		panic(err)
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		depths := make(map[string]int64, len(m.lastSuccess))
		for worker, name := range m.workers {
			depths[name] += int64(len(worker.queue))
		}
		for name, timestamp := range m.lastSuccess {
			observer.ObserveInt64(m.depth, depths[name], endpointOption(name))
			observer.ObserveInt64(m.success, timestamp, endpointOption(name))
		}
		return nil
	}, m.depth, m.success)
	if err != nil {
		panic(err)
	}
	return m
}

func endpointOption(name string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("endpoint", name))
}

// RecordReload accounts for load/build failures as well as publications.
func RecordReload(ok bool) {
	outcome := "failed"
	if ok {
		outcome = "ok"
	}
	defaultTelemetry().reload.Add(context.Background(), 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}
