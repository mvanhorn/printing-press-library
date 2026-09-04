# Splitwise CLI — Ecosystem Absorb Refresh (2026-09-03)

Refresh of the 2026-05-28 absorb manifest
(`manuscripts/splitwise/20260528-002755/research/2026-05-28-002755-feat-splitwise-pp-cli-absorb-manifest.md`,
itself a reconciliation of the 2026-05-25 print's full `Absorbed` table). Landscape verdict is
**unchanged**: every live third-party tool is still a thin wrapper over the same 27-endpoint
Splitwise API. Nothing found this pass adds a feature the ecosystem has that we lack outright —
one prior gap (duplicate-expense detection) turns out to already be closed by our own CLI's
`audit` command, which shipped after 2026-05-28 and wasn't reflected in that manifest.

## Source tools

| Tool | URL | Language | Stars | Tag |
|---|---|---|---|---|
| Official API docs | https://github.com/splitwise/api-docs | — | — | PRIOR |
| tarunn2799/splitwise-mcp | https://github.com/tarunn2799/splitwise-mcp | Python | 11 | PRIOR |
| svarun115/splitwise-mcp-server | https://github.com/svarun115/splitwise-mcp-server | Python | 1 | PRIOR |
| namaggarwal/splitwise (Python SDK) | https://github.com/namaggarwal/splitwise | Python | 213 | PRIOR |
| keriwarr/splitwise (TS SDK) | https://github.com/keriwarr/splitwise | TypeScript | 79 | PRIOR |
| anvari1313/splitwise.go | https://github.com/anvari1313/splitwise.go | Go | 12 | PRIOR |
| aanzolaavila/splitwise.go | https://github.com/aanzolaavila/splitwise.go | Go | 0 | PRIOR |
| riyaz489/Splitwise (local-DB clone, non-API) | https://github.com/riyaz489/Splitwise | Python | — | PRIOR |
| Abhinav-Git19/Splitwise-CLI (non-API) | https://github.com/Abhinav-Git19/Splitwise-CLI | — | — | PRIOR |
| bhvkmuni/splitwise-mcp | https://github.com/bhvkmuni/splitwise-mcp | Python | 1 | NEW |
| vishnujayvel/splitwise-mcp | https://github.com/vishnujayvel/splitwise-mcp | Python | 0 | NEW |
| @rfdez/n8n-nodes-splitwise | https://github.com/rfdez/n8n-nodes-splitwise (npm: `@rfdez/n8n-nodes-splitwise`) | TypeScript | — | NEW |
| Splitwise Automation (Composio/Rube Claude Code skill) | https://mcpmarket.com/tools/skills/splitwise-automation (+ `-2` variant) | — | — | NEW |
| danthareja/splitwise_for_ynab | https://github.com/danthareja/splitwise_for_ynab | JavaScript | — | NEW |
| gcflames5/ynab-splitwise-integration | https://github.com/gcflames5/ynab-splitwise-integration | — | — | NEW |
| splitwise-to-ynab (npm) | https://www.npmjs.com/package/splitwise-to-ynab | JavaScript | — | NEW |
| prometheus-splitwise-exporter (PyPI) | https://pypi.org/project/prometheus-splitwise-exporter/ | Python | — | NEW |
| alexcarol/splitwise (non-API CLI) | https://github.com/alexcarol/splitwise | — | — | NEW (found this pass, not previously surveyed) |
| dylburger/splitwise-cli (non-API CLI) | https://github.com/dylburger/splitwise-cli | — | — | NEW (found this pass, not previously surveyed) |

Not included: `paulakimenko/splitwise_mcp` — surfaced in search snippets but `gh api repos/paulakimenko/splitwise_mcp`
returns 404 (repo missing/renamed/private); excluded rather than reported on unverified content.
`gh api repos/anthropics/claude-plugins-official/contents/external_plugins` was checked directly —
no `splitwise` entry in the official plugin marketplace (list: asana, context7, discord, fakechat,
firebase, github, gitlab, greptile, imessage, laravel-boost, linear, playwright, serena, telegram,
terraform).

## Absorbed

Carried forward verbatim from the prior manifests (2026-05-25 full table + 2026-05-28's approved
transcendence additions), plus new rows appended this pass.

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
| 39 | Push synced data to an external automation/workflow platform (n8n node) | @rfdez/n8n-nodes-splitwise (NEW) | (behavior in splitwise-pp-cli --deliver flag) --deliver webhook:<url> | Generic sink flag on every read command routes output to a webhook, covering the same "wire Splitwise into an external workflow" job without a dedicated per-platform integration |
| 40 | Export/sync Splitwise expenses into an external budgeting tool (YNAB) | danthareja/splitwise_for_ynab, gcflames5/ynab-splitwise-integration, splitwise-to-ynab (npm) (NEW) | splitwise-pp-cli report (+ import for the reverse direction) | Partial match / **gap**: `report` exports offline trip/period spend to md/csv/json and `import` upserts via API from JSONL, but neither speaks YNAB's transaction format directly — a real gap, not a full absorb (see Delta) |

**Hand-code count: 10** (balances, debts, ledger, spend, settle-up, activity, split, recurring, audit-as-duplicate-detection, plus the fuzzy-resolve helper) + framework `search` (spec-emits) + framework `--deliver` flag (framework behavior).
Row 40 is the one row this pass where "Our Implementation" is an honest partial match, not a clean absorb.

## Delta

- **New tools found (9):** bhvkmuni/splitwise-mcp, vishnujayvel/splitwise-mcp, @rfdez/n8n-nodes-splitwise,
  the Composio/Rube "Splitwise Automation" Claude Code skill (two listings on mcpmarket.com), danthareja/splitwise_for_ynab,
  gcflames5/ynab-splitwise-integration, splitwise-to-ynab (npm), prometheus-splitwise-exporter (PyPI),
  plus two previously-unsurveyed non-API CLIs (alexcarol/splitwise, dylburger/splitwise-cli). None expose an
  endpoint beyond the same 27-endpoint API surface; bhvkmuni's server (`get_current_user`, `list_currencies`,
  `list_groups`, `get_group`, `list_expenses`, `get_expense`, `create_expense`, `update_expense`, `delete_expense`,
  `create_payment`) is a strict subset of what we already emit.
