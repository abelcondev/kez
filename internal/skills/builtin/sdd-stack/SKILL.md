---
name: sdd-stack
description: Use after a proposal is approved and before writing code — choose the stack, libraries, and project conventions research-first, then record them as an architecture decision. Fixes the "jumped to the first library" failure.
---

# Stack & architecture

Choosing the stack is the **user's decision**, informed by your research — never yours to make silently. This is the single most common failure: the agent picks a framework/UI/test-runner the user never chose and presents it as settled. Do not. Ask first, research, then propose only what the user did not pin down.

## How to run it

1. **Ask the user which technologies they want — before deciding anything.** This phase opens like discovery: a plain-text conversation, not a verdict. Start from the constraints in the approved decision (offline, money handling, single-device, realtime, team familiarity), then ask, a few at a time, in your reply:
   - Language / framework? (e.g. Next.js, Remix, SvelteKit, vanilla)
   - UI: component library / design system, or hand-rolled?
   - Test runner and file convention?
   - Anything already decided or off-limits (existing infra, licensing, hosting)?

   Offer to recommend if they have no preference — but let them say so. **Honor every technology the user has already named** (e.g. they chose the DB) as fixed; do not re-litigate it, only validate it fits.

2. **Research before fixing.** For each piece the user left open, use web search/fetch to check it (maintenance, fit, gotchas) before proposing. Do not assume; do not pick blind. If the network is unavailable, say the research is incomplete — do not silently commit to a stack you could not verify.

3. **Propose, do not impose.** Bring back a concrete recommendation for the open pieces with the trade-off you accepted, and confirm it with the user before recording. No layer of the stack gets written into the decision without the user having chosen or okayed it.

4. **Record the conventions the rest of the loop depends on**, in the architecture decision and reflected in `sdd/index.md`:
   - **Test runner and test file convention** — e.g. Vitest with `*.test.ts`.
   - **Test location** — a dedicated tests folder that mirrors the source tree (e.g. `tests/caja/vuelto.test.ts` for `src/caja/vuelto.ts`), unless the user prefers colocated. This is what `sdd-test` will honor.
   - **Test import convention** — how tests import the code under test: the configured path alias (e.g. `$lib`, `~/`, a `tsconfig`/`vitest` alias) rather than deep relative paths. Record the exact alias so `sdd-test` never has to guess `../../..` depth (a repeated source of red-then-fix churn). If the project has no alias, say so and set up one during the first task if the runner supports it.
   - **UI component library / design system** — so UI work builds from it instead of hand-rolled CSS.
   - **Lint / typecheck / build commands** — the validators the loop runs.
   - **Cheap review checks** — a few deterministic greps (under `## Cheap review checks` in `sdd/index.md`) that `sdd-review` runs *before* the expensive model lenses, so repeatable smells never cost a review round. Seed the ones that fit the stack: no literal glyph/emoji chars in UI templates (use the icon library), domain types defined in one module only, no `TODO`/`FIXME` introduced in the diff. Keep them few and fast.

5. **Persist it — only after the user has okayed the choices:**

   ```
   kez sdd propose "Architecture: <stack>, <core libs>, test = <runner> in <folder>, UI = <lib>"
   ```

   `propose` opens this decision's own branch (`sdd/prop-<slug>`); the doc, its approval, and the code all land in one PR. Then stop at the approval gate.

## After approval

Add tasks against this decision (`sdd-task`). Only now is scaffolding/installing appropriate, as the first task's work.

## Anti-patterns

- ❌ **Deciding the stack without asking the user** — the top failure. Framework, UI library, test runner: the user chooses, you inform.
- ❌ Presenting a fully-chosen stack as settled and reducing the user to approve/reject a finished doc.
- ❌ Overriding or re-litigating a technology the user already named.
- ❌ Committing to a stack when the network was down and you could not actually research it — say so instead.
- ❌ `npm create …` before the stack is an approved decision.
- ❌ Leaving the test folder / runner unspecified — it makes `sdd-test` guess.
