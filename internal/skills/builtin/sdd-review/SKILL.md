---
name: sdd-review
description: Use after a task is green and before opening its PR — run a fresh-context review of the diff across two lenses (correctness+security, and craft/maintainability via a reviewer subagent), auto-remediate the findings in one pass, then re-review. High-severity findings block done/PR. Precedes (does not replace) the human's PR review.
---

# Review (automated quality gate)

A second pass with clean context catches blind spots the implementing context misses. This gate runs **before the PR** and does not replace the human review at merge — it precedes it, so the human sees already-audited code.

Review has **two lenses** and a **remediation loop**: find issues with fresh eyes, fix them, then re-check that the fixes hold.

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

1. **Round 1** — run Lens A and Lens B; collect all findings.
2. **Fix** — address every **high** and **medium** finding: split the file, extract the helper, rewire, dedupe, delete the dead code. Fix the cause, not the symptom. Keep the diff scoped to the task — this is remediation, not gold-plating.
3. **Re-run validators** (tests, typecheck, lint, build) after the fixes.
4. **Round 2** — re-run the craft subagent on the new diff to confirm the fixes hold and introduced no new smells.
5. Stop after **2 rounds** — do not iterate forever chasing polish.

## Gate

- **Do not run `kez sdd done` while any high-severity finding is unresolved** — correctness, security, or craft. If you judge one a false positive, say why rather than silently ignoring it.
- Any **residual medium/low** you consciously chose not to fix goes in the PR description as a "known residual" with a one-line justification. Nothing is hidden.

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
