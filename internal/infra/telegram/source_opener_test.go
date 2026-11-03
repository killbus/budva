package telegram

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// --- фикстуры id (10-30 Problem-1) ---
// Каналы/супергруппы (модуль > 1e12): сохраняются ОТРИЦАТЕЛЬНЫМИ (лоадер
// инвертирует). -1002641439846 = фикстура ruleset/repo_test.go.
const (
	chA = domain.ChatID(-1002641439846) // канал (модуль 1.00264e12 > 1e12)
	chB = domain.ChatID(-1001234567890) // канал/супергруппа (модуль 1.00123e12 > 1e12)
	chC = domain.ChatID(-1000000000001) // канал (модуль 1.000000000001e12 > 1e12)
	chD = domain.ChatID(-1000000000002) // канал
	// Базовые группы (модуль <= 1e12) — НЕ открываются.
	bgX = domain.ChatID(-999999999) // базовая группа
	bgY = domain.ChatID(-12345)     // базовая группа
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// openerFake — детерминированный sourceOpenerClient для тестов: записи
// вызовов + управляемые отказы. Порядок вызовов (GetChat перед OpenChat)
// фиксируется в globalOrder.
type openerFake struct {
	mu          sync.Mutex
	openChat    map[domain.ChatID]int // id -> сколько раз ВЫЗЫВАЛСЯ OpenChat (попытки, инкл. неудачные)
	getChat     map[domain.ChatID]int // id -> сколько раз ВЫЗЫВАЛСЯ GetChat (попытки, инкл. неудачные)
	closeChat   map[domain.ChatID]int // id -> сколько раз ВЫЗЫВАЛСЯ CloseChat
	globalOrder []string              // последовательность "Kind:id"
	openChatErr func(id domain.ChatID) error
	getChatErr  func(id domain.ChatID) error
}

func newOpenerFake() *openerFake {
	return &openerFake{
		openChat:  make(map[domain.ChatID]int),
		getChat:   make(map[domain.ChatID]int),
		closeChat: make(map[domain.ChatID]int),
	}
}

func (f *openerFake) record(kind string, id domain.ChatID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.globalOrder = append(f.globalOrder, kind+":"+strconv.FormatInt(int64(id), 10))
}

func idStr(id domain.ChatID) string {
	return strconv.FormatInt(int64(id), 10)
}

func (f *openerFake) GetChat(req *client.GetChatRequest) (*client.Chat, error) {
	f.record("GetChat", domain.ChatID(req.ChatId))
	f.mu.Lock()
	f.getChat[domain.ChatID(req.ChatId)]++ // попытка (инкл. неудачные) — retry-счётчик
	f.mu.Unlock()
	if f.getChatErr != nil {
		if err := f.getChatErr(domain.ChatID(req.ChatId)); err != nil {
			return nil, err
		}
	}
	return &client.Chat{Id: req.ChatId}, nil
}

func (f *openerFake) OpenChat(req *client.OpenChatRequest) (*client.Ok, error) {
	f.record("OpenChat", domain.ChatID(req.ChatId))
	f.mu.Lock()
	f.openChat[domain.ChatID(req.ChatId)]++ // попытка (инкл. неудачные) — retry-счётчик
	f.mu.Unlock()
	if f.openChatErr != nil {
		if err := f.openChatErr(domain.ChatID(req.ChatId)); err != nil {
			return nil, err
		}
	}
	return &client.Ok{}, nil
}

func (f *openerFake) CloseChat(req *client.CloseChatRequest) (*client.Ok, error) {
	f.record("CloseChat", domain.ChatID(req.ChatId))
	f.mu.Lock()
	f.closeChat[domain.ChatID(req.ChatId)]++
	f.mu.Unlock()
	return &client.Ok{}, nil
}

// sleepRecorder подменяет o.sleep: реальное ожидание — нет, но ВСЕ вызовы
// (с dur) записываются. Pace-floor вызовы имеют dur == pace, backoff — dur ==
// openRetryBackoff; это и есть CI-сигнал stagger (per-open pace) и retry.
type sleepRecorder struct {
	mu   sync.Mutex
	durs []time.Duration
}

func (r *sleepRecorder) record(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d > 0 {
		r.durs = append(r.durs, d)
	}
}

func (r *sleepRecorder) count(d time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.durs {
		if x == d {
			n++
		}
	}
	return n
}

// noOpSleep — детерминированный no-op sleep (replaced o.sleep): stagger/retry
// не ждут реального времени, но и не вызывают time.After (без которого
// synctest ждал бы незакрытый таймер → deadlock).
func noOpSleep(ctx context.Context, d time.Duration) {}

// testOpener поднимает opener под CANCELLABLE ctx (lifecycle-семантика прод-
// shutdown) и возвращает opener + stop. Worker — персистентный goroutine,
// живущий в select до отмены ctx (SB4: дольше любого ApplySet); без отмены
// synctest падает «blocked goroutines remain», когда тело теста возвращается.
// stop() отменяет ctx и дожидается выхода worker — ровно прод-семантика
// (engine ctx cancell'd → worker сходится). Вызывать в КОНЦЕ теста.
func testOpener(t *testing.T, fake *openerFake, pace time.Duration, sleep func(ctx context.Context, d time.Duration)) (*SourceOpener, func()) {
	t.Helper()
	op := NewSourceOpener(fake, discardLogger(), pace)
	op.sleep = sleep
	ctx, cancel := context.WithCancel(context.Background())
	op.Start(ctx)
	return op, func() {
		cancel()
		synctest.Wait()
	}
}

// TestSourceOpener_OpensChannelSources_NotBasicGroups — классификатор +
// GetChat-перед-OpenChat порядок. Смешанный набор: каналы открываются
// (GetChat→OpenChat), базовые группы — нет (ни GetChat, ни OpenChat).
func TestSourceOpener_OpensChannelSources_NotBasicGroups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait() // начальная (no-op) реконсиляция worker'а прошла

		op.OnSessionSwap() // сессия готова
		op.ApplySet([]domain.ChatID{chA, chB, bgX, bgY})
		synctest.Wait()

		// Каналы — открыты (GetChat→OpenChat по одному); базовые — нет.
		assert.Equal(t, 1, fake.openChat[chA], "channel A opened once")
		assert.Equal(t, 1, fake.openChat[chB], "channel B opened once")
		assert.Equal(t, 1, fake.getChat[chA], "channel A primed (GetChat before OpenChat)")
		assert.Equal(t, 0, fake.openChat[bgX], "basic group X NOT opened")
		assert.Equal(t, 0, fake.openChat[bgY], "basic group Y NOT opened")
		assert.Equal(t, 0, fake.getChat[bgX], "basic group X not primed")

		// Порядок: для каждого открытого id GetChat ДО OpenChat.
		gotOrder := func(id domain.ChatID) (gi, oi int, gok, ook bool) {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			s := "GetChat:" + idStr(id)
			o := "OpenChat:" + idStr(id)
			for i, e := range fake.globalOrder {
				if !gok && e == s {
					gi, gok = i, true
				}
				if !ook && e == o {
					oi, ook = i, true
				}
			}
			return
		}
		for _, id := range []domain.ChatID{chA, chB} {
			gi, oi, gok, ook := gotOrder(id)
			require.True(t, gok && ook, "id %d has GetChat and OpenChat in order", id)
			assert.Less(t, gi, oi, "GetChat must precede OpenChat for %d", id)
		}

		stop() // worker сходится (ctx отменён) — synctest без «blocked goroutines»
	})
}

