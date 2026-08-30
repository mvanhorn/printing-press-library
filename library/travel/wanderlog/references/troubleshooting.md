# Troubleshooting Reference

Command failures, live API surprises, validation blocks, and verification quirks. Auth cookie rules: `SKILL.md`.

## Auth And Access

- `wanderlog-pp-cli doctor` first.
- Public reads work without auth; private reads/edits need the saved cookie.
- If `doctor` says API unreachable in a sandbox, retry with network before concluding Wanderlog is down.
- Shared-view URL may be readable while the editable key still needs account access.

## ShareDB Subscribe Failures

`ShareDB subscribe did not return target snapshot` means the WebSocket subscribed but did not return usable `data.data`. Check for an upstream error frame if debugging the CLI.

`expense.splitWith.users is not iterable`: the expense `splitWith` lacks a `users` array. Shape rules: `budget.md`. If a disposable clone cannot subscribe, make a fresh private clone — do not keep mutating a real collaborative plan.

## Live Test Hygiene

Use a disposable clone. After undo/redo tests, leave the plan clean and remove disposable journal records so future `redo` cannot reinsert test content. Track clone keys in session notes; do not hard-code them into the skill.

## Skill Verifier Quirk

The Printing Press skill verifier parses bash snippets conservatively. Keep verifier-facing bash in `SKILL.md` to commands that parse cleanly; put deep nested examples in references.

Exit codes and feedback: `SKILL.md`.
