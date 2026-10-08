# undo: sandbox write lifecycle proof

`undo <batch-id>` previews or reverses a batch that `apply --yes` recorded in the local journal. With no such batch it exits 2 ("batch not found"), so the live dogfood matrix cannot give it a real happy path. This file records the real lifecycle it was verified with.

Sandbox: one top-level folder, `/pp-sandbox-20261006-161517-620ff191`, created for the test and soft-deleted at the end. Isolated CLI home and a separate test index (`<run>/sandbox.db`). No token values, account ids, or shared-link URLs are recorded here. Full step log: `2026-10-06-161517-sandbox-write-proof.md` in this directory.

## Commands and exit codes

| step | exit | command |
|---|---|---|
| create sandbox | 0 | `dropbox-pp-cli files create-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| upload 5 fixtures | 0 | `dropbox-pp-cli files upload <file> /pp-sandbox-20261006-161517-620ff191/<file> --agent` |
| index | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| plan | 0 | `dropbox-pp-cli dupes --under /pp-sandbox-20261006-161517-620ff191 --plan dupes-plan.json --db <run>/sandbox.db --agent` |
| check | 0 | `dropbox-pp-cli plan check dupes-plan.json --db <run>/sandbox.db --agent` |
| apply | 0 | `dropbox-pp-cli apply dupes-plan.json --yes --db <run>/sandbox.db --agent` (2 soft deletes, ok=2) |
| verify delete | 0 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191/dup-b.txt --include-deleted --agent` (deleted) |
| journal | 0 | `dropbox-pp-cli journal 20261007-191611-631c --ops --db <run>/sandbox.db --agent` |
| undo preview | 0 | `dropbox-pp-cli undo 20261007-191611-631c --db <run>/sandbox.db --agent` (would restore 2, no writes) |
| undo | 0 | `dropbox-pp-cli undo 20261007-191611-631c --yes --db <run>/sandbox.db --agent` (ok=2) |
| verify undo | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` (all 5 files back, original content hashes; batch status undone) |
| organize apply | 0 | `dropbox-pp-cli apply organize-plan.json --yes --db <run>/sandbox.db --agent` (mkdir ok=2, move ok=1) |
| organize undo | 0 | `dropbox-pp-cli undo 20261007-191914-0f21 --yes --db <run>/sandbox.db --agent` (move ok=1; the 2 created folders are left in place with warnings, as documented) |
| verify organize undo | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` (file back at the sandbox root) |

## Cleanup

`dropbox-pp-cli files delete --path /pp-sandbox-20261006-161517-620ff191 --agent` exit 0. `files get-metadata --include-deleted` on the sandbox returns `deleted` (soft delete, restorable for 30 days). No other path in the account was written.
