# Amend build log — scoping `db schema` to the owned store

Branch `feat/granola-db-schema` after merging current `main`.
Raw transcripts, unedited except for home-path redaction.

## Build

```
$ go build ./...
exit=0
```

## Vet

```
$ go vet ./...
exit=0
```

## Test suite

```
$ go test ./...
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/cmd/granola-pp-cli	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/cmd/granola-pp-mcp	[no test files]
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cache	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cli	(cached)
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/client	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil	(cached)
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/config	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/mcp	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/mcp/cobratree	(cached)
ok  	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store	(cached)
?   	github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/types	[no test files]
```

## SKILL.md verifier

```
$ python3 .github/scripts/verify-skill/verify_skill.py --dir library/productivity/granola/
=== granola ===
  ✓ All checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command)
```

## Supply chain

```
$ govulncheck ./...

No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
This scan also found 0 vulnerabilities in packages you import and 1
vulnerability in modules you require, but your code doesn't appear to call these
vulnerabilities.
Use '-show verbose' for more details.
```

## Live smoke, read-only against the real owned store

The store file was hashed before and after the run:

```
before  40f937541884fedb60aad862eb486d2dd38293c4edd8a8a12875bd8656fb0466
after   40f937541884fedb60aad862eb486d2dd38293c4edd8a8a12875bd8656fb0466
```

Identical, so inspection wrote nothing.

```
$ granola-pp-cli db schema --help
Print the store's path and its tables and columns.

The target is the CLI's own store, always. Inspection is read-only and
WAL-aware: the schema shown is current, including changes not yet
checkpointed, and nothing is written. To inspect a store from elsewhere
— another machine, a backup — put the file at the CLI's own store path
and read it there.

Usage:
  granola-pp-cli db schema [flags]

Examples:
  # Every table and column, human-readable
  granola-pp-cli db schema

  # Machine-readable, for scripts that query the store directly
  granola-pp-cli db schema --json

  # Just one table's columns
  granola-pp-cli db schema --json --select tables.name,tables.columns

$ granola-pp-cli db schema        # live, real owned store (stdout not a tty -> JSON)
{
  "path": "<HOME>/.local/share/granola-pp-cli/data.db",
  "tables": [
    {
      "name": "attendees",
      "columns": [
        {
          "name": "meeting_id",
          "type": "TEXT",
          "not_null": true,
          "pk": true
        },
        {
          "name": "email",
          "type": "TEXT",
          "not_null": true,
          "pk": true
        },
  ... truncated; exit=0

$ granola-pp-cli db schema --db /tmp/x.db
unknown flag: --db
exit=1  (flag removed)

$ HOME=<isolated> granola-pp-cli db schema
no local store at <ISOLATED_HOME>/.local/share/granola-pp-cli/data.db — run sync first
exit=3  (not-found, not generic 1)
```
