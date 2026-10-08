<!-- slop-gate: off -->
# dropbox-pp-cli sandbox write lifecycle proof

Scope: every write was confined to the top-level folder `/pp-sandbox-20261006-161517-620ff191`, created at the start and soft-deleted at the end. Binary built from the CLI working tree (commits 9c53c2c, cbd543d). Isolated home (copies of config and credentials), separate test index `<run>/sandbox.db`. PRINTING_PRESS_VERIFY / PRINTING_PRESS_DOGFOOD unset so `apply --yes` and `undo --yes` could run. No token values, account ids, or full shared-link URLs are recorded here.

## Fixtures
Five tiny text files: `dup-a.txt` and `dup-b.txt` (identical), `notes.txt`, `notes (Test's conflicted copy 2026-10-07).txt` (identical to notes.txt), `unique.txt`.

## Results
1. Create + upload: folder created, 5 uploads exit 0; Dropbox content_hash identical for each identical pair.
2. `index --root <sandbox>`: 6 entries, root complete.
3. `dupes --plan`: 2 groups, 2 soft-delete ops (dup-b.txt, the conflicted copy), keepers dup-a.txt and notes.txt. `conflicts --plan`: 1 conflicted_copy pair classified identical, 1 delete op.
4. `plan check` ok (0 errors, 0 warnings). `apply` preview: would 1 delete batch of 2, no writes.
5. `apply --yes`: batch complete, delete ok=2. Verified: list-folder shows dup-a.txt, notes.txt, unique.txt; get-metadata --include-deleted on dup-b.txt returns `deleted`.
6. `journal` / `journal <batch> --ops`: batch recorded with rev, entry id, async job id, result ok for both ops; restore window 30 days.
7. `undo <batch>` preview: would restore 2. `undo --yes`: complete ok=2. Verified: all 5 files back with the original content hashes. Original batch status `undone`.
8. `organize --match 'unique*.txt' --to <sandbox>/{year}/{month} --tz UTC --plan`: 2 mkdir + 1 move; plan check ok; preview mkdir=2, move batches [1]. `apply --yes`: mkdir ok=2, move ok=1; file present at `<sandbox>/2026/10/unique.txt`, old path not_found. `undo --yes`: move ok=1, the 2 created folders left in place with warnings (documented behavior); unique.txt back at the sandbox root.
9. Shared link + dangling-link revoke:
   - `sharing create-shared-link` on unique.txt: public link created. `links audit` (no plan) listed it with flags public, no_expiry.
   - Soft-deleted unique.txt via a one-op plan: plan check ok (no link-target warning for deleting a linked file), `apply --yes` delete ok=1, metadata `deleted`.
   - `links audit --revoke dangling --plan`: dangling=0, plan written with 0 ops. Dropbox removed the link itself when its target was deleted (account link total dropped by one; get_shared_link_metadata on the old URL returns 409 shared_link_not_found). plan check on the empty plan ok; `apply --yes` returned status noop, revoke counts all 0.
   - Exact answer to "did the revoke happen or did the get_shared_link_metadata guard skip it": neither. No dangling link existed to revoke, so no revoke op was planned and the apply-time guard was never reached. On this account, deleting a file removes its shared links, so a delete-then-audit sequence cannot produce a dangling link.
   - Guard coverage that was reachable: a hand-written revoke plan for the auto-removed URL with expect_dangling was rejected by plan check (`unknown_link: link is not in the local index`, exit 2, no write). A second link on dup-a.txt with expect_dangling was rejected by plan check (`link_target_exists`, exit 2, no write). The same link without expect_dangling: plan check ok, preview lists revoke_link as irreversible, `apply --yes` revoke ok=1; get_shared_link_metadata afterwards returns shared_link_access_denied (revoked). The runtime guard inside apply (target reappeared between plan and apply) stays covered only by unit tests.
   - Final `links audit`: 0 links remain under the sandbox folder.
10. Endpoint mirrors that fail in the live matrix only for lack of real fixtures, run here with real sandbox inputs, all exit 0: `files list-revisions`, `files get-temporary-link`, `files tags-get`, `files delete-batch-check` (real async job id from step 6), `files list-folder-continue` (real cursor from `files get-latest-cursor`).
11. Cleanup: `files delete --path <sandbox>` exit 0; get-metadata --include-deleted returns `deleted` (soft delete, restorable for 30 days).

