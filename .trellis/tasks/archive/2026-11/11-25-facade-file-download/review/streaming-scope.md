# Owner correction — progressive streaming

The owner asked whether gRPC can stream bytes and then explicitly rejected
"先让 TDLib 下载完整文件，再通过 gRPC 流式发送". The operative requirement is
Telegram -> TDLib progressive availability -> bounded gRPC byte chunks ->
client-local output, with data arriving before upstream completion.

This supersedes the former path-only PRD and invalidates the former design
lock for further implementation/merge decisions. Old evidence is retained
byte-for-byte under review/path-only, including design SHA256
e6f3d19ff4edf88df1138e56fa0c86533281359670cf8775393311b3b4b90c75.
No claim is made that the old CI PASS verifies progressive streaming.

Baseline for the revision is 518bfcc3b167512f089403d1709fb5bd38a6c2e0.
Original public API compatibility is measured against main fbc22cc (15
RPCs). The two new path-only RPCs are unreleased/unmerged and may be revised
in place. The user authorized the download work and its acceptance image;
that work continues with the corrected requirements. Publication remains
stopped until the corrected implementation is reviewed and verified. Merge
remains an owner gate. Do not recreate a broad autonomous publication timer.

Workflow: orchestrator revises PRD -> Design Reviser rewrites the invalid
design against the new requirements and pinned facts -> independent Design
Reviewer gates -> new hash lock -> Implementer -> independent code Reviewer
-> Linux validation and acceptance image. Revised design/spec artifacts are
authorized planning changes, not an implementation agent moving its ruler.
Record the new planning baseline/hash before code review.

The Design Reviser exhausted its output budget twice without artifacts;
the orchestrator has taken over design authorship while preserving the
independent Design Reviewer gate. A narrow Linux regression script plus
one call from the existing ci-test job implement PRD A7's executed red/green
requirement. Pins/cache/build flags/workflow triggers are unchanged. The
acceptance client is a cgo-free host command; Docker packaging is unchanged.
