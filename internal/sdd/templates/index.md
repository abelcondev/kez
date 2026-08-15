# Knowledge Index

This is the Spec-Driven Development (SDD) knowledge base for this project, in
Open Knowledge Format (OKF). It is read first at the start of every session.

- `proposal.md` — the current, in-review proposal (transient; cleared on approval).
- `decisions/` — approved, numbered architectural decisions (historical).
- `designs/` — approved UI designs (the live `/design-system` workbench route + screenshots), the gate before UI code.
- `tasks/` — units of work with Gherkin acceptance criteria.
- `context.md` — durable, proposal-scoped working memory (API shapes, key files, gotchas) so a new turn doesn't re-derive what the last one already found.
- `log.md` — append-only history of what happened and when.

## The loop

Everything is one loop — discovery, stack, architecture, foundations, and every
feature are all passes through it. Run `kez sdd next` any time to get the single
next step from disk state; do only that step, and stop at every gate.

```
propose (what & why, no code)
    → approve (→ decisions/NNN)          ── human gate
    → [after stack] design system         ── build base components in /design-system, review live
    → [if UI] design (→ designs/NNN)     ── human gate: components in the /design-system workbench
    → task (Gherkin acceptance)
    → branch: one feature branch per proposal (feat/NNN-slug)
    → implement (TDD: red → green), close with `kez sdd done`
    → one PR per proposal → merge → back to propose
```

## Branch & PR policy

The default branch is protected: every change lands via a feature branch and a
pull request. Branch once per proposal (`feat/NNN-<decision-slug>`) so all of a
decision's tasks land in one PR. Never merge or push to the protected branch.

## UI conventions

<!-- Record the project's /design-system workbench here so every UI task builds
from it instead of hand-rolling. The design system is built in code, in an
isolated route, and reviewed live — not mocked up in an external tool. Example:
- Workbench route: <e.g. /design-system>. A gallery rendering every component in
  every state; it is the review surface and the source of truth for UI.
- Design tokens / theme: <where they live>. Screens bind to these — no magic values.
- Rule: build screens from the workbench components; do not reimplement with raw
  CSS/utility classes when a component exists. If a primitive is missing, add it to
  the workbench first (a small design-system pass), then compose from it. -->

## UI model

Two classes of UI, kept separate:

- **Base** — tokens, primitives (Button, Input, Card…), and generic business-agnostic patterns (FormField, Modal, Table). Built once in the `/design-system` workbench, reviewed live. Reusable across every feature.
- **Product** — feature-specific, business-meaning screens and blocks (a sign-in screen, a cash-register view, an order card). **Coded hi-fi directly** in the app, composed from the base. Never a workbench placeholder.

Dividing test: *business-agnostic and reusable across features?* → base. *Feature-specific / carries business meaning?* → product.

For product UI the design step is a lightweight spec — the user's visual references + an ASCII wireframe in `sdd/designs/NNN.md`, approved before code — not a built placeholder. The human's visual QA is the live screen on localhost; screenshots are not committed.

## Test conventions

<!-- Recorded by sdd-stack; honored by sdd-test. Example:
- Runner & file pattern: <e.g. Vitest, *.test.ts> / <go test, *_test.go>.
- Location: <dedicated tests/ mirroring src/, or colocated>.
- Import path: <the alias tests use, e.g. $lib / ~/ — NOT deep relative ../../..>.
- Validators (the loop runs these before review): <test / typecheck / lint / build commands>. -->

## Cheap review checks

<!-- Deterministic, project-specific greps the review runs FIRST, before the
expensive model-driven lenses — so cheap, repeatable smells never cost a review
round. Recorded by sdd-stack, run by sdd-review. Each is a one-line rule + the
command that detects a violation. Examples:
- No literal glyph/emoji chars in UI templates — use the icon library:
  `rg -n "[✓⌫←→✕]" src --glob "*.svelte"`
- Domain types live in one module, never redefined:
  `rg -n "type StaffRole" src | rg -v "lib/staff-role"`
- No TODO/FIXME introduced in the diff:
  `git diff --cached | rg -n "^\+.*(TODO|FIXME)"`
Keep these few and fast; they prompt a look, they do not replace review. -->

## Decisions

<!-- Newest last. One line per approved decision, linking its file:
- [001 — Title](decisions/001-name.md) — one-line summary. -->

Everything here is written in English regardless of conversation language.
