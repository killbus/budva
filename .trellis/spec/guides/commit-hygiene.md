# Commit & Merge Hygiene

> Branch history is an iteration trace; main history is a logical
> timeline. The two shapes are produced by two different disciplines.

---

## Branch discipline (iteration trace)

- Work happens on a feature branch, one task's scope per branch.
- Commit per increment (code, then its evidence/docs as separate small
  commits). Fine granularity is **intentional** on the branch: it keeps
  a traceable RED/GREEN record, a per-commit CI history, and cheap
  rollback units.
- A branch may trigger its own CI (push of the feature branch or a
  `--ref`-pinned dispatch); the branch never targets main directly.

## Main discipline (logical timeline)

- Main accepts changes **only** through a PR, merged as **squash**.
- Before opening (or re-basing) the PR, run the **commit clean-up step**:
  1. Group the branch commits into logical units (one per coherent
     deliverable, e.g. `feat: <sink>`, `feat: <serializer>`,
     `test: <locks>`, `ci: <workflow>`, `docs: <records>`).
  2. Collapse each group with a scripted interactive rebase (`pick`
     first + `fixup` rest + `exec git commit --amend -F <msg>` per
     group; `GIT_SEQUENCE_EDITOR` script, no interactive session).
  3. Verify the cleaned tip's **tree identity** against the pre-clean
     tip: `git diff <pre-clean-tip> <cleaned-tip>` must be empty (or
     exactly the intended delta, if the clean-up folds in a pending
     fix).
  4. Run the repo's standard CI on the cleaned tip before merging.
- The PR then lands exactly one logical commit per unit on main.
- Direct pushes to main and any history rewrite of main (force-push)
  happen **only** under explicit owner approval, and use
  `--force-with-lease`.

## Planning-phase task records (no feature branch yet)

Task records created or updated while a task is still in `planning` —
`task.json`, `prd.md`, `research/`, `implement.jsonl`, `check.jsonl`,
`progress.md` — and the **archive move** of a just-completed task
(`task.py archive`) are record changes, not an exception to the main
discipline. They land on main **only via a branch → PR → squash-merge**,
exactly like code. There is **no direct-to-main bookkeeping exception**:
"it is only task bookkeeping" is not a reason to skip the PR, and it is not
approval.

Practical path while no feature code exists yet:

- Open a lightweight branch from current `main` (e.g. `docs/<task-id>-records`
  or `chore/<task-id>-records`).
- Put the record edits there as small logical commits (one per coherent
  unit: research/design, audit corrections, a model correction, archive move).
- Open a PR targeting `main`, let CI run green, then squash-merge.

A consolidated main tip must keep the **tree identity** of the branch tip
(`git diff <branch-tip> <main-tip>` empty) — the PR changes the *shape* of the
history, never its content.

Direct pushes to main and any history rewrite of main (force-push) happen
**only** under explicit owner approval, and use `--force-with-lease` (see the
Main discipline above). A discovered deviation (commits already on main
outside a PR) is corrected by an owner-approved consolidation to a
logical commit set, recorded in the affected task's `progress.md` — never
left in place as precedent.

## Out of scope

- Release tags, image publication and deployment follow their own
  contracts (see `.trellis/spec/backend/docker-ci-guidelines.md`).
- Mirror remotes (e.g. non-GitHub remotes) are synced deliberately,
  never implied by a main merge; record the sync state in the task's
  `progress.md` when it changes.
