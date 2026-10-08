<!-- slop-gate: off -->
# Dropbox CLI Absorb Manifest

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List folder | dbxcli ls; Dropbox MCP ListFolder (100/call) | (generated endpoint) files list-folder | 2000/page, --json/--select, recursive flag |
| 2 | Continue a listing | Stone files/list_folder/continue | (generated endpoint) files list-folder-continue | cursor passthrough for agents |
| 3 | File/folder metadata | dbxcli; Dropbox MCP GetFileMetadata | (generated endpoint) files get-metadata | include_media_info flag |
| 4 | Server-side search | dbxcli search; Dropbox MCP Search | (generated endpoint) files search | filename_only, extensions, categories, deleted filters |
| 5 | Offline path search | none | (behavior in dropbox-pp-cli search) FTS5 over indexed names and paths | instant, works offline, SQL composable |
| 6 | Download a file | dbxcli get; Dropbox MCP GetFileContent (5MB cap) | dropbox-pp-cli files download | no size cap, streams to --output, header-arg escaping for non-ASCII names |
| 7 | Upload a file | dbxcli put; Dropbox-Uploader upload | dropbox-pp-cli files upload | --mode add/overwrite, --autorename, refuses >150MB with clear error |
| 8 | Move | dbxcli mv; Dropbox MCP Move | (generated endpoint) files move | --dry-run, autorename |
| 9 | Copy | dbxcli cp; Dropbox MCP Copy | (generated endpoint) files copy | --dry-run |
| 10 | Delete (soft, restorable) | dbxcli rm; Dropbox MCP Delete | (generated endpoint) files delete | --dry-run; never permanent |
| 11 | Create folder | dbxcli mkdir; Dropbox MCP CreateFolder | (generated endpoint) files create-folder | --dry-run |
| 12 | List revisions | dbxcli revs; Dropbox MCP ListFileRevisions | (generated endpoint) files list-revisions | --json |
| 13 | Restore a revision | dbxcli restore; Dropbox MCP RestoreFileRevision | (generated endpoint) files restore | --dry-run |
| 14 | Temporary download link | Stone files/get_temporary_link | (generated endpoint) files get-temporary-link | agent-shareable 4h link |
| 15 | Thumbnails (batch of 25) | Stone files/get_thumbnail_batch | (generated endpoint) files get-thumbnail-batch | photo triage without downloads |
| 16 | File tags get/add/remove | Stone files/tags/* (preview API) | (generated endpoint) files tags-get / files tags-add / files tags-remove | reversible labeling |
| 17 | Batch move/copy/delete + job check | rclone batch modes; Dropbox staff advice on too_many_write_operations | (generated endpoint) files move-batch / files copy-batch / files delete-batch and their check endpoints | raw access for agents; apply wraps these safely |
| 18 | Create shared link | dbxcli share; Dropbox MCP CreateSharedLink | (generated endpoint) sharing create-shared-link | visibility/expiry settings |
| 19 | List shared links | dbxcli; ngs/dropbox-mcp-server | (generated endpoint) sharing list-shared-links | cursor, direct_only |
| 20 | Revoke shared link | dbxcli; ngs/dropbox-mcp-server | (generated endpoint) sharing revoke-shared-link | --dry-run |
| 21 | Modify shared link settings | dbxcli update | (generated endpoint) sharing modify-shared-link-settings | set expiry/visibility |
| 22 | Shared folders and members | Stone sharing/list_folders, list_folder_members | (generated endpoint) sharing list-folders / sharing list-folder-members | who has access |
| 23 | Space usage | Dropbox-Uploader space; Dropbox MCP GetUsageAndQuota | (generated endpoint) users get-space-usage | --json |
| 24 | Account info | Dropbox-Uploader info | (generated endpoint) users get-current-account | account_type, root namespace |
| 25 | File requests list + cleanup | Stone file_requests | (generated endpoint) file-requests list / file-requests delete-all-closed | one-shot cleanup |
| 26 | Content-hash dedupe | rclone dedupe --by-hash | dropbox-pp-cli dupes | from local index, zero downloads, keeper rules, emits a plan instead of deleting |
| 27 | Size/usage tree | rclone ncdu / size | dropbox-pp-cli tree | offline, per-folder counts+bytes, depth-limited agent output |
| 28 | Incremental change tracking | Dropbox-Uploader monitor; Stone list_folder/continue+longpoll | (behavior in dropbox-pp-cli index) cursor-based incremental crawl | resumable, per-root checkpoints |
| 29 | OAuth with refresh token | dbxcli auth | (behavior in dropbox-pp-cli auth login) PKCE + token_access_type=offline | no client secret, auto refresh |
| 30 | Agent JSON output | dbxcli JSON envelopes | (behavior in dropbox-pp-cli files list-folder) global --json/--agent/--select on every command | consistent across all commands |

Absorbed rows requiring hand-code after generate: `files download`, `files upload` (Dropbox-API-Arg header transport), `dupes`, `tree`, and `index` (POST-body cursor sync on a sibling continue route). Endpoint names for `(generated endpoint)` rows are fixed by the internal YAML spec authored in Phase 2.

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Conflicted-copy cleanup | conflicts [--plan <file>] | 10/10 | hand-code | Regex on indexed names for "(… conflicted copy YYYY-MM-DD)", pair with original in same parent, compare content_hash; identical pairs become soft-delete plan ops | Thousands-of-conflicted-copies forum threads; 1000-selection UI cap | Use this command for Dropbox "conflicted copy" files and their originals. Do NOT use it for general byte-identical duplicates; use 'dupes' instead. |
| 2 | Shared-link audit and bulk revoke | links audit [--plan <file>] | 10/10 | hand-code | sharing/list_shared_links into shared_links table joined to files for dangling links, plus list_folders/list_folder_members; --plan emits revoke ops | "Delete shared links in bulk" forum idea; console-JS workaround | Use this command to audit link exposure across the account and plan bulk revokes. Do NOT use it to list or revoke a single link; use 'sharing list-shared-links' or 'sharing revoke-shared-link' instead. |
| 3 | Rule-based organizer | organize --match <glob> --under <path> --to <template> | 10/10 | hand-code | Glob over indexed files under a path, expand {year}/{month}/{ext} from client_modified, emit move + mkdir ops with revs, flag case-insensitive collisions | Camera Uploads move threads; staff advice to use move_batch_v2 | Use this command to build a move plan from a glob and a destination template. Do NOT use it to find problems; use 'mess' or 'conflicts' instead. |
| 4 | Account overview | overview | 9/10 | hand-code | users/get_space_usage plus local aggregates by top-level folder, type, year, with dupe and conflicted-copy headline counts | Brief workflow 1; official MCP has only GetUsageAndQuota | Use this command for a one-screen account summary across the whole Dropbox. Do NOT use it to inspect one folder's structure; use 'tree' instead. |
| 5 | Batched, journaled apply | apply <file> [--yes] | 9/10 | hand-code | Dry run by default; --yes runs create_folder_batch / move_batch_v2 / delete_batch / revoke_shared_link in <=1000 chunks, polls .tag check routes, honors Retry-After, serializes per namespace, refuses rev mismatches, journals every op | too_many_write_operations threads; rclone batch modes | Use this command to preview or execute a plan file. Do NOT use it to reverse a past change; use 'undo' instead. |
| 6 | Structure lint | mess [--plan <file>] | 8/10 | hand-code | Local queries for empty and single-file folders, loose root files, junk names, deep nesting, near-duplicate sibling folder names; --plan soft-deletes empty folders only | Brief users; plan section 6 | Use this command for structural problems: empty or near-duplicate folders, loose root files, junk names. Do NOT use it for conflicted copies; use 'conflicts' instead. Do NOT use it for rule-based filing; use 'organize' instead. |
| 7 | Plan validator | plan check <file> | 8/10 | hand-code | Validates ops against the index: rev match, NOCASE destination collisions, case-only renames, cross-shared-folder moves, missing parents, op cap; typed exit codes 0,2 | Plan section 7; shared-folders forum thread | Use this command to validate a plan and get a pass/fail verdict with typed exit codes. Do NOT use it to preview execution batches or execute; use 'apply' instead. |
| 8 | Undo a batch | undo <batch-id> [--yes] | 7/10 | hand-code | Inverts journaled ops: moves back via move_batch_v2, restores soft deletes via files/restore by rev; warns past the account's restore window | User approved soft deletes conditioned on undo; no Dropbox batch undo | Use this command to reverse one applied batch. Do NOT use it to list batches; use 'journal' instead. |
| 9 | Change journal | journal | 7/10 | hand-code | Lists journal_batches with op result counts, async job ids, restore-window days remaining | User vision; plan section 9 | Use this command to list applied batches and their outcomes. Do NOT use it to reverse a batch; use 'undo' instead. |

Killed from the user's plan (overridable at the gate): `cold` (overview's by-year view covers it; archive moves are an organize plan), `trash` (needs a per-file list_revisions call for deletes made outside the CLI; CLI deletes show restore days in journal). Full kill list in the novel-features brainstorm audit trail.
