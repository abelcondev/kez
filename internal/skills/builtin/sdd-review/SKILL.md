---
name: sdd-review
description: Use after a task is green and before opening its PR — run a fresh-context review of the diff for correctness and security, fix every high-confidence finding, then proceed to done/PR. Precedes (does not replace) the human's PR review.
---

# Review (automated quality gate)

A second pass with clean context catches blind spots the implementing context misses. This gate runs **before the PR** and does not replace the human review at merge — it precedes it, so the human sees already-audited code.

## How to run it

1. **Review the diff, not the whole repo.** Focus on what this task changed.

2. **Use the existing review machinery** — do not reinvent it:
   - `/code-review` for correctness bugs and reuse/simplification findings.
   - `/security-review` for the pending changes (secrets, injection, authz, unsafe I/O).

3. **Address every high-confidence finding before closing.** Fix the cause, not the symptom; re-run the validators after fixing. If you judge a finding a false positive, say why rather than silently ignoring it.

4. **Gate:** do not run `kez sdd done` while a high-confidence correctness or security finding is unresolved.

## What this is not

- Not the human's PR review — that still happens at merge. This is the automated pre-pass.
- Not a rewrite pass — keep the diff scoped to the task; fix findings, don't gold-plate.

## After this

Findings cleared → load `sdd-ship` to close the task and open the PR.

## Anti-patterns

- ❌ Skipping review and going straight from green to `done`.
- ❌ Suppressing a real finding to close faster.
- ❌ Expanding scope under the banner of "review" — that is a new task.
