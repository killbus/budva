# Streaming review follow-up

Independent static code review PASS, round 1, zero blocking findings, recorded
in streaming-findings.json. Design/PRD remain locked. The following changes
address its four nonblocking observations without changing production code:

- NB-1: Recounted all six Telegram test files: 5+1+26+10+5+24=71.
  Updated both package tables (facade 36, grpc 65, telegram 71), listed all
  Telegram test files and corrected the displayed-row total to 529.
- NB-2: Renamed the successful passthrough subtest and added actual 429
  responses for GetFile, GetFileDownloadedPrefixSize and ReadFilePart, with
  typed FloodWaitError and retry-delay assertions.
- NB-3: Added DeadlineExceeded-to-DeadlineExceeded cases to both streaming
  RPC error tables.
- NB-4: verify-facade-streaming.sh was staged and committed with mode 100755,
  verified before publication; the existing CI bash invocation stays unchanged.

These are test/documentation follow-ups to the independent static PASS.
Linux vet/tests and A1 green/red/green passed at source 2af222b in run
45733805744; see streaming-ci-evidence.md. Image publication and live Telegram
acceptance remain pending. Historical path-only results do not satisfy them.
