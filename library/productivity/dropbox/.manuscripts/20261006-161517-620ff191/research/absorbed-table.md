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
