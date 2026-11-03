package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	aenv "github.com/pure-golang/adapters/env"
	"github.com/pure-golang/platform/monitoring"

	"github.com/pure-golang/budva-claude/internal/app/auth"
	"github.com/pure-golang/budva-claude/internal/app/handler"
	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/infra/ruleset"
	"github.com/pure-golang/budva-claude/internal/infra/state"
	"github.com/pure-golang/budva-claude/internal/infra/telegram"
	"github.com/pure-golang/budva-claude/internal/infra/term"
	"github.com/pure-golang/budva-claude/internal/service/album"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/limiter"
	"github.com/pure-golang/budva-claude/internal/service/message"
	"github.com/pure-golang/budva-claude/internal/service/push"
	"github.com/pure-golang/budva-claude/internal/service/transform"
	tterm "github.com/pure-golang/budva-claude/internal/transport/term"
)

// version прошивается при сборке через -ldflags "-X main.version=<tag>"
// (Dockerfile build arg VERSION, CI-релизы). "dev" — локальная сборка.
var version = "dev"

func main() {
	// Баннер версии до загрузки конфигурации: `engine --version` не требует
	// env-переменных и не стартует движок. stdout — значение, которое
	// утверждает scripts/docker-smoke.sh и CI.
	if versionRequested(os.Args[1:]) {
		fmt.Println(version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// versionRequested сообщает, запрошен ли баннер версии первым аргументом.
// Проверка до envconfig: `--version` не должен падать на required-переменных.
func versionRequested(args []string) bool {
	return len(args) > 0 && args[0] == "--version"
}

func run() error {
	// 1. Контекст с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Конфигурация
	var cfg config.Config
	if err := aenv.InitConfig(&cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 3. Monitoring (logger, tracing, metrics)
	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	logger := slog.Default().With("module", "main")
	defer func() {
		if err := closeMonitoring(); err != nil {
			logger.Error("Failed to close monitoring", slog.Any("err", err))
		}
	}()

	logger.Info("Starting engine")

	// 4. Infra
	stateRepo := state.New(cfg.Storage)
	if err := stateRepo.Start(ctx); err != nil {
		return fmt.Errorf("state repo: %w", err)
	}
	defer func() {
		if err := stateRepo.Close(); err != nil {
			logger.Warn("Failed to close state repo", slog.Any("err", err))
		}
	}()

	rulesetRepo := ruleset.New(cfg.Ruleset)

	telegramRepo := telegram.New(cfg.Telegram, telegram.BusinessUpdates)

	// 10-30 Problem-1: Budva владеет границей аквизиции источников.
	// SourceOpener открывает (open-once-keep) каналы/супергруппы-источники в
	// собственной сессии. КРИТИЧНО (SB2): RegisterSessionSwap ДО Start —
	// runAuthLoop one-shot, и если callback не зарегистрирован к swap-строке,
	// fix никогда не применится (тихая перманентная тишина, воспроизводимая
	// рестартом). Worker живёт под engine ctx (SB4: lifecycle ctx дольше
	// любого ApplySet).
	sourceOpener := telegram.NewSourceOpener(telegramRepo, logger, cfg.Telegram.SourceOpenPace)
	telegramRepo.RegisterSessionSwap(sourceOpener.OnSessionSwap)

	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer func() {
		if err := telegramRepo.Close(); err != nil {
			logger.Warn("Failed to close telegram repo", slog.Any("err", err))
		}
	}()
	sourceOpener.Start(ctx)

	queueRepo := queue.New()
	if err := queueRepo.StartContext(ctx); err != nil {
		return fmt.Errorf("queue repo: %w", err)
	}
	defer func() {
		if err := queueRepo.Close(); err != nil {
			logger.Warn("Failed to close queue repo", slog.Any("err", err))
		}
	}()

	// 5. Сервисы
	authService := auth.New(telegramRepo)
	authService.Start(ctx)

	messageService := message.New()
	transformService := transform.New(telegramRepo, stateRepo)
	filterService := filters.New()
	albumService := album.New()

	limiterService := limiter.New()

	handlerService := handler.New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		transformService,
		albumService,
		queueRepo,
		limiterService,
		func(dsts []int64) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	// 6. Ruleset
	pushManager := push.NewManager()
	defer pushManager.Close()
	loadSnapshot := func(rs *domain.RuleSet) error {
		configs := make(map[string]push.EndpointConfig, len(rs.PushTargets))
		for name, target := range rs.PushTargets {
			timeout := 5 * time.Second
			if target.TimeoutSeconds > 0 {
				timeout = time.Duration(target.TimeoutSeconds) * time.Second
			}
			schema, err := target.EffectiveSchemaVersion()
			if err != nil {
				// The ruleset loader already rejects unsupported schemaVersion
				// values, so this default is unreachable in practice; keep a
				// v1 fallback rather than failing the whole generation load.
				logger.Warn("Invalid push target schemaVersion; defaulting to v1",
					slog.String("target", name), slog.Any("err", err))
				schema = 1
			}
			configs[name] = push.EndpointConfig{
				Timeout: timeout,
				Sink: push.NewWebhookSink(push.WebhookConfig{
					URL: target.URL, Headers: target.Headers, HMACSecret: target.HMACSecret,
					Client: &http.Client{Timeout: timeout},
					Schema: push.DeliverySchemaVersion(schema),
				}),
			}
		}
		// 10-30 Problem-1 (SB3): open-set гонится ПЕРЕД переключением push-
		// поколения — так backfill (open-собственный diff-fetch) стартует как
		// можно раньше, и инвариант «eventually open + backfilled» удерживается
		// (push в open in-flight-окне задерживается, но не теряется).
		// ApplySet идемпотентен и async: diff считается синхронно, RPC — в
		// фоновом worker (не держит push-менеджеров замок).
		sourceOpener.ApplySet(rulesetSourceIDs(rs))
		return pushManager.Replace(configs, func(g *push.Generation) { handlerService.SetRuleSetWithPush(rs, g) })
	}
	rs, err := rulesetRepo.Load()
	if err != nil {
		push.RecordReload(false)
		logger.Warn("Failed to load ruleset", slog.Any("err", err))
	} else {
		err = loadSnapshot(rs)
		push.RecordReload(err == nil)
		if err != nil {
			logger.Error("Failed to initialize push generation")
		}
	}

	// 7. Watcher для hot-reload
	if err := rulesetRepo.WatchContext(ctx, func() {
		newRS, loadErr := rulesetRepo.Load()
		if loadErr != nil {
			push.RecordReload(false)
			logger.Error("Failed to reload ruleset", slog.String("outcome", "reload_failed"), slog.Any("err", loadErr))
			return
		}
		if err := loadSnapshot(newRS); err != nil {
			push.RecordReload(false)
			logger.Error("Failed to reload push generation", slog.String("outcome", "reload_failed"))
			return
		}
		push.RecordReload(true)
		logger.Info("Ruleset reloaded")
	}); err != nil {
		logger.Warn("Failed to watch ruleset", slog.Any("err", err))
	}
	defer func() {
		if err := rulesetRepo.Close(); err != nil {
			logger.Warn("Failed to close ruleset repo", slog.Any("err", err))
		}
	}()

	// 8. Telegram update loop
	go handlerService.Run(ctx)

	// 9. Terminal transport
	termRepo := term.New(os.Stdin, os.Stdout, syscall.Stdin)
	termTransport := tterm.New(authService, telegramRepo, termRepo, cfg.Telegram.Phone)
	go func() {
		if err := termTransport.Run(ctx, cancel); err != nil {
			logger.Error("Terminal transport error", slog.Any("err", err))
		}
	}()

	logger.Info("Engine started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down engine")

	return nil
}

// rulesetSourceIDs — полный набор идентификаторов источников ruleset (ключи
// rs.Sources). Это ПОЛНЫЙ набор для SourceOpener.ApplySet: классификация по
// типу (открыть / не открыть) происходит внутри opener (domain
// .IsChannelOrSupergroup), поэтому здесь передаются ВСЕ ключи, включая
// приватные/базовые — opener сам отберёт каналы/супергруппы. Порядок
// недетерминирован (map), но для diff-семантики неважен.
func rulesetSourceIDs(rs *domain.RuleSet) []domain.ChatID {
	ids := make([]domain.ChatID, 0, len(rs.Sources))
	for id := range rs.Sources {
		ids = append(ids, id)
	}
	return ids
}
