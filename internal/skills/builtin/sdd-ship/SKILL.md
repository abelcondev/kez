---
name: sdd-ship
description: Use to close a reviewed task and land it in the proposal's pull request — mark it done, push the branch, keep a draft PR open until the proposal is complete. One PR per proposal; never merge to the protected branch yourself.
---

# Ship

Close the task and keep the proposal's PR current. This is the end of one turn of the feature loop. The PR is opened as a **draft** on the first task and only marked ready once the whole proposal is done — so CI runs and reviewers can follow along the whole time, but nobody is pinged to review or tempted to merge a half-built proposal.

## Preconditions

- Validators pass (tests, typecheck, lint, build).
- `sdd-test` coverage in place for every `code`-level Given/When/Then.
- `sdd-review` run across both lenses (correctness+security, craft); every high-severity finding fixed, residual medium/low listed in the PR.

## How to run it

This step is **resumable**. A previous turn may have run out of budget partway through — so don't blindly redo finished work or open a second PR. Check what's already true, then do only the missing steps. Front-load the close and push (steps 1 and 3) before any optional polish, so a budget cutoff never leaves the task unclosed or the work unpushed.

1. **Close the task** — flips its status and appends to `sdd/log.md`. Skip if it's already `done`:

   ```
   kez sdd done <task-name>
   ```

   Its output prints the proposal's task checklist (done/pending) and tells you whether the proposal is now complete. Use that checklist verbatim for the PR body in step 5, and its complete/incomplete verdict for step 6.

2. **One PR per proposal.** All tasks of a proposal share one branch and land in a single PR. Never open a PR per task.

3. **Push the branch** — no upstream tracking. `-u` writes `.git/config`, which the sandbox refuses (`Operation not permitted`); it is not needed:

   ```
   git push origin HEAD
   ```

4. **Open the draft PR, or reuse the existing one.** Check before creating so you never open a second PR:

   ```
   gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --draft
   ```

   Never push to or merge into the protected branch (`main`/`master`) yourself.

5. **Refresh the PR body** with the current task checklist from step 1's output plus the `manual`-level acceptance criteria (browser/visual/e2e) the human still needs to verify — the tests cover the code, the human covers the screen:

   ```
   gh pr edit --body "<checklist + manual QA notes>"
   ```

6. **Mark the PR ready only when the proposal is complete** — i.e. step 1 reported no pending tasks left. Until then leave it a draft and move to the next task:

   ```
   gh pr ready
   ```

## After merge

Back to the top of the feature loop: the next task or the next proposal. Run `kez sdd next` for the single next step.

## Anti-patterns

- ❌ Merging to `main` yourself.
- ❌ A separate PR for each task of the same proposal.
- ❌ Marking the PR ready (or opening it non-draft) while tasks of the proposal are still pending.
- ❌ `git push -u …` — the `-u` fails in the sandbox; push without it.
- ❌ Opening a second PR when the proposal already has one — reuse it and refresh its body.
- ❌ Closing without listing what the human still has to verify manually.
