# Orchestrator preflight evidence

2026-11-26, while Implementer repairs the first partial delivery.

## Proto descriptor compatibility

Compiled both the published baseline proto (2af222b) and the candidate proto with pinned libprotoc 34.2 into FileDescriptorSets. A temporary Go checker uses the actual protobuf descriptor types and proto.Equal to verify each baseline message by name and every existing service method definition (including request/response types and streaming flags). Legacy enums, file/package/options and all remaining descriptor fields also match after removing the additive declarations.

Result: 17 legacy RPCs, 38 messages and 1 enum unchanged. Added exactly one unary ListMessageFiles RPC, four messages and three enums.

- Baseline descriptor SHA256: `54cad940b202334dcada44fa15684cda92fe307e6d2c2cb398ea9deab0b1e64f`.
- Candidate descriptor SHA256: `5ccb25fbb9c558e723f3491156ed351173551124931eb4a3666b7603f7158c5c`.
- Runtime checker: `.trellis/.runtime/message-files-proto-compat.go`; descriptors under `.trellis/.runtime/message-files-compat/`.

The initial checker compared message declarations by array position and rejected FileDownloadMetadata after new messages were inserted before it. That was a checker assumption about declaration ordering, not wire incompatibility. Corrected to match existing messages by name, preserving exact definition comparison; the corrected check succeeded. This evidence does not claim generated-file hash equality, runtime tests or architecture review yet.

## First-delivery repairs requested

The initial agent notification did not establish completion. Actual source inspection found compilation and behavior-test gaps: invalid multi-return call composition, index type mismatch, unused durationpb import, raw error identity incorrectly asserted after status mapping, cancel-after-return fixture canceled before invocation, and a bufconn case with already-complete upstream state instead of the required causal barrier. Evidence and acceptance files listed by responses.json did not yet exist. Sent concrete correction directions to the same Implementer, preserving the locked design and source scope. No independent Reviewer PASS is claimed before repair completion.

## Native validation readiness

See validation-environment.md. Authentic static TDLib link and baseline package compilation succeeded in an isolated Linux directory. Source snapshot tooling is prepared to hash every copied source file before testing changed code. This readiness check is not ListMessageFiles acceptance and does not validate a new published image.

## Independent binding regeneration

Re-generated both Go bindings into `.trellis/.runtime/message-files-regen` using libprotoc 34.2, protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.5.1, with the canonical source path and source_relative options. Both files match the implementation bytes:

- facade.pb.go: `a38e8d82a5a04da196ca25878e00e4029b358ac7667fd584a086cb518601df67`.
- facade_grpc.pb.go: `47f38d2410f9f71748411cb1448e9f0b87866277782bf619f70a141279b726b8`.
- api/proto/facade.proto remains git mode 120000, blob `e7a8db424bf9ead3f69b52ae76ad08243b47177a`.
- Ruler hashes still match approval.md: design `6f99747c01261442760108e8377199410eea48e535980a8918e74b41b1849059`, PRD `8d0dd16fa2a12420d635cd044c8b21c8a2bdf31a4051693d8eaaa062bb5068a7`.

## Executed enumeration omission proof

On the isolated Linux snapshot, temporarily inserted `result.Items = nil` immediately after the unique real messageMediaItems assignment. This removes the enumerated result without changing compilation or using a production test hook.

`go test -p 1 -count=1 -timeout=60s ./internal/app/facade -run '^TestService_ListMessageFiles_(PhotoMutationBindsExtraction|PhotoSizesMetadataOnly)$'` returned 1. Both named tests failed literal length assertions (expected one item, got zero); PhotoSizesMetadataOnly also detected the lost default file ID. The mutation runner rejects build failures and panics as evidence. Restored exact original bytes in a finally block, checked SHA256 `a7fb0c334a020566120711e7751817c39e719952bd184f643541a2a630eb0015`, and the same tests returned 0.

The snapshot manifest was `0e10b077959d7f7f4e5850b1e6399bd50eaa927416d30071d9d1db86f8f124c9`. This proof covers the enumerator and those two tests at this snapshot; other focused tests still had the fixture failures recorded in progress.md. Later test repairs require a new snapshot and full runtime gate; do not treat this isolated proof as overall acceptance.
