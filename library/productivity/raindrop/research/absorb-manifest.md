# Raindrop.io Printing Press Absorb Manifest

## Absorbed REST and ecosystem features

| # | Feature | Best source | Our implementation | Added value |
|---|---|---|---|---|
| 1 | Token auth and identity | Official API, kyoji2 | `raindrop auth login`, `raindrop whoami`, `raindrop doctor` | Env, token-file, secure config, actionable diagnostics |
| 2 | Account statistics/context | kyoji2, adeze | `raindrop context` | Compact JSON/table/agent output |
| 3 | Bookmark search and filters | Official API/MCP | `raindrop bookmark search` | All query operators, pagination, field selection |
| 4 | Bookmark get | Official API | `raindrop bookmark get` | Typed response and stable exit codes |
| 5 | Bookmark create | Official API/MCP | `raindrop bookmark create` | Stdin, idempotency check, dry-run |
| 6 | Bookmark update | Official API/MCP | `raindrop bookmark update` | Patch flags or JSON payload |
| 7 | Bookmark delete/trash | Official API/MCP | `raindrop bookmark delete` | Confirmation and dry-run |
| 8 | URL metadata parse | Official API | `raindrop bookmark parse-url` | Agent-friendly metadata |
| 9 | URL existence check | Official API/MCP | `raindrop bookmark exists` | Duplicate-safe capture |
| 10 | Bookmark suggestions | Official API/MCP, kyoji2 | `raindrop bookmark suggest` | Tags, collections, covers |
| 11 | Bulk bookmark create | Official API/MCP | `raindrop bookmark batch-create` | Bounded chunks and receipts |
| 12 | Bulk bookmark update/move/tag | Official API/MCP, kyoji2 | `raindrop bookmark batch-update` | Safe selectors and dry-run |
| 13 | Bulk bookmark delete | Official API/MCP | `raindrop bookmark batch-delete` | Confirmation and resumable chunks |
| 14 | File upload/download/delete | Official API | `raindrop file upload|get|delete` | Streaming and file-safe output |
| 15 | Bookmark cover upload | Official API | `raindrop bookmark cover` | Multipart support |
| 16 | Permanent cached copy | Official API | `raindrop bookmark cache` | Fetch/status operations |
| 17 | Reminder management | Community complete spec | `raindrop bookmark reminder set|delete` | Exact timestamp validation |
| 18 | Collection list/tree/get | Official API/MCP | `raindrop collection list|tree|get` | Breadcrumb paths and counts |
| 19 | Collection create/update/delete | Official API/MCP | `raindrop collection create|update|delete` | Nested parents and safe deletion |
| 20 | Bulk collection delete | Official API, kyoji2 | `raindrop collection batch-delete` | Dry-run and confirmation |
| 21 | Collection reorder/expand | Official API, kyoji2 | `raindrop collection reorder|expand` | Deterministic hierarchy control |
| 22 | Collection merge/clean | Official API/MCP | `raindrop collection merge|clean` | Preview affected counts |
| 23 | Empty trash | Official API/MCP | `raindrop trash empty` | Explicit confirmation required |
| 24 | Collection cover/icon search/upload | Official API, kyoji2 | `raindrop collection cover|icons` | URL/file support |
| 25 | Collection sharing collaborators | Official API | `raindrop collection sharing list|invite|update|remove|join` | Typed roles and safe mutation |
| 26 | Tag list | Official API/MCP | `raindrop tag list` | Global or collection scope |
| 27 | Tag rename/merge/delete | Official API/MCP | `raindrop tag rename|merge|delete` | Preview and dry-run |
| 28 | Highlight list | Official API/MCP | `raindrop highlight list` | Global, collection, bookmark scope |
| 29 | Highlight create/update/delete | Official API/MCP | `raindrop highlight create|update|delete` | Color validation and safe patching |
| 30 | Filter vocabulary/statistics | Official API | `raindrop filters` | Domains, tags, types, health counts |
| 31 | Broken/duplicate/untagged audit | adeze MCP | `raindrop audit` | Full pagination and machine-readable findings |
| 32 | Import URL/file | Official API | `raindrop import parse|exists|file` | Status polling and duplicate reporting |
| 33 | Export collection | Official API | `raindrop export` | HTML, CSV and supported archive formats |
| 34 | Backup list/download/create | Official API | `raindrop backup list|get|create` | Streaming files and metadata |
| 35 | Wayback lookup | kyoji2 | `raindrop bookmark wayback` | Optional external archive check |
| 36 | Rate limiting and retries | adeze, kyoji2 | `(behavior in raindrop)` | Retry-After, jitter, bounded exponential backoff |
| 37 | JSON, table and agent output | kyoji2, Printing Press | `(behavior in raindrop)` | `--json`, `--agent`, `--select`, jq composition |
| 38 | Global dry-run | kyoji2, Printing Press | `(behavior in raindrop)` | Mutation plans without API writes |
| 39 | CLI/MCP parity | Official MCP, Printing Press | `raindrop-mcp` | Same Cobra command tree exposed through MCP |
| 40 | Shell completion | Printing Press | `raindrop completion` | Bash, zsh, fish, PowerShell |

## Transcendence features

| # | Feature | Command | Buildability | Why only this CLI can do it |
|---|---|---|---|---|
| 1 | Incremental SQLite mirror and offline FTS5 | `raindrop sync`, `raindrop sync status`, `raindrop search --offline` | hand-code | Persistent checkpoints, transactions and FTS avoid repeated full API scans |
| 2 | Resumable Unsorted inbox review | `raindrop inbox review`, `raindrop inbox apply` | hand-code | Durable sessions prevent repeat prompts and confirm each writeback |
| 3 | Historical field-level changes | `raindrop changes`, `raindrop sync diff` | hand-code | Snapshot history answers questions unavailable from current API state |
| 4 | Tag-health discovery and merge plans | `raindrop tag health`, `raindrop tag plan-merges`, `raindrop tag apply-plan` | hand-code | Local joins find normalized collisions and co-occurrence candidates safely |
| 5 | Metadata-preserving duplicate plans | `raindrop duplicates plan`, `raindrop duplicates apply` | hand-code | Canonical URL groups plus persisted plans preserve tags, notes and highlights |
| 6 | Forgotten-item resurfacing and reading queue | `raindrop revisit`, `raindrop reading queue`, `raindrop reading done` | hand-code | Local history prevents repetitive recommendations and supports bounded queues |
| 7 | Offline related bookmarks and clusters | `raindrop related`, `raindrop clusters` | hand-code | FTS5 BM25 plus tag/domain overlap yields explainable local similarity |
| 8 | Highlight digests and incremental export | `raindrop highlights digest`, `raindrop highlights export` | hand-code | Local joins and export cursors create deduplicated study outputs |
| 9 | Crash-safe triage workflow engine | `raindrop workflow triage`, `raindrop workflow status`, `raindrop workflow retry` | hand-code | SQLite lifecycle states generalize proven old processor queue semantics |

## Scope commitment

- 40 absorbed feature groups spanning complete documented REST CRUD, bulk operations, maintenance, files, import/export/backups, output, safety and MCP.
- 9 hand-written transcendence groups, estimated 4.4k-6.3k Go LoC plus generated client/command surface.
- No stubs approved by default.
- SQLite/FTS5 is foundational, not optional decoration.
