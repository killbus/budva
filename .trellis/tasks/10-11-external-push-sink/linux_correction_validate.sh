#!/bin/sh
set -eu
BASE=/home/ying/budva-push-validation-20261011
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export GOTOOLCHAIN=auto GOCACHE="$BASE/go-cache" GOMODCACHE="$BASE/go-mod"
export CGO_ENABLED=1 CGO_CFLAGS="-I$BASE/linux-deps/include"
export CGO_LDFLAGS="-L$BASE/linux-deps" LD_LIBRARY_PATH="$BASE/linux-deps"
cd "$HERE/source"
exec /usr/local/go/bin/go "$@"
