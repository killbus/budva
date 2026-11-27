# Progressive streaming Linux evidence

- Source commit: `2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4`.
- PR: https://github.com/killbus/budva/pull/16
- Run: https://github.com/killbus/budva/actions/runs/45733805744
- Job: https://github.com/killbus/budva/actions/runs/45733805744/job/139622063872
- Result: SUCCESS, completed 2026-11-26T01:10:18Z.

The pinned real TDLib installation was restored from cache and unpacked.
This run did not repeat the historical cold build. `go vet ./...` passed
at 01:09:59Z and `go test ./internal/...` passed at 01:10:07Z, including
the final review-follow-up flood-wait and deadline assertions.

## Executed causal streaming proof

`bash scripts/verify-facade-streaming.sh` passed at 01:10:15Z:

1. Original source: both `TestProgressiveDownloadBeforeCompletion/message`
   and `/file-id` passed at 01:10:10Z.
2. Temporary mutation: changed the unique starter from `Synchronous:false`
   to `true`. Both modes failed at 01:10:13Z with
   `PROGRESSIVE_PAYLOAD_BEFORE_COMPLETION`; the named parent test failed too.
   The script rejected build/setup/panic/global-timeout/no-test failures.
3. Original bytes restored and compared; both modes and their parent passed
   again at 01:10:15Z. No deliberately broken source was committed.

The test uses the real facade app, transport and generated gRPC client over
bufconn, with a controlled fake TDLib completion barrier. It demonstrates
nonempty client payload before releasing upstream completion for both RPCs.
It is deterministic architecture/protocol evidence, not real Telegram timing.

Raw job log was inspected and saved locally to
`D:/Repositories/budva/.trellis/.runtime/streaming-ci-45733805744.log`.
The log is scratch evidence, excluded from Git; the run URL is the durable
reference. Historical path-only CI results remain separate.

## Publication and remaining acceptance

Existing docker-publish workflow was dispatched only after checking this
successful run and the feature branch both resolve to the source SHA above.
It must build and smoke-test native amd64 and arm64 before publishing the
`dispatch-test` manifest. Publication details belong in streaming-acceptance.md.
No live Telegram download or merge is claimed by this record.
