#!/bin/sh
set -eu
ROOT=/home/ying/budva-push-validation-20261011
export GOTOOLCHAIN=auto GOCACHE="$ROOT/go-cache" GOMODCACHE="$ROOT/go-mod"
export CGO_ENABLED=1 CGO_CFLAGS="-I$ROOT/linux-deps/include"
export CGO_LDFLAGS="-L$ROOT/linux-deps" LD_LIBRARY_PATH="$ROOT/linux-deps"
cd "$ROOT/source"
exec /usr/local/go/bin/go "$@"