- **New features added (rows 38–40):** duplicate-expense detection (row 38) — turned out to be a
  **non-issue**: our own `audit` command already covers it, added to the CLI sometime after the
  2026-05-28 manifest was written (the manifest's transcendence table is now stale against the
  installed binary, which also has grown `fairness`, `forecast`, `net`, `normalize`, `report`, `tail`,
  `workflow`, `import`, and `which` — none of those were checked against this pass's ecosystem search
  but are worth folding into the next full manifest rewrite). Workflow-platform push (row 39) is covered
  generically by `--deliver webhook:<url>`. YNAB budgeting sync (row 40) is a **real, unclosed gap** —
  no command speaks YNAB's transaction/category format; `report`/`import` are adjacent but not a fit.
- **Prior tools that look archived/dead:** `namaggarwal/splitwise` (Python SDK, 213 stars, no push since
  2024-05-21 — 2+ years dormant), `anvari1313/splitwise.go` and `aanzolaavila/splitwise.go` (both no push
  since 2022-11, 12 and 0 stars) — all three still function as thin API wrappers (the API hasn't changed)
  but are unmaintained. `tarunn2799/splitwise-mcp` and `svarun115/splitwise-mcp-server` are both still
  active (last pushed 2026-04-26 and 2026-07-19 respectively) — no change in their status.
- **Not pursued:** `prometheus-splitwise-exporter` is a niche ops/monitoring exporter (Splitwise → Prometheus
  metrics), out of scope for a personal-finance CLI — noted as a source tool but no Absorbed row. The Rube/Composio
  "Splitwise Automation" skill is agent-orchestration plumbing around the same API + fuzzy resolve we already
  match — no new row.
