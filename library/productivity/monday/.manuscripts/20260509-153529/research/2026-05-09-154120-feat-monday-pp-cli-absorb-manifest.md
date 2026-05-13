# Monday.com Absorb Manifest

## Source tools surveyed

| Tool | Type | Stars | Role |
|------|------|-------|------|
| mondaycom/mcp | Official MCP server | 401 | Comprehensive — ~50 hand-tuned platform-API tools |
| mondaycom/monday-graphql-api | Official Node SDK (`@mondaydotcomorg/api`) | n/a | Generated GraphQL client + types; primary online wrapper |
| mondaycom/monday-api-python-sdk | Official Python SDK | n/a | Boards, items, columns, docs, updates, activity_logs modules |
| ProdPerfect/monday | Community Python wrapper | n/a | Alternate Python client; smaller surface |
| GearPlug/monday-python | Community Python wrapper | n/a | Adds webhook helpers |
| mondaycom/monday-sdk-js | Legacy Node SDK | n/a | Server-side deprecated; client-only going forward |
| mondaycom/monday-apps-cli (`mapps`) | Different scope | n/a | NOT a competitor — apps-framework deploy CLI, not data CLI |
| ScriptRunner Advanced Automations | Paid 3rd-party | n/a | TS/JS scripting on Monday boards; in-product, not a CLI |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Get current user context | mondaycom/mcp `get_user_context` | `monday whoami` | Persisted to local store; joined to teams + workspaces |
| 2 | List workspaces | mondaycom/mcp `list_workspaces` | `monday workspaces list` | FTS5 + offline + SQL composable |
| 3 | Workspace details | mondaycom/mcp `workspace_info` | `monday workspaces get <id>` | Cached locally |
| 4 | Create workspace | mondaycom/mcp `create_workspace` | `monday workspaces create --name --kind` | --dry-run, --stdin batch |
| 5 | Update workspace | mondaycom/mcp `update_workspace` | `monday workspaces update <id>` | --dry-run |
| 6 | List/get/create/update folders | mondaycom/mcp `create/update_folder` | `monday folders list/get/create/update` | Hierarchy persisted |
| 7 | Get board info | mondaycom/mcp `get_board_info` | `monday boards get <id>` | Cached column schema |
| 8 | Full board data | mondaycom/mcp `get_full_board_data` | `monday boards full <id>` | One-call hydration to local store |
| 9 | Create board | mondaycom/mcp `create_board` | `monday boards create` | --from-template flag |
| 10 | Get board items page | mondaycom/mcp `get_board_items_page` | `monday items list --board <id>` | Auto-paginates cursor; materializes typed columns |
| 11 | Board activity log | mondaycom/mcp `get_board_activity` | `monday boards activity <id>` | Persisted; queryable by user/column/window |
| 12 | Board insights / aggregates | mondaycom/mcp `board_insights` | `monday boards insights <id>` | Local re-computable from store |
| 13 | Universal search | mondaycom/mcp `search` | `monday search "<term>"` | FTS5 offline; covers items+updates+docs+columns |
| 14 | Get column type info | mondaycom/mcp `get_column_type_info` | `monday columns types` | Static reference + live |
| 15 | Get GraphQL schema | mondaycom/mcp `get_graphql_schema` | `monday schema` | Cached schema file |
| 16 | Type introspection | mondaycom/mcp `get_type_details` | `monday schema type <name>` | Offline introspection |
| 17 | Create item | mondaycom/mcp `create_item` | `monday items create --board --group --name --column-values` | --dry-run, batch via --stdin |
| 18 | Delete item | mondaycom/mcp `delete_item` | `monday items delete <id>` | --dry-run, batch |
| 19 | Move item to group | mondaycom/mcp `move_item_to_group` | `monday items move <id> --group <id>` | Batch |
| 20 | Change item column values | mondaycom/mcp `change_item_column_values` | `monday items set <id> --column status=Done` | Typed column validation, batch |
| 21 | Create column | mondaycom/mcp `create_column` | `monday columns create --board <id> --type status` | --dry-run |
| 22 | Update column | mondaycom/mcp `update_column` | `monday columns update <id>` | --dry-run |
| 23 | Delete column | mondaycom/mcp `delete_column` | `monday columns delete <id>` | --dry-run |
| 24 | Create group | mondaycom/mcp `create_group` | `monday groups create --board <id> --title <name>` | --dry-run |
| 25 | Move group/object | mondaycom/mcp `move_object` | `monday groups move`, `monday boards move` | Generic move |
| 26 | Get item updates | mondaycom/mcp `get_updates` | `monday updates list --item <id>` | Persisted, full-text indexed |
| 27 | Create update | mondaycom/mcp `create_update` | `monday updates create --item <id> --body` | --dry-run, multi-line --stdin |
| 28 | Read docs | mondaycom/mcp `read_docs` | `monday docs get <id>` | Cached locally |
| 29 | Create doc | mondaycom/mcp `create_doc` | `monday docs create` | From file |
| 30 | Update doc | mondaycom/mcp `update_doc` | `monday docs update <id>` | From file, append mode |
| 31 | List users + teams | mondaycom/mcp `list_users_and_teams` | `monday users list`, `monday teams list` | Persisted |
| 32 | Get assets | mondaycom/mcp `get_assets` | `monday assets list --item <id>` | Metadata + download |
| 33 | Update assets on item | mondaycom/mcp `update_assets_on_item` | `monday assets set --item <id>` | Upload |
| 34 | Create notification | mondaycom/mcp `create_notification` | `monday notify --user <id> --target-id <id> --text` | Batch, --dry-run |
| 35 | Notetaker meetings | mondaycom/mcp `get_notetaker_meetings` | `monday meetings list` | Persisted, transcripts indexed |
| 36 | Dev sprint boards | mondaycom/mcp `get_monday_dev_sprints_boards` | `monday sprints boards` | Persisted |
| 37 | Sprint metadata | mondaycom/mcp `get_sprints_metadata` | `monday sprints metadata --board <id>` | Cached |
| 38 | Sprint summary | mondaycom/mcp `get_sprint_summary` | `monday sprints summary --board <id>` | Cached |
| 39 | Raw GraphQL passthrough | mondaycom/mcp `all_monday_api` | `monday gql --query <file>` | Saved queries, stored params |

