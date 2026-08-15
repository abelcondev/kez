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

5. **Review before you ship.** Once green, load **`sdd-review`**: a fresh-context pass over the diff across two lenses — correctness + security, and craft/maintainability (a reviewer subagent that flags oversized files, poor wiring, leaked coupling, missed reuse). Remediate the findings in one pass, then re-review. Every high-severity finding must be fixed before closing.

6. **Close and open the PR.** Load **`sdd-ship`** for the pre-flight + close + PR steps:

   ```
   kez sdd ship <task-name>
   ```

## Working context (`sdd/context.md`)

Before implementing, read `sdd/context.md` — it holds the proposal's already-discovered surface (API shapes, store/module methods, key file paths, gotchas) so you don't re-explore the same files a previous turn already mapped. As you discover a stable fact worth the *next* turn not re-deriving, record it there under this proposal's branch heading — keep it a short map, not a log. This is what survives context compaction; lean on it instead of re-reading the backend each turn. When the proposal's PR merges, clear its section.

## Delegating to phase specialists (optional)

You (the orchestrator) can hand a phase to a fresh-context specialist via the swarm — smallest useful route, so delegate only where fresh context or parallelism pays:

- **`coder`** — a single, fully-specified task (its Given/When/Then are written). Spawn one per independent task to build them in parallel: `swarm_spawn agent_type=coder` with the task ref as the briefing, then `swarm_collect`.
- **`explorer`** — read-heavy investigation before implementing (where does X live, how is Y wired). Read-only, returns a conclusion.
- **`planner`** — drafting a proposal/decision or breaking work into tasks (see `sdd-discovery`, `sdd-stack`, `sdd-task`).
- **`reviewer`** — always, at the review gate (see `sdd-review`).

Each specialist starts with zero context and grounds itself in the on-disk `sdd/` artifacts, so brief it with the task/decision ref rather than pasting context. Members inherit your model, sandbox, and permission mode — never more authority.

## Definition of done

- Every `code`-level Given/When/Then has a passing test.
- Validators pass.
- `sdd-review` run (both lenses); every high-severity finding fixed, residual medium/low recorded as a follow-up task via `kez sdd ship <task> --residual "…"` (and noted in the PR).
- Branch pushed, PR prepared — never merged to the protected branch by you.

## Anti-patterns

- ❌ Writing code on `main`.
- ❌ Implementation before the test (that is not TDD).
- ❌ Marking the task done with failing validators or unaddressed review findings.
