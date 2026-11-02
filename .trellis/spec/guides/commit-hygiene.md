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

## Out of scope

- Release tags, image publication and deployment follow their own
  contracts (see `.trellis/spec/backend/docker-ci-guidelines.md`).
- Mirror remotes (e.g. non-GitHub remotes) are synced deliberately,
  never implied by a main merge; record the sync state in the task's
  `progress.md` when it changes.
