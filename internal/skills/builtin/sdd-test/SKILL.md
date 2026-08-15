---
name: sdd-test
description: Use while implementing a task to write its tests — code tests only (e.g. .test.ts), one per Given/When/Then, in the project's dedicated test folder. Browser/visual/e2e is manual human QA, never auto-generated.
---

# Test

Cover the task's acceptance criteria with **code tests only**. This is not a separate phase — tests are written first, inside implementation (TDD).

## Scope: code tests, not browser tests

- ✅ **Code tests** (unit/logic) in the project's runner and file convention — e.g. `*.test.ts` with Vitest, `*_test.go` with `go test`. The agent writes these. Cover calculations, reducers, validators, stores, formatting, edge cases.
- ❌ **Browser / visual / e2e** (Playwright, clicking through screens, "looks right on the tablet") — **manual QA owned by the human.** Do NOT auto-generate these. Criteria marked `manual` on the task are the human's to verify.

## Not Cucumber

The Given/When/Then on the task is a **prose specification**, not an executable `.feature` file. There are no step definitions and no BDD framework. You translate each criterion into an ordinary code test:

```
Given a comanda with 2 pollos at S/30   → arrange: build the comanda
When the cashier charges with S/100      → act:    charge(comanda, 100)
Then the change due is S/40              → assert: expect(change).toBe(40)
```

## Conventions come from sdd-stack

Read `sdd/index.md` for the runner, file pattern, and location decided in `sdd-stack`:

- **Dedicated test folder that mirrors the source tree** (e.g. `src/caja/vuelto.ts` → `tests/caja/vuelto.test.ts`), unless the project chose colocated. Put every test where that convention says, so tests stay easy to find.
- **Import via the recorded path alias, not deep relative paths.** `sdd-stack` records the project's test import convention in `sdd/index.md` (e.g. `$lib`, `~/`, a configured `tsconfig`/`vitest` alias). Use it: `import { charge } from '$lib/caja/vuelto'`, never `../../../src/lib/caja/vuelto`. Counting `../` depth by hand is a repeated red-then-fix waste; the alias is stable no matter where the test file sits. If no alias is recorded, match the existing sibling tests exactly.

## Coverage bar

- Every `code`-level Given/When/Then → at least one passing test.
- Add edge cases the criteria imply (empty, boundary, error), not just the happy path.
- Do not pad with trivial tests to inflate a number; cover behavior, not lines.

## Anti-patterns

- ❌ `.feature` files / Cucumber / step definitions.
- ❌ Auto-writing Playwright/e2e for the visual layer.
- ❌ Scattering tests next to source when the project chose a dedicated folder (or vice-versa).
