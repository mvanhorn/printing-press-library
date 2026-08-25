# Amend acceptance — `db schema` scoped to the CLI's own store

Contract-to-evidence table for the change on `feat/granola-db-schema`, taken
after merging current `main`. Raw transcripts are in
`20260817-amend-build-log.md` alongside this file.

## What changed and why

`db schema` used to accept `--db <path>` and inspect any SQLite file. On a file
the process does not own, SQLite offers two opens and each one gives up
something the command promises. A WAL-aware open creates a `-shm` next to a
file in someone else's directory. An immutable open creates nothing but reads
the schema as of the last checkpoint, so it can be stale without saying so.
Five review rounds went into choosing between them; there is no third open.

Scoping the command to the CLI's own store removes the choice. Every granola
command already opens that file and already maintains its `-shm`, so the
WAL-aware read-only open is ordinary operation and the schema printed is
current.

## Contract → evidence

| Contract | Evidence |
|---|---|
| `db schema` prints the owned store's path, tables, and columns | Live run against the real store returns the JSON shape with `attendees`, `meetings`, `folders` and their columns |
| The columns published are the ones scripts actually hit | `TestDBSchemaListsRealColumns` asserts `meetings.row_source` present / `meetings.source` absent, `folders.title` present / `folders.name` absent |
| Inspection never writes to the store | sha256 of `data.db` identical before and after a live run |
| `--db` is gone, not merely undocumented | `db schema --db /tmp/x.db` exits 1 with `unknown flag: --db` |
| A missing store is a typed not-found, not a generic failure | Under an isolated `HOME`, exit 3 with `no local store at … — run sync first`; `TestDBSchemaMissingStoreExitsNotFound` pins it |
| No dead code left behind by the removal | `store.OpenImmutable`, `store.WALPending` and `snapshot_test.go` deleted in the same commit; `go vet` clean |
| The docs describe the shipped behavior | `verify_skill.py` passes all checks, including flag-names against `internal/cli/*.go` |

## Declared caveats

The live smoke ran against a real store, so its output is truncated in the
transcript and every home path is redacted to `<HOME>`. Table and column names
are the CLI's own schema, not user content, so nothing personal appears.

Reading a store copied from another machine now requires putting the file at
the CLI's own store path. That is a real reduction in reach, taken deliberately:
a flag whose two possible semantics were both rejected in review is worse than
a documented manual step.

`db_test.go` reaches the command through `HOME` rather than a target flag. Five
other granola test files already use `t.Setenv("HOME", …)`, so the pattern is
the repo's, not a new convention introduced here.
