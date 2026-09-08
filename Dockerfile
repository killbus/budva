# VERSION — тег релиза, прошиваемый в бинарники через -ldflags "-X main.version".
# По умолчанию "dev" (локальная сборка). CI передаёт тег/v*-строку.
# Объявлен ДО первого FROM (глобальная область видимости) и повторно
# объявлен в builder-стадии: ARG до FROM виден только в FROM-строках,
# внутри стадии его нужно переобъявить.
ARG VERSION=dev

# Stage 0: Сборка TDLib C++
FROM dockerhub.timeweb.cloud/library/debian:bookworm AS tdlib-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++ gperf libssl-dev zlib1g-dev php-cli git make ca-certificates

RUN git clone https://github.com/tdlib/td.git /td && \
    cd /td && git checkout 22d49d5

# --parallel: cmake использует все ядра раннера. Коммит TDLib закреплён,
# поэтому слой детерминирован и полностью попадает в BuildKit-кэш CI.
RUN cd /td && mkdir build && cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local .. && \
    cmake --build . --parallel --target prepare_cross_compiling && \
    cd .. && php SplitSource.php && cd build && \
    cmake --build . --parallel --target install

# Stage 1: Go builder
FROM dockerhub.timeweb.cloud/library/golang:1.25.9-bookworm AS builder

# Переобъявление глобального ARG — см. комментарий у объявления до FROM.
ARG VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    libssl-dev zlib1g-dev && \
    rm -rf /var/lib/apt/lists/*

COPY --from=tdlib-builder /usr/local/include/td /usr/local/include/td/
COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Без -a (форсирующая пересборка всех зависимостей): три бинарника в одном
# RUN-слое делят общий кэш компиляции — полная сборка выполняется один раз.
# version прошивается в facade и engine (`--version`); stand значение не
# печатает, но включён в ту же сборку без специального случая.
# ${VERSION:-dev} — страховка: если ARG потерян, прошиваем "dev", а не пустую
# строку (пустой -X затирает дефолт "dev" в var version пустотой).
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -X main.version=${VERSION:-dev}" -o /bin/facade ./cmd/facade && \
    CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -X main.version=${VERSION:-dev}" -o /bin/engine ./cmd/engine && \
    CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -X main.version=${VERSION:-dev}" -o /bin/stand ./cmd/stand

# Stage 2: Runtime
FROM dockerhub.timeweb.cloud/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 libssl3 zlib1g && \
    rm -rf /var/lib/apt/lists/*

COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

RUN adduser --disabled-password --gecos '' appuser && \
    mkdir -p /app && chown appuser:appuser /app
USER appuser
WORKDIR /app

COPY --from=builder --chown=appuser:appuser /bin/facade /app/facade
COPY --from=builder --chown=appuser:appuser /bin/engine /app/engine
COPY --from=builder --chown=appuser:appuser /bin/stand /app/stand
COPY --from=builder --chown=appuser:appuser /app/ruleset.yml /app/ruleset.yml
COPY --from=builder --chown=appuser:appuser /app/.env.example /app/.env

# HTTP (7070, WEBSERVER_PORT) и gRPC (50051, GRPC_PORT).
EXPOSE 7070 50051
