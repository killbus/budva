# Planning and execution: facade update ownership

## Goal

Implement and verify the reviewed facade zero-business-subscriber liveness repair, preserving internal event handling and subscribed delivery semantics. The original design-only deliverable is complete; the user authorized implementation with “开始推进” on 2026-09-11.

## Current phase

Native behavioral red/green execution and independent evidence reconciliation are complete. The approved six-file evidence commit 48f7a36 was pushed under separate user authority; PR #2 passed automatic ci-test 38103363230 and was subsequently merged to main with explicit user approval after the absent formal review approval was disclosed. The isolated-account actual-facade probe remains required before production closure, not evidence supplied by Docker health smoke or by merge. No deployment or live-account use occurred. The task is ready for Trellis archival after this five-file merge-state record is committed. Receipts and limitations are in research/native-red-green.md, research/findings.md, and research/progress.md.

## 2026-09-12 continuation gate

The user has explicitly approved creating/pushing temporary validation refs and dispatching CI for the missing negative controls and restored full-suite green. Keep fix/facade-update-ownership at a314b0002e46b5a3ffeeef22c503a486a5293b13 and preserve its dirty work. Use a clean worktree under .trellis/.runtime/validation/facade-update-ownership-20260912 and the temporary branch validation/facade-update-ownership-20260912. Authorization is validation-only: no PR creation, merge, publish, deployment, account traffic, or unrelated commit.

### Authorized validation sequence

1. Prepare a clean temporary worktree from the exact patch SHA; review scoped changes before each checkpoint. Only repo.go and a temporary CI test-command change belong in validation commits; existing task records and unrelated WIP remain excluded.
2. Completed: unconditional business allocation/publication control at c4df60fa4c00475ff57c678c478e7833295979ab; ci-test 34631556246 reached actual T2 blocked burst progress and exited normally.
3. Completed: early absent-outlet guard control at 679d8c045d1cca890ff4defb720c7a84223de300; ci-test 34632417620 reached the missing internal tail effect and exited normally. T3 stopped at its initial tail prerequisite, not its later waiter assertions.
4. Completed: subscribed drop-on-full control at 260cac52b682d7e34e50d8be5b865d5fe24f9589; ci-test 34633303904 reached the T4 count failure (104 expected, 103 received) and exited normally. Later identity assertions were not reached in this red.
5. Completed: restored commit eb90ad511083c7e507f351a3829988074f05e0fb and existing ci-test 34634421836 passed whole-repository vet and the uncached internal suite. Final repo.go equals the original Git blob; only temporary CI test flags differ from the baseline. Main source/workflow fingerprints are unchanged.
6. Completed on 2026-10-11: independent executed-evidence reconciliation confirms run/commit correlation, intended assertion failures, ordinary exits, and restored source/green receipts with no blocker. Preserve all PR/merge/account/rollout authority gates.

## Planning phases

1. Establish task, authorization, and existing-work boundaries — complete.
2. Verify construction, internal-event obligations, and validation seams — complete; both researcher completion receipts collected.
3. Write `prd.md`, `design.md`, `implement.md`, and context manifests — complete; v2 incorporates review feedback.
4. Independent design review, including an adversarial reviewer; resolve findings — complete; both approve, no design blockers.
5. Validate planning artifacts and prepare handoff at the implementation-approval gate — complete; authorization was subsequently received as recorded below.

## Implementation phases (authorized 2026-09-11)

6. Record authorization, preserve dirty work, create task branch, and activate — complete; reused the already-existing task branch.
7. Implement explicit construction modes, migrate callers, and add production-loop regression tests — complete; Gibbs delivered and released source ownership.
8. Run available static/behavioral validation and record native-CI evidence boundaries — complete; static checks, initial native green, all three required behavioral reds with ordinary teardown, and exact-source-restored final vet/internal-suite green are verified. This is adapter-seam evidence, not actual-account/SDK integration acceptance.
9. Independent full-scope check, adversarial/structure audit, simplification, and spec sync — complete for source review; no blocking finding, fresh structure audit returned, source-only simplification complete, and existing spec sync verified. This does not satisfy native acceptance.
10. Present and execute the authorized task-only checkpoint — complete; the approved 31-file commit is a314b0002e46b5a3ffeeef22c503a486a5293b13. The subsequent push and workflow runs occurred; unrelated dirty work remains excluded. Earlier pending-delivery notes below are historical.
11. Assess merge readiness — complete. Evidence commit 48f7a36 was committed/pushed with separate approvals, PR #2 passed its PR ci-test, the absence of formal review approval was disclosed, explicit merge approval was obtained, and the PR was merged to main. Live-account acceptance, rollout, and production closure remain separate gates.

## Decisions and constraints

- Original planning authorization excluded implementation. The later “开始推进” authorizes the reviewed implementation; activate only after branch and context checks.
- Accepted invariant: facade must serve its existing request API without a business-update subscriber; callers must not drain an unpublished internal queue to keep it alive.
- Primary deliverable is this facade invariant. Examine engine and SDK lifecycle hazards for dependencies/regressions, without claiming the whole pipeline has been repaired.
- Reuse Trellis as the artifact owner. The planning-with-files working notes live in this task research directory, not in a competing root-level plan.
- Runtime code and task/spec documentation are now in scope. No dependency/CI redesign, remote write, commit, merge, deployment, or unrelated workspace change is implicit.
- Before implementation: explicit user approval, a task branch, reviewed artifacts, then activation.

