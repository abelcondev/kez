---
name: sdd-implement
description: Use when implementing a pending SDD task — TDD red→green→refactor on a feature branch, honoring the task's Given/When/Then, then chain to sdd-test, sdd-review, and sdd-ship before closing. One PR per proposal.
---

# Implement

Build the task with test-driven discipline, on a feature branch, and carry it all the way through review to a PR. This skill orchestrates the tail of the loop.

## Order of operations

1. **Stay on the proposal's branch.** `kez sdd propose` already opened this proposal's branch (`sdd/prop-<slug>`); all of its tasks share it — one branch, one PR. You should already be on it, so implement here. Only if HEAD somehow sits on `main`/`master` (a compiled guard refuses code writes there), return to the proposal's branch — check `git branch --list 'sdd/prop-*'` — or, as a fallback, `git checkout -b feat/<decision-slug>`.

2. **TDD: red → green → refactor.** Write the failing test first, then the minimal code to pass, then refactor. Translate each **Given/When/Then** acceptance criterion into a test. For the test conventions (only `.test.ts` code tests, dedicated folder, what the agent writes vs. what the human verifies manually), load **`sdd-test`**.

3. **Match the codebase.** Smallest diff that fully solves the task, in the surrounding style. No speculative abstraction, no unrelated refactors, no dependency churn the task did not require.

4. **Run the validators** scoped to the change — tests, typecheck, lint, build — as recorded in `sdd/index.md`. Never proceed while they fail.

5. **Review before you ship.** Once green, load **`sdd-review`**: a fresh-context pass over the diff (correctness + security). Fix every high-confidence finding before closing.

6. **Close and open the PR.** Load **`sdd-ship`** for the done + PR steps:

   ```
   kez sdd done <task-name>
   ```

## Definition of done

- Every `code`-level Given/When/Then has a passing test.
- Validators pass.
- `sdd-review` findings addressed.
- Branch pushed, PR prepared — never merged to the protected branch by you.

## Anti-patterns

- ❌ Writing code on `main`.
- ❌ Implementation before the test (that is not TDD).
- ❌ Marking the task done with failing validators or unaddressed review findings.
