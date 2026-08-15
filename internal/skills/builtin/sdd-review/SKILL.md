---
name: sdd-review
description: Use after a task is green and before opening its PR — run a fresh-context review of the diff across two lenses (correctness+security, and craft/maintainability via a reviewer subagent), auto-remediate the findings in one pass, then re-review. High-severity findings block done/PR. Precedes (does not replace) the human's PR review.
---

# Review (automated quality gate)

A second pass with clean context catches blind spots the implementing context misses. This gate runs **before the PR** and does not replace the human review at merge — it precedes it, so the human sees already-audited code.

## Scale the review to the task's tier

The task carries a **tier** (shown by `kez sdd next`/`status`, resolved from its frontmatter or inferred). Match the review depth to it so a placeholder screen doesn't pay the same tax as the payment flow:

- **trivial** — Lens 0 (cheap greps) + a single correctness pass. Skip the craft subagent and skip round 2. **Do not refactor adjacent files** under review — fix only what the task touched.
- **standard** (default) — Lens 0 + Lens A + Lens B, with the remediation loop below (round 2 only if round 1 changed code).
- **critical** (money, auth, data) — the full thing: both rounds **always** run, and `/security-review` (Lens A) is mandatory, never skipped.

If a task tagged trivial/standard turns out to touch money, auth, or data, **bump it up** — say so and review at the higher tier. Never silently under-review; only escalate, never quietly downgrade.

Review has a **cheap static pre-pass**, **two model-driven lenses**, and a **remediation loop**: knock out the deterministic smells for free, find the rest with fresh eyes, fix them, then re-check that the fixes hold.

## Lens 0 — cheap static pre-pass (run first)

Before spending a single model-driven review round, run the deterministic checks recorded under `## Cheap review checks` in `sdd/index.md` against the diff (they are plain greps — literal glyphs in UI templates, a domain type redefined outside its home module, `TODO`/`FIXME` introduced, etc.). Fix every hit now. These are cheap and repeatable, so catching them here keeps Lens A/B focused on real correctness and design judgment instead of burning a 3–4 minute round on a duplicated label or an emoji that should be an icon. If `sdd/index.md` records no checks yet, skip this pass (and consider adding the obvious ones for the stack).

## Lens A — correctness & security

Use the existing review machinery; do not reinvent it:

- `/code-review` — correctness bugs and reuse/simplification findings.
- `/security-review` — the pending changes (secrets, injection, authz, unsafe I/O).

## Lens B — craft & maintainability (reviewer specialist)

Delegate to a **fresh-context `reviewer` specialist** — spawn it via the swarm (`swarm_spawn agent_type=reviewer` with the diff/task as its briefing), then `swarm_collect` its findings. Its independence is the point: it did not write this code, so it will not rationalize it. The `reviewer` already carries the rubric below in its own prompt; keep this here so you (the orchestrator) can interpret and remediate what it returns. It judges *how the feature was built* — not whether it works, but whether it was built well — and returns structured findings:

- **Size smells** — a file past ~300–400 lines, an over-long function/component, or a component with too many props. Flag as a split candidate. This is a *smell*, not an automatic fail: a cohesive file can be long, so the finding must say *why* splitting helps.
- **Wiring & coupling** — are modules wired cleanly? No circular or awkward imports; layering respected. For UI: screens compose from the `/design-system` workbench (not hand-rolled), and the presentation-vs-behavior split holds — data, handlers, and API calls do not leak into presentational components.
- **Reuse** — duplicated logic that should be a shared util/component; hand-rolled UI where a workbench component already exists.
- **Cohesion & naming** — one responsibility per module, clear names, no dead code, no leftover TODOs or commented-out blocks.
- **Conventions** — matches `sdd/index.md` (stack, UI conventions, test conventions).

Have it return findings as a list of `{ file, lines, category, severity (high|medium|low), why, concrete_fix }`. Severity guide:

- **high** — broken/confused wiring, a layering violation, an unmanageably large file, or duplicated logic that will bite. **Blocks done/PR.**
- **medium** — a clear improvement (extract a helper, tighten a boundary). Fixed in the remediation pass.
- **low** — a nit (naming, a small dead branch). Fixed if cheap; otherwise noted.

## The remediation loop

1. **Round 1** — run Lens 0 (cheap greps) first and fix its hits, then run Lens A and Lens B; collect all findings.
2. **Fix** — address every **high** and **medium** finding: split the file, extract the helper, rewire, dedupe, delete the dead code. Fix the cause, not the symptom. Keep the diff scoped to the task — this is remediation, not gold-plating.
3. **Re-run validators** (tests, typecheck, lint, build) after the fixes.
4. **Round 2 — only if round 1 changed code** (always for `critical`). If round 1 found nothing to fix, the diff is unchanged and a second pass is wasted work — skip it. Otherwise re-run the craft subagent on the new diff to confirm the fixes hold and introduced no new smells.
5. Stop after **2 rounds** — do not iterate forever chasing polish.

## Gate

- **Do not run `kez sdd done` while any high-severity finding is unresolved** — correctness, security, or craft. If you judge one a false positive, say why rather than silently ignoring it.
- Any **residual medium/low** you consciously chose not to fix must become **tracked work**, not a line that dies in a merged PR body. Pass it to the close step as `kez sdd ship <task> --residual "<finding>"` (repeatable) so it lands as a follow-up task on the same decision. Still note it in the PR description too, with a one-line justification. Nothing is hidden and nothing is forgotten.

## What this is not

- Not the human's PR review — that still happens at merge. This is the automated pre-pass.
- Not a rewrite pass — keep the diff scoped to the task; fix findings, don't gold-plate.

## After this

Findings cleared → load `sdd-ship` to close the task and open the PR.

## Anti-patterns

- ❌ Skipping review and going straight from green to `done`.
- ❌ Reviewing craft in the implementing context instead of a fresh-context subagent (it will rationalize its own choices).
- ❌ Suppressing a real finding to close faster.
- ❌ Expanding scope under the banner of "review" — that is a new task.
- ❌ Splitting a file just to hit a line count when it is genuinely cohesive — the number is a prompt to look, not a rule.
