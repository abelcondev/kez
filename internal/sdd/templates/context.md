# Working context

Durable, proposal-scoped working memory: the surface the current proposal's
tasks keep re-deriving — API shapes, store/module methods, key file paths, and
gotchas discovered while implementing. It survives turns and context compaction,
so a new turn reads this instead of re-exploring the same files.

Maintained by `sdd-implement`; read at the start of every implement turn. Keep
it short and current — it is a map, not a log. When a proposal's PR merges,
clear its section so the notes don't outlive the work.

<!-- Group notes under the proposal's branch so several in-flight proposals don't
mix. Example:

## feat/002-owner-auth

- API: `POST /api/staff/signin { accountId, pin } → { token, account }` (public).
- Store: `WorkspaceStore.findFirst()` returns the single MVP workspace (no owner filter).
- Types: `StaffRole = 'caja' | 'cocina'` lives in `src/lib/staff-role.ts` — never redefine.
- Key files: `src/routes/api/staff/*`, `src/lib/staff-session.ts`.
- Gotcha: the InstantDB fake in tests must mirror the real store, or 404s slip past green tests.
-->
