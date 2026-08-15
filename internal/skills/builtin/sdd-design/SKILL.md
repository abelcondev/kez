---
name: sdd-design
description: Use when a feature task ships user-facing UI and has no approved design yet — gather the user's visual references, sketch the layout as an ASCII wireframe in the design doc, stop at the approval gate, then code the screen hi-fi directly (no placeholder build).
---

# Design (UI tasks only)

A task that ships UI must have an **approved design before any UI code**. If the task does not touch UI, skip this phase entirely.

The design is **not a placeholder build** anymore. Product screens are coded **hi-fi, directly** — so the design artifact is a lightweight, fast-to-review spec: the user's visual references plus an ASCII wireframe of the layout. The human approves that spec (this is where they catch a badly-distributed layout before any code exists), then the screen is coded once, for real. One build, one review — no throwaway placeholder to build and review first.

## Base vs product — what this phase designs

- **Base components** (Button, Input, Card, tokens, generic patterns like FormField/Modal) live in the `/design-system` workbench and are built once via `sdd-design-system`. This phase does **not** rebuild them.
- **Product components/screens** (a sign-in screen, a cash-register view, an order card) are **coded directly** in the app, composed from the base. This phase designs *those* — as a wireframe, not a workbench story.

The dividing test: *business-agnostic and reusable across features?* → base (workbench). *Feature-specific / carries business meaning?* → product (code it directly, design it here).

## How to run it

1. **Gather references first — ask the user.** Since there is no placeholder to iterate on, you need to know what they want it to look like *before* sketching. Ask directly (plain reply, not a command):
   > ¿Tenés una referencia para esta pantalla? Figma, una imagen o boceto (Pencil), un sitio/producto a imitar, o me lo describís?
   - **Figma URL** → use the figma MCP to pull frames, tokens, and layout.
   - **Image / Pencil / screenshot** → have them paste it; save it under `sdd/designs/<NNN>/`.
   - **A site/product to emulate** → note the URL and what to borrow.
   - **Only a description** → capture it in words.
   Record what they give in the design doc's `## References`. If they have none, say so and describe the intended look yourself for them to confirm.

2. **Sketch the layout as an ASCII wireframe.** In the design doc, draw a line/box sketch of each screen and key state (default, empty, loading, error) — the kind of layout sketch that shows regions and where each component sits. This is the artifact the human reviews; it must communicate distribution and hierarchy, not pixels.

3. **Record which base components compose it.** List the workbench components the screen builds from, the new product components you'll code directly, and any base primitive that is missing and must be added to the workbench first (a small `sdd-design-system` pass — never hand-roll a missing primitive inside the screen).

4. **Record the design and open the gate:**

   ```
   kez sdd design <decisions/NNN-name.md> "<screen or flow>"
   ```

   Fill in `## References`, `## Wireframe`, and `## Composition` in the created doc.

5. **Stop at the gate.** The human reads the wireframe + references and checks how you're going to approach the UI. Ask them to reply with a short approval (e.g. "aprobado") — don't make them type a command; when they approve, run `kez sdd approve-design <designs/NNN-slug>` yourself on their behalf. Without an approved design the loop refuses UI code — do not route around it.

## After approval

The design unblocks the task's UI code. Load `sdd-implement` and code the screen **hi-fi, directly**, composing from the approved base components and following the wireframe. There is no placeholder to reuse — build the real thing.

## Verification split

- **Layout/visual correctness** — verified by the human on **localhost** (manual QA). No screenshots are committed; a running screen beats a saved image.
- **Logic behind the UI** (formatting, state, calculations) — covered by code tests in `sdd-test`.

## Anti-patterns

- ❌ Building a placeholder/workbench composition of a product screen before coding it — that double-work is gone; code it hi-fi directly.
- ❌ Sketching a wireframe without asking the user for references first (you'll guess the look and get it wrong).
- ❌ Writing UI code before the wireframe is approved.
- ❌ Coding a product screen with raw CSS when a base component exists — compose from the workbench; add a missing primitive there first.
- ❌ Committing screenshots as a deliverable — the human's QA is the live localhost screen.
