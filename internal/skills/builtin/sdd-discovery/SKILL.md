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

4. **Do not invent scope.** Write only what the user actually stated. If a gap needs an assumption to proceed (e.g. "does v1 include a sales history?"), **ask it in your reply** — do not fold a silent assumption into the proposal for the user to catch later. A proposal is the user's, in the user's terms.

5. **When the what/why is settled, write the proposal** (no code):

   ```
   kez sdd propose "<the what & why, in the user's own terms>"
   ```

   `propose` opens this proposal's own branch (`sdd/prop-<slug>`) and writes the doc there, so the proposal, its approval, and the implementation all land in a single PR — not on the default branch. Then stop at the approval gate: point the user at `sdd/proposal.md` and ask them to reply with a short approval (e.g. "aprobado") or the changes they want. Don't ask them to type a command — when they approve, run `kez sdd approve` yourself on their behalf.

## After approval

The next deliberate step is choosing the stack and libraries — that is its own decision, not an afterthought. Load the `sdd-stack` skill for it. Do not scaffold or install anything until then.

## Anti-patterns

- ❌ Calling `ask_user` with 2–4 fixed options for a broad discovery question.
- ❌ Running `npm create` / scaffolding before there is an approved proposal.
- ❌ Deciding the stack inside discovery.
- ❌ Folding in scope or assumptions the user never stated — ask instead.
- ❌ Producing a plan the user never got to shape — ask, then propose.
