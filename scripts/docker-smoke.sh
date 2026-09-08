#!/usr/bin/env bash
# Docker smoke tests for a built budva image.
#
# Pins three product contracts, in order:
#   1. version banner — `docker run <image> /app/facade --version` and
#      `/app/engine --version` must print the expected version exactly
#      (the image must not lie about its version);
#   2. facade health — with dummy Telegram credentials (config validation
#      requires non-empty values; TDLib auth can never complete with them,
#      and nothing in the checks below depends on it) the facade serves
#      /live, /ready, /healthcheck with 200;
#   3. engine startup — same dummy credentials, the engine logs
#      "Engine started" (TDLib client init runs in a background goroutine;
#      the log line does not wait for it).
#
# Usage:
#   scripts/docker-smoke.sh <image> <expected-version>
#
# The checks are deterministic and use no real Telegram credentials, no
# real network endpoints. Container ports are published on fixed high
# loopback ports (18070 for facade): no knobs.
#
# Reference pattern: ../reproxy scripts/docker-smoke.sh.

set -euo pipefail

usage() {
  echo "usage: $0 <image> <expected-version>" >&2
  echo "  <image>             single image reference, e.g. budva:local" >&2
  echo "  <expected-version>  exact string 'docker run <image> /app/<bin> --version' must print" >&2
}

if [[ "$#" -ne 2 ]]; then
  usage
  exit 2
fi

image="$1"
expected="$2"

# Run from the repo root (this script's parent directory), not the caller's
# cwd — anchoring on the script's own location makes that true from any
# invocation directory.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${script_dir}/.."

# Dummy Telegram credentials used below (checks 2 and 3): config.go marks
# TELEGRAM_API_ID/HASH/PHONE as required (envconfig refuses empty), but every
# check in this script passes before any Telegram interaction could matter.
# Real credentials must never appear here.
#
# Shared cleanup: every container this script starts is removed on exit,
# success or failure. `docker rm -f` on a never-started or already-removed
# container is shielded by `|| true`, so the unconditional rm is safe at any
# point.
cleanup() {
  docker rm -f budva-smoke-facade >/dev/null 2>&1 || true
  docker rm -f budva-smoke-engine >/dev/null 2>&1 || true
}
trap cleanup EXIT

# ---- Check 1: version banner ---------------------------------------------
# The image's version must not lie: both binaries must print the expected
# version exactly. The early-exit lives before config loading, so an image
# with a broken env cannot paper over a wrong version string. No env needed:
# the early-exit precedes envconfig (that is part of what this pins).
echo "check 1/3: version banner"
for bin in facade engine; do
  actual="$(docker run --rm "${image}" "/app/${bin}" --version)"
  echo "  ${bin}: expected '${expected}', actual '${actual}'"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "check 1/3 (version banner) FAILED: ${bin} version '${actual}' does not match expected '${expected}'"
    exit 1
  fi
done
echo "check 1/3: version banner OK"

# ---- Check 2: facade health endpoints ------------------------------------
# Start the facade with dummy credentials and probe the health endpoints.
# /live and /ready and /healthcheck are dependency-free 200s in the facade
# (no pingers registered — cmd/facade/main.go health.New() with no args).
echo "check 2/3: facade health endpoints"
docker run --detach --name budva-smoke-facade \
  -p 127.0.0.1:18070:7070 \
  -e TELEGRAM_API_ID=1 -e TELEGRAM_API_HASH=smoke -e TELEGRAM_PHONE=+10000000000 \
  "${image}" /app/facade

# Wait for the facade to accept connections (bounded retries; the
# if-condition context shields failed probes from the default bash -e so
# the loop can retry).
ready=""
for _ in $(seq 1 30); do
  if curl -s -o /dev/null --max-time 2 http://127.0.0.1:18070/live; then
    ready=1
    break
  fi
  sleep 1
done
if [[ -z "${ready}" ]]; then
  echo "check 2/3 (facade health) FAILED: facade did not become ready on 127.0.0.1:18070"
  docker logs budva-smoke-facade || true
  exit 1
fi

for path in /live /ready /healthcheck; do
  status="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:18070${path}")"
  echo "  ${path}: ${status}"
  if [[ "${status}" != "200" ]]; then
    echo "check 2/3 (facade health) FAILED: GET ${path} returned ${status}, want 200"
    docker logs budva-smoke-facade || true
    exit 1
  fi
done
echo "check 2/3: facade health OK"

docker rm -f budva-smoke-facade >/dev/null

# ---- Check 3: engine startup log -----------------------------------------
# The engine must reach its startup log line. No TTY and no stdin attached:
# the terminal transport goroutine hits stdin EOF and exits quietly (the
# main process keeps running) — this is exactly the CI condition.
echo "check 3/3: engine startup"
docker run --detach --name budva-smoke-engine \
  -e TELEGRAM_API_ID=1 -e TELEGRAM_API_HASH=smoke -e TELEGRAM_PHONE=+10000000000 \
  "${image}" /app/engine

started=""
for _ in $(seq 1 30); do
  if docker logs budva-smoke-engine 2>&1 | grep -q "Engine started"; then
    started=1
    break
  fi
  sleep 1
done
if [[ -z "${started}" ]]; then
  echo "check 3/3 (engine startup) FAILED: engine did not log 'Engine started'"
  docker logs budva-smoke-engine || true
  exit 1
fi
echo "check 3/3: engine startup OK"
