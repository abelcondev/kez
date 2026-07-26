---
name: new-app
description: Guided workflow for starting a brand-new application from scratch and driving it forward. Use whenever the user wants to begin a new project, bootstrap a codebase, scaffold an app, or set up a fresh repo — covers discovery, stack selection with version research, native SDD + git, architecture, and foundations, all as passes through one uniform proposal→decision→task→implement loop. Triggers on phrases like "new app", "start a project", "empezar un proyecto", "arrancar de cero", "bootstrap".
---

# new-app — one loop from blank directory to shipping

Everything is the same loop: **discovery, stack, architecture, foundations, and
every feature are all passes through `propose → approve → task → implement`.**
There is no separate "setup mode" that hands off to a "feature loop" later — the
project's first commits are just its first iterations. You accompany the human
through each gate; you never run ahead of them.

Respond in the user's language (mirror whatever they write in).

## Driver — the invariants (these always hold)

- **One step at a time.** After any sub-step that writes files, runs commands,
  or installs deps: report what you did and stop. On "continue"/"sigue", do only
  the *next* sub-step — never the rest of a phase. The human owns every gate.
- **The human owns decisions.** Never assume a package manager. Never use
  `@latest` — pin every version. If a user's choice conflicts with a
  requirement, name the conflict and let them decide; never substitute silently.
- **Resume from disk, not from this skill.** To recover "where am I", run
  **`kez sdd next`** (the single recommended step) or **`kez sdd status`** (full
  loop position). Do NOT re-load this skill every turn — its guidance does not
  change; only the loop position does, and that lives in `sdd/` + git.

## The seed (the only thing outside the loop — two commands)

The loop needs a repo and a knowledge base to write into. That is the whole
bootstrap:

1. **`git init`** — inside the project directory, never a parent (never their
   home). Verify with `git rev-parse --show-toplevel`.
2. **`kez sdd init`** — scaffold the OKF knowledge base (`sdd/`: index, log,
   proposal, decisions/, tasks/).

Then everything runs the loop below.

## The loop (repeat for discovery, architecture, foundations, every feature)

```
kez sdd propose "<what & why>"      → drafts sdd/proposal.md (no code)
        ↓  ── human review gate ──
kez sdd approve --title "..."        → promotes to decisions/NNN, logs, indexes
        ↓
kez sdd task <decision-ref> "<t>"    → pending task with Gherkin acceptance
        ↓
git checkout -b feat/<slug>          → branch before any code (guard enforces it)
        ↓
implement (TDD: red → green), keep files small, verify green
        ↓
push → prepare a `gh pr create …` command for the user to run
        ↓
        └────── merge → back to propose (run `kez sdd next` to confirm)
```

`kez sdd propose` uses the agent to draft the proposal; if no provider is
configured it degrades to a seeded skeleton you fill in by hand — either way the
loop keeps moving. Never write code during a proposal.

## What each kind of pass needs

The loop is identical; only the content of the proposal differs. Consult the
relevant note once, when you actually enter that pass — do not front-load them.

- **Discovery** (→ decision, no code). Ask only questions whose answer changes
  the architecture, 2–3 at a time: users/roles/tenancy; offline & sync;
  device/hardware; legal (e-invoicing, data residency); the one flow that must
  never fail; deployment/hosting/deadline. Summarize, confirm, then approve.
- **Stack** (→ decision, no code). Ask their base tech, package manager, fixed
  pieces, expertise, what to avoid — *first*. Then research **every** library
  (theirs and any you add): resolve real pinned stable versions (docs MCP or the
  registry — never guess), cross-check peer deps together, flag any clash with a
  discovery requirement. Report pinned versions + compatibility.
- **Architecture** (→ decision, no code), held to a senior bar: modular by
  domain (`features/orders/` with its own components/stores/types), thin routes,
  shared primitives in `components/ui/`, each folder annotated. Explicit
  boundaries: routes → features → shared; features never import each other.
  Small files (kez's quality gate blocks any source file over 300 lines — design
  for it). Data model with relations/constraints/indexes. The critical flow
  engineered step by step, including what happens when each step fails (offline,
  hardware down, double-submit, races). Every non-obvious choice justified
  against a discovery answer or a research finding. Zero filler.
- **Foundations** (→ **task**, code — the first implement pass). Scaffold with
  pinned versions and the chosen package manager (if the scaffolder refuses a
  non-empty dir because `sdd/`+`.git` exist, scaffold into a temp subdir and
  move files up). Pin deps in the manifest, install, verify no peer warnings.
  Pin the test runner, co-locate `*.test.*`, get an empty suite green. Add a
  documented `.env.example`; `.gitignore` ignores `.env` and local `.kez/*` but
  keeps `.kez/require-branch` tracked (`.env` / `.kez/*` / `!.kez/require-branch`)
  — never commit `.env`. Write the empty `.kez/require-branch` marker to turn on
  the branch guard so the policy travels with the repo. Acceptance is concrete:
  suite green, `build` passes, git clean, no secrets. Like any task, it lands via
  a branch + PR — not straight to the protected branch.
- **Features / fixes** (→ task, code). Same loop, one branch each.

## Repository & handoff

Once foundations merge, ask: new GitHub repo or existing remote. For a new repo,
give them the `gh repo create …` command to run themselves (do not run it).
Offer minimal CI (lint + typecheck) only if they accept; recommend host branch
protection (require PRs) so the local guard and the host rule reinforce each
other. Nothing lands on the protected branch except through a merged PR —
including small fixes. Never merge or push to the protected branch yourself.

---

The point: **one loop, no special setup mode.** The human keeps control at every
gate; their stack is the base, not something you impose; the project is born with
a durable decision record (`sdd/`), tests running, and clean git — and you resume
by asking the repo (`kez sdd next`), never by reloading this skill.
