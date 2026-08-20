# Lunch Money CLI Brief

## API Identity
- **Domain:** Personal finance / budgeting. SaaS at lunchmoney.app ($5/mo paid; no free tier). Single-user budgets with shared family access.
- **Users:** Power-user budgeters who hand-categorize transactions, want manual control vs YNAB rigidity. Heavy users of Plaid-connected accounts, manual asset tracking, multi-currency, crypto, and per-category budgeting.
- **Data profile:** Transactions (rich filters), categories (hierarchical), tags, manual + Plaid accounts, manual + synced crypto, balance history, budgets (period rollovers), recurring items, file attachments.

## API Version Choice: v2 (alpha, but official)
- **v2.11.0-preview.2** has an official OpenAPI 3.0.2 spec published to npm as `@lunch-money/v2-api-spec` (11,733 lines, 59 operations across 16 tags).
- Lunch Money recommends v2 for new projects; v1 docs explicitly state "v1 will not progress to GA."
- v2 reaches GA "early 2026"; we are May 2026 and the npm spec ships `2.11.0-preview.2`. Risk of breakage during alpha period is real but manageable.
- **Two live servers:** `https://api.lunchmoney.dev/v2` (real data) and `https://mock.lunchmoney.dev/v2` (accepts any 11+ char bearer; perfect for live testing without touching real money data).

## Reachability Risk
- **Low.** Both live and mock servers return HTTP 200 on `GET /me`. Rate limit header: `x-ratelimit-limit: 100`. No open issues on `juftin/lunchable` (top Python wrapper) matching 403/blocked/broken/rate-limit. Token-auth REST API, no Cloudflare/bot challenge.
- **Rate-limit awareness needed:** 100 calls per window. Generated client should respect `x-ratelimit-remaining` / `x-ratelimit-reset` and back off.

## Top Workflows
1. **Categorize uncategorized transactions** — list `status=unreviewed` or `category_id=0`, fix in bulk via `PUT /transactions` (bulk update).
2. **Review imported transactions** — pull recent `is_pending=false`, mark reviewed, optionally adjust category/payee/notes.
3. **Track budget progress mid-month** — `GET /summary?include_totals=true&include_occurrences=true` for current period; compare to target spend.
4. **Audit recurring expenses** — list `/recurring_items`; cross-check against transactions for missed/extra charges.
5. **Bulk fix-ups** — find transactions matching a pattern (merchant name regex, tag, amount range), update many at once. Currently painful in the web UI.
6. **Reconcile manual account balances** — update `manual_accounts` and `balance_history` snapshots for assets without Plaid sync.

## Table Stakes (must match)
- Full CRUD on all 16 resource tags (me, summary, categories, tags, transactions, transactions-group/split/files, manual_accounts, plaid_accounts, plaid fetch trigger, crypto-manual, crypto-synced, balance_history, recurring_items, budgets).
- Rich transaction filters: date range, category, tag, account, status, pending, group, splits, metadata, attachments.
- Budget summary with occurrences, totals, rollover_pool.
- Bulk operations on transactions (POST/PUT/DELETE on `/transactions`).
- File attachments on transactions.
- Plaid sync trigger.
- Crypto manual + synced + currency lookup.

## Data Layer (local SQLite store)
- **Primary entities to sync:** transactions, categories, tags, manual_accounts, plaid_accounts, recurring_items, balance_history, budgets (settings + occurrences), crypto-manual, crypto-synced. The `user` is a singleton.
- **Sync cursor:** `updated_since` on transactions (ISO 8601). Categories/tags/accounts are small; full-fetch each sync. Balance history is append-mostly.
- **FTS/search:** transactions full-text on `payee`, `notes`, `external_id`, plus category name and tag list. Categories/tags FTS on names.
- **Useful joins not available via API in one call:**
  - transactions × recurring_items (cross-reference to find "missed" recurring charges)
  - transactions × tags (tag-coverage report by category)
  - balance_history × accounts (net worth trajectory)
  - transactions self-join (duplicate detection: same merchant, ±$1, same day)
  - transactions windowed (subscription detection: regular cadence not yet flagged recurring)

