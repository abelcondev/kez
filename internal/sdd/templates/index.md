# Knowledge Index

This is the Spec-Driven Development (SDD) knowledge base for this project, in
Open Knowledge Format (OKF). It is read first at the start of every session.

- `proposal.md` — the current, in-review proposal (transient; cleared on approval).
- `decisions/` — approved, numbered architectural decisions (historical).
- `designs/` — approved UI designs (the live `/design-system` workbench route + screenshots), the gate before UI code.
- `tasks/` — units of work with Gherkin acceptance criteria.
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

## Decisions

<!-- Newest last. One line per approved decision, linking its file:
- [001 — Title](decisions/001-name.md) — one-line summary. -->

Everything here is written in English regardless of conversation language.
