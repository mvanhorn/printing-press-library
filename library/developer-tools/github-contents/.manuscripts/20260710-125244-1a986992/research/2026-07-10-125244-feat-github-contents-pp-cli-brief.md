# GitHub Contents CLI Brief

## API Identity
- Domain: GitHub REST API, scoped to repository contents — contents, git trees, git blobs, releases, repo metadata, rate limit. Deliberately NOT the full ~1000-operation GitHub API (user-approved scope).
- Users: developers and agents who need files/folders out of a GitHub repo *without* cloning — template extraction, dataset/book/document collections, vendored subtrees, CI artifacts.
- Data profile: read-only, public-first (unauthenticated works at 60 req/h; token raises to 5000 req/h). File bytes stream from raw.githubusercontent.com / download_url, which does NOT count against the API rate limit.

## Reachability Risk
- None. `GET /repos/mjwoon/AI-readings/contents/books` returned HTTP 200 unauthenticated (probed 2026-07-10). Official, documented, stable public API.
- Probe-safe endpoint used: GET /rate_limit (does not count against quota).

## Top Workflows
1. **Download a repo subdirectory recursively, preserving structure** (the headline; immediate use case: `mjwoon/AI-readings/books`, 122 files / ~1.97 GB, 6 subfolders, paths with spaces).
2. Preview what a download would fetch (file list + sizes + total) before committing bandwidth.
3. Verify/re-sync a previously downloaded folder against the remote (only fetch changed/new files).
4. Single-file grab (including >1 MB files where the contents API refuses inline content).
5. List/inspect repo trees offline after a sync (what's in this repo, biggest files, by extension).

## Table Stakes (from degit/tiged/giget/gitdir/DownGit/download-directory)
- `owner/repo[/subpath][#ref]` addressing; ref = branch, tag, or SHA
- Recursive directory download preserving structure; custom output dir; force-overwrite
- Include/exclude globs (giget `--ignore`); flatten mode (gitdir `--flatten`)
- Token auth via env (GITHUB_TOKEN / GH_TOKEN); public repos need none
- Tarball mode for whole-repo snapshots
- Non-interactive, scriptable, JSON output (none of the incumbents do JSON — we beat them there)

## Known API gotchas the CLI must handle (evidence: docs.github.com + community discussions)
- Contents API inlines base64 only ≤1 MB (105 of the 122 target files exceed it) → stream via download_url / raw media type
- Directory listing caps at 1,000 entries → prefer git trees API (1 request, recursive)
- Trees truncate at 100k entries / 7 MB (`truncated: true`) → fall back to per-subtree walk
- Paths with spaces/special chars 404 unless each segment is URL-escaped
- Symlinks: API silently resolves in-repo targets; external targets return `type: symlink`
- Submodules report `type:"file"` in listings (disambiguate via `submodule_git_url`); trees report `type:"commit"` — skip, don't fetch
- LFS pointers: contents/blob return the pointer text, not the binary — detect and warn (v1) rather than silently writing pointer files
- Private repo without token → 404 (not 403); actionable error must mention auth
- Secondary rate limit ≈100 concurrent → bounded download concurrency (default 8)
- download_url for private repos expires; re-derive rather than cache

## Data Layer
- Stateless read-through by default (cache disabled per skill guidance — no syncable account-scoped resource; repo trees are per-invocation input).
- The generated local store still backs offline `search`/`sql` over previously fetched listings; novel commands write a `fetch manifest` record per download for verify/re-sync.

## Codebase Intelligence
- Source: official github/github-mcp-server README. Auth: `GITHUB_PERSONAL_ACCESS_TOKEN` env; header `Authorization: Bearer <token>`. Tools it proves out: get_file_contents(owner,repo,path,ref), get_repository_tree(recursive, path_filter), list_branches, list_commits, list_releases, get_latest_release. Our CLI mirrors that read surface and adds the local-download writer none of the MCP servers have.

## User Vision
- "Download all books from https://github.com/mjwoon/AI-readings/tree/main/books ... maintain the folder structure." Acceptance test for this run: the generated CLI performs that download end-to-end.

## Product Thesis
- Name: github-contents (binary github-contents-pp-cli)
- Why it should exist: `gh` has no "download this subdirectory as plain files" command (cli/cli#4306 open request) — you either clone the whole repo + history or hand-roll `gh api` + jq + curl recursion. degit/tiged/giget only snapshot repo roots/templates via tarball and need Node. This is a single Go binary that addresses `owner/repo/path#ref`, plans before it downloads, streams big files without rate-limit burn, preserves structure, verifies integrity by git blob SHA, and re-syncs incrementally — agent-native (--json everywhere) which no incumbent offers.

## Build Priorities
1. `fetch` — recursive download preserving structure (trees API + streamed raw downloads, include/exclude globs, --ref, --out, --flatten, --force, bounded concurrency, skip-existing)
2. `plan` — dry-run listing with sizes/total (what fetch would do)
3. `verify` — compare a local dir against remote via git blob SHA (compute sha1("blob <len>\0"+bytes) locally, no re-download)
4. `sync-dir` — incremental re-fetch of only changed/new files (uses verify's diff)
5. `stats` — size breakdown of a repo path by folder/extension from one tree call
