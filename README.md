> "Можно потратить годы и целое состояние на изучение мастерства убивать драконов, чтобы в итоге осознать: в реальном мире драконов не существует" - Claude Code решил проблему невероятно быстрой переработки legacy.

# budva

Сервис пересылки Telegram-сообщений на чистом Go. Принимает обновления из нескольких источников, применяет правила фильтрации и трансформации, доставляет в целевые чаты. Поддерживает горячую перезагрузку правил, дедупликацию, группировку альбомов и rate limiting.

API доступно через GraphQL и gRPC. Аутентификация — интерактивный терминал.

Каждый пакет снабжён `doc.go` — техническая документация всегда актуальна. Бизнес-логика покрыта BDD-сценариями (`test/bdd/`) — живая спецификация вместо устаревших интеграционных тестов.

Легаси-проект на 5 000 строк (`budva43`) переработан за неделю [c покрытием тестами на 98%](docs/TEST-MATRIX.md) — результат тюнинга ИИ-генерации кода: частично применяемые интерфейсы дают несвязанные модули, каждый из которых легко тестируется и документируется независимо.

## Установка TDLib

TDLib — нативная библиотека Telegram. Требуется перед сборкой проекта. Инструкция: https://github.com/zelenin/go-tdlib/blob/master/README.md


## Установка проекта

Репозитории должны лежать рядом:

```
../level85
../budva
```

```bash
git clone https://github.com/pure-golang/level85.git
cd level85 && make
cd ..
git clone https://github.com/pure-golang/budva.git
cd budva
go mod download
```

`level85` подключает общий `Taskfile.yml`, skills и агенты-ревьюверы.

## Конфигурация

Создайте `.env` на основе `.env.example`:

```bash
cp .env.example .env
```

Обязательные переменные:

```env
TELEGRAM_API_ID=<your_api_id>
TELEGRAM_API_HASH=<your_api_hash>
TELEGRAM_PHONE=<+7xxxxxxxxxx>
```

API-ключи получить на https://my.telegram.org

Остальные настройки опциональны: `WEBSERVER_PORT` (default `7070`), `GRPC_PORT` (default `50051`), `STORAGE_PATH` (default `.data/badger`), `RULESET_PATH` (default `ruleset.yml`).

## Запуск

```bash
# HTTP + gRPC API сервер
go run ./cmd/facade

# Движок пересылки сообщений (с терминальной авторизацией)
go run ./cmd/engine

# Создать тестовые чаты (для разработки)
go run ./cmd/stand --up

# Удалить тестовые чаты
go run ./cmd/stand --down
```

## Docker

Сборка образа (то же, что использует CI — единственный путь сборки):

```bash
docker build -t budva:local .
docker build --build-arg VERSION=v0.1.0 -t budva:v0.1.0 .
```

Версия прошивается в бинарники (`facade --version` / `engine --version`, без аргументов — `dev`).

Готовые образы публикуются в GHCR по тегу `v*`:

```bash
docker pull ghcr.io/killbus/budva:latest
docker run --rm ghcr.io/killbus/budva:latest /app/facade --version
```

Запуск facade (переменные окружения — как в `.env.example`):

```bash
docker run -d --name budva-facade \
  -p 7070:7070 -p 50051:50051 \
  -e TELEGRAM_API_ID=... -e TELEGRAM_API_HASH=... -e TELEGRAM_PHONE=... \
  -v budva-data:/app/.data \
  ghcr.io/killbus/budva:latest /app/facade
```

Для engine замените команду на `/app/engine`.

## Правила пересылки

Редактируйте `ruleset.yml` — перезагрузка происходит без перезапуска сервиса. Схема конфига: `sources`, `destinations`, `forwardRules`.

## Тесты

```bash
# Unit-тесты (быстро)
task test-short

# Полный прогон включая BDD (требует Docker, ~12 мин)
task test
```

## Архитектура

```
cmd/
  engine/    — движок обработки сообщений
  facade/    — HTTP/gRPC API
  stand/     — утилита тестовых фикстур
api/
  graph/     — публичный GraphQL-контракт
  proto/     — публичный gRPC-контракт
internal/
  domain/    — бизнес-модели
  app/       — application services: auth, facade, handler
  service/   — domain services: album, dedup, filters, limiter, message, transform
  infra/      — infrastructure layer: Telegram, state, queue
  transport/ — user interface layer: HTTP, gRPC, terminal, health endpoints
test/
  bdd/       — BDD-сценарии (godog), живая бизнес-спецификация
  smoke/     — smoke-тесты liveness/readiness собранного стека
```