## Resolved design questions

1. Use a required immutable typed construction mode; no facade/stand business queue, with engine/LiveStack retaining the existing outlet. A disabled Updates access is a programming error, not a subscription operation.
2. Preserve standard reception, separate auth/startup routes, and the existing listener for private send results plus readiness/convergence. Shared-Repo compatibility, not an invented stand send-wait obligation, justifies private result preservation.
3. T1-T5 bind the real loop, after-burst internal effects, attempted overflow while full, exact subscribed events, and deterministic failure cleanup. Mandatory behavioral controls and a separate actual-binary probe prevent vacuous acceptance.
4. Source review established no inseparable prerequisite for engine/SDK redesign. Their overload/closure risks remain open; the repaired edge is only the request-only unused outlet. Reopen for a concrete lost responsibility or newly caused regression.

## Errors and recovery

- A JavaScript string parse error prevented one combined tool call from executing. Changed to template-string input.
- Two malformed patch-context attempts were rejected atomically. Verified target files were unchanged, then used smaller exact-context hunks.
- A Windows rg call treated README* as a literal path and failed. Retried with the exact README.md path; no file change was involved.

| Observation | Resolution |
| --- | --- |
| Initial combined output truncated the workflow middle | Read the missing Phase 1 and routing sections separately. |
| `.agents/skills` and `.codex` do not exist here | Use the installed Trellis scripts and `workflow.md`; no `trellis-brainstorm` is available. |
| Git warns that the global ignore file cannot be read | Status still succeeded; do not modify global Git configuration. |
| Resumption's combined workflow output was truncated | Re-read the missing middle explicitly; execution and commit gates restored. |
| Helper discovery again confirmed `.codex` absent and `.agents` a file | Use workflow-guided generic implement/check agents; do not modify tooling setup. |
| Windows sandbox process creation and patching failed with ACL error 5; earlier approval requests timed out | After three verified blocked turns and user resumption, explicitly approved outside-sandbox reads work. Use the same apply_patch executable through an approved command; do not alter sandbox ACLs. |
| Resumed combined reads exceeded the output budget | Re-read the complete workflow and missing findings/progress before proceeding. |
| An approved CLI apply_patch invocation failed on its third file after updating the first two | Verified the partial state with rg, retained the successful edits, and corrected only the remaining exact-context hunk. Do not assume CLI multi-file patches are atomic. |
| Earlier parent progress/spec patches and static-check approvals timed out | No successful write/check was inferred; runtime delivery and formatter results were subsequently recovered from the worker's actual handoff. |
| Latest resumption again hit the same Windows sandbox ACL startup failure | Single-file reads and approved outside-sandbox status/workflow/catchup checks succeeded. The normal apply_patch failure is followed by one approved same-editor recovery attempt; no ACL or tooling changes. |
| Two constructor searches lost regex escaping through JavaScript strings | Corrected with fixed-string `rg -n -F`; both constructor inventories succeeded. |
| Normal reads briefly recovered, then sandbox ACL error 5 recurred; a combined escalated read timed out | Retried once with already-approved single-file read commands; findings, progress, and TDLib spec reads succeeded. No ACL, configuration, or tooling changes. |
| A broad filename inventory escalation was rejected after the approval reviewer reported stream disconnected / adapter_eof | Did not retry the rejected inventory. Used the materially narrower, already-known CI spec and two explicit build/config files; no permission or tooling changes. |
| First native-CI handoff patch approval timed out | Confirmed the new file did not exist, then one scoped retry through the same native apply_patch editor succeeded. No write was inferred from the timeout. |
| Merge-readiness recovery output exceeded the tool output budget | Re-read the missing plan/findings and workflow routing in bounded calls. |
| Sandbox process creation failed again with helper_sandbox_lock_failed / SetNamedSecurityInfoW error 5 | Scoped, approved read-only retries succeeded; no ACL or tooling changes. |
| PR-list approval failed: Automatic approval review failed: unexpected status 502 Bad Gateway: context ~241758 tokens exceeds the model window 200000 | Initially continued only with the safer local audit. Following a renewed retry request, the same query was approved normally and returned an empty all-state list. No alternate PR/API route was used; recovery evidence is in research/findings.md. |
| First merge-readiness patch call had SyntaxError: Unexpected identifier 'commit' | No tool or patch executed. Corrected JavaScript string quoting before retrying the editor. |
| Independent CI-step query was rejected before execution after approval-service rate limiting | Reported the failure. Following the renewed retry request, the same read-only command was approved normally and succeeded. No alternate PR/API route or new workflow run; step evidence is in research/findings.md. |
| Latest recovery encountered orchestrator_helper_launch_canceled: ShellExecuteExW failed to launch setup helper: 1223, then helper_sandbox_lock_failed / SetNamedSecurityInfoW error 5 | Retried the required read-only operations through normal scoped approval. Successful history, diff, status, catchup, and run-metadata receipts are recorded; no ACL/configuration change or alternate authorization route. |
