# Progressive streaming design replacement

The Design Reviser agent a98b44dc-f2e8-49cc-af9e-84ae855617c9 exhausted its
output limit twice without writing either requested artifact. The orchestrator
therefore authored design.md against the corrected PRD and pinned dependency
facts; the independent Design Reviewer remains the gate owner.

The replacement specifies two server-stream RPCs, a synchronous app callback,
async TDLib start, bounded prefix reads, state-before-prefix EOF handling,
consumer-only cancellation and explicit SDK timeout limits. It preserves
the original-main RPCs and supersedes this ticket's unreleased path contract.

Acceptance includes a real app/transport bufconn barrier test, an executed
Linux synchronous-starter mutation with restore/green, and a cgo-free client
that writes a partial local file and finalizes only after complete + clean
EOF. A minimal existing-CI step supplies the Linux mutation channel. No
Docker packaging change is needed for the acceptance client.

No source was changed by this design replacement. Historical green CI and
reviews are not streaming evidence. Design is pending independent review;
the prior approval is still superseded and no new lock is claimed.
