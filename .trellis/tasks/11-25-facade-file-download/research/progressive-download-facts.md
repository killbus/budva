# Progressive download facts — pinned dependencies

## Source identity

- go-tdlib: github.com/zelenin/go-tdlib v0.7.6, pinned in go.mod.
- Native TDLib: 22d49d5b87a4d5fc60a194dab02dd1d71529687f. The full SHA
  appears twice in successful ci-test run 45630679946's CMake Git state;
  its checkout step reports HEAD at 22d49d5b8.
- Public native source fetched at that exact full SHA:
  https://github.com/tdlib/td/blob/22d49d5b87a4d5fc60a194dab02dd1d71529687f/td/telegram/files/FileManager.cpp
- Local verification copies (not staged): main checkout
  `.trellis/.runtime/tdlib-22d49d5-streaming/`.
- Short SHA raw and commit-API URLs returned 404/422, so the full SHA from
  the successful CI log was used. Do not interpret that URL failure as
  evidence that the pinned checkout is invalid.

## Verified API and native behavior

1. client/function.go:11910-11924: DownloadFileRequest.Synchronous=false
   returns current file state just after initiating download. Offset/Limit
   are available; Limit=0 means no limit. The false value does not wait for
   full-file completion.
2. function.go:11948-11974: GetFileDownloadedPrefixSize takes FileId and
   Offset, reporting a contiguous readable prefix from that offset.
3. type.go:4209-4228: DownloadedPrefixSize relates to DownloadOffset.
   DownloadedSize is progress only: local physical size may include holes
   or garbage. It is not a safe read boundary.
4. function.go:12186-12220: ReadFilePart takes FileId, Offset, Count and
   returns FilePart.Data ([]byte, type.go:46995). Count=0 reads everything
   available; streaming must use a positive bounded count. Asking for more
   than is available returns an error.
5. Native FileManager.cpp:1042-1061: partial prefix uses the ready bitmask
   and part size; full files return size minus offset (zero at/past EOF).
   SecureEncrypted partial files return zero until decrypted and verified.
   No truthful design can emit bytes TDLib has not yet made readable.
6. Native FileManager.cpp:2836-2908: read_file_part verifies the requested
   range against downloaded_prefix. It resolves partial/full local paths
   internally. On partial-file read errors, it retries the temporary-to-
   persistent rename case after 0.01 seconds, bounded by left_tries. This
   is a concrete lifecycle benefit of ReadFilePart compared with app-owned
   handles to TDLib temporary paths. Its docs still note direct reads are
   faster; selection is a design tradeoff, not a performance guarantee.
7. function.go:11977-11999: CancelDownloadFile targets a file ID, not an
   individual RPC consumer. Cancellation ownership must account for two
   simultaneous readers of the same file.
8. client/client.go:104-129: SDK Send uses a background timeout with default
   catchTimeout=60 seconds, independent of an RPC context. Generated methods
   do not accept context. Do not claim immediate abort of an already-issued
   native call merely because the grpc context was cancelled.

## Existing repository boundary

`cmd/facade` constructs Repo with NoBusinessUpdates. The existing SDK pump
and Repo listener remain active but no business outlet is allocated. The
guidelines explicitly forbid treating Updates() as a subscription or adding
an unconsumed drain queue. Request streaming must not steal shared events
or apply client backpressure to the global TDLib receiver.

These are source facts, not an approved streaming design or live test.
The design gate must still choose exact signatures, ownership, polling or
notifications, terminal rules, and executable first-payload evidence.
