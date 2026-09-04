# Splitwise Absorb Manifest (reprint 2026-09-03)

## Source tools

| Tool | URL | Tag |
|---|---|---|
| Official API docs | https://github.com/splitwise/api-docs | PRIOR |
| tarunn2799/splitwise-mcp | https://github.com/tarunn2799/splitwise-mcp | PRIOR |
| svarun115/splitwise-mcp-server | https://github.com/svarun115/splitwise-mcp-server | PRIOR |
| namaggarwal/splitwise (Python SDK) | https://github.com/namaggarwal/splitwise | PRIOR |
| keriwarr/splitwise (TS SDK) | https://github.com/keriwarr/splitwise | PRIOR |
| anvari1313/splitwise.go | https://github.com/anvari1313/splitwise.go | PRIOR |
| aanzolaavila/splitwise.go | https://github.com/aanzolaavila/splitwise.go | PRIOR |
| riyaz489/Splitwise (local-DB clone, non-API) | https://github.com/riyaz489/Splitwise | PRIOR |
| Abhinav-Git19/Splitwise-CLI (non-API) | https://github.com/Abhinav-Git19/Splitwise-CLI | PRIOR |
| bhvkmuni/splitwise-mcp | https://github.com/bhvkmuni/splitwise-mcp | NEW |
| vishnujayvel/splitwise-mcp | https://github.com/vishnujayvel/splitwise-mcp | NEW |
| @rfdez/n8n-nodes-splitwise | https://github.com/rfdez/n8n-nodes-splitwise (npm: `@rfdez/n8n-nodes-splitwise`) | NEW |
| Splitwise Automation (Composio/Rube Claude Code skill) | https://mcpmarket.com/tools/skills/splitwise-automation (+ `-2` variant) | NEW |
| danthareja/splitwise_for_ynab | https://github.com/danthareja/splitwise_for_ynab | NEW |
| gcflames5/ynab-splitwise-integration | https://github.com/gcflames5/ynab-splitwise-integration | NEW |
| splitwise-to-ynab (npm) | https://www.npmjs.com/package/splitwise-to-ynab | NEW |
| prometheus-splitwise-exporter (PyPI) | https://pypi.org/project/prometheus-splitwise-exporter/ | NEW |
| alexcarol/splitwise (non-API CLI) | https://github.com/alexcarol/splitwise | NEW |
| dylburger/splitwise-cli (non-API CLI) | https://github.com/dylburger/splitwise-cli | NEW |

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get current user | MCP get-current-user / SDK getCurrentUser | (generated endpoint) users get_current_user | --json, offline cache, --select |
| 2 | Get user by id | MCP get-user / SDK getUser | (generated endpoint) users get_user | --json |
| 3 | Update user profile | SDK updateUser | (generated endpoint) users update_user | --dry-run |
| 4 | List groups (+balances) | MCP get-groups / SDK getGroups | (generated endpoint) groups get_groups | offline cache, --select, FTS |
| 5 | Get group | MCP get-group / SDK getGroup | (generated endpoint) groups get_group | --json |
| 6 | Create group | MCP create-group / SDK createGroup | (generated endpoint) groups create_group | --dry-run |
| 7 | Delete group | MCP delete-group / SDK deleteGroup | (generated endpoint) groups delete_group | --dry-run |
| 8 | Undelete group | Splitwise API undelete_group | (generated endpoint) groups undelete_group | --dry-run |
| 9 | Add user to group | MCP add-user-to-group / SDK addUserToGroup | (generated endpoint) groups add_user_to_group | fuzzy name resolve |
| 10 | Remove user from group | MCP remove-user-from-group | (generated endpoint) groups remove_user_from_group | fuzzy name resolve |
| 11 | List friends (+balances) | MCP get-friends / SDK getFriends | (generated endpoint) friends get_friends | offline cache |
| 12 | Get friend | MCP get-friend | (generated endpoint) friends get_friend | --json |
| 13 | Add friend | Splitwise API create_friend | (generated endpoint) friends create_friend | --dry-run |
| 14 | Add friends (bulk) | Splitwise API create_friends | (generated endpoint) friends create_friends | --dry-run |
| 15 | Delete friend | Splitwise API delete_friend | (generated endpoint) friends delete_friend | --dry-run |
| 16 | List expenses (filtered) | MCP get-expenses / SDK getExpenses | (generated endpoint) expenses get_expenses | incremental sync, offline cache |
| 17 | Get expense | MCP get-expense | (generated endpoint) expenses get_expense | --json |
| 18 | Create expense (split) | MCP create-expense / SDK createExpense | (generated endpoint) expenses create_expense | --dry-run, equal/exact shares |
| 19 | Update expense | MCP update-expense | (generated endpoint) expenses update_expense | --dry-run |
| 20 | Delete expense | MCP delete-expense | (generated endpoint) expenses delete_expense | --dry-run |
| 21 | Undelete expense | Splitwise API undelete_expense | (generated endpoint) expenses undelete_expense | --dry-run |
| 22 | List comments | MCP get-comments | (generated endpoint) comments get_comments | --json |
| 23 | Create comment | MCP create-comment | (generated endpoint) comments create_comment | --dry-run |
| 24 | Delete comment | MCP delete-comment | (generated endpoint) comments delete_comment | --dry-run |
| 25 | Get notifications | SDK getNotifications | (generated endpoint) notifications get_notifications | offline cache |
| 26 | List currencies | MCP get-currencies | (generated endpoint) currencies get_currencies | offline cache |
| 27 | List categories | MCP get-categories | (generated endpoint) categories get_categories | offline cache, fuzzy resolve |
| 28 | Fuzzy resolve friend/group/category by name | MCP resolve-friend/resolve-group/resolve-category | (behavior in splitwise-pp-cli resolve) reused by create/add commands | name→ID against the local store, offline, so users/agents never paste numeric IDs |
| 29 | Net balance overview | No competing tool aggregates groups+friends into one view | splitwise-pp-cli balances | Joins synced groups+friends derived balances into one net-position view; --by-currency per currency |
| 30 | Debt aging | No competing tool | splitwise-pp-cli debts --aged | Lists non-zero balances sorted by days since oldest unsettled expense per relationship |
| 31 | Group ledger w/ running balance | No competing tool | splitwise-pp-cli ledger "<group>" | Replays synced expenses in date order with cumulative per-member running balance |
| 32 | Spend analytics rollups | No competing tool (API has no aggregation endpoint) | splitwise-pp-cli spend --group-by category\|group\|month | Sums synced expense amounts bucketed locally |
| 33 | Settle-up plan (min-transfer) | No competing tool | splitwise-pp-cli settle-up "<group>" | Min-cash-flow graph over per-member net balances; --record creates payment:true expenses |
| 34 | Activity diff | No competing tool | splitwise-pp-cli activity | Diffs synced notifications + updated_after expenses against last-sync cursor |
| 35 | Split calculator / share builder | No competing tool | splitwise-pp-cli split | Computes per-user paid_share/owed_share for equal/exact/%/shares and previews the create_expense body; --record submits |
| 36 | Recurring-expense detector | No competing tool | splitwise-pp-cli recurring | Groups synced expenses by normalized description + cadence to surface repeating charges |
| 37 | Offline expense full-text search | No competing tool | splitwise-pp-cli search "term" --type expenses | FTS5 over synced expenses/comments/group/friend names via the framework search command |
| 38 | Atomic duplicate-expense detection (reserve-then-create pattern, fuzzy-match on desc/amount/date) | vishnujayvel/splitwise-mcp (NEW) | splitwise-pp-cli audit | Their pattern is scoped to two-person splits and expires reservations after 5 min; our `audit` runs against the full synced offline history (any group size) and also flags per-category cost outliers in the same pass — audit already closes this gap, shipped after the 2026-05-28 manifest and not previously recorded here |
| 39 | Push synced data to an external automation/workflow platform (n8n node) | @rfdez/n8n-nodes-splitwise (NEW) | (behavior in splitwise-pp-cli report) JSON/CSV output piped to any sink | Generic sink flag on every read command routes output to a webhook, covering the same "wire Splitwise into an external workflow" job without a dedicated per-platform integration |
| 40 | Export/sync Splitwise expenses into an external budgeting tool (YNAB) | danthareja/splitwise_for_ynab, gcflames5/ynab-splitwise-integration, splitwise-to-ynab (npm) (NEW) | splitwise-pp-cli report | Partial match / **gap**: `report` exports offline trip/period spend to md/csv/json and `import` upserts via API from JSONL, but neither speaks YNAB's transaction format directly — a real gap, not a full absorb (see Delta) |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|---------------|--------------------------|-------------------|
| 1 | Net balance overview | `balances` | hand-code | Joins synced `groups.members.balance` + `friends.balance` in local SQLite into one net-position view; `--by-currency`, `--by-group` | Use this command for your current net position (per friend, per currency, or per group). Do NOT use it for how long a debt has been open; use 'debts --aged'. Do NOT use it to collapse balances across groups into real-world transfers; use 'net'. Do NOT use it to plan a group settlement; use 'settle-up'. |
| 2 | Debt aging | `debts --aged` | hand-code | Reads synced expenses + `payment:true` settlements to compute days since the friend's last-settled point (`friendOpenDebt`), sorted desc | Use this command for who owes whom sorted by staleness. Do NOT use it for a plain net-position snapshot; use 'balances'. Do NOT use it for carrier-vs-rider or collection-risk classification; use 'fairness'. |
| 3 | Group ledger w/ running balance | `ledger "<group>"` / `ledger --friend "<name>"` | hand-code | Replays synced expenses (per-user paid/owed shares) in date order from local SQLite, cumulative per member; `--friend` replays one person across all groups + non-group | Use this command to see how balances got to where they are, expense by expense — '<group>' for one group's members, '--friend' for one person across every group. Do NOT use it for the current snapshot; use 'balances --by-group'. Do NOT use it to compute transfers; use 'settle-up'. Do NOT use it for spend totals; use 'spend'. |
| 4 | Spend analytics rollups | `spend --group-by category\|group\|month` | hand-code | Sums synced expense `cost` bucketed by category/group/month in local SQLite with `--since`/`--until` | Use this command for "how much did we spend on X / in <month> / in <group>". Do NOT use it for who owes whom; use 'balances'. Do NOT use it for a formatted export; use 'report'. Do NOT use it to convert currencies; use 'normalize'. |
| 5 | Offline expense search | `search "term" --type expenses` | spec-emits | Framework FTS over synced expenses/comments/names with scan-based word-boundary relevance | none |
| 6 | Settle-up plan | `settle-up "<group>"` | hand-code | Min-cash-flow graph over per-member net balances from local SQLite; `--record` POSTs `create_expense` with `payment:true` | Use this command to zero out ONE group in the fewest transfers, and to record those payments. Do NOT use it for netting across many groups and non-group balances; use 'net'. Do NOT use it to check the data first; use 'audit'. Do NOT use it to log a new shared expense; use 'split'. |
| 7 | Activity diff | `activity` | hand-code | Diffs synced notifications + `updated_after` expenses against the local last-sync cursor | Use this command for what changed since your last sync. Do NOT use it for a one-shot compact state digest; use 'brief'. Do NOT use it to verify the local store against the live API; use 'reconcile'. |
| 8 | Split calculator | `split` | hand-code | Computes paid_share/owed_share arrays (equal/exact/%/shares) summing exactly to total; previews the `create_expense` body; `--record` submits | Use this command to build and preview a new shared expense's shares. Do NOT use it to record a settlement payment; use 'settle-up --record'. |
| 9 | Recurring-expense detector | `recurring` | hand-code | Groups synced expenses by normalized description in SQLite, applies a cadence regularity gate, flags a missing cycle | Use this command to find repeating bills and a cycle that was not logged. Do NOT use it to project upcoming amounts; use 'forecast'. |
| 10 | Fairness lenses | `fairness --by risk\|contribution\|collectability` | hand-code | Classifies members carrier-vs-rider from local paid/owed shares; collectability from settlement episodes; emits `projected_days_out` | Use this command for who is carrying cost vs riding, and how collectable a debt is. Do NOT use it for a plain aged list; use 'debts --aged'. Do NOT use it for net position; use 'balances'. |
| 11 | Fairness nudge | `fairness nudge <friend>` | hand-code | Resolves friend → open expense locally, previews a reminder comment; `--send` POSTs `create_comment` | Use this command to post a payment reminder as a comment. Do NOT use it to record a payment; use 'settle-up --record'. |
| 12 | Cross-group netting | `net` | hand-code | Builds a debt graph over all synced group + friend balances, cancels cycles, emits minimum real-world transfers | Use this command when one person's balance spans many groups and non-group expenses and you want the minimum real-world transfers. Do NOT use it for a single group; use 'settle-up'. Do NOT use it for the per-group snapshot; use 'balances --by-group'. |
| 13 | Audit | `audit` | hand-code | Detects duplicate settlement rows across full synced history + median/MAD cost outliers per category; `--since`/`--until` | Use this command BEFORE settling to catch duplicate settlements and cost outliers. Do NOT use it to see what changed recently; use 'activity'. Do NOT use it to check the local store is complete vs the API; use 'reconcile'. |
| 14 | Forecast | `forecast` | hand-code | Projects next-due shared obligations from the `recurring` cadence model over local SQLite | Use this command for what shared bills are expected next. Do NOT use it to detect which bills recur; use 'recurring'. |
| 15 | Normalize | `normalize --base <ccy> --rate <ccy=x>` | hand-code | Converts local spend totals to a base currency using user-supplied rates; lists unconverted currencies | Use this command to express spend in one currency using rates you supply. Do NOT use it for spend buckets; use 'spend'. |
| 16 | Report | `report` | hand-code | Renders summary + per-person + per-category sections from local SQLite to md/csv/json | Use this command for a shareable end-of-trip / period export. Do NOT use it for an interactive rollup query; use 'spend'. Do NOT use it for the running history; use 'ledger'. |
| 17 | Balances by group | `balances --by-group` | hand-code | One row per group per currency, non-zero only, from local SQLite | (mode of #1; covered by #1's Long) |
| 18 | Which | `which "<phrase>"` | spec-emits (framework `which` in 4.31.6 already ranks research.json novel features by keyword; prior alias patch absorbed) | Matches a phrase against a static keyword-alias table mirroring the SKILL trigger phrases; no data read | none |
| 19 | Agent brief | `brief` | hand-code | Composes local reads (net headline from `balances`, top-N stalest from `debts --aged`, cursor diff from `activity`) into one bounded, `--compact`-safe payload | Use this command for a one-shot compact "what does the user need to know" state. Do NOT use it when the question is specifically balances, aging, or recent changes; use 'balances', 'debts --aged', or 'activity' for the full detail. |
| 20 | Store reconcile | `reconcile [--since 30d]` | hand-code | Calls live `get_expenses` with `updated_after` (paging until a short page) and compares IDs/`updated_at`/`deleted_at` against local SQLite; reports missing, stale, remotely-deleted | Use this command to verify the local store matches Splitwise before trusting a settle-up or report. Do NOT use it to see recent changes; use 'activity'. Do NOT use it for duplicate/outlier checks; use 'audit'. |

## Reprint verdicts

| Prior feature | Command | Verdict | Score | Justification |
|---------------|---------|---------|-------|---------------|
| Net balance overview | `balances` | keep | 10/10 | Core to all four personas; command name reused for compatibility. |
| Debt aging | `debts --aged` | keep | 9/10 | Riley's weekly "who never pays" question; carry forward the episode model and honest "-" for un-datable residuals. |
| Group ledger w/ running balance | `ledger "<group>"` | keep | 8/10 | Sam's audit trail; the new `--friend` flag extends it without renaming (no reframe). |
| Spend analytics rollups | `spend --group-by …` | keep | 9/10 | Workflow 2; carry forward `--since`/`--until` and `--csv`/`--plain` honesty. |
| Offline expense search | `search "term" --type expenses` | keep | 7/10 | Spec-emits framework command; carry forward scan-based word-boundary relevance. |
| Settle-up plan | `settle-up "<group>"` | keep | 10/10 | Workflow 1; carry forward resolver ambiguity errors (load-bearing for `--record`). |
| Activity diff | `activity` | keep | 7/10 | Sam/Avery reconcile-before-settle; Long now redirects to `brief` and `reconcile`. |
| Split calculator | `split` | keep | 8/10 | Avery's primary write action with preview-first; multi-word positional rejoin carried forward. |
| Recurring-expense detector | `recurring` | keep | 8/10 | Riley's missed-bill check; regularity gate carried forward (trips/settlements excluded). |
| Fairness lenses | `fairness --by risk\|contribution\|collectability` | keep | 9/10 | Workflow 3; `projected_days_out` retained but flagged low-confidence verifiability. |
| Fairness nudge | `fairness nudge <friend>` | keep | 8/10 | Riley's reminder ritual; preview default / `--send` retained. |
| Cross-group netting | `net` | keep | 9/10 | Workflow 4; distinct from `settle-up` by scope, enforced via Long redirects. |
| Audit | `audit` | keep | 10/10 | Workflow 5; the only prior feature with two independent evidence sources (competitor MCP + brief). |
| Forecast | `forecast` | keep | 7/10 | Riley's "what's due next"; kept above the `budget` candidate it obsoletes. |
| Normalize | `normalize` | keep | 7/10 | Devon's multi-currency need; user-supplied rates only, unconverted surfaced — unchanged by design. |
| Report | `report` | keep | 9/10 | Sam's end-of-trip export and the generic sink for the n8n/YNAB gap. |
| Balances by group | `balances --by-group` | keep | 8/10 | Mode of `balances`; offline per-group-per-currency non-zero view retained. |
| Which | `which` | keep | 6/10 | Lowest scorer but above the 5/10 floor; Avery's CLI-side sibling disambiguator matching SKILL trigger phrases. |

Dropped prior features: none.

## Stubs

None — every row ships fully.

## Hand-code commitment

Hand-code count: **19**. Command names: `balances`, `debts --aged`, `ledger`, `spend`, `settle-up`, `activity`, `split`, `recurring`, `fairness`, `fairness nudge`, `net`, `audit`, `forecast`, `normalize`, `report`, `balances --by-group`, `which`, `brief`, `reconcile`.

Spec-emits count: **1**. `search` (framework FTS command, generator-emitted).
