---
name: sdd-discovery
description: Use at the START of any new app/feature or when the SDD loop points here — turn a raw idea into an approved proposal by refining requirements with the user in plain text. Do NOT scaffold or pick a stack yet.
---

# Discovery

You are turning a raw idea into a written proposal. **No code, no scaffolding, no stack decisions, no `ask_user` tool.** This phase is a conversation, not a build.

## How to run it

1. **Refine in plain text, iteratively.** End your turn with open questions written directly in your reply so the user answers freely. Do NOT use the `ask_user` tool — its fixed options anchor the user to your assumptions and cut off the context you need most. Ask a few at a time, then react to the answers and go deeper.

2. **Cover the what & why before anything else:**
   - Who uses it, and in what setting? (e.g. a cashier on a tablet, mid-rush)
   - What is the ONE flow that must not fail?
   - What are the hard constraints? (offline? one device? peak load? money handling?)
   - What is explicitly out of scope for v1?
   - What does "done" look like for the first slice?

3. **Do not jump to solutions.** If the user names a stack or library, note it for the stack phase — do not start designing around it here. Discovery is about the problem, not the implementation.

4. **When the what/why is settled, write the proposal** (no code):

   ```
   kez sdd propose "<the what & why, in the user's own terms>"
   ```

   Then stop at the approval gate — the user reviews `sdd/proposal.md` and runs `kez sdd approve`.

## After approval

The next deliberate step is choosing the stack and libraries — that is its own decision, not an afterthought. Load the `sdd-stack` skill for it. Do not scaffold or install anything until then.

## Anti-patterns

- ❌ Calling `ask_user` with 2–4 fixed options for a broad discovery question.
- ❌ Running `npm create` / scaffolding before there is an approved proposal.
- ❌ Deciding the stack inside discovery.
- ❌ Producing a plan the user never got to shape — ask, then propose.
