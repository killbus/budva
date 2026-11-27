# Acceptance client and CI verification preparation

Orchestrator-owned files: cmd/facade-file-download-client/main.go,
scripts/verify-facade-streaming.sh, one added step in ci-test.yml, and
review/streaming-acceptance.md. Implementer owns internal production/tests.
These changes follow locked design §10-§12; no design/spec changes.

## Executed on Windows, 2026-11-26

- CGO_ENABLED=0, `go build -mod=readonly -o <runtime>/facade-file-download-client.exe ./cmd/facade-file-download-client`: exit 0.
- Built client `-h`: exit 0; expected flags available.
- Temporary cgo-free gRPC fixture launched on loopback and ran the actual
  executable. Twelve checks succeeded: complete exact bytes + known digest,
  missing completion, non-OK status after completion, wrong offset, data after
  completion, empty complete file, chunk before metadata, oversized chunk,
  duplicate metadata, digest mismatch, pre-existing target, pre-existing part.
  Failed cases left .part without final output; existing files remained exact.
- Fixture output/logs: C:/Users/ying/AppData/Local/Temp/facade-client-smoke-3240299027.
  Temporary fixture source is in main .trellis/.runtime/streaming-client/
  and feature .trellis/.runtime/client-smoke/ (never stage these).
- `bash -n scripts/verify-facade-streaming.sh`: exit 0 (Git Bash).
- `yq '.' .github/workflows/ci-test.yml`: exit 0.
- Scoped `git diff --check` clean; workflow diff adds only one named script
  call after existing internal tests, preserving pins/cache/build/triggers.

These checks validate the standalone consumer only. No production TDLib
build/test was executed on Windows, and no live Telegram result is claimed.
The Linux mutation script still awaits the real A1 test and execution in CI.

## Static Phase A observations sent to Implementer

Generated streaming handler signature/import and app-vs-pb callback mismatch,
incomplete fake ServerStream interface, nil success error mapping, stale
math import, nil media wrappers, wrapped flood-wait precedence, and context
checks were identified by reading actual source. Implementer has a scoped
repair plus Phase B1 test instruction; these are not resolved merely by
this record. Independent code review and Linux execution remain required.
