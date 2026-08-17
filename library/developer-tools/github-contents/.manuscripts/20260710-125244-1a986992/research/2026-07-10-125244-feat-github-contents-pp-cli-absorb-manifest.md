# github-contents Absorb Manifest

Sources searched: degit, tiged, giget (unjs), gitdir (sdushantha), githubdl (wilvk), DownGit, download-directory.github.io, download-git-repo, github-download-directory (npm), git sparse-checkout, gh CLI, github/github-mcp-server + 4 community MCP servers. Evidence in the research brief.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | owner/repo[/path][#ref] one-arg addressing | degit / giget | (behavior in github-contents-pp-cli fetch) parses owner/repo[/path][#ref] and full github.com URLs | no URL surgery |
| 2 | Subdirectory-only recursive extraction, structure preserved | degit subdir / gitdir | github-contents-pp-cli fetch | streams via raw CDN (no API burn per file), JSON report |
| 3 | Ref = branch/tag/SHA | degit #ref | (behavior in github-contents-pp-cli fetch) --ref flag or #ref suffix | uniform across novel commands |
| 4 | Custom output dir | gitdir -d | (behavior in github-contents-pp-cli fetch) --out | |
| 5 | Force overwrite | degit --force | (behavior in github-contents-pp-cli fetch) --force | skip-existing-identical is the default |
| 6 | Flatten mode | gitdir --flatten | (behavior in github-contents-pp-cli fetch) --flatten | |
| 7 | Include/exclude globs | giget --ignore | (behavior in github-contents-pp-cli fetch) --include / --exclude | |
| 8 | Token auth via env | giget GIGET_AUTH | generated auth: GITHUB_TOKEN / GH_TOKEN | unauthenticated works for public repos |
| 9 | Directory listing | gh api contents | (generated endpoint) contents get | |
| 10 | Recursive tree listing | github-mcp-server get_repository_tree (source) | (generated endpoint) trees get --recursive | |
| 11 | Single-file download incl. >1 MB files | githubdl dl_file | (behavior in github-contents-pp-cli fetch) file target streams via download_url | contents API refuses inline >1 MB |
| 12 | List branches | githubdl | (generated endpoint) repos branches | |
| 13 | List commits scoped to a path | github-mcp-server list_commits (source) | (generated endpoint) repos commits --path | |
| 14 | Releases list / latest | gh release | (generated endpoint) releases list / releases latest | |
| 15 | Release asset download by pattern | gh release download | github-contents-pp-cli releases download | streams browser_download_url; typed errors |
| 16 | Rate-limit check | gh api /rate_limit | (generated endpoint) rate-limit show | |
| 17 | Whole-repo tarball snapshot | degit --mode=tar | github-contents-pp-cli tarball | one request, zero per-file API calls |
| 18 | JSON/agent output + field selection | (no incumbent) | (behavior in github-contents-pp-cli fetch) global --json/--select/--compact/--csv on every command | agent-native |
| 19 | Blob fetch by SHA (large-file fallback) | octokat #261 workaround | (generated endpoint) blobs get | documented 1-100 MB path |
| 20 | Repo metadata / default branch | gh repo view | (generated endpoint) repos get | novel commands auto-resolve default branch |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|---------|--------------|-------------------------|------------------|
| 1 | Dry-run download plan | plan | 10/10 | Tom, Cass | hand-code | One trees call + rate_limit joined into a decision surface (files, sizes, total, API cost, LFS warnings) no single endpoint provides; no incumbent previews | Use this command to preview what 'fetch' would download (files, sizes, total, API cost) without writing anything. Do NOT use it to compare an already-downloaded local directory against the remote; use 'verify' instead. |
| 2 | Blob-SHA integrity verify | verify | 10/10 | Maya, Cass | hand-code | Git blob SHA is recomputable locally (sha1("blob <len>\0"+bytes)) — integrity for a whole dir costs one tree call and zero downloads; the API never hashes local files | Use this command to check whether a previously downloaded directory matches the remote at a ref. Do NOT use it to download the differences; use 'sync-dir' instead. |
| 3 | Incremental re-sync | sync-dir | 9/10 | Maya | hand-code | Cross-source join: local blob hashes × remote tree → stream only changed/new files via raw CDN (rate-limit-free bytes) | Use this command to update an existing downloaded directory in place, fetching only changed or new files. Do NOT use it for a first-time download into an empty directory; use 'fetch' instead. |
| 4 | Path size breakdown | stats | 7/10 | Maya, Tom | hand-code | Trees API returns a flat list; grouping by folder/extension + top-N is local computation the API can't do | Use this command for size and file-type breakdowns of a remote repo path. Do NOT use it to preview a specific download's file list; use 'plan' instead. |
| 5 | Offline tree search | search | 7/10 | Cass, Maya | spec-emits | Local SQLite over listings written by plan/fetch — zero API calls | none |

Killed candidates + customer model: see 2026-07-10-125244-novel-features-brainstorm.md (audit trail).

No stubs. All absorbed rows and transcendence rows are shipping scope.
