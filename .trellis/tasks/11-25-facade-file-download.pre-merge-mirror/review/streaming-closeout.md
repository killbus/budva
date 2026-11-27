# Progressive streaming delivery status

This record supersedes closeout.md for the current scope. The older path-only
implementation and reports remain historical evidence.

## Completed gates

- Independent design review PASS, locked SHA256
  ff408bb8ae45f9155468a271ac285ac2bf5578f53d08bf010d1010e2d475147d.
- Independent static code review PASS, round 1, zero blockers;
  streaming-findings.json and streaming-review-followup.md.
- Source committed and pushed: 2af222b66fee16ed9b4c0eca3f80d10b4d3bcaf4.
  The 15 main RPCs remain unchanged; the two new RPCs stream bytes.
- Windows cgo-free acceptance client build and 12 simulated cases passed.
- Linux CI 45733805744 passed on that source: real pinned TDLib, vet,
  internal tests and both-RPC causal green/red/green mutation proof.
- PR #16 description now reflects progressive byte delivery and executed CI.
- Native amd64/arm64 build, smoke and image publication succeeded in run
  45734082328 at 2026-11-26T01:46:04Z. Verified registry index content digest:
  sha256:9e4cd5de3f3cacde1110b304921ebf31f0d5d9bec4843d9c9910b6ac4e2221ac.
  Index and both image manifests identify source 2af222b and version
  dev-run23-2af222b. Read-only heartbeat 42f00328 was deleted.

## In progress and remaining

1. Pinned manifest reference, proto retrieval, link-to-ID lookup and client
   commands are delivered in streaming-acceptance.md.
2. Live Telegram acceptance needs an accessible logged-in facade, a suitable
   uncached file and its independent digest. Automated fake-upstream proof
   and startup smoke do not establish real Telegram timing or integrity.
3. Merge only with explicit owner approval, then archive/journal the task.

The task remains in progress. No live acceptance or merge is claimed.
