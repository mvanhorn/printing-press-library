<!-- slop-gate: off -->
# Novel-features subagent output (verbatim sections)

## Customer model

**Persona 1: Dana, the ten-year personal account holder**
- Today: Has used Dropbox since ~2014 across 3-4 laptops and a phone. Web UI open, scrolling a root full of loose PDFs, "Copy of" files, and dozens of "(Dana's conflicted copy 2017-03-02)" files. Dropbox Find duplicates is web-only, no API. Cannot answer "where is my 1.8 TB going?" or "how many of these are byte-identical?" without clicking into every folder. Official Dropbox MCP lists 100 entries per call and can't see the whole tree.
- Weekly ritual: Intense first-week cleanup, then roughly weekly the agent tidies whatever landed loose (Downloads, root, new conflicted copies) and files it into the structure.
- Frustration: Can't trust any bulk change. Web UI caps bulk delete at 1000 selections; an agent has no undo once it starts moving thousands of files.

**Persona 2: Ravi, the freelance consultant with per-client shared folders**
- Today: Seven years of per-client shared folders, some with ex-clients still members. Reorganizes by dragging in the web UI and sometimes learns afterward that a move broke a share or exposed a file to the wrong client ("Organizing shared folders" thread). Can't list which folders are shared and with whom without opening each share dialog.
- Weekly ritual: End of each engagement week: file deliverables into client folders, archive finished projects, clean up working copies.
- Frustration: Any move might cross a share boundary and change who can see a file, with no warning.

**Persona 3: Lena, the phone-dump photographer**
- Today: Camera Uploads holds ~40k photos in one flat folder. Forum answers are desktop drag jobs that stall or hit too_many_write_operations. Has re-uploaded the same shoot from two devices.
- Weekly ritual: After every shoot or week, move new uploads into dated folders and drop duplicate imports.
- Frustration: No way to move a month of photos into dated folders in one safe, repeatable step.

**Persona 4: Sam, the privacy-minded link auditor**
- Today: Years of "anyone with the link" shares. No bulk revoke in the UI ("Delete shared links in bulk" idea). Has pasted browser-console JS to revoke links in a loop.
- Weekly ritual: After sending files out and in a quarterly sweep, check which links are public, have no expiry, or point at things that no longer exist; revoke the ones that should be gone.
- Frustration: No single view of every public, non-expiring link, and no bulk revoke.

## Candidates (pre-cut)

| # | Name | Command | Persona | Source | Notes |
|---|------|---------|---------|--------|-------|
| C1 | Account overview | overview | Dana | (e),(a) | Long desc redirects to tree |
| C2 | Conflicted-copy cleanup | conflicts [--plan] | Dana | (a),(b),(e) | Long desc redirects to dupes |
| C3 | Structure lint | mess [--plan] | Dana, Ravi | (e),(b) | Long desc redirects to conflicts, organize |
| C4 | Cold-file report | cold --older-than 5y | Dana, Ravi | (e) | |
| C5 | Shared-link audit + bulk revoke | links audit [--plan] | Sam, Ravi | (a),(e),(c) | Long desc redirects to sharing list/revoke |
| C6 | Restore-window trash view | trash | Dana | (e) | flagged: needs per-file list_revisions for non-CLI deletes |
| C7 | Rule-based organizer | organize --match --under --to | Lena, Dana, Ravi | (a),(e) | |
| C8 | Plan validator | plan check | All | (e),(f) | |
| C9 | Batched, journaled apply | apply [--yes] | All | (e),(f) | |
| C10 | Undo a batch | undo | All | (e) | |
| C11 | Change journal | journal | All | (e) | |
| C12 | Camera Uploads sorter | photos sort | Lena | (a) | |
| C13 | Duplicate folder trees | dupe-folders | Dana | (c) | |
| C14 | Share-boundary exposure | exposure | Ravi, Sam | (c),(a) | |
| C15 | Changes since last index | changes --since 7d | Dana, Lena | (c) | |
| C16 | Local-vs-cloud hash check | has <local-dir> | Lena | (b) | flagged: no evidence, outside thesis |
| C17 | Revision pruning | revisions prune | Dana | (b) | cut: requires permanently_delete |
| C18 | Auto-categorize by content | categorize | Dana | (a) | cut: LLM dependency |

Inline checks: C1-C11 read the local store populated by real list_folder/list_shared_links crawls or call real batch/restore endpoints. Store-read commands call hintIfUnsynced/hintIfStale and use drain-first rows. apply never calls store.Upsert inside an open write tx.

## Survivors and kills

### Survivors

