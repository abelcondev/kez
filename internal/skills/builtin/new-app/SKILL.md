---
name: new-app
description: Guided workflow for starting a brand-new application from scratch. Use this whenever the user wants to begin a new project, bootstrap a codebase, scaffold an app, or set up a fresh repo — covers discovery, stack selection with version research, native SDD + git setup with an architecture proposal, and project scaffolding. Triggers on phrases like "new app", "start a project", "empezar un proyecto", "arrancar de cero", "bootstrap".
---

# new-app — the discovery-to-foundations copilot

You are guiding a human from a blank directory to a project with durable
decisions recorded, tests running, and git clean — **before** the first line of
feature code. You accompany them; you do not run ahead of them.

Respond in the user's language (mirror whatever they write in).

## The golden rule — one step at a time

After every sub-step that writes files, runs commands, or installs
dependencies: **report what you did and stop for confirmation.** If the user
says "continue" / "sigue", do only the *next* sub-step — never the whole
remaining phase. The human owns every gate.

Never assume a package manager (bun / pnpm / npm / yarn). Never use `@latest`;
pin every version. Never substitute a user's decision silently — if their choice
conflicts with a requirement, name the conflict and let them decide.

---

## Phase 1 — Discovery

Ask only questions whose answer changes the architecture. Group them 2–3 per
message, not all at once. Cover, as relevant:

- **Users & concurrency**: roles, concurrent users, single- vs multi-tenant.
- **Connectivity**: must it work offline? sync on reconnect?
- **Device & interaction**: desktop / tablet / mobile / kiosk; hardware
  (thermal printer, scanner, camera…).
- **Legal & compliance**: e-invoicing (SUNAT/SAT/AFIP…), data residency (GDPR…).
- **Business logic**: the one flow that must never fail; external integrations;
  migration of existing data.
- **Deployment**: who hosts, budget, who maintains, deadline.

**Gate:** hand back a short summary of the answers and get confirmation before
moving on.

## Phase 2 — Their stack first, research second

Never propose a stack without asking. Ask conversationally first: base
technologies already in mind (frontend, backend/BaaS, DB), package manager,
pieces already decided, where they have the most expertise, what to avoid.

Then, mandatory research for **each** library (theirs and the ones you add — UI,
icons, state, forms, validation):

1. If a docs MCP is connected (e.g. context7): `resolve-library-id` → fetch the
   library docs. If none is available, use the npm registry / the library's
   release page directly. Either way, **do the research** — do not guess versions.
2. Cross-check peer dependencies across all candidates together.

For each library report: pinned stable version (never `@latest`), confirmed
compatibility, relevant breaking changes, and whether it is production-ready for
the Phase 1 requirements. If a choice of theirs clashes with a discovery
requirement (e.g. offline-first vs a server-only stack), flag it — they decide.

## Phase 3 — SDD + Git + Architecture proposal

One sub-step at a time:

1. **`git init`** — inside the project directory, never a parent (never their
   home). Verify with `git rev-parse --show-toplevel` before proceeding.
2. **`kez sdd init`** — scaffold the native OKF knowledge base (`sdd/`: index,
   log, proposal, decisions/, tasks/).
3. **Record the discovery** — `kez sdd propose "Discovery: <summary of Phase 1
   answers>"`, review, then `kez sdd approve --title "Discovery"`. This writes
   `sdd/decisions/001-discovery.md`, appends `log.md`, updates the index.
4. **Architecture proposal** — `kez sdd propose "<architecture>"` to draft
   `sdd/proposal.md`, held to a senior quality bar:
   - **Modular structure by domain** (`features/orders/` with its own
     components/stores/types), thin routes, shared primitives in
     `components/ui/`. Annotate each folder with its purpose. Keep files small —
     kez's compiled quality gate blocks any source file over 300 lines, so a
     god-file architecture will not even write. Design modular from the start.
   - **Explicit module boundaries**: dependency direction routes → features →
     shared; features never import each other.
   - **Data model with intent**: relations, constraints, indexes for hot queries.
   - **The critical flow, engineered**: step by step and what happens when each
     step fails (offline, hardware down, double-submit, races).
   - Every non-obvious decision justified against a discovery answer or a
     research finding. Zero generic filler.

**Hard gate:** tell them to review `sdd/proposal.md` and STOP. No scaffold, no
installs, no more files until explicit approval. If they change the stack, go
back to Phase 2 research and update the proposal.

## Phase 4 — Setup (only after approval)

1. **Archive the decision** — `kez sdd approve --title "Architecture"` promotes
   the proposal to `sdd/decisions/002-architecture.md`, logs it, resets the
   proposal stub.
2. **Scaffold the app** — with pinned versions and the chosen package manager.
   If the scaffolder refuses a non-empty directory (`sdd/` and `.git` already
   exist), scaffold into a temp subdirectory and move the files up. Show the
   generated structure.
3. **Dependencies** — pin in the manifest, install, verify no peer-dependency
   warnings.
4. **Test runner** — pin it, co-locate `*.test.*` files (the SDD/TDD loop
   depends on this), add the test script, get an empty suite green.
5. **Env & gitignore** — a documented `.env.example`; `.gitignore` ignores
   `.env`, local `.kez/*` state, and OS/editor noise, but KEEPS the shared
   `.kez/require-branch` marker tracked so the branch policy travels with the
   repo:

   ```gitignore
   .env
   .kez/*
   !.kez/require-branch
   ```
   Never commit `.env`.
6. **First commit** — check `git status` for leaks, propose the message, verify
   with `git log -1`. (This foundations commit is the one time code lands on the
   default branch directly; everything after goes through a branch + PR.)
7. **Require feature branches** — write an empty `.kez/require-branch` marker.
   This turns on kez's compiled branch guard: from now on it refuses code writes
   while HEAD is on a protected branch (`main`/`master`/the remote default), so
   every feature must start on a branch. Because the marker is committed,
   teammates cloning the repo get the same policy. Commit it (e.g. `chore:
   require feature branches for all changes`). (Override per-run with
   `KEZ_REQUIRE_BRANCH=off` only when you know what you're doing.)
8. **Repository** — ask: new GitHub repo or existing remote. For a new repo,
   give them the `gh repo create …` command to run themselves (do not run it).
   Offer minimal CI (lint + typecheck) only if they accept. If the remote's
   default branch supports it, recommend enabling branch protection (require PRs)
   on the host — the local guard and the host rule then reinforce each other.
9. **Handoff** — the workflow ends here. From now on EVERY change runs the
   native SDD loop on its own feature branch:
   `git checkout -b feat/<task-slug>` → mini-discovery → `kez sdd propose` →
   approval → `kez sdd task <decision-ref> <title>` → implement (TDD) → push →
   prepare a `gh pr create …` command for the user to run. Nothing lands on the
   protected branch except through a merged PR — including small fixes. Never
   merge or push to the protected branch yourself.

---

The point: the human keeps control at every gate (discovery summary,
architecture proposal, every setup sub-step); their stack is the base, not
something you impose; and the project is born with a durable decision record
(`sdd/`), tests running, and clean git — before any feature code exists.
