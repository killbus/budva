# Planning and execution: facade update ownership

## Goal

Implement and verify the reviewed facade zero-business-subscriber liveness repair, preserving internal event handling and subscribed delivery semantics. The original design-only deliverable is complete; the user authorized implementation with “开始推进” on 2026-09-11.

## Current phase

Phase 2.2 native validation gate: source implementation, static reviews/checks, structure audit, and spec consistency are complete. The existing ci-test workflow on killbus/budva is confirmed active; its latest green covers only the unchanged baseline, not this patch. A task-only local checkpoint commit plan is ready in research/native-ci-handoff.md and awaits confirmation. Push/dispatch, native behavioral controls/final green, and actual-binary acceptance remain separate pending gates. No usable local Linux/TDLib runner was established.

## Planning phases

1. Establish task, authorization, and existing-work boundaries — complete.
2. Verify construction, internal-event obligations, and validation seams — complete; both researcher completion receipts collected.
3. Write `prd.md`, `design.md`, `implement.md`, and context manifests — complete; v2 incorporates review feedback.
4. Independent design review, including an adversarial reviewer; resolve findings — complete; both approve, no design blockers.
5. Validate planning artifacts and prepare handoff at the implementation-approval gate — complete; authorization was subsequently received as recorded below.

## Implementation phases (authorized 2026-09-11)

6. Record authorization, preserve dirty work, create task branch, and activate — complete; reused the already-existing task branch.
7. Implement explicit construction modes, migrate callers, and add production-loop regression tests — complete; Gibbs delivered and released source ownership.
8. Run available static/behavioral validation and record native-CI evidence boundaries — in_progress; local static checks passed and the existing Linux CI channel is confirmed. Native tests for the patch and behavioral controls await an authorized validation revision/run.
9. Independent full-scope check, adversarial/structure audit, simplification, and spec sync — complete for source review; no blocking finding, fresh structure audit returned, source-only simplification complete, and existing spec sync verified. This does not satisfy native acceptance.
10. Present the task-only commit plan at the confirmation gate — in_progress; one 31-file checkpoint commit is proposed in research/native-ci-handoff.md, distinguishing takeover edits, inherited task changes, and all excluded dirty paths. Confirmation is pending. No staging, commit, push, CI dispatch, account traffic, or deployment occurred during takeover.

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
