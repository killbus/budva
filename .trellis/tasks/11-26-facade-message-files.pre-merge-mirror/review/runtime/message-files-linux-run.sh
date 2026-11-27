#!/usr/bin/env bash
set -euo pipefail
validation_root=/tmp/budva-message-files-aQCDMb
export PATH="$validation_root/go/bin:$PATH"
export GOTOOLCHAIN=local
export GOPATH="$validation_root/gopath"
export GOMODCACHE="$validation_root/gomodcache"
export GOCACHE="$validation_root/gocache"
export GOMAXPROCS=2
export CGO_ENABLED=1
export CGO_CFLAGS="-I$validation_root/tdlib/include"
export CGO_LDFLAGS="-L$validation_root/tdlib/lib"
cd "$validation_root/source"
case "${1:-focused}" in
  focused)
    go test -p 1 -count=1 -timeout=120s ./internal/app/facade ./internal/transport/grpc -run 'ListMessageFiles'
    ;;
  vet)
    go vet -p 1 ./...
    ;;
  suite)
    go test -p 1 -count=1 ./internal/...
    ;;
  progressive)
    bash scripts/verify-facade-streaming.sh
    ;;
  enumeration)
    python3 "$validation_root/message-files-enumeration-mutation.py"
    ;;
  selection)
    python3 "$validation_root/message-files-selection-mutation.py"
    ;;
  *)
    echo 'Unknown validation mode' >&2
    exit 2
    ;;
esac
