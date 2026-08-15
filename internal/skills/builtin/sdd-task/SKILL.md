---
name: sdd-task
description: Use when opening a new unit of work (feature or fix) in the SDD loop — cut it into a task with Given/When/Then acceptance criteria linked to a decision, before any implementation.
---

# Task

A task is the smallest shippable slice with a clear definition of done. It is linked to the decision it serves and carries acceptance criteria the tests will satisfy.

## How to run it

1. **Scope one slice.** One task = one feature or one fix that lands in a single PR. If it needs two PRs, it is two tasks.

2. **Write acceptance criteria as Given / When / Then, in prose.** This is a *specification*, not executable Cucumber — there are no `.feature` files. Each criterion maps 1:1 to a code test later (Given = arrange, When = act, Then = assert):

   ```
   Given a comanda with 2 pollos at S/30
   When the cashier charges with S/100
   Then the change due is S/40
   ```

3. **Mark the level of each criterion**, because it decides who verifies it:
   - **code** → will become a `.test.ts` written by the agent (`sdd-test`).
   - **manual** → browser / visual / e2e behavior the human verifies. The agent does NOT auto-write these.

4. **Tag UI work.** If the task ships user-facing UI, it needs an approved design first — the loop will route you to `sdd-design` before code.

5. **Create it, linked to its decision:**

   ```
   kez sdd task <decisions/NNN-name.md> "<task title>"
   ```

   Then fill the Given/When/Then in the generated task file.

6. **Tier is auto-inferred — override only when it's wrong.** kez classifies the task's weight (`trivial | standard | critical`) from its title/tags and shows it in `kez sdd next`/`status`; it drives how much design gate and review the task gets. You don't set it normally. Override only when inference misreads the risk — e.g. force `critical` on a task that handles money/auth/data but doesn't say so, or `trivial` on a pure copy/rename:

   ```
   kez sdd task <decisions/NNN-name.md> "<title>" --tier critical
   ```

## After this

The loop routes to design (if UI) or straight to implementation. Load `sdd-implement` when it is time to write code.

## Anti-patterns

- ❌ A task with no decision link — it has no "why".
- ❌ Vague criteria ("works well") that no test can pin down.
- ❌ Bundling several features into one task / PR.
