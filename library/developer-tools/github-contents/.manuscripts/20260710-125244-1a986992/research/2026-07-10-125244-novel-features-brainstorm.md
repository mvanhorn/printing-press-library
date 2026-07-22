# Novel-features brainstorm (github-contents, first print) — full subagent output

## Customer model

**Maya — the dataset/collection curator.** Maya maintains local mirrors of document collections that live in GitHub repos — the immediate case is `mjwoon/AI-readings/books` (122 files, ~1.97 GB, 6 subfolders, paths with spaces).

- **Today (without this CLI):** She either clones the whole repo (history she doesn't want, on a repo dominated by binary PDFs) or clicks through download-directory.github.io, which chokes on big folders and gives no receipt of what it grabbed. She keeps the repo's GitHub tree view open in a tab to eyeball "did anything new land?", and she cannot answer "which of my 122 local files differ from remote?" without re-downloading everything.
- **Weekly ritual:** Check the upstream collection for new/changed documents, pull down only what's new, keep the folder structure intact for her reading tools.
- **Frustration:** Re-downloading gigabytes to pick up three new files, because nothing compares her local copy to the remote by content.

**Tom — the scaffold/vendoring engineer.** Tom starts projects from template subdirectories and vendors specific subtrees (a `config/` dir, a `proto/` dir) into codebases without submodules.

- **Today (without this CLI):** He uses degit/tiged for repo-root templates, but they tarball whole repos, need Node, and can't address a subdirectory of an arbitrary repo at an arbitrary SHA cleanly. For subtrees he hand-rolls `gh api ... | jq | xargs curl` (the exact gap in cli/cli#4306). He can't see what a grab will cost in files/bytes before running it.
- **Weekly ritual:** Extract a template or vendored subtree at a pinned ref into a new or existing project directory.
- **Frustration:** No dry-run — he finds out a "template" was 400 MB of fixtures after the download starts, and glob-excluding blind means re-running until the output looks right.

**Cass — the agent/CI pipeline author.** Cass writes autonomous-agent workflows and CI jobs that pull files, trees, and release assets from GitHub programmatically.

- **Today (without this CLI):** A brittle stack of `gh api` + `jq` + `curl` with hand-rolled pagination, URL-escaping for paths with spaces, and no plan for the 1 MB contents-API cutoff. Jobs die mid-run on the 60 req/h unauthenticated limit with no warning, and nothing emits machine-readable reports her agents can act on (no incumbent does JSON at all).
- **Weekly ritual:** Scheduled/triggered fetches of repo content into build workspaces; post-fetch assertions that the right bytes arrived.
- **Frustration:** Rate-limit surprises and unverifiable downloads — she can't cheaply prove "the workspace matches ref X" without re-fetching, and can't predict whether a fetch will blow the remaining API budget.

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona | Source | Inline kill/keep | Long Description |
|---|-----------|---------|-------------|---------|--------|------------------|------------------|
| 1 | Dry-run plan | `plan owner/repo/path#ref` | List what a fetch would download: files, per-file sizes, total bytes, API-request cost vs remaining rate budget, LFS-pointer warnings | Tom, Cass | (a)+(e) — brief Workflow 2, User Vision acceptance path | KEEP — one trees call, mechanical, verifiable | Use this command to preview what 'fetch' would download (files, sizes, total, API cost) without writing anything. Do NOT use it to compare an already-downloaded local directory against the remote; use 'verify' instead. |
| 2 | Local integrity verify | `verify <localdir> owner/repo/path#ref` | Compare a local dir against remote by computing git blob SHA (`sha1("blob <len>\0"+bytes)`) locally — match/changed/missing/extra, zero re-download | Maya, Cass | (b)+(f) — blob-SHA content pattern; github-mcp-server has no integrity surface | KEEP — pure local hash + one tree call | Use this command to check whether a previously downloaded directory matches the remote at a ref. Do NOT use it to download the differences; use 'sync-dir' instead. |
| 3 | Incremental re-sync | `sync-dir <localdir> owner/repo/path#ref` | Fetch only changed/new files into an existing local dir, driven by verify's blob-SHA diff | Maya | (a) — Maya's frustration verbatim; Workflow 3 | KEEP | Use this command to update an existing downloaded directory in place, fetching only changed or new files. Do NOT use it for a first-time download into an empty directory; use 'fetch' instead. |
| 4 | Path size breakdown | `stats owner/repo/path#ref` | Size/count breakdown by subfolder and extension, plus top-N largest files, from a single recursive tree call | Maya, Tom | (b) — trees API gives whole listing in 1 request; Workflow 5 | KEEP | Use this command for size and file-type breakdowns of a remote repo path. Do NOT use it to preview a specific download's file list; use 'plan' instead. |
| 5 | Offline tree search | `search "<pattern>" --limit 20` | Query previously fetched tree listings in the local store without network | Maya, Cass | (c) — Data Layer: store backs offline search over fetched listings; Workflow 5 | KEEP — spec-emits, generated framework command | none |
| 6 | What's-new precheck | `whats-new <localdir>` | Join local fetch-manifest timestamp with `repos commits --path` to report upstream changes since last fetch | Maya | (c) | SOFT — one commits call + manifest read; overlaps sync-dir's job | none |
| 7 | Ref-to-ref tree diff | `diff-refs owner/repo/path ref1 ref2` | Diff two refs' trees for a path: added/removed/changed files | Tom | (b) — two tree calls, local set-diff | SOFT — mechanical and cheap, but cadence questionable | none |
| 8 | Rate budget estimator | `rate-budget owner/repo/path` | Predict API-request cost of a fetch vs remaining `rate_limit` quota | Cass | (b)+(c) — raw CDN downloads are free; only tree/listing calls burn quota | REFRAME — belongs inside `plan` output, not standalone | none |
| 9 | LFS pointer scan | `lfs-scan owner/repo/path` | Detect LFS pointer files in a tree before download (brief gotcha: v1 warns) | Maya | (b) | REFRAME — a warning column in `plan`, not a command | none |
| 10 | Biggest files | `biggest owner/repo --top 20` | Top-N largest files in a repo from one tree call | Maya | (b) | KILL-lean — subset of `stats` | none |
| 11 | SHA lockfile | `lock owner/repo/path` | Emit a SHA-pinned manifest so later fetches are byte-reproducible | Tom | (a) | SOFT — `#ref`=SHA addressing (manifest row 3) plus fetch's manifest record already pin; speculative extra | none |
| 12 | Fetch history | `fetches list` | List past downloads from local manifest records (what/when/where/ref) | Cass | (c) | SOFT — occasional bookkeeping, not a ritual | none |
| 13 | Tree snapshot save | `tree save owner/repo#ref` | Explicitly snapshot a recursive listing into the local store for later offline queries | Cass | (c) | KILL-lean — `plan`/`fetch` already write listings via the generated sync hint helpers | none |
| 14 | Watch/mirror daemon | `mirror --watch owner/repo/path` | Poll upstream and auto-sync a local mirror continuously | Maya | (a) | KILL — scope creep (persistent process); descoped version IS `sync-dir` on cron | none |

Kill/keep check notes: no candidate has LLM dependency; no external services beyond api.github.com/raw CDN (in spec); all read-only under existing optional-token auth; #14 fails scope creep; none reimplement API responses — `verify`/`sync-dir` compute over local bytes + real tree calls (local-data commands per rubric); all dogfood-verifiable against a public repo.

## Survivors and kills

**Pass 3 force-answers**

1. **plan** — Weekly: yes; Tom runs it before every extract, Cass embeds it in pipelines as a pre-flight (User Vision acceptance flow starts here). Wrapper: no — joins trees API + rate_limit + LFS-pointer detection into one decision surface no single endpoint provides. Transcendence: service-specific content pattern (trees = whole listing in 1 request; raw CDN = free bytes, so the plan can promise "1 API call, N free downloads") + agent-shaped JSON. Sibling kill: rate-budget (#8) — merged in as `api_cost`/`remaining_quota` fields; lfs-scan (#9) merged as a warning column. Buildability: hand-code (`// pp:data-source live`). Long-desc: references `verify` — survives.
2. **verify** — Weekly: yes; Maya's precheck before every re-sync, Cass's post-fetch assertion. Wrapper: no endpoint does this — the API never hashes your local files. Transcendence: service-specific content pattern — git blob SHA is recomputable locally from bytes, so integrity costs one tree call and zero downloads. Sibling kill: fetches-list (#12) — verify consumes manifest records internally; standalone history browsing is occasional. Buildability: hand-code (`// pp:data-source live` — one tree call + local hashing). Long-desc: references `sync-dir` — survives.
3. **sync-dir** — Weekly: yes; it IS Maya's weekly ritual (Workflow 3 verbatim). Wrapper: no — composes verify's diff with selective raw-CDN streaming. Transcendence: cross-source join (local blob hashes × remote tree) + free-bytes pattern. Sibling kill: whats-new (#6) — sync-dir's dry output already reports what changed, and diff-refs (#7) answers the between-refs variant nobody runs weekly. Buildability: hand-code (`// pp:data-source live`). Long-desc: references `fetch` — ships per absorb manifest row 2.
4. **stats** — Weekly: borderline-yes for Maya (collection growth check) and Tom (is this template bloated?); Build Priority 5 makes it deliberate scope. Wrapper: no — trees API returns a flat list; grouping by folder/extension and top-N is local computation the API can't do. Transcendence: service-specific pattern (1 recursive tree call) + local aggregation. Sibling kill: biggest (#10) — strict subset, now `stats` top-N output. Buildability: hand-code (`// pp:data-source live`). Long-desc: references `plan` — survives.
5. **search (tree listings)** — Weekly: yes for Cass's agents ("where is file X in what we already fetched" without burning quota) and Maya offline. Wrapper: no — it's local-store, zero API calls. Transcendence: local SQLite. Sibling kill: tree-save (#13) — redundant writer; `plan`/`fetch` populate the store via the generated sync hint helpers. Buildability: spec-emits (generated framework command over the store). Long-desc: none needed — offline/local intent doesn't collide with the live commands.

Dropped at Pass 3 beyond the merges: lock (#11) — `#ref`-as-SHA plus fetch's manifest record already give reproducibility; weekly use speculative.

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Dry-run download plan | `plan owner/repo/path#ref` | 10/10 (3+3+2+2) | Tom, Cass | hand-code | Uses one `GET /repos/{o}/{r}/git/trees/{sha}?recursive=1` call plus `GET /rate_limit` to compute file list, sizes, total bytes, API-request cost, and LFS-pointer warnings with no external dependencies. | Brief Workflow 2 explicit; no incumbent (degit/gitdir/DownGit) previews; cli/cli#4306 gap; 1.97 GB target makes blind download costly | Use this command to preview what 'fetch' would download (files, sizes, total, API cost) without writing anything. Do NOT use it to compare an already-downloaded local directory against the remote; use 'verify' instead. |
| 2 | Blob-SHA integrity verify | `verify <dir> owner/repo/path#ref` | 10/10 (3+3+2+2) | Maya, Cass | hand-code | Uses one recursive trees call plus locally computed `sha1("blob <len>\0"+bytes)` per file to report match/changed/missing/extra with zero re-downloads. | Brief Workflow 3 explicit; Build Priority 3; github-mcp-server (Codebase Intelligence) exposes reads but no integrity surface | Use this command to check whether a previously downloaded directory matches the remote at a ref. Do NOT use it to download the differences; use 'sync-dir' instead. |
| 3 | Incremental re-sync | `sync-dir <dir> owner/repo/path#ref` | 9/10 (3+3+2+1) | Maya | hand-code | Uses verify's blob-SHA diff (one trees call + local hashing) to stream only changed/new files from download_url, which bypasses the API rate limit. | Brief Workflow 3 explicit; Maya's frustration (re-downloading GBs for 3 new files); no incumbent attempts re-sync | Use this command to update an existing downloaded directory in place, fetching only changed or new files. Do NOT use it for a first-time download into an empty directory; use 'fetch' instead. |
| 4 | Path size breakdown | `stats owner/repo/path#ref` | 7/10 (2+2+2+1) | Maya, Tom | hand-code | Uses one recursive trees call and aggregates locally by subfolder and extension plus top-N largest files, with no external dependencies. | Brief Workflow 5 + Build Priority 5; trees-API single-request pattern from Known Gotchas | Use this command for size and file-type breakdowns of a remote repo path. Do NOT use it to preview a specific download's file list; use 'plan' instead. |
| 5 | Offline tree search | `search "<pattern>" --limit 20` | 7/10 (2+2+2+1) | Cass, Maya | spec-emits | Uses the generated local store (tree listings written by plan/fetch via sync hint helpers) to answer path/pattern queries with zero API calls. | Brief Workflow 5 ("inspect repo trees offline after a sync"); Data Layer section names the offline search backing | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| whats-new precheck (#6) | sync-dir's diff output already reports upstream changes; a standalone commits-join adds a second command for the same question | sync-dir |
| diff-refs (#7) | Between-refs tree diff is a monthly-at-best curiosity for these personas — soft-kill on weekly-use | sync-dir |
| rate-budget (#8) | Not a command — merged into plan's output as api_cost/remaining_quota fields | plan |
| lfs-scan (#9) | Not a command — merged into plan as an LFS-pointer warning column (brief mandates warn-in-v1) | plan |
| biggest files (#10) | Strict subset of stats' top-N largest output | stats |
| SHA lockfile (#11) | `#ref`-as-SHA addressing plus fetch's manifest record already pin content; standalone lockfile is speculative | verify |
| fetch history (#12) | Manifest browsing is occasional bookkeeping, not a weekly ritual; verify/sync-dir consume manifests internally | verify |
| tree snapshot save (#13) | Redundant store writer — plan/fetch already persist listings via the generated sync hint helpers | search |
| watch/mirror daemon (#14) | Scope creep: persistent background process; the descoped one-shot version is sync-dir on a scheduler | sync-dir |
