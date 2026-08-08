---
name: sdd-design-system
description: Use right after the stack is chosen (or when a UI task needs a component that doesn't exist yet) to build the project's base components in code, in an isolated /design-system workbench route, and review them live before composing them into real screens. This replaces external mockup tools (Figma/Penpot) — components are designed directly in code.
---

# Design system (code-first workbench)

The design system is built and reviewed **as real code, in an isolated route** — not mocked up in an external design tool. External mockup tools are not the gate here; the running `/design-system` route is. This is the foundations pass: it comes right after the stack decision and before any feature UI, and it is the source of truth every later screen composes from.

## Why a workbench route

A dedicated route (e.g. `/design-system`, a Storybook-lite you own) renders every component in isolation, in every state. The human reviews the **live, rendered components** there — hover, focus, loading, error, empty, disabled, each variant — which is far more honest than a static frame. Once approved, screens are assembled from these components, never hand-rolled.

## How the workbench is structured

Three tiers, each a section in the gallery:

1. **Foundations** — tokens: color, spacing, typography scale, radius, shadow, light/dark. Everything binds to these; no magic values downstream.
2. **Primitives** — the atoms: Button, Input, Textarea, Select, Checkbox/Radio, Switch, Label, Badge, Avatar, Spinner, Text/Heading. Each shows every interactive state.
3. **Compositions** — presentational blocks built from primitives: small ones (`FormField` = Label + Input + error, `Card`, `Modal`, `SidebarItem`) and larger ones (`LoginCard`, `Sidebar`, `ContactForm`). These live in the workbench too — a login card, a sidebar, and a form are compositions, not screens.

**The dividing line — presentation vs behavior.** The workbench holds only **presentational, props-driven** components: every state is triggered by props, with no real logic. Data, handlers, validation, routing, and API calls are the *feature's* job, not the design system's.

- Workbench: `<LoginCard state="error" />`, `<LoginCard state="loading" />` — each state rendered from mocked props.
- Feature screen: `<LoginCard onSubmit={realAuth} />` — the real login route wires behavior and covers it with tests.

So a composition like `LoginCard` is designed and reviewed in the workbench in all its visual states; the screen that gives it real auth is separate feature work (`sdd-implement` + `sdd-test`).

### The shell — keep the gallery ordered, not a dump

The route is not a flat scroll of components. Give it a shell so it stays browsable as it grows:

- **A navigation sidebar grouped by the three tiers** (Foundations / Primitives / Compositions), each group listing its components with the active one highlighted. A search/filter on top is a plus.
- **A main panel** rendering the selected component only.
- **A theme toggle** (light/dark) in the header so both themes are reviewable.

**Drive the nav from a registry, never by hand.** Keep a stories manifest — an array of `{ tier, name, component }` — that feeds BOTH the sidebar and the routes, sorted by tier then name. Adding a component is one entry in the registry; it then appears in its group, in order, automatically. The menu can never drift from what exists.

**Deep-linkable routes, one per component** — `/design-system/button`, `/design-system/login-card`. The design gate records the exact component's URL so the reviewer opens straight to it.

**One consistent story frame per component:** title + one-line description → a labeled grid of every state → variants → sizes. Every component reads the same way.

### Suggested layout (adapt to the stack)

```
design-system/
  <route>             # deep-linkable per component: /design-system/:name
  shell               # sidebar (grouped by tier) + main panel + theme toggle
  registry            # the stories manifest: [{ tier, name, component }] — drives nav AND routes
  stories/            # one entry per component, all its states (button, form-field, login-card, sidebar)
components/
  ui/                 # primitives
  patterns/           # compositions (login-card, sidebar, forms)
theme/                # tokens
```

### Adding a component later

When a feature needs a new component (say a login card), do NOT build it inline in the screen. Add it to the workbench first: create `LoginCard` (plus any missing primitive) with every state, its `story`, and one entry in the registry (`{ tier: 'Compositions', name: 'login-card', … }`) — that entry makes it show up in the sidebar and get its own `/design-system/login-card` route. Review it live at the gate, then let the screen compose it. Each new component is a new, ordered gallery section, reviewed in isolation before it touches a real screen. This is the same `sdd-design` gate — the workbench simply grows.

### Iterating on a component (design fixes & improvements)

There is **one source of truth**: the component module. The workbench imports the exact same modules the app screens import — it holds no copies. So a design fix is a one-place edit that updates the gallery AND every screen at once; nothing to "propagate" or sync. This is the whole point of the code-first workbench over an external mockup that drifts from code.

- **Visual change** (spacing, color, radius): edit the component. Workbench + all consumers update together.
- **Token change** (e.g. the primary color): edit the token. Everything bound to it moves together — this is the lever for system-wide design improvements.
- **API/prop change** (rename/add/remove a prop): edit the component AND its consumers plus their tests — it can break screens. Use the workbench (all states) and a consumer check to catch regressions before merge.

A design change still goes through the gate, on a feature branch (so the app only changes on the branch until reviewed):

1. `kez sdd design <decisions/NNN-design-system.md> "Button: tighter padding"` — creates a fresh design artifact, in-review.
2. The in-review design blocks the loop (the design gate has priority), so the human reviews the changed states live in the workbench.
3. `kez sdd approve-design <designs/NNN-slug>` — every consumer already reflects a visual change; no follow-up wiring.

Scope it right: a **fix or improvement to an existing component** is this lightweight pass under the existing `design-system` decision. A **change of design direction** (new token system, a restyle across the board) is a new proposal/decision — record it, because it is a design decision.

## How to run it

1. **Build the isolated route with its shell.** Add a `/design-system` route (or equivalent for the stack) that is not part of the product flow — with the shell from "The shell" below: a tier-grouped sidebar, a main panel, a theme toggle, and a registry that drives both the nav and the per-component routes. Building this route is allowed code even before the design gate — the workbench *is* the design activity.

2. **Build the base tiers, in order** (see "How the workbench is structured" below):
   - **Foundations** — tokens/theme: color, spacing, radius, typography scale, dark/light. Everything else binds to these; no magic values downstream.
   - **Primitives** — Button, Input, Select, Checkbox, Label, Text, Link, plus layout (container, stack/grid, card, separator). Every interactive state visible in the gallery.
   - Only what the product actually needs — don't build a component with no consumer. Feature-specific compositions (LoginCard, Sidebar, a concrete form) are NOT built in this foundations pass; they get added later, per feature, through the same gate.

3. **Show every state.** For each component render: default, hover, focus, active, disabled, loading, error, empty, and each size/variant. If a state can't be triggered by props alone, add a toggle in the gallery so the reviewer can see it.

4. **Record the design and open the gate:**

   ```
   kez sdd design <decisions/NNN-design-system.md> "Design system workbench"
   ```

   In the artifact, link the **running route** — the index (`http://localhost:<port>/design-system`) plus the deep link of each component (`/design-system/<name>`) — and drop a screenshot of each component's section. List the components the workbench now provides.

   ```
   kez sdd approve-design <designs/NNN-slug>
   ```

5. **Stop at the gate.** The human opens the route and exercises the components live. Ask them to reply with a short approval (e.g. "aprobado") — don't make them type a command; when they approve, run `kez sdd approve-design <designs/NNN-slug>` yourself on their behalf. Without approval the loop refuses feature UI — do not route around it.

## Recording it in the index

After approval, record the workbench in `sdd/index.md` under **UI conventions**: the route path, where tokens/theme live, and the rule that screens compose from these components. Later UI tasks read this and build from the workbench; a missing primitive is added here first (a small design-system pass) before the screen that needs it.

## Verification split

- **Component look & states** — verified live by the human in the workbench against the approved screenshots.
- **Component logic** (formatting, validation, state transitions) — covered by code tests via `sdd-test`.

## After approval

The workbench unblocks feature UI. Feature screens load `sdd-design` (which now also points at this workbench) and `sdd-implement`, and compose from the approved components — never re-implementing a primitive that already exists.

## Anti-patterns

- ❌ Mocking the design system in an external tool instead of building it in code.
- ❌ Building screens or features before the workbench is approved.
- ❌ Hand-rolling raw CSS / utility-class soup in a screen when a workbench component exists (or should).
- ❌ Padding the library with components no screen consumes.
- ❌ Hardcoding color/spacing values instead of binding to the tokens.
- ❌ A flat, unnavigable dump of components — no sidebar, no tiers. Use the shell.
- ❌ Maintaining the sidebar/nav by hand instead of generating it from the registry (it will drift).
