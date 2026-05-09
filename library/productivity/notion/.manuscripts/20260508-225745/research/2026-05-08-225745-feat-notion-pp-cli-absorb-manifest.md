# Notion CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Workspace full-text search | notion-cli (4ier) + official MCP | FTS5 offline search on synced titles+content | Works offline, regex-composable, instant |
| 2 | Page view/get | notion-cli (4ier), official MCP `fetch_page` | `pages get <id>` with --json, --select | Agent-native output |
| 3 | Page create | notion-cli (4ier), official MCP `create_page` | `pages create --title --parent --db` with --dry-run | Idempotent, scriptable |
| 4 | Page update | notion-cli (4ier), official MCP `update_page` | `pages update <id>` with --dry-run | Typed exit codes |
| 5 | Page delete/archive | notion-cli (4ier), official MCP `delete_page` | `pages delete <id>` with --dry-run | |
| 6 | Page restore | notion-cli (4ier) | `pages restore <id>` | |
| 7 | Page move | notion-cli (4ier), official MCP `move_page` | `pages move <id> --to <parent>` | |
| 8 | Page open (browser) | notion-cli (4ier) | `pages open <id>` (--launch flag required; print by default) | Side-effect safe, verify-friendly |
| 9 | Page properties | notion-cli (4ier) | `pages props <id>` --json, --select | |
| 10 | Page as markdown | notion-cli (4ier), suekou MCP `notion_read_page` | `pages export <id> --format markdown` | Depth control, --file output |
| 11 | List pages | notion-cli (4ier) | `pages list` --filter --limit --json | |
| 12 | Database list | notion-cli (4ier) | `databases list` --json | |
| 13 | Database view/get | notion-cli (4ier), official MCP `retrieve_data_source` | `databases get <id>` --json | Schema output |
| 14 | Database query with filters | notion-cli (4ier), suekou MCP `notion_query_data_source_by_values` | `databases query <id> --filter --sort --limit` | Human-readable filter syntax |
| 15 | Database create | notion-cli (4ier), official MCP `create_data_source` | `databases create --title --parent` | |
| 16 | Database update | notion-cli (4ier), official MCP `update_data_source` | `databases update <id>` | |
| 17 | Add database record | notion-cli (4ier), suekou MCP `notion_create_data_source_item_from_values` | `databases add <id> --props` | Schema-validated |
| 18 | Add database records bulk | notion-cli (4ier) `add-bulk` | `databases add-bulk <id> --from-json --from-csv` | Retry + progress |
| 19 | Block list | notion-cli (4ier) | `blocks list <page-id>` --depth --json | |
| 20 | Block get | notion-cli (4ier) | `blocks get <id>` --json | |
| 21 | Block append | notion-cli (4ier), suekou MCP `notion_append_content` | `blocks append <page-id> --text --type` | Safe block types |
| 22 | Append markdown | notion-cli (4ier), suekou MCP `notion_append_markdown` | `blocks append-md <page-id> --file` | Markdown subset support |
| 23 | Block update | notion-cli (4ier), suekou MCP `notion_update_content` | `blocks update <id> --text` | |
| 24 | Block delete | notion-cli (4ier) | `blocks delete <id>` --dry-run | |
| 25 | Comment list | notion-cli (4ier), official MCP `get_comments` | `comments list <page-id>` --json | |
| 26 | Comment add | notion-cli (4ier), official MCP `create_comment` | `comments add <page-id> --text` | |
| 27 | Comment get | notion-cli (4ier) | `comments get <id>` | |
| 28 | User list | notion-cli (4ier), official MCP `list_users` | `users list` --json | |
| 29 | User get | notion-cli (4ier), official MCP `get_user` | `users get <id>` | |
| 30 | Current user / me | notion-cli (4ier) | `users me` | |
| 31 | File upload | notion-cli (4ier) | `files upload <path>` --page-id | Resumable protocol |
| 32 | Raw API escape hatch | notion-cli (4ier) | `api GET/POST/PATCH/DELETE <path>` | Full passthrough |
| 33 | Auth set/login | notion-cli (4ier) | `auth set-token` / `auth status` / `auth logout` | |
| 34 | Doctor / health check | notion-cli (4ier) | `doctor` | Auth valid, API reachable |
| 35 | Sync workspace to SQLite | (novel — pattern from slack-pp-cli) | `sync --full / --incremental` | Incremental via last_edited_time |
| 36 | Compact structured search | suekou MCP `notion_find` | `search` --json, --select, --limit | FTS5 offline + live fallback |
| 37 | Schema inspect | suekou MCP `notion_inspect_data_source` | `databases schema <id>` | Property types, required fields |
| 38 | View list | official API `views` tag | `views list <db-id>` | |
| 39 | View query | official API | `views query <view-id>` | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Cross-database SQL joins | `sql "<query>"` | 10 | API has no join primitive; cross-database queries require the local store |
| 2 | Agent project context dump | `context <project>` | 10 | Token-efficient structured snapshot from local JOINs; live API needs 10-30+ sequential calls |
| 3 | Cross-client summary | `client-summary <name>` | 9 | Cross-database aggregation by shared property value; requires local join across data_source_items |
| 4 | Status stagnation detection | `status-drift --days N` | 9 | API cannot filter on "status value unchanged since N days" |
| 5 | Workspace change diff | `changed [--since <ts>]` | 9 | API is stateless; delta detection requires a prior snapshot |
| 6 | Stale page/record detection | `stale --days N [--db <id>]` | 9 | No API filter for "last-edited before date" across multiple databases simultaneously |
| 7 | Multi-hop relation traversal | `graph traverse <id> --depth N` | 9 | Multi-hop traversal is O(branching^hops) sequential API calls; local graph is BFS |
| 8 | Dead relation detection | `dead-links` | 8 | No reverse-lookup API; cross-references relation targets against pages.archived |
| 9 | Shortest relation path | `relation-path <from> <to>` | 8 | BFS on relation graph; API exploration is exponential in call count |
| 10 | Workspace health scorecard | `workspace-health` | 8 | Composite of orphan rate, stale rate, unowned rate, dead-link rate — all local aggregations |
| 11 | Inbound+outbound relation map | `relation-map <db-id>` | 8 | API only exposes outbound; inbound map requires local relations table |
| 12 | Orphan detection | `orphans` | 8 | API has no "who links to me" endpoint |
| 13 | Per-user load across databases | `owner-load` | 8 | Cross-database aggregation on person properties; no API aggregate |
| 14 | Near-duplicate titles | `duplicates [--threshold N]` | 7 | FTS5 trigram ranking across all page titles; API search is exact-keyword |
| 15 | Mention audit | `mention-audit <user>` | 7 | FTS5 reverse-index on @-mentions across pages and comments |
| 16 | Edit frequency heatmap | `activity-heatmap [--weeks N]` | 7 | Aggregates last_edited_by × week; no API aggregation endpoint |
| 17 | Relation population audit | `relation-audit <db-id>` | 7 | Computes fill-rate per relation property across all records |
| 18 | Scope comparison | `scope-diff <scope-a> <scope-b>` | 7 | Scopes are local concept; cross-scope comparison has no API equivalent |
| 19 | Timeline gap detection | `timeline gaps <db-id>` | 7 | Coverage gaps in date property; API can filter but not aggregate coverage |
| 20 | Unlinked database detection | `unlinked-databases` | 7 | Left-join databases against relations; no relation inventory API |
