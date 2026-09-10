package telegram

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// metrics.go — наблюдаемость wait-for-ready прогрева (SRE minimal set):
//   - warmMisses         — открытий wait-окна (miss + вход в ожидание);
//   - warmReadiness      — исходов окна: success / timeout (по label);
//   - warmTimeouts       — отдельный счётчик таймаутов (единственный
//     жёсткий алерт: доля timeout > 2% — дрейф);
//   - warmLoadChatsErrors— ошибок LoadChats-итераций;
//   - warmSessionSwaps   — смен TDLib-сессии (поколений таблицы);
//   - warmInvalidated    — списанных сертификатов при смене сессии;
//   - warmConvergence    — гистограмма сходимости холодного старта
//     (первый updateNewChat → последний);
//   - updatesBacklog     — глубина очереди r.updates (sentinel реального
//     риск-окна: whitelist-флуд × медленный consumer).
//
// Провайдер метрик в приложении пока не настроен (нет meter exporter):
// регистрация идёт через глобальный otel.Meter и до его подключения все
// инструменты — no-op. Это сознательно: контракт счётчиков фиксируется
// здесь, включение экспорта — отдельная операционная задача.
var (
	meter = otel.Meter("github.com/pure-golang/budva-claude/internal/infra/telegram")

	warmMisses    metric.Int64Counter
	warmReadiness metric.Int64Counter
	warmTimeouts  metric.Int64Counter

	warmLoadChatsErrors metric.Int64Counter
	warmSessionSwaps    metric.Int64Counter
	warmInvalidated     metric.Int64Counter

	warmConvergence metric.Int64Histogram

	updatesBacklog metric.Int64Gauge

	metricsOnce sync.Once
)

// warmReadinessOutcome — значения label outcome для warmReadiness.
const (
	outcomeSuccess = "success"
	outcomeTimeout = "timeout"
)

func initMetrics() {
	metricsOnce.Do(func() {
		var err error
		if warmMisses, err = meter.Int64Counter(
			"budva.telegram.warmup.misses",
			metric.WithDescription("Wait-windows opened on chat-not-found"),
		); err != nil {
			panic(err) // конфигурация API статична; ошибка — программный дефект
		}
		if warmReadiness, err = meter.Int64Counter(
			"budva.telegram.warmup.readiness",
			metric.WithDescription("Wait-window outcomes by kind"),
		); err != nil {
			panic(err)
		}
		if warmTimeouts, err = meter.Int64Counter(
			"budva.telegram.warmup.timeouts",
			metric.WithDescription("Deadline expiries (alert: rate > 2%)"),
		); err != nil {
			panic(err)
		}
		if warmLoadChatsErrors, err = meter.Int64Counter(
			"budva.telegram.warmup.loadchats_errors",
			metric.WithDescription("LoadChats iteration errors"),
		); err != nil {
			panic(err)
		}
		if warmSessionSwaps, err = meter.Int64Counter(
			"budva.telegram.warmup.session_swaps",
			metric.WithDescription("TDLib session changes (table swaps)"),
		); err != nil {
			panic(err)
		}
		if warmInvalidated, err = meter.Int64Counter(
			"budva.telegram.warmup.invalidated",
			metric.WithDescription("Certificates dropped at session swap"),
		); err != nil {
			panic(err)
		}
		if warmConvergence, err = meter.Int64Histogram(
			"budva.telegram.warmup.convergence_ms",
			metric.WithDescription("Cold-start convergence: first to last updateNewChat, ms"),
			metric.WithUnit("ms"),
		); err != nil {
			panic(err)
		}
		if updatesBacklog, err = meter.Int64Gauge(
			"budva.telegram.updates_backlog",
			metric.WithDescription("r.updates channel backlog depth"),
		); err != nil {
			panic(err)
		}
	})
}

// outcomeAttr — атрибут исхода для warmReadiness.
func outcomeAttr(outcome string) attribute.KeyValue {
	return attribute.String("outcome", outcome)
}

// convergenceTracker копит окно сходимости холодного старта: от первого
// updateNewChat сессии до каждого последующего; разрыв между update-ами
// больше convergenceGap сбрасывает окно (новый холодный старт / новая
// сессия), а запись в гистограмму делается по каждому update в окне —
// распределение «сколько длится волна материализации».
type convergenceTracker struct {
	mu      sync.Mutex
	started time.Time
	first   bool
}

const convergenceGap = 30 * time.Second

func newConvergenceTracker() *convergenceTracker {
	return &convergenceTracker{first: true}
}

// record отмечает updateNewChat. Потокобезопасно: вызывается из
// listenUpdates (единственный горутина-потребитель listener-а).
func (t *convergenceTracker) record() {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.first || now.Sub(t.started) > convergenceGap {
		// Первая волна (или давно было тихо) — открываем новое окно.
		t.started = now
		t.first = false
		return
	}
	elapsed := now.Sub(t.started)
	if elapsed > 0 {
		warmConvergence.Record(context.Background(), elapsed.Milliseconds())
	}
}
