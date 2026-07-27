---
name: sdd-design
description: Use when a task ships user-facing UI and has no approved design yet — design the screen/flow in Penpot/Figma via MCP from the project's component library, then stop at the approval gate before writing any UI code.
---

# Design (UI tasks only)

A task that ships UI must have an **approved design before any UI code**. If the task does not touch UI, this phase does not apply — skip it entirely.

## How to run it

1. **Design in Penpot/Figma via the MCP tools**, not in code. Build the screen or flow visually first.

2. **Build from the project's design system.** Use the component library recorded in `sdd/index.md` ("UI conventions"). Do not hand-roll raw CSS or utility-class soup when a component exists. If a primitive is missing, add it to the library first, then compose from it.

3. **Record the design and open the gate:**

   ```
   kez sdd design <decisions/NNN-name.md> "<screen or flow>"
   ```

   Fill in the frames/screenshots, then:

   ```
   kez sdd approve-design <designs/NNN-slug>
   ```

4. **Stop at the gate.** The human reviews the frames and approves. Without an approved design, the loop refuses UI code — do not try to route around it.

## Verification split

- **Layout/visual correctness** is verified by the human (manual QA) against the approved frames.
- **Logic behind the UI** (formatting, state, calculations) is still covered by code tests in `sdd-test`.

## After approval

The design unblocks the task's UI code. Load `sdd-implement` to build it — from the approved frames and the component library.

## Anti-patterns

- ❌ Writing UI code before the design is approved.
- ❌ Hand-rolling components that already exist in the library.
- ❌ Trying to auto-generate Playwright/e2e for the visual layer — that is manual QA.
