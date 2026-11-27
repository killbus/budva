# B1 integration fixes before independent code review

Baseline remains 8faac1bf45319b917936d2e1f923f5af28b0582c. Locked design
ff408bb8ae45f9155468a271ac285ac2bf5578f53d08bf010d1010e2d475147d is unchanged.
These fixes supplement the Implementer's partial B1 report; that report's
static-completeness claims are not accepted as runtime evidence.

## Concrete source observations and fixes

- App service test used an unqualified wantMime; corrected to table value.
  Two failure fixtures modeled an active download forever while expecting
  an error; they now model inactive incomplete state, as design §7 requires.
- Transport A1 lacked io import, initialized file ID to zero, inspected a
  mock call log that its overridden methods never populated, and released
  only a channel without publishing completion on the async path. Added
  actual request recording, valid ID, locked/idempotent state transition.
- A1 called t.Run inside synctest.Test, which Go 1.25 forbids. Each ordinary
  subtest now creates its own bubble with a context deadline, diagnostic
  assertion on failed payload reception and cleanup that releases upstream
  waiters. Bufconn uses passthrough resolver; connections/listener/server are
  closed in bubble cleanup. Assertions check that a nonempty client chunk
  precedes opening the completion barrier, then check exact final bytes.
- Send-failure fixture failed the first send while asserting it succeeded;
  made metadata successful and the next send fail. Context transport test
  now asserts the gRPC status instead of errors.Is against a raw context
  error (status errors intentionally replace raw errors on the wire).
- App tests lacked sync import; prefix boundary fixtures returned a slice
  larger than the requested count and expected a truncated cursor to equal
  the source length. Replaced with event-driven prefix exposure and bounded
  real slices, exercising a full 256KiB chunk and exact complete bytes.
- Callback-error/cancel-after-chunk tests acted on metadata while expecting
  later read calls. They now act on actual chunks. Deadline fake now waits
  for the actual context deadline inside synctest. Unknown final Size test
  drains the true remaining bytes in completed state before successful EOF.
- Fixed a completion-race fixture that promised a 100-byte success after zero
  delivered bytes; empty completion now asserts zero inside a timer bubble.
  Backpressure fixture waits for a durably blocked callback and checks state
  calls before release. Same-file fixture's observation channel is nonblocking
  after initial synchronization so later prefix queries cannot deadlock.
- Added photo tie/missing-message, nil/negative prefix, nil-part and cancelled
  completion-callback coverage. App checks context at each loop entrance and
  after successful completion emit. Removed manual min helper in favor of
  built-in min, and uses timer.Stop on cancellation with Go 1.25 timers.

## Validation state

Executed gofmt and git diff --check: clean. Design hash unchanged. No Windows
TDLib Go compilation/test execution was attempted. Linux full CI and A1
mutation remain required; these fixes are not an independent review verdict.

TEST-MATRIX now documents 36 app/facade, 65 grpc and 70 telegram top-level
test functions across each changed package. Other summary rows retain their
historical inventory (some are stale); displayed-row sum is 528. Coverage
percentages remain explicitly historical, not recomputed. New rows are marked
Linux pending. Test counts do not prove correctness or architecture.

Client simulated evidence and CI script are in streaming-client-evidence.md;
independent reviewer must include those orchestrator-owned artifacts as well
as Implementer-owned internal files in the final diff.