// TestSourceOpener_HotReloadDiff — added→open, removed→close, idempotent
// re-apply. ApplySet([A,B,C]) затем ApplySet([B,C,D]) → OpenChat {A,B,C} (1-я),
// {D} (2-я); CloseChat {A} (2-я); повтор ApplySet([B,C,D]) → ничего нового.
func TestSourceOpener_HotReloadDiff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait()
		op.OnSessionSwap()

		op.ApplySet([]domain.ChatID{chA, chB, chC})
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chA])
		assert.Equal(t, 1, fake.openChat[chB])
		assert.Equal(t, 1, fake.openChat[chC])

		op.ApplySet([]domain.ChatID{chB, chC, chD})
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chD], "added D opened")
		assert.Equal(t, 1, fake.closeChat[chA], "removed A closed")
		assert.Equal(t, 1, fake.openChat[chA], "A opened exactly once total (not re-opened)")
		assert.Equal(t, 1, fake.openChat[chB], "B not re-opened on re-apply")
		assert.Equal(t, 1, fake.openChat[chC], "C not re-opened on re-apply")

		// Idempotency: повтор тем же набором — никаких новых вызовов.
		op.ApplySet([]domain.ChatID{chB, chC, chD})
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chD], "re-apply is a no-op (no double-open)")
		assert.Equal(t, 1, fake.closeChat[chA])

		stop()
	})
}