## Bugs found and fixed during this run
- apply/undo pre-write refresh crawled every top-level folder when the index was built with `index --root`. The first `apply --yes` attempt started a full-account metadata crawl into the test DB; it was stopped before any write (no journal batch, all 5 files intact) and the test DB was discarded. Fix (commit 9c53c2c): when the account root is not tracked, refresh only the roots recorded in dbx_index_state. Test `TestApplyRefreshStaysInsideRootOnlyIndex` fails on the old code. Rerun: refresh touched only the sandbox root (incremental, 1 page).
- `organize --agent` dropped `from`/`to` from move ops in `ops_preview` when the preview mixed mkdir and move ops (compact key-frequency rule). Fix (commit cbd543d): organize prints through the keep-floor helper with plan op fields. Test `TestOrganizeAgentPreviewKeepsMoveEndpoints` fails on the old code.
- A stray organize plan (`photos.json`, 2029 ops over real account paths, written into the CLI directory by the live matrix's organize example) had been committed to the CLI repo. Removed from the tree and gitignored (commit 9c53c2c). It remains in older local history only.

## Batches (test DB journal)
| batch | source | status | counts |
|---|---|---|---|
| 20261007-191611-631c | dupes | undone | ok 2 |
| 20261007-191626-848c | undo | complete | ok 2 |
| 20261007-191914-0f21 | organize | undone | ok 3 |
| 20261007-191924-57fd | undo | complete | ok 1, skipped 2 (created folders left) |
| 20261007-192126-76a8 | sandbox-test (delete linked file) | complete | ok 1 |
| 20261007-192500-064f | sandbox-test (revoke link) | complete | ok 1 |

## Step log
Attempt 1 (a1) ran before the refresh fix; its test DB was discarded. Attempt 2 (a2) reused the uploaded fixtures with a fresh test DB.

| step | exit | command |
|---|---|---|
| a1/00-doctor | 0 | `dropbox-pp-cli doctor --agent` |
| a1/01-precheck | 5 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a1/02-mkdir | 0 | `dropbox-pp-cli files create-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a1/03-upload | 0 | `dropbox-pp-cli files upload dup-a.txt /pp-sandbox-20261006-161517-620ff191/dup-a.txt --agent` |
| a1/04-upload | 0 | `dropbox-pp-cli files upload dup-b.txt /pp-sandbox-20261006-161517-620ff191/dup-b.txt --agent` |
| a1/05-upload | 0 | `dropbox-pp-cli files upload notes.txt /pp-sandbox-20261006-161517-620ff191/notes.txt --agent` |
| a1/06-upload | 0 | `dropbox-pp-cli files upload 'notes (Test'\''s conflicted copy 2026-10-07).txt' '/pp-sandbox-20261006-161517-620ff191/notes (Test'\''s conflicted copy 2026-10-07).txt' --agent` |
| a1/07-upload | 0 | `dropbox-pp-cli files upload unique.txt /pp-sandbox-20261006-161517-620ff191/unique.txt --agent` |
| a1/08-index | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| a1/09-dupes | 0 | `dropbox-pp-cli dupes --under /pp-sandbox-20261006-161517-620ff191 --plan <run>/sandbox/files/dupes-plan.json --print-plan --db <run>/sandbox.db --agent` |
| a1/10-conflicts | 0 | `dropbox-pp-cli conflicts --plan <run>/sandbox/files/conflicts-plan.json --print-plan --db <run>/sandbox.db --agent` |
| a1/11-plan-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/dupes-plan.json --db <run>/sandbox.db --agent` |
| a1/12-apply-preview | 0 | `dropbox-pp-cli apply <run>/sandbox/files/dupes-plan.json --db <run>/sandbox.db --agent` |
| a1/13-apply-yes | 143 (stopped by operator during pre-write refresh; no writes) | `dropbox-pp-cli apply <run>/sandbox/files/dupes-plan.json --yes --db <run>/sandbox.db --agent` |
| a1/14-list-after-apply | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent --select entries.name` |
| a1/14-list-after-kill | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a2/08-index | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| a2/09-dupes | 0 | `dropbox-pp-cli dupes --under /pp-sandbox-20261006-161517-620ff191 --plan <run>/sandbox/files/dupes-plan.json --db <run>/sandbox.db --agent` |
| a2/10-conflicts | 0 | `dropbox-pp-cli conflicts --plan <run>/sandbox/files/conflicts-plan.json --db <run>/sandbox.db --agent` |
| a2/11-plan-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/dupes-plan.json --db <run>/sandbox.db --agent` |
| a2/12-apply-preview | 0 | `dropbox-pp-cli apply <run>/sandbox/files/dupes-plan.json --db <run>/sandbox.db --agent` |
| a2/13-apply-yes | 0 | `dropbox-pp-cli apply <run>/sandbox/files/dupes-plan.json --yes --db <run>/sandbox.db --agent` |
| a2/14-list-after-apply | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a2/15-meta-deleted | 0 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191/dup-b.txt --include-deleted --agent` |
| a2/16-journal | 0 | `dropbox-pp-cli journal --db <run>/sandbox.db --agent` |
| a2/17-journal-ops | 0 | `dropbox-pp-cli journal 20261007-191611-631c --ops --db <run>/sandbox.db --agent` |
| a2/18-undo-preview | 0 | `dropbox-pp-cli undo 20261007-191611-631c --db <run>/sandbox.db --agent` |
| a2/19-undo-yes | 0 | `dropbox-pp-cli undo 20261007-191611-631c --yes --db <run>/sandbox.db --agent` |
| a2/20-list-after-undo | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a2/21-index-refresh | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| a2/22-organize | 0 | `dropbox-pp-cli organize --match 'unique*.txt' --under /pp-sandbox-20261006-161517-620ff191 --to '/pp-sandbox-20261006-161517-620ff191/{year}/{month}' --tz UTC --plan <run>/sandbox/files/organize-plan.json --db <run>/sandbox.db --agent` |
| a2/23-organize-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/organize-plan.json --db <run>/sandbox.db --agent` |
| a2/24-organize-preview | 0 | `dropbox-pp-cli apply <run>/sandbox/files/organize-plan.json --db <run>/sandbox.db --agent` |
| a2/22b-organize-json | 0 | `dropbox-pp-cli organize --match 'unique*.txt' --under /pp-sandbox-20261006-161517-620ff191 --to '/pp-sandbox-20261006-161517-620ff191/{year}/{month}' --tz UTC --db <run>/sandbox.db --json` |
| a2/22c-organize-agent-fixed | 0 | `dropbox-pp-cli organize --match 'unique*.txt' --under /pp-sandbox-20261006-161517-620ff191 --to '/pp-sandbox-20261006-161517-620ff191/{year}/{month}' --tz UTC --db <run>/sandbox.db --agent` |
| a2/25-organize-apply | 0 | `dropbox-pp-cli apply <run>/sandbox/files/organize-plan.json --yes --db <run>/sandbox.db --agent` |
| a2/26-meta-moved | 0 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191/2026/10/unique.txt --agent` |
| a2/27-meta-old | 5 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191/unique.txt --agent` |
| a2/28-organize-undo | 0 | `dropbox-pp-cli undo 20261007-191914-0f21 --yes --db <run>/sandbox.db --agent` |
| a2/29-list-after-organize-undo | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a2/30-create-link | 0 | `dropbox-pp-cli sharing create-shared-link --path /pp-sandbox-20261006-161517-620ff191/unique.txt --agent` |
| a2/31-index-refresh | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| a2/32-delete-linked-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/delete-linked-plan.json --db <run>/sandbox.db --agent` |
| a2/33-links-audit-pre | 0 | `dropbox-pp-cli links audit --db <run>/sandbox.db --agent` |
| a2/34-delete-linked-check2 | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/delete-linked-plan.json --db <run>/sandbox.db --agent` |
| a2/35-delete-linked-apply | 0 | `dropbox-pp-cli apply <run>/sandbox/files/delete-linked-plan.json --yes --db <run>/sandbox.db --agent` |
| a2/36-meta-linked-deleted | 0 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191/unique.txt --include-deleted --agent` |
| a2/37-index-refresh | 0 | `dropbox-pp-cli index --root /pp-sandbox-20261006-161517-620ff191 --db <run>/sandbox.db --agent` |
| a2/38-links-audit-plan | 0 | `dropbox-pp-cli links audit --revoke dangling --plan <run>/sandbox/files/links-dangling-plan.json --print-plan --db <run>/sandbox.db --agent` |
| a2/39-links-plan-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/links-dangling-plan.json --db <run>/sandbox.db --agent` |
| a2/40-links-apply | 0 | `dropbox-pp-cli apply <run>/sandbox/files/links-dangling-plan.json --yes --db <run>/sandbox.db --agent` |
| a2/41-old-link-meta | 5 | `dropbox-pp-cli sharing get-shared-link-metadata --url '<sandbox-link-url> --agent` |
| a2/42-b0-check | 2 | `dropbox-pp-cli plan check <run>/sandbox/files/revoke-b0.json --db <run>/sandbox.db --agent` |
| a2/43-b0-apply | 2 | `dropbox-pp-cli apply <run>/sandbox/files/revoke-b0.json --yes --db <run>/sandbox.db --agent` |
| a2/44-create-link2 | 0 | `dropbox-pp-cli sharing create-shared-link --path /pp-sandbox-20261006-161517-620ff191/dup-a.txt --agent` |
| a2/45-links-audit-index | 0 | `dropbox-pp-cli links audit --db <run>/sandbox.db --agent` |
| a2/46-b1-check | 2 | `dropbox-pp-cli plan check <run>/sandbox/files/revoke-b1.json --db <run>/sandbox.db --agent` |
| a2/47-b2-check | 0 | `dropbox-pp-cli plan check <run>/sandbox/files/revoke-b2.json --db <run>/sandbox.db --agent` |
| a2/48-b2-preview | 0 | `dropbox-pp-cli apply <run>/sandbox/files/revoke-b2.json --db <run>/sandbox.db --agent` |
| a2/49-b2-apply | 0 | `dropbox-pp-cli apply <run>/sandbox/files/revoke-b2.json --yes --db <run>/sandbox.db --agent` |
| a2/50-link2-meta | 5 | `dropbox-pp-cli sharing get-shared-link-metadata --url '<sandbox-link-url> --agent` |
| a2/51-mirror-list-revisions | 0 | `dropbox-pp-cli files list-revisions --path /pp-sandbox-20261006-161517-620ff191/notes.txt --mode path --agent` |
| a2/52-mirror-temp-link | 0 | `dropbox-pp-cli files get-temporary-link --path /pp-sandbox-20261006-161517-620ff191/notes.txt --agent` |
| a2/53-mirror-tags-get | 0 | `dropbox-pp-cli files tags-get --paths '["/pp-sandbox-20261006-161517-620ff191/notes.txt"]' --agent` |
| a2/54-mirror-delete-batch-check | 0 | `dropbox-pp-cli files delete-batch-check --async-job-id <real-async-job-id> --agent` |
| a2/55-mirror-list-folder | 0 | `dropbox-pp-cli files list-folder --path /pp-sandbox-20261006-161517-620ff191 --limit 1 --agent` |
| a2/56-list-folder-get-latest-cursor | 2 | `dropbox-pp-cli files list-folder-get-latest-cursor --path /pp-sandbox-20261006-161517-620ff191 --json` |
| a2/56-get-latest-cursor | 0 | `dropbox-pp-cli files get-latest-cursor --path /pp-sandbox-20261006-161517-620ff191 --json` |
| a2/57-mirror-list-folder-continue | 0 | `dropbox-pp-cli files list-folder-continue --cursor <real-cursor> --agent` |
| a2/58-links-audit-final | 0 | `dropbox-pp-cli links audit --db <run>/sandbox.db --agent` |
| a2/59-journal-final | 0 | `dropbox-pp-cli journal --db <run>/sandbox.db --agent` |
| a2/60-delete-sandbox | 0 | `dropbox-pp-cli files delete --path /pp-sandbox-20261006-161517-620ff191 --agent` |
| a2/61-sandbox-gone | 0 | `dropbox-pp-cli files get-metadata --path /pp-sandbox-20261006-161517-620ff191 --include-deleted --agent` |