9 survivors (one over target): the plan/check/apply/undo/journal safety set is 4 commands and the user conditioned soft-delete approval on reversibility.

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description | Persona |
|---|---------|---------|-------|--------------|--------------|----------|------------------|---------|
| 1 | Account overview | overview | 9/10 | hand-code | pp:data-source auto: one live users/get_space_usage + GROUP BY over local files (top-level folder, extension, client_modified year) + headline dupe and conflicted-copy counts | Brief workflow 1; official MCP only has GetUsageAndQuota; rclone ncdu per-folder only | Use this command for a one-screen account summary across the whole Dropbox. Do NOT use it to inspect one folder's structure; use 'tree' instead. | Dana |
| 2 | Conflicted-copy cleanup | conflicts [--plan <file>] | 10/10 | hand-code | pp:data-source local: regex on name for "(… conflicted copy YYYY-MM-DD)", strip suffix to find original in same parent_lower, compare content_hash; identical -> delete ops with rev; different -> report only | Brief workflow 3; thousands-of-conflicted-copies threads; 1000-selection UI cap | Use this command for Dropbox "conflicted copy" files and their originals. Do NOT use it for general byte-identical duplicates; use 'dupes' instead. | Dana |
| 3 | Structure lint | mess [--plan <file>] | 8/10 | hand-code | pp:data-source local: empty folders, single-child folders, root-level files, junk-name patterns, depth, normalized sibling-name collisions; --plan emits soft deletes for empty folders only | Brief users; plan section 6 | Use this command for structural problems: empty or near-duplicate folders, loose root files, junk names. Do NOT use it for conflicted copies; use 'conflicts' instead. Do NOT use it for rule-based filing; use 'organize' instead. | Dana, Ravi |
| 4 | Shared-link audit and bulk revoke | links audit [--plan <file>] | 10/10 | hand-code | pp:data-source auto: refresh shared_links via sharing/list_shared_links, join to files for dangling, report visibility/expiry/age, plus list_folders + list_folder_members; --plan emits revoke ops for apply | Brief workflow 5; bulk-revoke forum idea; console-JS workaround | Use this command to audit link exposure across the account and plan bulk revokes. Do NOT use it to list or revoke a single link; use 'sharing list-shared-links' or 'sharing revoke-shared-link' instead. | Sam, Ravi |
| 5 | Rule-based organizer | organize --match <glob> --under <path> --to <template> | 10/10 | hand-code | pp:data-source local: glob under path, expand {year}/{month}/{ext} from client_modified, emit move ops with rev + mkdir ops, flag NOCASE collisions | Brief workflow 4; Photographers board; staff advice to use move_batch_v2 | Use this command to build a move plan from a glob and a destination template. Do NOT use it to find problems; use 'mess' or 'conflicts' instead. | Lena, Dana, Ravi |
| 6 | Plan validator | plan check <file> | 8/10 | hand-code | pp:data-source local: source exists at rev, destination free (NOCASE), case-only rename, shared_folder_id differs (cross-share), missing parents, op cap; pp:typed-exit-codes 0,2 | Plan section 7; case-insensitive paths; shared-folders thread | Use this command to validate a plan and get a pass/fail verdict with typed exit codes. Do NOT use it to preview execution batches or execute; use 'apply' instead. | All |
| 7 | Batched, journaled apply | apply <file> [--yes] | 9/10 | hand-code | pp:data-source live: dry run prints check verdict + chunking; --yes refreshes index incrementally, refuses rev mismatch, runs create_folder_batch/move_batch_v2/delete_batch/revoke_shared_link in <=1000 chunks, polls .tag check routes, 429 Retry-After, per-namespace serialization, --allow-cross-share, journals, updates files | Batch routes + too_many_write_operations; 1000-selection cap; rclone batch modes | Use this command to preview or execute a plan file. Do NOT use it to reverse a past change; use 'undo' instead. | All |
| 8 | Undo a batch | undo <batch-id> [--yes] | 7/10 | hand-code | pp:data-source live: invert journal_ops (moves back via move_batch_v2, restores via files/restore by rev); account_type drives 30 vs 180-day warning; journals the undo as its own batch | User vision: soft deletes conditioned on undo; no Dropbox batch undo | Use this command to reverse one applied batch. Do NOT use it to list batches; use 'journal' instead. | All |
| 9 | Change journal | journal | 7/10 | hand-code | pp:data-source local: list journal_batches with ok/failed/skipped counts, async job ids, restore-window days remaining for deletes | User vision; plan section 9 | Use this command to list applied batches and their outcomes. Do NOT use it to reverse a batch; use 'undo' instead. | All |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C4 cold | Fails weekly use; overview's by-year breakdown shows old data; archive moves are an organize plan | overview |
| C6 trash | Fails weekly use; non-CLI deletes need per-file list_revisions; CLI-delete windows shown in journal | journal |
| C12 photos sort | Thin preset of organize --match '*' --under '/Camera Uploads' --to '/Photos/{year}/{month}' | organize |
| C13 dupe-folders | Grouping mode of absorbed dupes; near-duplicate folder names flagged by mess | conflicts |
| C14 exposure | Duplicates links audit; plan check covers move safety | links audit |
| C15 changes | No research evidence; re-running mess after index answers it | mess |
| C16 has | No evidence; reads local filesystem, outside thesis | organize |
| C17 revisions prune | Requires permanently_delete, which the user excluded | apply |
| C18 categorize | LLM dependency; the agent categorizes and writes a plan | organize |