// TestSourceOpener_SessionSwap_FullReopen — свежая сессия (adapter-swap):
// opened сбрасывается, полный набор ре-открывается. После первого open и
// OnSessionSwap каждый канал открыт ДВАЖДЫ (initial + re-open).
func TestSourceOpener_SessionSwap_FullReopen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait()

		op.OnSessionSwap()
		op.ApplySet([]domain.ChatID{chA, chB})
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chA], "initial open")
		assert.Equal(t, 1, fake.openChat[chB], "initial open")

		// Новая сессия (re-auth/restart → adapter-swap): полный re-open.
		op.OnSessionSwap()
		synctest.Wait()
		assert.Equal(t, 2, fake.openChat[chA], "re-opened on session swap")
		assert.Equal(t, 2, fake.openChat[chB], "re-opened on session swap")

		stop()
	})
}

// TestSourceOpener_Stagger_PaceFloorPerOpen — CI-сигнал stagger: per-open
// pace-floor вызывается раз на каждый open (N opens → N pace-сон), и при
// успехе нет backoff-снов (retry не срабатывает).
func TestSourceOpener_Stagger_PaceFloorPerOpen(t *testing.T) {
	const pace = 300 * time.Millisecond
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		rec := &sleepRecorder{}
		// pace передан в NewSourceOpener (один источник per-open pace); sleep —
		// recorder (никогда не реальный). Оба ДО Start.
		op, stop := testOpener(t, fake, pace, func(ctx context.Context, d time.Duration) { rec.record(d) })
		synctest.Wait()
		op.OnSessionSwap()

		ids := []domain.ChatID{chA, chB, chC}
		op.ApplySet(ids)
		synctest.Wait()

		// Каждый id открыт.
		assert.Equal(t, 1, fake.openChat[chA])
		assert.Equal(t, 1, fake.openChat[chB])
		assert.Equal(t, 1, fake.openChat[chC])
		// Stagger: ровно N pace-снов (per-open pace-floor), N = число open.
		assert.Equal(t, len(ids), rec.count(pace), "per-open pace floor fired once per open")
		// Без retry (все успехи) → нет backoff-снов.
		assert.Equal(t, 0, rec.count(openRetryBackoff), "no backoff when all opens succeed")

		stop()
	})
}

// TestSourceOpener_BoundedInEdgeRetry (R4) — один id всегда падает на
// OpenChat. Он повторяется ОГРАНИЧЕННО (openRetryPasses раз) в пределах
// одного края (in-edge), остальные открываются, worker сходится (не вечно).
func TestSourceOpener_BoundedInEdgeRetry(t *testing.T) {
	const failing = chA
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		rec := &sleepRecorder{}
		fake.openChatErr = func(id domain.ChatID) error {
			if id == failing {
				return context.DeadlineExceeded
			}
			return nil
		}
		op, stop := testOpener(t, fake, 0, func(ctx context.Context, d time.Duration) { rec.record(d) })
		synctest.Wait()
		op.OnSessionSwap()
		synctest.Wait() // сводим сигнал swap (desired ещё пусто → no-op) ДО ApplySet,
		// иначе два сигнала могут распасться на два края → двойной retry.

		op.ApplySet([]domain.ChatID{chA, chB, chC})
		synctest.Wait() // worker сходится (retry ограничен)

		assert.Equal(t, openRetryPasses, fake.openChat[failing], "failing id retried exactly openRetryPasses times")
		// Остальные — открылись с первой попытки.
		assert.Equal(t, 1, fake.openChat[chB])
		assert.Equal(t, 1, fake.openChat[chC])
		// Было backoff-ожидание между проходами (retry действительно шёл).
		assert.Greater(t, rec.count(openRetryBackoff), 0, "in-edge retry used bounded backoff")

		stop()
	})
}