## Transcendence (only possible with our approach)

Bottom 4 (score 6/10) flagged as "scope-trim candidates" — user can drop at gate.

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|-------------------------|---------|
| 1 | Cross-board activity since window | `monday since <window> [--board ...] [--user ...]` | 9/10 | Local activity_logs table + thin API tail-fetch; web UI's universal Activity caps at 50 events; per-board API requires separate calls | Priya, Devraj |
| 2 | Per-person workload across boards | `monday whoami-load [--user @me] [--window 7d]` | 8/10 | SQLite join across items + person column_values + boards + status that no single API call returns | Priya |
| 3 | Status-bottleneck dwell-time | `monday bottleneck --board <id> [--column status]` | 8/10 | Reads activity_logs locally; per-status median + p90 dwell time computed from column-value transitions | Devraj, Priya |
| 4 | Sprint velocity & slip report | `monday velocity --board <sprint-id> [--last N]` | 8/10 | Joins sprint metadata + activity log + items; counts committed/done/slipped/added-mid-sprint per cycle | Devraj |
| 5 | Column-schema drift detector | `monday column-drift [--board <id>]` | 8/10 | Diffs cached boards.columns JSON vs fresh response; the polymorphic-typed-columns problem is THE central pain | Sam, Priya |
| 6 | Cross-board contextual mentions | `monday mentions "<text>" [--scope items,updates,docs]` | 7/10 | FTS5 over indexed item names + update bodies + doc bodies + text columns; results hydrated with board+group+owner context | Sam |
| 7 | Pre-flight complexity budget | `monday complexity-budget [--query <file>]` | 7/10 | GraphQL `complexity { before after reset_in_x_seconds }` is queryable but never surfaced as an affordance; predicts points + remaining budget | Sam |
| 8 | Bulk column-value editor with typed dry-run | `monday bulk-edit --from items.csv [--dry-run]` | 7/10 | CSV → typed-schema validation → unified diff → resume-on-failure apply; absorbed `items set --stdin` doesn't validate or diff | Sam, Priya |
| 9 | Mirror & formula resolver (scope-trim) | `monday resolve --item <id> [--column <id>]` | 6/10 | Walks local column_values rows of type mirror/formula; follows board_relation linkage to source; lists formula column refs | Priya, Sam |
| 10 | External-CSV reconcile (scope-trim) | `monday reconcile --against external.csv --key <col>` | 6/10 | Local items + CSV join; emits only-in-monday/only-in-csv/diff sets | Sam |
| 11 | Per-board health scorecard (scope-trim) | `monday boards health <id>` | 6/10 | Aggregates local store: % owner-set, % due-set, % overdue, % updated-7d, count empty-status, count broken-mirror | Priya |
| 12 | Item full-context dump (scope-trim) | `monday context <item-id>` | 6/10 | One SQLite join over items + column_values + updates + replies + docs + assets + activity_logs + mirror sources for one item-id; ideal for `--json` to LLM | Devraj, Sam |
| 13 | Cross-system ID exporter | `monday cross-ref --board <id> --link-column <col>` | 8/10 | Joins items to a designated cross-system ID column (Linear id, Notion page-id, Slack thread-ts) and emits a structured `{monday_id, linker, item, status, owner, board}` list ready to pipe into another CLI. Pure local store query | User-stated workflow (Plexiz creatives crossing between Monday and Linear constantly) |

## Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Stale items detector | Already in standard PP transcendence | #3 bottleneck |
| Saved query library | Wrapper over absorbed `monday gql` | gql (absorbed) |
| Notetaker action-item extractor | Requires LLM classification of free transcript | #12 context |
| What's-on-my-plate digest | Sibling-kills with #2 whoami-load | #2 whoami-load |

## Stubs

None. Every absorbed and transcendence feature is shipping-scope. If the user trims scope at the gate, the dropped features are removed from the manifest, not shipped as stubs.