## Codebase Intelligence
- **Source: source-extracted from `robshox/lunchmoney-mcp` (TS) and `leafeye/lunchmoney-mcp-v2` (Python), plus official `lunch-money/lunch-money-js-v2`.**
- **Auth:** HTTP Bearer JWT. Token from `my.lunchmoney.app/developers`. Single token, no refresh, no expiry surfaced.
- **Auth env-var convention chaos:** ecosystem uses 5 different names. We will accept all five as compatibility aliases.
  - Primary (official): `LUNCHMONEY_ACCESS_TOKEN`
  - Aliases: `LUNCHMONEY_API_KEY` (user's existing var), `LUNCHMONEY_TOKEN`, `LUNCHMONEY_API_TOKEN` (lunchtui), `LUNCH_MONEY_API_KEY` (robshox MCP)
- **Data model:** transactions are the gravity object. Categories form a 2-level group/leaf hierarchy. Transaction groups + splits create parent/child relationships requiring careful tree handling. Balance history is per-account-type (manual / plaid / crypto-synced).
- **Rate limiting:** `x-ratelimit-limit: 100`, returns 429 with `x-ratelimit-reset` Unix timestamp. The generated rate limiter should respect this.
- **Architecture:** Express-served REST; OpenAPI 3.0.2; mock server with same surface; v2 builds on v1 under the hood (per spec description). v2.11.0 introduces balance_history endpoints.

## User Vision
The user requested **Codex mode** for code-writing tasks. Coding (store layer, novel commands, README cookbook, dead-flag fixes) will be delegated to Codex; research/product/decisions/verification stay on Claude.

## Product Thesis
- **Name:** `lunch-money-pp-cli` (binary `lunch-money-pp-cli`)
- **Display name:** "Lunch Money"
- **Why it should exist:**
  1. **Only Go single-binary CLI built on the official v2 OpenAPI spec.** lunchtui is Rust + a TUI (browse-only); lunchable is Python + uses v1; no Go-with-v2 exists.
  2. **MCP-native by default.** The same binary is your CLI AND your MCP server (Cobra-tree mirror). No separate Python/Node install for AI use.
  3. **Offline SQLite store** — none of the existing tools cache data. Search, joins, drift analysis, and net worth queries become a `WHERE` clause instead of paginated API loops.
  4. **Agent-native I/O on every command:** `--json`, `--select` dotted paths, `--csv`, `--dry-run`, `--compact`, typed exit codes. No tool in this ecosystem has this combination.
  5. **Transcendence:** local joins enable subscription detection, duplicate detection, stale-category audit, spend drift, balance-update staleness alerts, vendor-merge suggestions — workflows users today do by exporting CSVs to spreadsheets.

## Build Priorities
1. **Foundation (P0):** Data layer for transactions, categories, tags, manual_accounts, plaid_accounts, recurring_items, balance_history, budgets, crypto-manual, crypto-synced. Sync via `updated_since` on transactions; full-pull for small resources. FTS5 on transactions and categories/tags. Rate-limit-aware client.
2. **Absorb (P1):** All 59 spec operations exposed as Cobra commands. Bulk transaction operations. File attachments. Plaid fetch. Budget upsert. Crypto refresh. Match `lunchtui` (transaction insert, categories list, accounts list) and `lunchable` (full CRUD, generic `http` escape hatch).
3. **Transcend (P2):** Local-join novel commands (see Phase 1.5c.5 manifest). Subscription detective, duplicate dedupe, spend-drift, stale-category audit, net-worth-at-date, balance-staleness, vendor-merge, tag-coverage.
4. **Polish (P3):** Auth env-var aliases. Rate-limit retry. Help-text enrichment with realistic IDs from the user's actual data shape.

## Competitor Landscape (used for absorb)
| Tool | Lang | Stars | Surface | Notes |
|------|------|-------|---------|-------|
| lunchtui (Rshep3087) | Go | 23 | TUI + a few CLI cmds | Closest Go competitor. Browse-focused. Anthropic SDK linked (AI categorizer). |
| lunchable (juftin) | Python | 52 | Full CLI + plugins | Most established. v1 only. Plugin framework + Splitwise/Amazon plugins. |
| icco/lunchmoney | Go | 10 | Library only | Older Go wrapper, no CLI. |
| leafeye/lunchmoney-mcp-v2 | Python | 0 | MCP, 14 tools, v2 | First v2 MCP. Manage/bulk/split/group meta-tools. |
| robshox/lunchmoney-mcp | TS | n/a | MCP, 15 tools, v1 | Granular tools per resource. |
| akutishevsky/lunchmoney-mcp | Python | n/a | MCP, full v1 | LUNCHMONEY_ACCESS_TOKEN. |
| ConnorDBurge/lunchmoney-mcp-v2 | Python | n/a | MCP, v2 SDK | Built on official v2 client. |
| Lunch Money Raycast | TS | n/a | Raycast extension | Quick data access from Raycast launcher. |
| BBudget, Bento Cash, etc. | Various | n/a | Mobile companion apps | Out of scope for CLI absorb but inform UX. |

## References
- Official docs (v1): https://lunchmoney.dev
- Official docs (v2 alpha): https://alpha.lunchmoney.dev/v2/docs
- Official OpenAPI spec npm package: `@lunch-money/v2-api-spec@2.11.0-preview.2`
- awesome-lunchmoney (71 stars): https://github.com/lunch-money/awesome-lunchmoney
- Developer key page: https://my.lunchmoney.app/developers
