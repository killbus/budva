package config

import (
	"time"

	agrpc "github.com/pure-golang/adapters/grpc/std"
	ahttp "github.com/pure-golang/adapters/httpserver/std"
	"github.com/pure-golang/platform/monitoring"
)

// Config описывает конфигурацию приложения из переменных окружения.
type Config struct {
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	Monitoring  monitoring.Config
	HTTPServer  ahttp.Config
	GRPCServer  agrpc.Config
	Telegram    TelegramConfig
	Storage     StorageConfig
	Ruleset     RulesetConfig
}

// TelegramConfig описывает параметры подключения к Telegram API.
type TelegramConfig struct {
	APIID               int32  `envconfig:"TELEGRAM_API_ID" required:"true"`
	APIHash             string `envconfig:"TELEGRAM_API_HASH" required:"true"`
	Phone               string `envconfig:"TELEGRAM_PHONE" required:"true"`
	DatabaseDirectory   string `envconfig:"TELEGRAM_DATABASE_DIR" default:".data/tdlib"`
	FilesDirectory      string `envconfig:"TELEGRAM_FILES_DIR" default:".data/tdlib-files"`
	SystemLanguageCode  string `envconfig:"TELEGRAM_SYSTEM_LANG" default:"en"`
	DeviceModel         string `envconfig:"TELEGRAM_DEVICE_MODEL" default:"Server"`
	LogVerbosityLevel   int32  `envconfig:"TELEGRAM_LOG_VERBOSITY" default:"0"`
	UseFileDatabase     bool   `envconfig:"TELEGRAM_USE_FILE_DB" default:"true"`
	UseChatInfoDatabase bool   `envconfig:"TELEGRAM_USE_CHAT_INFO_DB" default:"true"`
	UseMessageDatabase  bool   `envconfig:"TELEGRAM_USE_MESSAGE_DB" default:"true"`
	UseSecretChats      bool   `envconfig:"TELEGRAM_USE_SECRET_CHATS" default:"false"`
	SystemVersion       string `envconfig:"TELEGRAM_SYSTEM_VERSION" default:""`
	ApplicationVersion  string `envconfig:"TELEGRAM_APP_VERSION" default:"1.0.0"`
	LogDirectory        string `envconfig:"TELEGRAM_LOG_DIR" default:".data/tdlib-logs"`
	LogMaxFileSize      int64  `envconfig:"TELEGRAM_LOG_MAX_SIZE" default:"10"`

	// WarmupDeadline — окно wait-for-ready: сколько запрос готов ждать
	// материализации chat на холодной БД, прежде чем вернуть
	// ChatNotReadyError (маппится в gRPC Unavailable + RetryInfo).
	// Дефолт 5s = ~2.5s LoadChats-бутстрап + ~2s серверной пагинации +
	// запас (решение шестого раунда экспертной сессии, 4/4).
	WarmupDeadline time.Duration `envconfig:"TELEGRAM_WARMUP_DEADLINE" default:"5s"`
	// WarmupTicker — период тика ожидания (каждый тик ведёт LoadChats-драйв
	// с singleflight/rate-limit и повторяет вызов). Должен быть много
	// меньше WarmupDeadline.
	WarmupTicker time.Duration `envconfig:"TELEGRAM_WARMUP_TICKER" default:"500ms"`
}

// StorageConfig описывает параметры KV-хранилища BadgerDB.
type StorageConfig struct {
	DatabaseDirectory string `envconfig:"STORAGE_PATH" default:".data/badger"`
}

// RulesetConfig описывает параметры загрузки правил пересылки.
type RulesetConfig struct {
	Path string `envconfig:"RULESET_PATH" default:"ruleset.yml"`
}
