# Approval — progressive streaming design lock

- task: `.trellis/tasks/11-25-facade-file-download`
- revision_baseline: `518bfcc3b167512f089403d1709fb5bd38a6c2e0`
- public_api_baseline: `fbc22cc`
- design: `design.md`
- design_sha256: `ff408bb8ae45f9155468a271ac285ac2bf5578f53d08bf010d1010e2d475147d`
- prd_sha256: `6d0055f5f8e05acbb7c3f3a8983940b1ac4381126bd791c3da4df4386de9a57d`
- approved_round: 1 (fresh independent streaming gate)
- verdict: PASS, done=true, D1-D8 met, blocking_findings empty
- reviewer: configured Design Reviewer profile, agent
  `36594bbf-03bb-4428-b10e-c5920fb60433`, server `srv_mJRvlG6BX3ck`
- findings: `review/streaming-design-findings.json`
- recorded_at: 2026-11-26
- implementation_base: planning-only commit containing this lock, subject
  `docs(task): lock progressive facade download design (11-25)`;
  orchestrator supplies its resolved SHA to implementation/review dispatch.

The reviewed design bytes are unchanged. Its opening proposed-status line
describes the submission state; this signed-off gate record establishes the
current implementation lock without modifying the reviewed artifact.

This lock supersedes the rejected path-only design. Historical approval and
reviews are retained in review/path-only; old green CI validates only the
old implementation. This is a design gate, not runtime or owner acceptance.

The reviewer verified pinned SDK signatures, native safe-prefix/EOF semantics,
main compatibility and every A1-A8 verification landing point. Non-blocking
implementation reminders: model actual Full-location remaining bytes in the
unknown-size fake; enforce a unique CI mutation anchor; label all old CI as
historical. See the report for the complete assumptions list.

Implementer may not change PRD/design/plan/spec. If a real conflict is found,
write review/spec-gap.md and stop the affected portion. Merge remains an owner
gate; test image publication requires reviewed streaming code and Linux green.