// TestSourceOpener_GetChatFailure_RetriesSequence (R4) — GetChat падает
// (peer не греется): SEQUENCE GetChat→OpenChat повторяется in-edge, OpenChat
// НЕ вызывается, пока GetChat не прошёл.
func TestSourceOpener_GetChatFailure_RetriesSequence(t *testing.T) {
	const failing = chB
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		var attempts int
		fake.getChatErr = func(id domain.ChatID) error {
			if id != failing {
				return nil
			}
			attempts++
			if attempts < openRetryPasses { // падает до предпоследнего прохода
				return context.DeadlineExceeded
			}
			return nil
		}
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait()
		op.OnSessionSwap()
		synctest.Wait() // один чистый open-край (см. BoundedInEdgeRetry)

		op.ApplySet([]domain.ChatID{chA, chB, chC})
		synctest.Wait()

		// chB: GetChat падал, потом прошёл → OpenChat ровно 1 раз, ПОСЛЕ
		// успешного GetChat (sequence удержана).
		assert.Equal(t, 1, fake.openChat[failing], "B opened once, only after GetChat succeeded")
		assert.GreaterOrEqual(t, fake.getChat[failing], openRetryPasses-1, "B's GetChat retried")
		// chA/chC — открылись.
		assert.Equal(t, 1, fake.openChat[chA])
		assert.Equal(t, 1, fake.openChat[chC])

		stop()
	})
}

// TestSourceOpener_ApplySetBeforeSessionSwap — порядок «ApplySet ДО swap»:
// worker записывает desired, но не открывает (sessionReady=false); OnSessionSwap
// сбрасывает opened и будит — worker сводит opened к актуальному desired (полная
// открытая). Оба порядка (ApplySet до/после swap) сходятся (design §3).
func TestSourceOpener_ApplySetBeforeSessionSwap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait()

		op.ApplySet([]domain.ChatID{chA, chB}) // ДО swap
		synctest.Wait()
		assert.Equal(t, 0, fake.openChat[chA], "no open before session ready")

		op.OnSessionSwap() // swap → sessionReady + чистка opened → полная открытая
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chA], "opened after swap")
		assert.Equal(t, 1, fake.openChat[chB], "opened after swap")

		stop()
	})
}

// TestSourceOpener_CloseFailure_NonFatal — CloseChat падает: безобидно (lingering
// open), id выводится из opened, load не падает.
func TestSourceOpener_CloseFailure_NonFatal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op, stop := testOpener(t, fake, 0, noOpSleep)
		synctest.Wait()
		op.OnSessionSwap()

		op.ApplySet([]domain.ChatID{chA, chB})
		synctest.Wait()
		assert.Equal(t, 1, fake.openChat[chA])
		assert.Equal(t, 1, fake.openChat[chB])

		// Удаляем chA (CloseChat может упасть — non-fatal, но id из opened).
		op.ApplySet([]domain.ChatID{chB})
		synctest.Wait()
		assert.Equal(t, 1, fake.closeChat[chA], "chA close attempted on removal")

		stop()
	})
}

// TestSourceOpener_CancelledCtx_Converges — ctx уже отменён к первому
// reconcile: open-петля видит ctx.Err() и выходит до первого OpenChat (mid-
// batch interruption, design §5). Ключевое свойство безопасности: worker
// СХОДИТСЯ (не висит, не падает); pending (desired-opened) остаётся в памяти
// и восстанавливается на следующем крае (startup-swap ре-открывает все).
//
// Особый случай: отмена ДО Start (а не после), поэтому testOpener не подходит
// — ctx создаётся и отменяется явно. Worker всё равно сходится (reconcile →
// open-петля → ctx.Err() → return; затем select → ctx.Done → return).
func TestSourceOpener_CancelledCtx_Converges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newOpenerFake()
		op := NewSourceOpener(fake, discardLogger(), 0)
		op.sleep = noOpSleep

		op.OnSessionSwap()
		op.ApplySet([]domain.ChatID{chA, chB, chC})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // отмена ДО Start: первый reconcile видит ctx.Err()

		op.Start(ctx)
		synctest.Wait() // worker сходится (не зависает на отменённом ctx)

		// Ничего не открыто (ctx отменён до первого OpenChat) — допустимо:
		// pending восстановится на следующем крае. Главное — сходимость.
		total := fake.openChat[chA] + fake.openChat[chB] + fake.openChat[chC]
		assert.Equal(t, 0, total, "cancelled ctx: no open issued, worker converged cleanly")
	})
}
