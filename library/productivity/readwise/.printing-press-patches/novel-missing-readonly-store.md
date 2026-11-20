# Novel Missing Read-Only Store Grace

## Why this local patch exists

Novel read commands open the local SQLite database read-only to avoid
`SQLITE_BUSY` failures under concurrent readers. SQLite read-only open fails
when the database file has never been created, which broke fresh users and
verify/scorecard environments that have not run `readwise-pp-cli sync`.

## Reprint guard

On reprint, preserve the missing-store behavior for all novel read commands:

- If `--db-path` exists, open it with `store.OpenReadOnly`.
- If `--db-path` does not exist, return exit 0 with the command's normal empty
  JSON/human view and a sync-first `note`.
- Do not fall back to read-write open for read-only commands.
- Keep `dryRunOK` early returns before any database check.

The regression coverage must include missing `--db-path` JSON cases for
`backlog`, `read-next`, `ask`, and `triage stale`.
