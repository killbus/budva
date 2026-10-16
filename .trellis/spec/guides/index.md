# Cross-Package Guides

> Repo-wide conventions that apply to every package and every task,
> independent of which layer or CI channel is involved.

---

## Guide Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Commit & Merge Hygiene](./commit-hygiene.md) | Branches keep granular iteration commits; main accepts only cleaned, squash-merged PRs | Filled |

---

## How This Directory Is Used

- **Per-package conventions** live under `.trellis/spec/<package>/...`; this
  directory is for rules that cross package boundaries (history shape,
  merge policy, credential handling, remote-mirror policy).
- A guideline here is addressed to any task, on any branch, in any
  package. When a task's `implement.jsonl` covers cross-cutting
  delivery (PRs, merges, release work), register the relevant guide
  file in the manifest.
- Write guidelines **positively**: state the workflow to follow.
  Task-specific incident records stay in `.trellis/tasks/*/progress.md`
  and link here; they are not the home of standing rules.
