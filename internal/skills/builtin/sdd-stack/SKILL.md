---
name: sdd-stack
description: Use after a proposal is approved and before writing code — choose the stack, libraries, and project conventions research-first, then record them as an architecture decision. Fixes the "jumped to the first library" failure.
---

# Stack & architecture

Choosing the stack is a **deliberate decision**, not the first name that comes to mind. Do it research-first and record it, so every later task inherits it.

## How to run it

1. **Start from the constraints in the approved decision**, not from a favorite stack. Money handling, offline, single-device, realtime, team familiarity — let these drive the choice.

2. **Research before fixing.** When a library, framework, or service is external or you are not current on it, use web search/fetch to check it (maintenance, fit, gotchas) before committing. Do not assume; do not pick blind. If the user named a stack, still validate it fits the constraints and say so.

3. **Decide concretely — no "Option A vs B" left open.** Pick one stack, one set of core libraries, and state the trade-off you accepted.

4. **Record the conventions the rest of the loop depends on**, in the architecture decision and reflected in `sdd/index.md`:
   - **Test runner and test file convention** — e.g. Vitest with `*.test.ts`.
   - **Test location** — a dedicated tests folder that mirrors the source tree (e.g. `tests/caja/vuelto.test.ts` for `src/caja/vuelto.ts`), unless the user prefers colocated. This is what `sdd-test` will honor.
   - **UI component library / design system** — so UI work builds from it instead of hand-rolled CSS.
   - **Lint / typecheck / build commands** — the validators the loop runs.

5. **Persist it:**

   ```
   kez sdd propose "Architecture: <stack>, <core libs>, test = <runner> in <folder>, UI = <lib>"
   ```

   Then stop at the approval gate.

## After approval

Add tasks against this decision (`sdd-task`). Only now is scaffolding/installing appropriate, as the first task's work.

## Anti-patterns

- ❌ `npm create …` before the stack is an approved decision.
- ❌ Picking a DB/framework you did not verify is current and fits.
- ❌ Leaving the test folder / runner unspecified — it makes `sdd-test` guess.
