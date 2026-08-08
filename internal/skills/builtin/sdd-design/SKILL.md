---
name: sdd-design
description: Use when a feature task ships user-facing UI and has no approved design yet — build the screen's components in the project's /design-system workbench route (code, not an external mockup tool), review them live, record the design, then stop at the approval gate before writing the screen.
---

# Design (UI tasks only)

A task that ships UI must have an **approved design before any UI code**. If the task does not touch UI, this phase does not apply — skip it entirely.

The design is done **in code, in the project's `/design-system` workbench route** — not in an external mockup tool. External design apps are not the gate; the running workbench is. If the project has no workbench yet, build it first with `sdd-design-system` (that is the foundations pass).

## How to run it

1. **Work in the `/design-system` workbench, not in the screen.** Build (or assemble) the components this task's screen needs there first, in isolation, with every state visible — default, hover, focus, loading, error, empty, each variant. This includes composed blocks: a login card, a sidebar, or a concrete form is a **composition** and belongs in the workbench (tier 3), built from primitives, driven entirely by props — `<LoginCard state="error" />`, not real auth.

2. **Compose from the existing design system.** Use the workbench components recorded in `sdd/index.md` ("UI conventions"). Do not hand-roll raw CSS or utility-class soup when a component exists. If a primitive is missing, add it to the workbench first (a small design-system pass), then compose from it — never inline it into the screen.

3. **Keep behavior out of the workbench.** Only presentation lives there — states are triggered by props. Data, handlers, validation, routing, and API calls belong to the screen (the feature `implement` step), not the design system.

4. **Record the design and open the gate:**

   ```
   kez sdd design <decisions/NNN-name.md> "<screen or flow>"
   ```

   In the artifact, link the **running workbench route** for the components used and add a screenshot of each state. Then:

   ```
   kez sdd approve-design <designs/NNN-slug>
   ```

5. **Stop at the gate.** The human opens the workbench and exercises the components live. Ask them to reply with a short approval (e.g. "aprobado") — don't make them type a command; when they approve, run `kez sdd approve-design <designs/NNN-slug>` yourself on their behalf. Without an approved design, the loop refuses UI code — do not try to route around it.

## Verification split

- **Layout/visual correctness** is verified by the human (manual QA) against the live workbench.
- **Logic behind the UI** (formatting, state, calculations) is still covered by code tests in `sdd-test`.

## After approval

The design unblocks the task's UI code. Load `sdd-implement` to build the screen — composed from the approved workbench components.

## Anti-patterns

- ❌ Mocking the screen in an external design tool instead of building components in the workbench.
- ❌ Writing UI code before the design is approved.
- ❌ Hand-rolling components that already exist in the workbench.
- ❌ Trying to auto-generate Playwright/e2e for the visual layer — that is manual QA.
