---
name: sdd-ship
description: Use to close a reviewed task and open its pull request — mark it done, push the feature branch, prepare the PR. One PR per proposal; never merge to the protected branch yourself.
---

# Ship

Close the task and hand a clean PR to the human. This is the end of one turn of the feature loop.

## Preconditions

- Validators pass (tests, typecheck, lint, build).
- `sdd-test` coverage in place for every `code`-level Given/When/Then.
- `sdd-review` run across both lenses (correctness+security, craft); every high-severity finding fixed, residual medium/low listed in the PR.

## How to run it

1. **Close the task** — flips its status and appends to `sdd/log.md`:

   ```
   kez sdd done <task-name>
   ```

2. **One PR per proposal.** All tasks of a proposal share `feat/<decision-slug>` and land in a single PR. Do not open a PR per task if they belong to the same proposal.

3. **Push and prepare the PR** — do not merge:

   ```
   gh pr create --fill
   ```

   The human reviews and merges. Never push to or merge into the protected branch (`main`/`master`) yourself.

4. **Hand off the manual QA.** In the PR description, list the `manual`-level criteria (browser/visual/e2e) the human still needs to verify — the tests cover the code, the human covers the screen.

## After merge

Back to the top of the feature loop: the next task or the next proposal. Run `kez sdd next` for the single next step.

## Anti-patterns

- ❌ Merging to `main` yourself.
- ❌ A separate PR for each task of the same proposal.
- ❌ Closing without listing what the human still has to verify manually.
