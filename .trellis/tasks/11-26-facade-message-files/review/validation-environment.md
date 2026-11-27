# Validation environment check

2026-11-26, read-only checks while the Implementer works.

## Local Windows

- No docker command found; WSL invocation returns install/help instead of an installed distribution.
- Pinned protoc 34.2 and both Go plugins are available as documented in the prior ticket.
- go-tdlib requires native Linux/Darwin cgo; Windows formatting/proto/static inspection cannot establish native tests passing.

## Existing debian SSH target

- Initial sandbox SSH could not verify the host key; rerunning through the approved escalation used the existing trust configuration successfully. No StrictHostKeyChecking bypass or host-key modification.
- uname reports Linux. /usr/local/go/bin/go reports go1.21.4 linux/amd64; project go.mod requires 1.25.9.
- Expected /usr/local/include/td/telegram/td_json_client.h and /usr/local/lib/libtdjson_static.a were absent.
- Docker executable exists, but docker version/image list cannot access /var/run/docker.sock as this account. A read-only sudo -n check reports a password is required. No privilege configuration was changed.
- No test source upload, package installation, container launch or remote filesystem edit was performed.

## Native verification channel

Use the existing .github/workflows/ci-test.yml after reviewed implementation publication under the applicable authorization: pinned real TDLib, go vet ./..., go test ./internal/..., and the existing progressive mutation script. New enumeration-specific mutation evidence must also be executed/recorded according to design section 9. Do not reuse the earlier 2af222b green run as evidence for ListMessageFiles. No workflow changes are in this task's source scope.

Current status: native verification pending. This environment check is not a task blocker for implementation/static independent review and is not runtime test evidence.

## Isolated user-directory validation fallback

Subsequent capability checks found GCC/G++ 12.2, OpenSSL/zlib development headers, Python 3.11 and approximately 6.5 GB initially free. A native validation environment was therefore prepared in `/tmp/budva-message-files-aQCDMb`, owned by the existing SSH account, without Docker, sudo, installed-system changes or services.

- Libraries: anonymous read of `ghcr.io/killbus/budva` immutable amd64 manifest `sha256:3fd626f51797bc8815725a078139970a05cf852dcf450d635b965d831c4dc8e0`; verified manifest/config hashes and revision annotation `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`. Selected the image's `COPY /usr/local/lib/libtd*` layer `sha256:e0ae5739f460e290cd4408421747cb09de44d6b683f41df6cb83782389303abe` by actual image history and verified its hash. Extracted only those library entries (208 MB) into the task prefix, with path/link guards. No account token was used or logged; the registry's anonymous pull token stays in process memory.
- Header: real td_json_client.h from native TDLib commit `22d49d5b87a4d5fc60a194dab02dd1d71529687f`. Short-SHA raw/API URLs returned 404; the previously recorded full SHA worked.
- tdjson_export.h is generated upstream by `generate_export_header(tdjson ...)` at that commit's CMakeLists.txt:1258. Generated the visibility-macro header with that same invocation and CMake 3.25.2 (checksum-verified PyPI wheel, task directory only). The minimal CMake probe configures a shared target to generate visibility macros; no probe library was built or linked, and no TDLib functions were replaced/stubbed. Actual static archives above remain the linker inputs.
- Go 1.25.9 linux/amd64: official archive SHA256 checked against go.dev release metadata; task-local extraction. Go module/build caches and CGO include/library overrides stay under the task directory. GOMAXPROCS=2 and build parallelism=1 bound resource use.
- Bootstrap and export-header evidence JSON remain in the remote temporary directory. Local scripts are under `.trellis/.runtime/`, not product source or CI changes.
- Published baseline source archive SHA256 `9bfbdbd3dee0b9d08a140d5c443c1db8ddb31a099106f1a12d2b77cfc5074c38` was checked remotely before extraction. The initial focused-name invocation on this baseline checks environment compilation/linking only: ListMessageFiles did not exist there and no matching test is expected. Never count this as new-feature test success.

Native link check completed: app/facade and transport/grpc test packages compiled and linked against the actual libraries, with `[no tests to run]` as expected for the baseline ListMessageFiles filter. A separate C probe linked the same SDK library set and called td_execute(getOption(version)), returning `{"@type":"optionValueString","value":"1.8.40"}` without creating a Telegram session. Its first manual link command omitted -ltdapi and failed; the corrected full SDK list succeeded. Neither the initial command failure nor its correction required product source changes.

Go archive SHA256: `00859d7bd6defe8bf84d9db9e57b9a4467b2887c18cd93ae7460e713db774bc1`. CMake wheel SHA256: `f9587645cea5298d5eb20649f58a83cc53c9efa7d57b0716449a39f366a74986`. Generated visibility header SHA256: `d6c175a6488bd60be4d626fa5a180eb8f44ed77c3cf9f1f287f797c9a7fca080`. Bootstrap JSON and environment-link-check.log were retrieved into local `.trellis/.runtime/message-files-compat/`.

After Implementer finishes repairs, replace source inputs using an allowlisted archive plus per-file SHA256 manifest, run focused tests, then vet/full internal tests and required mutation proofs. Final evidence must identify the changed-source manifest and actual results. GitHub CI remains a separate publication gate; no workflow has been triggered for this uncommitted implementation. Environment is ready; new-feature runtime verification is pending.
