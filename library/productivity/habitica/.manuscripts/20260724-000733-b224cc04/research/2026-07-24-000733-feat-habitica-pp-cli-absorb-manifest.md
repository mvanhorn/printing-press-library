# Habitica CLI Absorb Manifest

## Sources examined

- Official Habitica API documentation and current server controllers (`tasks.js`, `user.js`, `tags.js`, `notifications.js`, `shops.js`, authentication middleware).
- `melvio/hopla`: active Python CLI, especially due-dated checklist to-do creation, authentication, task/gameplay operations, user stats and inventory.
- Legacy `habitica` PyPI CLI: status, habits/dailies/to-dos list and scoring, and to-do creation.
- `iBreaker/habitica-mcp-server`: current MCP source with the same header auth and task/checklist/tag/inventory/reward/pet/mount/notification/skill operations.

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Authenticated profile and stats | iBreaker MCP (source) | (generated endpoint) user get | Structured output, `--select`, sensitive credentials |
| 2 | List all task types | Habitica Python CLI | (generated endpoint) tasks user | JSON filtering and local sync/search |
| 3 | Create habit, daily, todo, or reward | iBreaker MCP (source) | (generated endpoint) tasks create-user | Explicit mutation gate and `--dry-run` |
| 4 | Update or delete a task | iBreaker MCP (source) | (generated endpoint) tasks update/delete | Typed output and destructive annotations |
| 5 | Score a task up or down | Habitica Python CLI | (generated endpoint) tasks score | Explicit confirmation and JSON envelope |
| 6 | Move a task in its column | Official task controller (source) | (generated endpoint) tasks move | Scriptable order changes |
| 7 | Full checklist lifecycle | iBreaker MCP (source) | (generated endpoint) tasks add-checklist/score-checklist/update-checklist/delete-checklist | Typed commands and safe mutation gates |
| 8 | Add or remove task tags | Official task controller (source) | (generated endpoint) tasks add-tag/delete-tag | Agent-ready tagging |
| 9 | Manage tags | iBreaker MCP (source) | (generated endpoint) tags list/create/update/delete | JSON plus local search |
| 10 | Create due-dated to-do with checklist | Hopla | (behavior in habitica-pp-cli plan chores) preview then create API calls | Batch preview and explicit apply |
| 11 | Character inventory and purchasable gear | iBreaker MCP (source) | (generated endpoint) user get-buy-list | Local snapshots and `--select` |
| 12 | In-app and custom rewards | Official user controller (source) | (generated endpoint) user get-in-app-rewards; (generated endpoint) tasks user | Account-specific availability and prices |
| 13 | Buy gear, quest, potion, or item | iBreaker MCP (source) | (generated endpoint) user buy/buy-gear/buy-quest/purchase | Explicit confirmation and verify-safe no-op |
| 14 | Pets and mounts: list, feed, hatch, equip | iBreaker MCP (source) | (generated endpoint) user hatch/feed/equip | Typed inputs and mutation safety |
| 15 | Cast class skills | iBreaker MCP (source) | (generated endpoint) user cast | Typed output and confirmation |
| 16 | Read and acknowledge notifications | iBreaker MCP (source) | (generated endpoint) notifications list/read | Agent filtering and JSON |
| 17 | Shop/reward item browsing | iBreaker MCP (source) | (generated endpoint) user get-buy-list/get-in-app-rewards | Current account-specific availability |
| 18 | Local task and reward search | Printing Press baseline | (behavior in habitica-pp-cli search) local FTS over synced resources | Offline search and SQL composability |

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---:|---|---|---|
| 1 | Daily quest briefing | `today` | 9/10 | hand-code | Joins official task and user state with optional local task/tag data into a read-only daily ritual. | none |
| 2 | Structured morning quest plan | `plan chores --file chores.yaml --dry-run` | 9/10 | hand-code | Validates a local chore batch and shows exact API mutations before an explicit apply/confirmation. | none |
| 3 | Reward runway | `reward afford <reward-or-item> --reserve-gp 20` | 10/10 | hand-code | Joins real gold with custom, in-app, and buy-list reward prices while preserving user-selected reserve gold. | none |
| 4 | Tag workload report | `tag load` | 9/10 | hand-code | Computes due, overdue, active, and unfinished-checklist workload per tag from synced SQLite resources. | Use this command to compare workload across tags. Do NOT use it for today’s ordered action queue; use `today` instead. |
| 5 | Weekly task health | `week review` | 6/10 | hand-code | Compares real timestamped synced task snapshots for overdue, stalled, and completed-task trends. | Use this command for seven-day task changes from synced snapshots. Do NOT use it for today’s actionable queue; use `today` instead. |

## Mutation safety contract

Every generated or hand-authored external mutation uses the Printing Press transport verification guard, supports `--dry-run` where the command is composed, and requires an explicit confirmation/apply opt-in before score, create, update, delete, purchase, equip, hatch, feed, or cast actions. `today`, `reward afford`, `tag load`, and `week review` are read-only and receive `mcp:read-only` annotations.

## Buildability and scope

- Absorbed features: 18 across four independently useful public tools/sources.
- Novel features: 5, all hand-coded; no approved stubs and no auto-emitted novel feature rows.
- The generator will use `--docs https://apidoc.habitica.com/`, with an internal spec/overlay only where the APIDoc source lacks OpenAPI machine structure. The implementation must use verified official route names and must not hand-invent payloads.
